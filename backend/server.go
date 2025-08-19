package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// dbMiddleware 是一个 HTTP 中间件（Middleware）。
// 中间件是一种在处理 HTTP 请求之前或之后执行某些操作的函数。
// 这个特定的中间件的作用是将主数据库的连接池 (*sql.DB) 注入到每个 HTTP 请求的 context 中。
// 这样，下游的 HTTP Handler (如 getConnectionsHandler) 就可以从 context 中轻松地获取数据库连接，
// 而无需将其作为全局变量或通过函数参数层层传递。
func dbMiddleware(db *sql.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// `context.WithValue` 创建了一个新的 context，它包含了一个键值对 ("db", db)。
			ctx := context.WithValue(r.Context(), "db", db)
			// `next.ServeHTTP` 调用下一个中间件或最终的 Handler，并将带有数据库连接的新 context 传递下去。
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// archiveDBMiddleware 与 dbMiddleware 功能类似，但它注入的是归档数据库的连接池。
// 这使得需要同时操作两个数据库的 Handler (如 mergeConnectionsHandler) 可以方便地获取连接。
func archiveDBMiddleware(archiveDB *sql.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "archiveDB", archiveDB)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// StartWebServer 函数负责初始化和启动 Web 服务器。
// 它配置了所有的 API 路由、中间件和 CORS（跨域资源共享）策略。
func StartWebServer(db *sql.DB, archiveDB *sql.DB, port string) {
	// 创建一个新的 `gorilla/mux` 路由器实例。`mux` 提供了比标准库更强大的路由功能。
	r := mux.NewRouter()

	// 使用我们定义的中间件。中间件会按照它们被添加的顺序执行。
	r.Use(dbMiddleware(db))
	r.Use(archiveDBMiddleware(archiveDB))

	// --- API 路由定义 ---
	// `r.PathPrefix("/api")` 创建了一个子路由器，所有路径以 `/api` 开头的请求都将由它处理。
	// 这样做有助于将 API 路由和前端路由清晰地分离开。
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/connections", getConnectionsHandler).Methods("GET")
	apiRouter.HandleFunc("/summary/traffic", getTrafficSummaryHandler).Methods("GET")
	apiRouter.HandleFunc("/summary/hosts", getHostSummaryHandler).Methods("GET")
	apiRouter.HandleFunc("/hosts", getHostsHandler).Methods("GET")
	apiRouter.HandleFunc("/chains", getChainsHandler).Methods("GET")
	apiRouter.HandleFunc("/connections/merge", mergeConnectionsHandler).Methods("POST")
	apiRouter.HandleFunc("/connections/replace-host", replaceHostHandler).Methods("POST")

	// --- 前端路由处理 ---
	// 调用 `addFrontendRoutes` 函数来处理前端静态文件的服务。
	// 这个函数的具体实现由构建标签（build tags）决定：
	// - 在开发模式下 (`-tags dev`)，它是一个空函数 (来自 frontend_dev.go)。
	// - 在生产模式下，它会配置嵌入式文件系统 (来自 frontend_prod.go)。
	addFrontendRoutes(r)

	// --- CORS 配置 ---
	// CORS (Cross-Origin Resource Sharing) 是一种安全机制，用于控制来自不同源（域、协议、端口）的 Web 请求。
	// 在开发环境中，前端（如 localhost:5173）和后端（如 localhost:8088）通常在不同的源上，
	// 因此需要配置 CORS 策略以允许前端访问后端 API。
	// 这里的配置非常宽松 (`AllowedOrigins: []string{"*"}`)，允许来自任何源的请求，这在开发中很方便。
	// 在生产环境中，您可能希望将其收紧为只允许您的前端域名访问。
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})
	// 将 CORS 中间件包装在我们的主路由器上。
	handler := c.Handler(r)

	log.Printf("Web 服务器已启动，正在监听端口 %s", port)
	// `http.ListenAndServe` 启动 HTTP 服务器并开始监听指定的地址和端口。
	// 这是一个阻塞操作，因此我们通常在 main.go 中使用一个 Goroutine 来调用它。
	if err := http.ListenAndServe("0.0.0.0:"+port, handler); err != nil {
		log.Fatalf("启动 Web 服务器失败: %v", err)
	}
}
