package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// connectionsCache 是一个全局的、线程安全的内存缓存。
// 它使用 `sync.Map` 来存储从 Clash API 获取的最新连接信息。
// 这样做可以减少对 API 的请求频率，并将数据库写入操作批量化，提高性能。
// key 是连接的 ID (string)，value 是 Connection 结构体。
var connectionsCache = sync.Map{}

// main 函数是程序的入口点。
func main() {
	// 1. 定义命令行参数
	// 使用 flag 包来处理命令行输入。每个参数都用指针接收，以便后续判断是否由用户显式设置。
	clashAPIURL := flag.String("url", "", "Clash API 的 URL (例如：http://192.168.1.1:9090/connections)")
	clashAPIToken := flag.String("t", "", "Clash API 的 Token（secret）")
	databasePath := flag.String("db", "", "主数据库文件的路径 (例如：./clash_traffic.db)")
	archiveDatabasePath := flag.String("adb", "", "归档数据库文件的路径 (例如：./clash_traffic_archive.db)")
	dbWriteInterval := flag.Int("i", 0, "数据库写入间隔（分钟）")
	webPort := flag.String("p", "", "Web 服务监听的端口 (例如：8081)")

	// 自定义帮助信息
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  ./infoclash -url <api_url> -t <token> [options]\n\n")
		fmt.Fprintf(os.Stderr, "参数说明:\n")
		fmt.Fprintf(os.Stderr, "  -url string\n")
		fmt.Fprintf(os.Stderr, "        (必须) Clash API 的 URL, 例如：http://192.168.1.1:9090/connections\n")
		fmt.Fprintf(os.Stderr, "  -t string\n")
		fmt.Fprintf(os.Stderr, "        (必须) Clash API 的 Token（secret）（secret）\n")
		fmt.Fprintf(os.Stderr, "  -db string\n")
		fmt.Fprintf(os.Stderr, "        主数据库文件的路径 (默认: ./clash_traffic.db)\n")
		fmt.Fprintf(os.Stderr, "  -adb string\n")
		fmt.Fprintf(os.Stderr, "        归档数据库文件的路径 (默认: ./clash_traffic_archive.db)\n")
		fmt.Fprintf(os.Stderr, "  -i int\n")
		fmt.Fprintf(os.Stderr, "        数据库写入间隔,单位为分钟 (默认: 3)\n")
		fmt.Fprintf(os.Stderr, "  -p string\n")
		fmt.Fprintf(os.Stderr, "        Web 服务监听的端口 (默认: 8081)\n")
		fmt.Fprintf(os.Stderr, "  -h, -help, --help\n")
		fmt.Fprintf(os.Stderr, "        显示此帮助信息\n")
	}

	flag.Parse()

	// 2. 加载配置
	// 将解析到的命令行参数传递给 LoadConfig 函数。
	// LoadConfig 将处理优先级：命令行 > .env/环境变量 > 默认值。
	cfg := LoadConfig(
		*clashAPIURL,
		*clashAPIToken,
		*databasePath,
		*archiveDatabasePath,
		*webPort,
		*dbWriteInterval,
	)

	// 3. 初始化主数据库
	db, err := InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close() // 确保在 main 函数退出时关闭数据库连接。
	log.Println("数据库初始化成功。")

	// 3. 初始化归档数据库
	archiveDB, err := InitArchiveDB(cfg.ArchiveDatabasePath)
	if err != nil {
		log.Fatalf("初始化归档数据库失败: %v", err)
	}
	defer archiveDB.Close()
	log.Println("归档数据库初始化成功。")

	log.Printf("配置加载完成：数据库写入间隔为 %v。", cfg.DBWriteInterval)

	// --- 启动并发任务 ---
	// Go 语言的并发模型基于 Goroutine 和 Channel，非常适合处理这类需要同时执行多个独立任务的场景。

	// Goroutine 1: 定时从 Clash API 获取数据并更新到内存缓存。
	// 这个 Goroutine 的执行频率由配置中的 APISyncInterval 控制（当前为1秒）。
	apiTicker := time.NewTicker(cfg.APISyncInterval)
	defer apiTicker.Stop()

	go func() {
		for range apiTicker.C {
			connections, err := GetClashConnections(cfg.ClashAPIURL, cfg.ClashAPIToken, cfg.HostSuffixWhitelist)
			if err != nil {
				log.Printf("获取 Clash 连接信息失败: %v", err)
				continue // 如果获取失败，记录日志并等待下一次触发。
			}
			// 将获取到的连接信息存入 sync.Map。
			// Store 方法是线程安全的，可以安全地在多个 Goroutine 中调用。
			for _, conn := range connections.Connections {
				connectionsCache.Store(conn.ID, conn)
			}
			log.Printf("已从 API 同步 %d 个连接到内存。", len(connections.Connections))
		}
	}()

	// Goroutine 2: 定时将内存缓存中的数据批量写入数据库。
	// 这个 Goroutine 的执行频率由配置中的 DBWriteInterval 控制。
	// 这种“批处理”的方式可以显著减少数据库的写入次数，提高性能。
	dbTicker := time.NewTicker(cfg.DBWriteInterval)
	defer dbTicker.Stop()

	go func() {
		for range dbTicker.C {
			writeCacheToDB(db)
		}
	}()

	// Goroutine 3: 启动 Web 服务器。
	// Web 服务器在一个独立的 Goroutine 中运行，不会阻塞主线程。
	go StartWebServer(db, archiveDB, cfg.WebPort)

	// --- 优雅退出处理 ---
	// 为了防止在程序退出时丢失内存中尚未写入数据库的数据，我们需要实现“优雅退出”。
	// 这意味着程序在收到退出信号后，会先完成一些清理工作（比如保存数据），然后再真正退出。

	// 创建一个 channel 来接收操作系统信号。
	quitChan := make(chan os.Signal, 1)
	// `signal.Notify` 会将指定的信号（这里是 SIGINT 和 SIGTERM）转发到 quitChan。
	// SIGINT 通常是 Ctrl+C，SIGTERM 是 kill 命令的默认信号。
	signal.Notify(quitChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("程序已启动，按 Ctrl+C 退出。")
	// 程序会在这里阻塞，直到从 quitChan 中接收到一个信号。
	<-quitChan

	// 收到退出信号后，执行最后的清理工作。
	log.Println("接收到退出信号，正在将缓存数据写入数据库...")
	// 在退出前，最后一次将内存缓存中的所有数据写入数据库。
	writeCacheToDB(db)
	log.Println("数据已保存，程序即将退出。")
}

// writeCacheToDB 负责将全局内存缓存 `connectionsCache` 中的数据写入数据库。
func writeCacheToDB(db *sql.DB) {
	var connsToSave []Connection
	// `connectionsCache.Range` 是一个线程安全的方式来遍历 sync.Map。
	connectionsCache.Range(func(key, value interface{}) bool {
		connsToSave = append(connsToSave, value.(Connection))
		return true // 返回 true 以继续遍历。
	})

	if len(connsToSave) == 0 {
		log.Println("内存缓存为空，无需写入数据库。")
		return
	}

	log.Printf("准备将 %d 条连接数据从内存写入数据库...", len(connsToSave))
	if err := BulkUpsertConnections(db, connsToSave); err != nil {
		log.Printf("最终写入数据库失败: %v", err)
	} else {
		log.Println("缓存数据成功写入数据库。")
		// 写入成功后，清空缓存，避免重复写入。
		// 这里再次遍历并删除是 sync.Map 的一种清空方式。
		connectionsCache.Range(func(key, value interface{}) bool {
			connectionsCache.Delete(key)
			return true
		})
	}
}
