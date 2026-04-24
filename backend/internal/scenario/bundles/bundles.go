/* *
 * @Author: chengjiang
 * @Date: 2026-04-22
 * @Description: 非 sandbox 执行模式的前端资源包 embed 入口
 *   - static / wasm-linux / web-container 模式不起容器,
 *     前端 Runner 需要从后端拉取实际跑题用的 HTML / wasm / tarball
 *   - 这里用 embed.FS 把整个 bundles 目录打进二进制,
 *     运行时由 ServeBundle 根据 slug 下发
 *   - 所有 bundle 资源都是"答案暴露风险低"的题型(CSS 调试、HTML 小题),
 *     反正前端能 view-source 看到,不打算做鉴权/加密。真要做"需要防爆解"的题
 *     应当走 sandbox 模式,check.sh 700+root 才防得住
**/
package bundles

import (
	"embed"
	"io/fs"
	"strings"
)

// 注意:路径中的 "all:" 前缀让 embed 包含下划线开头和 "." 开头的文件
// 每多一个场景目录,这里加一行 embed 指令(embed 不支持一句话把根下多个同级目录全收)
//
//go:embed all:css-flex-center
//go:embed all:webcontainer-node-hello
//go:embed all:wasm-linux-hello
var bundlesFS embed.FS

// FS 返回整个 bundles 目录作为只读文件系统
// 调用方(HTTP handler)应当以 slug 作为子目录根
func FS() fs.FS {
	return bundlesFS
}

// Open 按 "{slug}/{path}" 打开 bundle 内的文件
// 前端请求形如 GET /v1/scenarios/css-flex-center/bundle/index.html,
// handler 会剥掉前缀后把 "css-flex-center/index.html" 丢进来
//
// 返回的 fs.File 由调用方负责关闭。路径做了最简单的 .. 防护,
// 不允许向上跳出 bundles/ 目录(embed.FS 本身也不允许,这里属于双保险)
func Open(slugPath string) (fs.File, error) {
	// 统一反斜杠(Windows 开发时 embed 仍用 /,但前端 URL 可能残留 \)
	p := strings.ReplaceAll(slugPath, "\\", "/")
	// 去掉任何前导 /
	p = strings.TrimPrefix(p, "/")
	// 粗略禁止 ".." 上跳
	if strings.Contains(p, "..") {
		return nil, fs.ErrNotExist
	}
	return bundlesFS.Open(p)
}

// Has 检查指定 slug 是否有 bundle 目录(用于 registry 装载时校验)
// 避免场景声明 ExecutionMode=static 但实际没放 bundle,导致运行时 404
func Has(slug string) bool {
	if slug == "" {
		return false
	}
	// 尝试列出根目录,看 slug 是否是子目录
	entries, err := fs.ReadDir(bundlesFS, slug)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
