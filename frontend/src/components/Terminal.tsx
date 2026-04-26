import { useCallback, useEffect, useRef, useState } from 'react'

// Terminal:ttyd 本来就是一个完整的 web ui,iframe 嵌进来即可
// 为什么不自己接 xterm.js?—— 后端只暴露 ttyd,自己接 websocket 要重写认证/键位,收益不划算
//
// ★★★ 2026-04-22 改造:走后端同源反代,不再指到 ttyd 宿主端口 ★★★
// 历史背景:
//   - 之前 src 是 "http://localhost:{port}/"(ttyd 容器映射端口),浏览器会把它
//     视为跨源,与父页 COEP=credentialless 组合产生两类失败:
//       1. ERR_BLOCKED_BY_RESPONSE.NotSameOriginAfterDefaultedToSameOriginByCoep
//          (require-corp 时),表现为 iframe 空白
//       2. NetworkError,Chrome 在中文 locale 下误译为"localhost 拒绝了我们的连接请求",
//          跟真正的 TCP ECONNREFUSED 无法区分
//   - 给 iframe 加 credentialless 属性只修了一半,不同 Chrome 版本实现不一致
// 新方案:
//   - 后端加 /v1/ttyd/{attemptId}/ 反向代理,WebSocket + HTTP 一并透传
//   - AttemptService.terminalUrl() 返回相对路径,iframe src 解析为同源
//   - vite proxy 开 ws:true,dev 也畅通
//   - 同源 iframe 完全不受 COEP 影响,不需要任何 credentialless 属性
// 副作用(正向):宿主 ttyd 端口不再外露,前端只能通过后端鉴权摸到终端

type ProbeState = 'idle' | 'probing' | 'ok' | 'fail'

// 指数退避参数 —— 容器通常 2-5s 起来,给到第 5 次重试累计等 ~7.5s
// 比单次 3.5s 宽容很多,但又不让用户面对"正在连接…"半分钟不动
const MAX_PROBE_RETRIES = 5
const PROBE_BASE_DELAY_MS = 500
const PROBE_MAX_DELAY_MS = 3000

export default function Terminal({ src }: { src: string }) {
  const [state, setState] = useState<ProbeState>('idle')
  const [attempt, setAttempt] = useState(0) // 用来强制刷新 iframe
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryCountRef = useRef(0)

  const cancelTimer = () => {
    if (timer.current) {
      clearTimeout(timer.current)
      timer.current = null
    }
  }

  // 走同源反代后 fetch 能正常拿 status,改为"HEAD 成功即放行、失败给兜底"
  // 注:反代对非 WS 请求返回 ttyd 的首页 200,对已过期 attempt 我们会返 404/410,
  // probe 据此区分"后端反代就绪 / attempt 已失效"两种场景
  //
  // 2026-04-24 加固:指数退避重试
  //   容器冷启动 2-5s,之前单次 3.5s 超时就 fail,用户被迫手点"重试"3-5 次。
  //   改成前 5 次失败自动退避重试(500→1000→2000→3000→3000 ms + 抖动),
  //   全部失败后才进 fail UI。用户手动点"重试连接"会清零计数器重新来一轮。
  const probe = useCallback(
    async (resetCount = false) => {
      if (resetCount) retryCountRef.current = 0
      setState('probing')
      const ctl = new AbortController()
      cancelTimer()
      timer.current = setTimeout(() => ctl.abort(), 3500)
      let ok = false
      try {
        const resp = await fetch(src, { signal: ctl.signal, cache: 'no-store' })
        ok = resp.ok
      } catch {
        ok = false
      } finally {
        cancelTimer()
      }
      if (ok) {
        retryCountRef.current = 0
        setState('ok')
        return
      }
      if (retryCountRef.current < MAX_PROBE_RETRIES) {
        const attemptN = retryCountRef.current
        retryCountRef.current += 1
        const base = Math.min(PROBE_BASE_DELAY_MS * 2 ** attemptN, PROBE_MAX_DELAY_MS)
        // ±25% 抖动,避免并发多实例同步重试打在反代同一个 tick
        const jitter = base * (0.75 + Math.random() * 0.5)
        timer.current = setTimeout(() => void probe(false), jitter)
        return
      }
      setState('fail')
    },
    [src],
  )

  useEffect(() => {
    if (!src) return
    void probe(true)
    return () => {
      cancelTimer()
    }
  }, [src, probe])

  const onRetry = () => {
    setAttempt((x) => x + 1)
    void probe(true)
  }

  if (state === 'probing' || state === 'idle') {
    return (
      <div className="h-full grid place-items-center text-slate-400 text-sm">
        正在连接终端…
      </div>
    )
  }

  if (state === 'fail') {
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-slate-800/70 border border-slate-700 rounded-lg p-5 text-slate-200 text-sm leading-relaxed">
          <div className="text-base font-medium text-rose-300 mb-2">终端连接失败</div>
          <div className="text-slate-300">
            后端反代 <code className="px-1 rounded bg-slate-900/80">{src}</code> 未能握手。
          </div>
          <ul className="mt-3 space-y-1 text-slate-400 list-disc list-inside">
            <li>
              <b>mock 模式</b>(RUNTIME_MODE=mock)不分配真实端口,terminalUrl 会是空串,
              这里不应出现。请切到 docker 模式。
            </li>
            <li>
              <b>docker 模式</b>:确认 Docker Desktop 在跑,
              镜像 <code className="px-1 rounded bg-slate-900/80">opslabs/hello-world:v1</code>{' '}
              已构建。
            </li>
            <li>
              <b>attempt 已过期</b>:后端反代会返 404/410,请重新进场景。
            </li>
            <li>ttyd 容器可能还没起来,等 1–2 秒再重试。</li>
          </ul>
          <div className="mt-4 flex gap-2">
            <button
              onClick={onRetry}
              className="px-3 h-8 rounded bg-brand-600 text-white hover:bg-brand-700 text-xs"
            >
              重试连接
            </button>
            <a
              href={src}
              target="_blank"
              rel="noreferrer"
              className="px-3 h-8 inline-flex items-center rounded border border-slate-600 text-slate-200 hover:bg-slate-700 text-xs"
            >
              新窗口打开
            </a>
          </div>
        </div>
      </div>
    )
  }

  return (
    <iframe
      key={`${src}#${attempt}`}
      src={src}
      title="opslabs terminal"
      className="w-full h-full border-0 bg-black block"
      allow="clipboard-read; clipboard-write"
    />
  )
}
