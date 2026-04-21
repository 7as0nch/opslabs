import { useCallback, useEffect, useRef, useState } from 'react'

// Terminal:ttyd 本来就是一个完整的 web ui,iframe 嵌进来即可
// 为什么不自己接 xterm.js?—— 后端只暴露 ttyd,自己接 websocket 要重写认证/键位,收益不划算
// 但 iframe 的一个坑:跨域下无法感知到 "ERR_CONNECTION_REFUSED",页面只会静默空白。
// 所以加载前先做一次 no-cors HEAD 探测,失败就展示兜底面板,给用户重试/诊断方向。

type ProbeState = 'idle' | 'probing' | 'ok' | 'fail'

export default function Terminal({ src }: { src: string }) {
  const [state, setState] = useState<ProbeState>('idle')
  const [attempt, setAttempt] = useState(0) // 用来强制刷新 iframe
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const probe = useCallback(async () => {
    setState('probing')
    const ctl = new AbortController()
    timer.current && clearTimeout(timer.current)
    timer.current = setTimeout(() => ctl.abort(), 3500)
    try {
      // no-cors 拿不到 status,但能区分"端口开 vs 拒绝/超时"
      await fetch(src, { mode: 'no-cors', signal: ctl.signal, cache: 'no-store' })
      setState('ok')
    } catch {
      setState('fail')
    } finally {
      timer.current && clearTimeout(timer.current)
    }
  }, [src])

  useEffect(() => {
    if (!src) return
    void probe()
    return () => {
      timer.current && clearTimeout(timer.current)
    }
  }, [src, probe])

  const onRetry = () => {
    setAttempt((x) => x + 1)
    void probe()
  }

  if (state === 'probing' || state === 'idle') {
    return (
      <div className="h-full grid place-items-center text-slate-400 text-sm">
        正在连接终端 {src}…
      </div>
    )
  }

  if (state === 'fail') {
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-slate-800/70 border border-slate-700 rounded-lg p-5 text-slate-200 text-sm leading-relaxed">
          <div className="text-base font-medium text-rose-300 mb-2">终端连接失败</div>
          <div className="text-slate-300">
            无法访问 <code className="px-1 rounded bg-slate-900/80">{src}</code>。
          </div>
          <ul className="mt-3 space-y-1 text-slate-400 list-disc list-inside">
            <li>
              如果你在 <b>mock 模式</b>(RUNTIME_MODE=mock),返回的是伪造端口,
              不会真的启动 ttyd。请切到 docker 模式体验真实终端。
            </li>
            <li>
              如果已经切到 <b>docker 模式</b>:确认 Docker Desktop 在跑、镜像
              <code className="px-1 rounded bg-slate-900/80">opslabs/hello-world:v1</code>
              已构建、并且容器 7681 端口已映射出来。
            </li>
            <li>可能是 ttyd 还没起来,等 1–2 秒再重试。</li>
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
