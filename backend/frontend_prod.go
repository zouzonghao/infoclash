//go:build !dev

// 这个文件定义了在生产模式下的前端路由处理逻辑。
// 文件顶部的 `//go:build !dev` 是一个 Go 构建标签（Build Tag）。
// `!dev` 表示“非 dev”，它告诉 Go 编译器：在 **不** 使用 `dev` 标签进行构建时（例如常规的 `go build`），
// 才包含这个文件。这使其成为生产环境的默认配置。

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// `//go:embed dist` 是一个编译器指令，它告诉 Go 编译器将 `dist` 目录下的所有文件
// 嵌入（embed）到 `embeddedFrontend` 这个变量中。
// 这使得前端的静态资源（HTML, CSS, JS）可以被打包进最终的 Go 可执行文件中，
// 从而实现单文件部署。
//
//go:embed dist
var embeddedFrontend embed.FS

// spaFileSystem 是一个自定义的文件系统处理器，专门为单页应用（SPA）设计。
// 它包装了标准的 http.FileSystem。
type spaFileSystem struct {
	root http.FileSystem
}

// Open 方法是 http.FileSystem 接口的核心。
// 我们重写这个方法来实现 SPA 的一个关键行为：当浏览器请求一个不存在的路径时
// （例如，直接访问 `/some/route`），服务器应该返回 `index.html`，
// 然后由前端的路由库（如 Vue Router）来处理这个路径。
// 如果请求的文件存在（如 `main.js` 或 `style.css`），则正常返回该文件。
func (fs spaFileSystem) Open(name string) (http.File, error) {
	f, err := fs.root.Open(name)
	// 检查错误是否为“文件不存在”。
	if os.IsNotExist(err) {
		// 如果文件不存在，则返回根目录下的 index.html。
		return fs.root.Open("index.html")
	}
	return f, err
}

// addFrontendRoutes 在生产模式下，负责将嵌入的前端静态文件服务配置到 Go 的路由中。
func addFrontendRoutes(r *mux.Router) {
	// `fs.Sub` 从嵌入的 `embeddedFrontend` 中创建一个子文件系统，其根目录指向 `dist` 目录。
	// 这样做是必要的，因为 `//go:embed` 会保留目录结构。
	frontendFS, err := fs.Sub(embeddedFrontend, "dist")
	if err != nil {
		log.Fatalf("创建前端子文件系统失败: %v", err)
	}

	// 使用我们自定义的 spaFileSystem 来包装这个子文件系统。
	spaFS := spaFileSystem{root: http.FS(frontendFS)}
	// 将所有未被 API 路由匹配的请求（路径前缀为 "/"）都交由这个文件服务器处理。
	r.PathPrefix("/").Handler(http.FileServer(spaFS))
}
