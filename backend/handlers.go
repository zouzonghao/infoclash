package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MergeRequest 定义了前端在请求合并连接记录时需要发送的 JSON 数据结构。
type MergeRequest struct {
	StartDate int64 `json:"startDate"` // 合并范围的开始时间戳（秒）。
	EndDate   int64 `json:"endDate"`   // 合并范围的结束时间戳（秒）。
	Interval  int   `json:"interval"`  // 合并的时间窗口大小（分钟）。
}

// ReplaceHostRequest 定义了替换主机后缀请求的 JSON 结构。
type ReplaceHostRequest struct {
	DomainSuffix string `json:"domainSuffix"` // 要替换成的域名后缀。
}

// mergeConnectionsHandler 是处理 `/api/connections/merge` POST 请求的 HTTP Handler。
// 它负责解析请求，调用核心的合并与归档逻辑，并返回操作结果。
func mergeConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 解析请求体中的 JSON 数据到 MergeRequest 结构体。
	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	// 2. 从请求的 context 中获取数据库连接。
	// 这是通过 server.go 中定义的 dbMiddleware 中间件注入的。
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}
	archiveDB, ok := r.Context().Value("archiveDB").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取归档数据库连接", http.StatusInternalServerError)
		return
	}

	// 3. 调用核心业务逻辑函数来执行合并和归档操作。
	err := mergeAndArchiveConnections(db, archiveDB, req.StartDate, req.EndDate, req.Interval)
	if err != nil {
		http.Error(w, fmt.Sprintf("合并失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. 合并成功后，对主数据库执行 VACUUM 操作。
	// VACUUM 可以重建数据库文件，清除已删除数据占用的空间，减小数据库文件大小。
	log.Println("数据合并成功，开始执行 VACUUM...")
	if _, vacErr := db.Exec("VACUUM"); vacErr != nil {
		// VACUUM 失败不应影响主操作的成功状态，仅记录日志。
		log.Printf("执行 VACUUM 失败: %v", vacErr)
	} else {
		log.Println("VACUUM 执行成功。")
	}

	// 5. 返回成功的 JSON 响应。
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "合并成功"})
}

// mergeAndArchiveConnections 包含了数据合并与归档的核心业务逻辑。
// 它在一个事务中完成以下操作：
// 1. 从主数据库查询指定时间范围内的数据。
// 2. 在内存中按主机和时间窗口对数据进行分组和聚合。
// 3. 将原始数据归档到归档数据库。
// 4. 从主数据库删除原始数据。
// 5. 将聚合后的新数据插入主数据库。
func mergeAndArchiveConnections(db, archiveDB *sql.DB, startDate, endDate int64, interval int) error {
	// 1. 查询需要合并的数据。
	query := "SELECT id, sourceIP, host, upload, download, start, chain FROM connections WHERE start >= ? AND start <= ?"
	rows, err := db.Query(query, startDate, endDate)
	if err != nil {
		return fmt.Errorf("查询数据失败: %w", err)
	}
	defer rows.Close()

	// 将查询结果扫描到 Connection 结构体切片中。
	var connectionsToMerge []Connection
	for rows.Next() {
		var conn Connection
		var start int64
		var metadata Metadata
		var chain sql.NullString
		err := rows.Scan(&conn.ID, &metadata.SourceIP, &metadata.Host, &conn.Upload, &conn.Download, &start, &chain)
		if err != nil {
			log.Printf("扫描数据库行失败: %v", err)
			continue
		}
		conn.Start = time.Unix(start, 0)
		conn.Metadata = metadata
		if chain.Valid {
			conn.Chains = []string{chain.String}
		} else {
			conn.Chains = []string{}
		}
		connectionsToMerge = append(connectionsToMerge, conn)
	}

	if len(connectionsToMerge) == 0 {
		return nil // 没有需要合并的数据，直接返回成功。
	}

	// 2. 数据分组与合并。
	// 使用 map 来存储合并后的结果，key 是由主机名和时间窗口组成的唯一标识。
	mergedConnections := make(map[string]Connection)
	groupKeyFormat := "2006-01-02 15:04:05" // Go 的标准时间格式化字符串。

	for _, conn := range connectionsToMerge {
		// `Truncate` 将时间向下取整到指定的时间窗口。
		timeSlot := conn.Start.Truncate(time.Duration(interval) * time.Minute).Format(groupKeyFormat)
		groupKey := fmt.Sprintf("%s-%s", conn.Metadata.Host, timeSlot)

		if existing, ok := mergedConnections[groupKey]; ok {
			// 如果 key 已存在，累加流量。
			existing.Upload += conn.Upload
			existing.Download += conn.Download
			mergedConnections[groupKey] = existing
		} else {
			// 如果 key 不存在，创建新条目。
			mergedConnections[groupKey] = conn
		}
	}

	// 3. 数据库事务处理。
	// 同时对主数据库和归档数据库开启事务，确保操作的原子性。
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启主数据库事务失败: %w", err)
	}
	archiveTx, err := archiveDB.Begin()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("开启归档数据库事务失败: %w", err)
	}

	// 使用 defer 确保在函数退出时，无论成功还是失败，事务都会被正确处理。
	defer func() {
		if err != nil {
			tx.Rollback()
			archiveTx.Rollback()
		} else {
			err = tx.Commit()
			if err == nil {
				archiveTx.Commit()
			}
		}
	}()

	// 准备用于归档、删除和插入的 SQL 语句。
	archiveStmt, err := archiveTx.Prepare("INSERT INTO connections_archive (id, sourceIP, host, upload, download, start, chain, archived_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("准备归档语句失败: %w", err)
	}
	defer archiveStmt.Close()

	deleteStmt, err := tx.Prepare("DELETE FROM connections WHERE id = ?")
	if err != nil {
		return fmt.Errorf("准备删除语句失败: %w", err)
	}
	defer deleteStmt.Close()

	// 遍历所有原始数据，执行归档和删除。
	now := time.Now().Unix()
	for _, conn := range connectionsToMerge {
		var chain string
		if len(conn.Chains) > 0 {
			chain = conn.Chains[0]
		}
		_, err = archiveStmt.Exec(conn.ID, conn.Metadata.SourceIP, conn.Metadata.Host, conn.Upload, conn.Download, conn.Start.Unix(), chain, now)
		if err != nil {
			return fmt.Errorf("归档数据失败: %w", err)
		}
		_, err = deleteStmt.Exec(conn.ID)
		if err != nil {
			return fmt.Errorf("删除原始数据失败: %w", err)
		}
	}

	// 准备插入语句，将合并后的数据写回主数据库。
	insertStmt, err := tx.Prepare("INSERT INTO connections (id, sourceIP, host, upload, download, start, chain) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("准备插入语句失败: %w", err)
	}
	defer insertStmt.Close()

	for _, conn := range mergedConnections {
		newID := uuid.New().String() // 为合并后的新记录生成唯一的 ID。
		var chain string
		if len(conn.Chains) > 0 {
			chain = conn.Chains[0]
		}
		_, err = insertStmt.Exec(newID, conn.Metadata.SourceIP, conn.Metadata.Host, conn.Upload, conn.Download, conn.Start.Unix(), chain)
		if err != nil {
			return fmt.Errorf("插入合并后数据失败: %w", err)
		}
	}

	return nil
}

// getConnectionsHandler 是处理 `/api/connections` GET 请求的 HTTP Handler。
// 它支持分页、排序和多种条件的过滤，用于在前端展示连接列表。
func getConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}

	// 从 URL 查询参数中解析分页、过滤和排序的选项。
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 20
	}
	host := r.URL.Query().Get("host")
	sourceIP := r.URL.Query().Get("sourceIP")
	startDate, _ := strconv.ParseInt(r.URL.Query().Get("startDate"), 10, 64)
	endDate, _ := strconv.ParseInt(r.URL.Query().Get("endDate"), 10, 64)
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	chain := r.URL.Query().Get("chain")

	// 动态构建 SQL 查询语句和参数列表，以避免 SQL 注入。
	query := "SELECT id, sourceIP, host, upload, download, start, chain FROM connections WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM connections WHERE 1=1"
	var queryArgs []interface{}
	var countArgs []interface{}

	if host != "" {
		clause := " AND host LIKE ?"
		query += clause
		countQuery += clause
		likeHost := "%" + host + "%"
		queryArgs = append(queryArgs, likeHost)
		countArgs = append(countArgs, likeHost)
	}
	if sourceIP != "" {
		clause := " AND sourceIP LIKE ?"
		query += clause
		countQuery += clause
		likeSourceIP := "%" + sourceIP + "%"
		queryArgs = append(queryArgs, likeSourceIP)
		countArgs = append(countArgs, likeSourceIP)
	}
	if startDate > 0 {
		clause := " AND start >= ?"
		query += clause
		countQuery += clause
		queryArgs = append(queryArgs, startDate)
		countArgs = append(countArgs, startDate)
	}
	if endDate > 0 {
		clause := " AND start <= ?"
		query += clause
		countQuery += clause
		queryArgs = append(queryArgs, endDate)
		countArgs = append(countArgs, endDate)
	}
	if chain != "" {
		clause := " AND chain = ?"
		query += clause
		countQuery += clause
		queryArgs = append(queryArgs, chain)
		countArgs = append(countArgs, chain)
	}

	// 首先执行 COUNT 查询，获取满足条件的总记录数，用于前端分页。
	var total int
	err := db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		http.Error(w, fmt.Sprintf("数据库查询失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 添加排序逻辑。
	orderByClause := " ORDER BY start DESC" // 默认按开始时间降序排序。
	if sortBy != "" {
		// 使用白名单验证 sortBy 参数，防止 SQL 注入。
		allowedSortBy := map[string]bool{
			"upload":   true,
			"download": true,
			"start":    true,
			"host":     true,
			"sourceIP": true,
		}
		// 前端传来的可能是 metadata.host，需要映射到数据库的 host 字段。
		dbSortBy := sortBy
		if sortBy == "metadata.host" {
			dbSortBy = "host"
		}
		if sortBy == "metadata.sourceIP" {
			dbSortBy = "sourceIP"
		}

		if allowedSortBy[dbSortBy] {
			order := "ASC"
			if strings.ToLower(sortOrder) == "desc" {
				order = "DESC"
			}
			orderByClause = fmt.Sprintf(" ORDER BY %s %s", dbSortBy, order)
		}
	}
	query += orderByClause

	// 添加分页逻辑。
	query += " LIMIT ? OFFSET ?"
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)

	// 执行最终的查询。
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		http.Error(w, fmt.Sprintf("数据库查询失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// 扫描查询结果到 ConnectionInfo 结构体切片中。
	var connections []ConnectionInfo
	for rows.Next() {
		var conn Connection
		var start int64
		var metadata Metadata
		var chain sql.NullString

		err := rows.Scan(&conn.ID, &metadata.SourceIP, &metadata.Host, &conn.Upload, &conn.Download, &start, &chain)
		if err != nil {
			log.Printf("扫描数据库行失败: %v", err)
			continue
		}

		conn.Start = time.Unix(start, 0)
		conn.Metadata = metadata
		if chain.Valid {
			conn.Chains = []string{chain.String}
		} else {
			conn.Chains = []string{}
		}

		connections = append(connections, ConnectionInfo{
			Host:     conn.Metadata.Host,
			SourceIP: conn.Metadata.SourceIP,
			Upload:   conn.Upload,
			Download: conn.Download,
			Start:    conn.Start,
			Chains:   conn.Chains,
		})
	}

	// 返回包含分页信息的 JSON 响应。
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      total,
		"page":       page,
		"pageSize":   pageSize,
		"totalPages": (total + pageSize - 1) / pageSize,
		"data":       connections,
	})
}

// getTrafficSummaryHandler 是处理 `/api/summary/traffic` GET 请求的 HTTP Handler。
// 它用于获取按时间（小时或天）分组的流量汇总数据，用于绘制图表。
func getTrafficSummaryHandler(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}

	// 解析查询参数：host, granularity, startDate, endDate。
	host := r.URL.Query().Get("host")
	granularity := r.URL.Query().Get("granularity")
	if granularity != "hour" && granularity != "day" {
		granularity = "day" // 默认粒度为天。
	}
	startDate, _ := strconv.ParseInt(r.URL.Query().Get("startDate"), 10, 64)
	endDate, _ := strconv.ParseInt(r.URL.Query().Get("endDate"), 10, 64)

	// 根据粒度选择不同的 `strftime` 格式。
	var format string
	if granularity == "hour" {
		format = "%Y-%m-%d %H:00:00"
	} else {
		format = "%Y-%m-%d 00:00:00"
	}

	// 构建 SQL 查询。
	query := `
		SELECT
			strftime(?, datetime(start, 'unixepoch')) as time,
			SUM(upload) as upload,
			SUM(download) as download
		FROM connections
		WHERE 1=1
	`
	args := []interface{}{format}

	if host != "" {
		query += " AND host = ?"
		args = append(args, host)
	}
	if startDate > 0 {
		query += " AND start >= ?"
		args = append(args, startDate)
	}
	if endDate > 0 {
		query += " AND start <= ?"
		args = append(args, endDate)
	}

	query += " GROUP BY time ORDER BY time"

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("数据库查询失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type TrafficSummary struct {
		Time     string `json:"time"`
		Upload   uint64 `json:"upload"`
		Download uint64 `json:"download"`
	}

	var summaries []TrafficSummary
	for rows.Next() {
		var summary TrafficSummary
		err := rows.Scan(&summary.Time, &summary.Upload, &summary.Download)
		if err != nil {
			log.Printf("扫描数据库行失败: %v", err)
			continue
		}
		summaries = append(summaries, summary)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// getHostSummaryHandler 是处理 `/api/summary/hosts` GET 请求的 HTTP Handler。
// 它用于获取按总流量排序的主机列表，即流量排行榜。
func getHostSummaryHandler(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}

	// 解析查询参数：limit, startDate, endDate。
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10 // 默认返回前 10 名。
	}
	startDate, _ := strconv.ParseInt(r.URL.Query().Get("startDate"), 10, 64)
	endDate, _ := strconv.ParseInt(r.URL.Query().Get("endDate"), 10, 64)

	query := `
		SELECT
			host,
			SUM(upload) as upload,
			SUM(download) as download,
			SUM(upload) + SUM(download) as total
		FROM connections
		WHERE host != ''
	`
	args := []interface{}{}

	if startDate > 0 {
		query += " AND start >= ?"
		args = append(args, startDate)
	}
	if endDate > 0 {
		query += " AND start <= ?"
		args = append(args, endDate)
	}

	query += " GROUP BY host ORDER BY total DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("数据库查询失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type HostSummary struct {
		Host     string `json:"host"`
		Upload   uint64 `json:"upload"`
		Download uint64 `json:"download"`
		Total    uint64 `json:"total"`
	}

	var summaries []HostSummary
	for rows.Next() {
		var summary HostSummary
		err := rows.Scan(&summary.Host, &summary.Upload, &summary.Download, &summary.Total)
		if err != nil {
			log.Printf("扫描数据库行失败: %v", err)
			continue
		}
		summaries = append(summaries, summary)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// getHostsHandler 是处理 `/api/hosts` GET 请求的 HTTP Handler。
// 它返回数据库中所有不重复的主机名列表，用于前端的筛选器。
func getHostsHandler(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}

	query := "SELECT DISTINCT host FROM connections WHERE host != '' ORDER BY host"
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("数据库查询失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hosts []string
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			log.Printf("扫描数据库行失败: %v", err)
			continue
		}
		hosts = append(hosts, host)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hosts)
}

// getChainsHandler 是处理 `/api/chains` GET 请求的 HTTP Handler。
// 它返回数据库中所有不重复的代理链名称列表，用于前端的筛选器。
func getChainsHandler(w http.ResponseWriter, r *http.Request) {
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}

	query := "SELECT DISTINCT chain FROM connections WHERE chain != '' ORDER BY chain"
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("数据库查询失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var chains []string
	for rows.Next() {
		var chain string
		if err := rows.Scan(&chain); err != nil {
			log.Printf("扫描数据库行失败: %v", err)
			continue
		}
		chains = append(chains, chain)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chains)
}

// replaceHostHandler 是处理 `/api/connections/replace-host` POST 请求的 HTTP Handler。
// 它用于将所有匹配特定后缀的主机名替换为该后缀本身，用于数据清洗。
func replaceHostHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 解析请求体。
	var req ReplaceHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.DomainSuffix == "" {
		http.Error(w, "域名后缀不能为空", http.StatusBadRequest)
		return
	}

	log.Printf("收到域名替换请求，后缀: %s", req.DomainSuffix)

	// 2. 获取数据库连接。
	db, ok := r.Context().Value("db").(*sql.DB)
	if !ok {
		http.Error(w, "无法获取数据库连接", http.StatusInternalServerError)
		return
	}

	// 3. 执行 UPDATE 操作。
	// `host LIKE ?` 会匹配所有以 `.%` 结尾的子域名，例如 `%.example.com`。
	// `host = ?` 会匹配域名本身。
	query := "UPDATE connections SET host = ? WHERE host LIKE ? OR host = ?"
	likePattern := "%." + req.DomainSuffix
	result, err := db.Exec(query, req.DomainSuffix, likePattern, req.DomainSuffix)
	if err != nil {
		http.Error(w, fmt.Sprintf("更新失败: %v", err), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("无法获取受影响的行数: %v", err)
		// 即使无法获取行数，操作也已成功，所以不返回错误。
	}

	log.Printf("域名替换成功，后缀: %s, 更新了 %d 条记录", req.DomainSuffix, rowsAffected)

	// 4. 返回响应。
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "替换成功",
		"rowsAffected": rowsAffected,
	})
}
