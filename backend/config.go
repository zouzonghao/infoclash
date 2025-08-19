package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 结构体用于存储从环境变量或 .env 文件加载的所有应用程序配置。
// 这样做的好处是集中管理配置，方便在程序各处使用。
type Config struct {
	ClashAPIURL         string        // Clash API 的 URL，用于获取连接信息。
	ClashAPIToken       string        // Clash API 的 Token（secret），用于认证。
	DatabasePath        string        // 主数据库文件的路径。
	ArchiveDatabasePath string        // 归档数据库文件的路径。
	DBWriteInterval     time.Duration // 将内存中的数据写入数据库的时间间隔。
	APISyncInterval     time.Duration // 从 Clash API 同步数据的频率。
	WebPort             string        // Web 服务器监听的端口。
	HostSuffixWhitelist []string      // 域名后缀名单，用于合并相同后缀的host
}

// Load 函数负责加载应用程序的配置。
// 它会首先尝试从项目根目录下的 .env 文件加载配置，
// 然后用任何已设置的环境变量覆盖这些值。
// LoadConfig 函数负责加载应用程序的配置。
// 它遵循以下优先级顺序来确定每个配置项的值：
// 1. 命令行参数 (最高)
// 2. .env 文件
// 3. 环境变量
// 4. 默认值 (最低)
func LoadConfig(
	clashAPIURL,
	clashAPIToken,
	databasePath,
	archiveDatabasePath,
	webPort string,
	dbWriteInterval int,
) *Config {
	// 尝试加载 .env 文件。这会把 .env 中的值加载到环境变量中，但不会覆盖已存在的环境变量。
	if err := godotenv.Load(); err != nil {
		log.Println("警告: 未找到 .env 文件，将仅使用命令行参数、环境变量或默认值。")
	}

	// --- 配置加载逻辑 ---
	// 为每个配置项决定最终使用哪个值。

	// Clash API URL
	finalAPIURL := getValue("CLASH_API_URL", clashAPIURL, "http://127.0.0.1:9090/connections")

	// Clash API Token
	finalAPIToken := getValue("CLASH_API_TOKEN", clashAPIToken, "") // Token 没有合理的默认值

	// Database Path
	finalDBPath := getValue("DATABASE_PATH", databasePath, "./clash_traffic.db")

	// Archive Database Path
	finalArchiveDBPath := getValue("ARCHIVE_DATABASE_PATH", archiveDatabasePath, "./clash_traffic_archive.db")

	// Web Port
	finalWebPort := getValue("WEB_PORT", webPort, "8081")

	// DB Write Interval
	var finalDBWriteIntervalMinutes int
	if dbWriteInterval > 0 {
		finalDBWriteIntervalMinutes = dbWriteInterval
	} else {
		dbWriteIntervalStr := os.Getenv("DB_WRITE_INTERVAL_MINUTES")
		interval, err := strconv.Atoi(dbWriteIntervalStr)
		if err != nil || interval <= 0 {
			finalDBWriteIntervalMinutes = 3 // 默认值
		} else {
			finalDBWriteIntervalMinutes = interval
		}
	}

	// Host Suffix Whitelist (仅从环境变量加载)
	hostSuffixWhitelistStr := os.Getenv("HOST_SUFFIX_WHITELIST")
	var hostSuffixWhitelist []string
	if hostSuffixWhitelistStr != "" {
		hostSuffixWhitelist = strings.Split(hostSuffixWhitelistStr, ",")
	}

	// 返回最终的配置
	return &Config{
		ClashAPIURL:         finalAPIURL,
		ClashAPIToken:       finalAPIToken,
		DatabasePath:        finalDBPath,
		ArchiveDatabasePath: finalArchiveDBPath,
		DBWriteInterval:     time.Duration(finalDBWriteIntervalMinutes) * time.Minute,
		APISyncInterval:     1 * time.Second, // API 同步间隔硬编码为1秒
		WebPort:             finalWebPort,
		HostSuffixWhitelist: hostSuffixWhitelist,
	}
}

// getValue 是一个辅助函数，用于根据优先级决定配置项的值。
// 优先级：命令行参数 > 环境变量 > 默认值。
func getValue(envKey, flagValue, defaultValue string) string {
	// 1. 检查命令行参数
	if flagValue != "" {
		return flagValue
	}
	// 2. 检查环境变量 (已经被 .env 文件填充)
	if envValue := os.Getenv(envKey); envValue != "" {
		return envValue
	}
	// 3. 使用默认值
	return defaultValue
}
