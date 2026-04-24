import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发端:5173
// 后端:6039(见 backend/configs/config.yaml)
// 代理 /v1 -> backend,前端 api/*.ts 里直接 fetch('/v1/...')
//
// ======================================================================
// COOP / COEP 跨源隔离头(web-container 模式强依赖)
// ----------------------------------------------------------------------
// WebContainer(StackBlitz)在浏览器里跑 Node.js 需要 SharedArrayBuffer,
// 浏览器对 SharedArrayBuffer 的要求是 window.crossOriginIsolated === true,
// 这就要求文档响应带上:
//   Cross-Origin-Opener-Policy:   same-origin
//   Cross-Origin-Embedder-Policy: credentialless
//
// **为什么 credentialless 而不是 require-corp**
// require-corp 会强制所有 cross-origin 子资源(包括 iframe / image / script)
// 必须带 Cross-Origin-Resource-Policy 才能加载,否则浏览器直接 block。
// 沙箱模式的 Terminal.tsx 把 ttyd 服务(`http://localhost:19997/`)挂进
// iframe,ttyd 默认不发 CORP 头,因此页面会报:
//   ERR_BLOCKED_BY_RESPONSE.NotSameOriginAfterDefaultedToSameOriginByCoep
// credentialless 这个变体允许 cross-origin 子资源不带 CORP 也能加载,
// 代价仅仅是请求时不附带 cookie / credentials —— ttyd 这种本来就无认证的
// 本地服务正合适。同时 credentialless 仍然让 crossOriginIsolated === true,
// SharedArrayBuffer / WebContainer 都正常工作。
// 浏览器支持:Chrome 110+ / Edge 110+(Firefox 134 也已开)。
// WebContainer 本来就只支持 Chromium 系,所以这个交集没增加新的兼容门槛。
//
// Resource-Policy 设 cross-origin 是兜底:有些反代回来的 /v1 子资源也走
// 同源策略校验,显式打开省得调试时碰上奇怪的"明明同源还是被拦"。
//
// 生产环境需要在反代 / CDN 上配同样的头,才能真正启用 WebContainer。
// ======================================================================
const crossOriginIsolationHeaders = {
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Embedder-Policy': 'credentialless',
  'Cross-Origin-Resource-Policy': 'cross-origin',
}

export default defineConfig({
  plugins: [
    react(),
    {
      // 统一给所有响应(包括代理回来的 /v1/*)打上 COOP/COEP 头
      // 放在 react() 之后,middleware 顺序没特殊要求
      name: 'opslabs-cross-origin-isolation',
      configureServer(server) {
        server.middlewares.use((_req, res, next) => {
          for (const [k, v] of Object.entries(crossOriginIsolationHeaders)) {
            res.setHeader(k, v)
          }
          next()
        })
      },
      configurePreviewServer(server) {
        server.middlewares.use((_req, res, next) => {
          for (const [k, v] of Object.entries(crossOriginIsolationHeaders)) {
            res.setHeader(k, v)
          }
          next()
        })
      },
    },
  ],
  server: {
    port: 5173,
    host: '0.0.0.0',
    // dev 模式下 /v1 走代理到后端 kratos
    //
    // ws: true —— /v1/ttyd/{id}/ws 是 ttyd 的 WebSocket endpoint,
    // 后端反代到 ttyd 容器靠 httputil.ReverseProxy 的 Upgrade hijack。
    // 链路:browser ──ws──► vite dev server ──ws──► kratos /v1/ttyd/.. ──ws──► ttyd
    // 少任何一段 ws 支持就会变成 ttyd 首页能加载但终端始终空白。
    proxy: {
      '/v1': {
        target: 'http://localhost:6039',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
