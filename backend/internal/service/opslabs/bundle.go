/* *
 * @Author: chengjiang
 * @Date: 2026-04-22
 * @Description: 非 sandbox 执行模式的静态资源下发 handler
 *   - 前端 Runner 需要 HTML / JS / wasm / tarball 等文件才能起来
 *   - 这些资源通过 scenario/bundles embed.FS 打进二进制,运行时流式下发
 *   - 路由形如 GET /v1/scenarios/{slug}/bundle/{file*},不走 proto RPC 路径
**/
package opslabs

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/7as0nch/backend/internal/scenario"
	"github.com/7as0nch/backend/internal/scenario/bundles"
)

// bundleStartTime 作为所有 embed 资源的虚拟 mtime
// 进程启动时固定,给 http.ServeContent 做 If-Modified-Since/ETag 计算用
// embed.FS 本身没有 mtime,用进程启动时间语义上等于"这一版二进制发布时间",
// 对前端缓存判定足够(改 bundle 必然要重新构建二进制 → 时间自动更新)
var bundleStartTime = time.Now()

// BundleURLPrefix 与 proto 里的 bundle_url 约定保持一致
// 前端 Runner 直接访问这个前缀下的文件;后端 Start 返回的 bundle_url 也按这个拼
const BundleURLPrefix = "/v1/scenarios/"

// BundleRelPath 固定的二级目录,区分 bundle 资源和 RPC 路径
// 完整 URL 形如 /v1/scenarios/{slug}/bundle/index.html
const BundleRelPath = "/bundle/"

// NewBundleHandler 构造下发 handler
// 上游在 cmd/server 那边把 srv.HandlePrefix("/v1/scenarios/", NewBundleHandler()) 注册进去
// **注意**:必须注册在 RegisterScenarioHTTPServer 之后,让 gorilla/mux 的 RPC 精确路由优先匹配
func NewBundleHandler() http.Handler {
	return http.HandlerFunc(serveBundle)
}

// serveBundle 解析 URL 里的 slug/file,从 embed.FS 读出来原路返回
//   - 不走鉴权:bundle 内容本来就 view-source 可见,不打算防盗用
//   - 路径里禁止 ..(embed 自身也拦,这里双保险)
//   - MIME 由 Go net/http DetectContentType 兜底,避免 IE/老浏览器走错
//   - **支持 HTTP Range**:v86 的 cdrom: {async:true} 用 Range 请求懒加载 ISO,
//     早期版本用 io.Copy 无 Range 支持,导致 BusyBox 内核引导时卡在 ISO 读盘,
//     shell prompt 永远不出现,opslabs:ready 不发,父页"加载题目 bundle…" 永远不消失。
//     http.ServeContent 原生处理 Range/If-Modified-Since/If-None-Match,一次性修好。
func serveBundle(w http.ResponseWriter, r *http.Request) {
	// URL:/v1/scenarios/{slug}/bundle/{rest}
	raw := r.URL.Path
	if !strings.HasPrefix(raw, BundleURLPrefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(raw, BundleURLPrefix) // {slug}/bundle/{rest}
	idx := strings.Index(rest, BundleRelPath)
	if idx <= 0 {
		http.NotFound(w, r)
		return
	}
	slug := rest[:idx]
	file := strings.TrimPrefix(rest[idx:], BundleRelPath)
	if file == "" {
		file = "index.html"
	}
	// 路径规范化,禁止向上跳
	cleaned := path.Clean("/" + file)
	if strings.Contains(cleaned, "..") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	cleaned = strings.TrimPrefix(cleaned, "/")

	// 读全量字节 —— embed 文件本来就常驻内存,bytes.NewReader 直接包现有切片,无拷贝
	// 用 fs.ReadFile 而不是 Open+io.ReadAll:少一次 Close 管理,语义更直接
	data, err := fs.ReadFile(bundles.FS(), slug+"/"+cleaned)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 设置 content-type,根据扩展名
	// 必须在 ServeContent 之前设置:ServeContent 只在 w.Header 没写 Content-Type 时才去 sniff
	if ct := contentTypeByExt(cleaned); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	// bundle 是静态的,给长缓存但带版本 query 的前端来刷新
	// V1 先保守:no-cache,改 bundle 立刻生效,免得调试期踩 CDN 缓存
	w.Header().Set("Cache-Control", "no-cache")
	// 静态 iframe 需要能被父窗口嵌入(同源 OK,不需要 X-Frame-Options)
	// 如果将来走跨域 CDN,这里需要放开 CSP

	// http.ServeContent 会自动:
	//   - 处理 If-Modified-Since / If-None-Match → 304
	//   - 处理 Range: bytes=X-Y → 206 Partial Content + Content-Range
	//   - 设置 Content-Length / Accept-Ranges: bytes
	// io.ReadSeeker 用 bytes.NewReader 包 embed 字节切片即可
	http.ServeContent(w, r, cleaned, bundleStartTime, bytes.NewReader(data))
}

// contentTypeByExt 按文件后缀推断 MIME
// 覆盖 bundle 里会用到的几种类型,其它交给 net/http DetectContentType
func contentTypeByExt(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".wasm":
		return "application/wasm"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".ico":
		return "image/x-icon"
	case ".map":
		return "application/json; charset=utf-8"
	case ".bin":
		// v86 的 seabios.bin / vgabios.bin:二进制 BIOS 镜像,
		// v86 用 fetch+arraybuffer 读取,浏览器不校验 MIME,兜底 octet-stream 即可
		return "application/octet-stream"
	case ".iso":
		// v86 的 linux.iso:BusyBox cdrom 启动盘。MIME 没严格标准,
		// application/x-iso9660-image 有些浏览器不识别,沿用 octet-stream 最保险
		return "application/octet-stream"
	}
	return ""
}

// BundleURLFor 给 slug 生成对应的 bundle 入口 URL(static / wasm-linux 用)
// Start 返回给前端的 bundle_url 用这个函数拼,保证和 handler 对称
func BundleURLFor(slug string) string {
	return BundleURLPrefix + slug + BundleRelPath + "index.html"
}

// BundleEntryURLFor 按执行模式返回 bundle 入口 URL
// - static / wasm-linux:入口是 index.html(iframe 直接 src)
// - web-container:入口是 project.json(FileSystemTree JSON,Runner 负责 fetch + mount)
// - sandbox:不需要 bundle,返回空串
func BundleEntryURLFor(mode, slug string) string {
	switch mode {
	case scenario.ExecutionModeWebContainer:
		return BundleURLPrefix + slug + BundleRelPath + "project.json"
	case scenario.ExecutionModeStatic, scenario.ExecutionModeWasmLinux:
		return BundleURLPrefix + slug + BundleRelPath + "index.html"
	}
	return ""
}
