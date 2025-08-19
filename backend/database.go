package main

import (
	"database/sql"
	"fmt"

	// 导入 "github.com/mattn/go-sqlite3" 驱动。
	// 下划线 `_` 表示我们只需要这个包的副作用（即注册 sqlite3 驱动），
	// 而不需要在代码中直接调用它的任何函数。
	_ "github.com/mattn/go-sqlite3"
)

// InitDB 函数负责初始化主数据库。
// 它接收一个文件路径作为参数，创建（如果不存在）一个 SQLite 数据库文件，
// 并执行 SQL 语句来创建 `connections` 表。
// 参数:
//
//	filepath: 数据库文件的路径。
//
// 返回值:
//
//	*sql.DB: 一个指向数据库连接池的指针。
//	error: 如果在打开数据库或创建表时发生错误，则返回一个错误。
func InitDB(filepath string) (*sql.DB, error) {
	// 构建数据源名称 (DSN)。
	// `_journal_mode=DELETE` 是一个优化选项，用于强制禁用 WAL (Write-Ahead Logging) 模式。
	// 在某些高并发写入场景下，WAL 可能会导致数据库锁定问题，这里显式禁用以确保稳定性。
	dsn := fmt.Sprintf("file:%s?_journal_mode=DELETE", filepath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// 定义用于创建 `connections` 表的 SQL 语句。
	// `IF NOT EXISTS` 确保了即使表已经存在，这条语句也不会报错。
	createTableSQL := `CREATE TABLE IF NOT EXISTS connections (
		"id" TEXT NOT NULL PRIMARY KEY,
		"sourceIP" TEXT,
		"host" TEXT,
		"upload" INTEGER,
		"download" INTEGER,
		"start" INTEGER,
		"chain" TEXT
	);`

	// 执行 SQL 语句。
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	// 返回初始化成功的数据库连接。
	return db, nil
}

// BulkUpsertConnections 函数使用单个事务来批量更新或插入（Upsert）连接信息。
// "Upsert" 是一种数据库操作，如果记录已存在，则更新它；如果不存在，则插入新记录。
// 这种方法比逐条检查和插入/更新要高效得多，尤其是在处理大量数据时。
// 参数:
//
//	db: 数据库连接池。
//	connections: 一个包含多个 Connection 对象的切片。
//
// 返回值:
//
//	error: 如果在事务处理过程中发生任何错误，则返回一个错误。
func BulkUpsertConnections(db *sql.DB, connections []Connection) (err error) {
	// 开始一个新的数据库事务。事务可以确保一系列操作要么全部成功，要么全部失败，从而保证数据的一致性。
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	// 使用 defer-recover 机制来确保事务在函数退出时能被正确处理（提交或回滚）。
	// 这是一个健壮的错误处理模式。
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback() // 如果发生 panic，回滚事务
			panic(p)
		} else if err != nil {
			tx.Rollback() // 如果函数返回错误，回滚事务
		} else {
			err = tx.Commit() // 否则，提交事务
		}
	}()

	// 定义 SQL Upsert 语句。
	// `ON CONFLICT(id) DO UPDATE SET ...` 是 SQLite 中实现 Upsert 的语法。
	// 当插入的记录 `id` 与表中现有记录冲突时，它会执行 `UPDATE` 部分。
	query := `
	INSERT INTO connections (id, sourceIP, host, upload, download, start, chain)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		upload = excluded.upload,
		download = excluded.download;
	`
	// 预编译 SQL 语句以提高性能。
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("准备 SQL 语句失败: %w", err)
	}
	defer stmt.Close()

	// 遍历所有待处理的连接。
	for _, conn := range connections {
		// 如果连接的 host 字段为空，则跳过该记录，不写入数据库。
		// 这是一个数据清洗步骤，确保数据库中存储的是有效数据。
		if conn.Metadata.Host == "" {
			continue
		}
		var chain string
		if len(conn.Chains) > 0 {
			// 我们只关心最终的出口节点，所以取链中的最后一个元素。
			chain = conn.Chains[len(conn.Chains)-1]
		}
		// 执行预编译的语句，传入连接的具体数据。
		_, err = stmt.Exec(conn.ID, conn.Metadata.SourceIP, conn.Metadata.Host, conn.Upload, conn.Download, conn.Start.Unix(), chain)
		if err != nil {
			// 如果执行失败，返回一个包含具体连接 ID 的错误信息，便于调试。
			return fmt.Errorf("在事务中执行语句失败 (ID: %s): %w", conn.ID, err)
		}
	}

	return nil
}

// InitArchiveDB 函数负责初始化归档数据库。
// 其功能与 InitDB 类似，但创建的是 `connections_archive` 表，用于存储已合并的旧数据。
// 参数:
//
//	filepath: 归档数据库文件的路径。
//
// 返回值:
//
//	*sql.DB: 一个指向归档数据库连接池的指针。
//	error: 如果在打开数据库或创建表时发生错误，则返回一个错误。
func InitArchiveDB(filepath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=DELETE", filepath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// `connections_archive` 表比 `connections` 表多一个 `archived_at` 字段，
	// 用于记录这条数据是何时被归档的。
	createTableSQL := `CREATE TABLE IF NOT EXISTS connections_archive (
		"id" TEXT NOT NULL,
		"sourceIP" TEXT,
		"host" TEXT,
		"upload" INTEGER,
		"download" INTEGER,
		"start" INTEGER,
		"chain" TEXT,
		"archived_at" INTEGER
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	return db, nil
}
