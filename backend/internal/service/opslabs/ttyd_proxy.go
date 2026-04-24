/* *
 * @Author: chengjiang
 * @Date: 2026-04-22
 * @Description: ttyd 反向代理 handler —— 把沙箱终端从 "http://localhost:{port}/"
 *               换成同源 "/v1/ttyd/{attemptId}/"
 *
 * 为什么要走反代:
 *   - 前端 vite 默认跨源隔离头(COEP=credentialless),iframe 直接指到
 *     http://localhost:{port}/(ttyd 容器映射端口)会被 Chrome 当 cross-origin
 *     处理。ttyd 本身不发 Cross-Origin-Resource-Policy 头,即便 iframe 加了
 *     credentialless 属性,某些 Chrome 版本的 navigation 检查仍会拦,表现成
 *     令人迷惑的 "ERR_CONNECTION_REFUSED"(中文: "localhost 拒绝了我们的连接请求")。
 *   - 把 ttyd 反代到后端同源前缀 "/v1/ttyd/{id}/" 后,iframe URL 变成同源
 *     相对路径,浏览器侧完全跳过 COEP 跨源检查,连 credentialless 属性都不用加。
 *
 * ttyd 协议细节:
 *   - HTML 首页: GET  /
 *   - 静态资源:  GET  /*.js, /*.css, /*.png ...
 *   - WebSocket: GET  /ws   (Upgrade: websocket)
 *   Go httputil.ReverseProxy 从 1.12 起原生支持 Upgrade hijack + 双向字节拷贝,
 *   不需要额外写 WS middleware。前提是 vite dev server 那边也要打开 ws: true。
 *
 * 安全:
 *   - 路径鉴权:校验 attemptId 在 store 里存在 + HostPort>0,否则 404
 *   - 未来可以在这里塞 auth 中间件,目前 Week 1 整组走白名单放行
**/
package opslabs

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/7as0nch/backend/internal/store"
	"go.uber.org/zap"
)

// TtydProxyURLPrefix 与 attempt.go terminalURL() 保持一致
//   - 末尾必须带 /(HandlePrefix 要求),也方便前端 iframe 直接用作 base URL
const TtydProxyURLPrefix = "/v1/ttyd/"

// NewTtydProxyHandler 构造 ttyd 反向代理 handler
//
// 参数:
//   - st    : AttemptStore,按 attemptId 反查 HostPort
//   - log   : 结构化日志,代理错误、upstream 挂掉都走这里
//
// 返回值满足 http.Handler,外层在 http.go 里 srv.HandlePrefix(TtydProxyURLPrefix, ...) 注册。
//
// 实现要点:
//   - 每个 attemptId 首次访问时 lazy 构造 *httputil.ReverseProxy,后续复用
//     (同一 attempt 的 JS/CSS/WS 都走同一个 proxy 实例,复用 transport 连接池)
//   - attemptId 对应的 HostPort 改变(理论上不会,但防御一下)时,下一次请求会
//     发现 target URL 变了,换新 proxy 实例
//   - 404 / 502 都走标准 http.Error,前端 probe() 能感知
func NewTtydProxyHandler(st *store.AttemptStore, log *zap.Logger) http.Handler {
	h := &ttydProxyHandler{
		store: st,
		log:   log,
		cache: make(map[int64]*proxyEntry),
	}
	return http.HandlerFunc(h.serve)
}

type proxyEntry struct {
	hostPort int
	proxy    *httputil.ReverseProxy
}

type ttydProxyHandler struct {
	store *store.AttemptStore
	log   *zap.Logger

	mu    sync.Mutex
	cache map[int64]*proxyEntry
}

// serve 主 handler
func (h *ttydProxyHandler) serve(w http.ResponseWriter, r *http.Request) {
	// 解析 /v1/ttyd/{attemptId}/{rest...}
	raw := r.URL.Path
	if !strings.HasPrefix(raw, TtydProxyURLPrefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(raw, TtydProxyURLPrefix)
	// 取第一段作为 attemptId,其余原样转给上游
	slash := strings.Index(rest, "/")
	var idStr, upstreamPath string
	if slash < 0 {
		idStr = rest
		upstreamPath = "/"
	} else {
		idStr = rest[:slash]
		upstreamPath = rest[slash:]
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad attempt id", http.StatusBadRequest)
		return
	}

	// 查 attempt 拿 HostPort
	// Redis 错误:返回 502 并打 Warn —— 不能冒充"attempt 已过期"误导用户重建
	a, ok, err := h.store.Get(r.Context(), id)
	if err != nil {
		h.log.Warn("ttyd proxy store Get failed",
			zap.Int64("attempt_id", id), zap.Error(err))
		http.Error(w, "attempt cache unavailable", http.StatusBadGateway)
		return
	}
	if !ok {
		// attempt 已过期 / 从未存在 —— 前端可以据此提示"会话已结束"
		http.Error(w, "attempt not found or expired", http.StatusNotFound)
		return
	}
	if a.HostPort <= 0 {
		// mock runtime 或 attempt 已 Stop,HostPort 被清零
		http.Error(w, "attempt has no live terminal", http.StatusGone)
		return
	}

	proxy := h.getOrCreateProxy(id, a.HostPort)
	// 覆盖 Path:砍掉 /v1/ttyd/{id} 前缀,留 upstreamPath 作为 ttyd 本体路径
	// RawPath 也要同步,httputil 保留原 query
	r2 := r.Clone(r.Context())
	r2.URL.Path = upstreamPath
	r2.URL.RawPath = "" // 强制 Encode 重算,避免残留 /v1/ttyd/{id} 编码
	proxy.ServeHTTP(w, r2)
}

// getOrCreateProxy 拿 attempt 对应的 ReverseProxy(按 HostPort 缓存)
//
// 加锁是因为:
//   - 多个并发请求同时进来时,避免重复 new ReverseProxy + 碎片化 transport pool
//   - cache map 必须串行读写
//
// 锁粒度小,只包住 map 访问,热路径上开销可忽略
func (h *ttydProxyHandler) getOrCreateProxy(id int64, hostPort int) *httputil.ReverseProxy {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e, ok := h.cache[id]; ok && e.hostPort == hostPort {
		return e.proxy
	}
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", hostPort))
	proxy := httputil.NewSingleHostReverseProxy(target)
	// 错误处理:默认 ReverseProxy 错误会写 502,日志太少,自己补上能帮线上定位
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
		h.log.Warn("ttyd proxy upstream error",
			zap.Int64("attempt_id", id),
			zap.Int("host_port", hostPort),
			zap.String("path", req.URL.Path),
			zap.Error(e))
		http.Error(rw, "terminal upstream error: "+e.Error(), http.StatusBadGateway)
	}
	// Director 默认就会改 req.URL.Host + scheme,再补一下 Host header 让 ttyd
	// 的 same-origin 检查(如果有)能过
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = target.Host
		// ttyd 本体的 CORS/Origin 默认宽松,不用再塞 header。
		// 若以后 ttyd 起 --origin-check,这里加 Origin override 即可
	}
	h.cache[id] = &proxyEntry{hostPort: hostPort, proxy: proxy}
	return proxy
}

// InvalidateTtydProxy 主动清掉指定 attempt 的 proxy 实例
// 目前用不到(cache 按 hostPort 自然对齐),将来 runner 换端口时挂一个钩子即可
func (h *ttydProxyHandler) InvalidateTtydProxy(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.cache, id)
}
