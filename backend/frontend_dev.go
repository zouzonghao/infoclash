//go:build dev

// 这个文件定义了在开发模式下的前端路由处理逻辑。
// 文件顶部的 `//go:build dev` 是一个 Go 构建标签（Build Tag）。
// 它告诉 Go 编译器：只有在使用 `dev` 标签进行构建或运行时（例如 `go run -tags dev .`），
// 才包含这个文件。在常规构建（`go build`）中，这个文件将被忽略。

package main

import "github.com/gorilla/mux"

// addFrontendRoutes 在开发模式下是一个空函数。
// 这是因为在开发环境中，前端静态资源是由 Vite 开发服务器（例如 http://localhost:5173）提供的，
// Go 后端只负责 API 接口。因此，我们不需要在 Go 的路由中添加任何处理前端文件的逻辑。
func addFrontendRoutes(r *mux.Router) {
	// 在开发模式下，此处无需添加任何前端路由。
}
