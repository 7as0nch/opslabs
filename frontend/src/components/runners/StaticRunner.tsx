import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { ClientCheckResult } from '../../types'

/**
 * StaticRunner
 *
 * static 执行模式的前端 Runner:
 *   - 直接把后端 embed.FS 下发的 bundle iframe 挂上去
 *   - 用 postMessage 协议跟 bundle 通信,完成判题
 *   - 上层通过 ref.triggerCheck() 拉起一次判题,拿到 ClientCheckResult
 *     再交给 /v1/attempts/{id}/check 带上去
 *
 * postMessage 协议(跟 bundles 各 slug 下的 index.html 约定):
 *   父 → 子:  { type: 'opslabs:check' }
 *   子 → 父:  { type: 'opslabs:ready' }
 *   子 → 父:  { type: 'opslabs:check-result', passed, exitCode, stdout, stderr }
 *
 * 为什么用 imperative handle(ref)而不是 props 传 "checkToken":
 *   - Scenario 页的 onCheck 是事件驱动(点击按钮),用 state 递增 token 也行,
 *     但需要在 Runner 里加一层 useEffect 去做 "token 变了就 post",
 *     不如直接给父层一个 async 方法清晰
 *   - 还可以把 5s 超时 + 结果 awaiting 封装在 Runner 内部,外面不用管
 */
export interface StaticRunnerHandle {
  /**
   * 触发一次判题,返回 Promise<ClientCheckResult>
   *
   * 失败(iframe 未就绪 / 超时 / bundle 内部错误)以 reject 抛出,
   * 上层可以拿 error.message 展示给用户
   */
  triggerCheck: () => Promise<ClientCheckResult>
  /** 是否收到过 opslabs:ready,UI 可据此禁用"检查答案"按钮 */
  ready: boolean
}

interface Props {
  /** bundle 入口 URL,值形如 /v1/scenarios/{slug}/bundle/index.html */
  bundleUrl: string
}

// CHECK_TIMEOUT_MS bundle 判题超时。5s 对 CSS / HTML 类题型绰绰有余,
// 调大会拉长用户等待,调小容易把慢机器的真实结果吃掉
const CHECK_TIMEOUT_MS = 5000

const StaticRunner = forwardRef<StaticRunnerHandle, Props>(function StaticRunner(
  { bundleUrl },
  ref,
) {
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const [ready, setReady] = useState(false)
  // iframe 文档加载完成标志
  //
  // 为什么要这个(2026-04-23 R7-04c):
  //   wasm-linux bundle 自己有完整的加载 UI —— boot overlay 显示"加载 v86 并引导
  //   BusyBox…"、顶栏有诊断面板(秒数 / serial 字节数),这些信息比父级一个笼统的
  //   "加载题目 bundle…" 友好得多。但之前父级 overlay 一直到 opslabs:ready 才消,
  //   bundle overlay 和父级 overlay 同时居中显示,看上去像重叠/错乱,用户以为坏了。
  //
  //   方案:iframe 的 HTML 文档一 onload 完成(bundle 的 <script> 已经开跑),立刻
  //   把父级 overlay 撤掉。bundle 从这一刻起全权负责展示"我在启动"的状态。
  //   opslabs:ready 仍然用来控制"检查答案"按钮能不能点(ready 语义不变)。
  const [bundleMounted, setBundleMounted] = useState(false)

  // 当前 in-flight 的 check:每次 triggerCheck 覆盖一份 resolver/timer
  // 同一时刻只允许一个 in-flight,再次触发会把老的超时清掉
  const pendingRef = useRef<{
    resolve: (r: ClientCheckResult) => void
    reject: (e: Error) => void
    timer: ReturnType<typeof setTimeout>
  } | null>(null)

  // 清理 in-flight 的 check,避免组件卸载或 bundleUrl 切换时 Promise 永不落地
  const clearPending = useCallback((err?: Error) => {
    const p = pendingRef.current
    if (!p) return
    clearTimeout(p.timer)
    pendingRef.current = null
    if (err) p.reject(err)
  }, [])

  // postMessage 监听
  // 用 window 级监听是因为 bundle iframe 的 contentWindow 在 srcdoc 内跑,
  // 直接给 iframe.contentWindow addEventListener 会被 sandbox 隔开
  useEffect(() => {
    const onMsg = (e: MessageEvent) => {
      // 宽松校验:只认自家协议 type,不校验 origin 因为 bundle 跟后端同源
      if (!e.data || typeof e.data !== 'object') return
      const { type } = e.data as { type?: string }
      if (type === 'opslabs:ready') {
        setReady(true)
        return
      }
      if (type === 'opslabs:check-result') {
        const p = pendingRef.current
        if (!p) return // 迟到消息,忽略
        clearTimeout(p.timer)
        pendingRef.current = null
        p.resolve({
          passed: !!e.data.passed,
          exitCode: typeof e.data.exitCode === 'number' ? e.data.exitCode : 0,
          stdout: typeof e.data.stdout === 'string' ? e.data.stdout : '',
          stderr: typeof e.data.stderr === 'string' ? e.data.stderr : '',
        })
      }
    }
    window.addEventListener('message', onMsg)
    return () => window.removeEventListener('message', onMsg)
  }, [])

  // bundleUrl 变化 ⇒ 重置 ready / bundleMounted 状态,等新 iframe 重新发 ready / onload
  useEffect(() => {
    setReady(false)
    setBundleMounted(false)
    return () => {
      clearPending(new Error('bundle unmounted'))
    }
  }, [bundleUrl, clearPending])

  useImperativeHandle(
    ref,
    () => ({
      ready,
      triggerCheck: () =>
        new Promise<ClientCheckResult>((resolve, reject) => {
          const iframe = iframeRef.current
          if (!iframe || !iframe.contentWindow) {
            reject(new Error('bundle iframe not attached'))
            return
          }
          if (!ready) {
            reject(new Error('bundle not ready yet, 请稍候再试'))
            return
          }
          // 替换上一次的 in-flight(兜底,正常不会出现)
          clearPending()
          const timer = setTimeout(() => {
            pendingRef.current = null
            reject(new Error(`bundle check timeout after ${CHECK_TIMEOUT_MS}ms`))
          }, CHECK_TIMEOUT_MS)
          pendingRef.current = { resolve, reject, timer }
          // 目标 origin 写 '*' 而不是精确 origin:
          //  - bundle 跟后端同源(同一台 kratos server 下发),理论上能写精确值
          //  - 但开发环境 vite dev server 走 proxy,iframe 的 origin 会变成 localhost:5173
          //    写精确反而更容易因为 origin 不一致被浏览器静默吞掉,'*' 是最省心的选择
          //  - bundle 内部也不做敏感操作,消息被拦截也无所谓
          iframe.contentWindow.postMessage({ type: 'opslabs:check' }, '*')
        }),
    }),
    [ready, clearPending],
  )

  return (
    <div className="w-full h-full relative bg-slate-900">
      <iframe
        ref={iframeRef}
        // 允许 same-origin 脚本(bundle 内部要跑判题逻辑);不允许 top navigation / popups
        sandbox="allow-scripts allow-same-origin"
        src={bundleUrl}
        onLoad={() => setBundleMounted(true)}
        title="opslabs static bundle"
        className="w-full h-full border-0 bg-white block"
      />
      {/* 只在 iframe 文档都还没 load 时显示父级加载提示;onload 之后交给 bundle
          自己的 UI(避免 wasm-linux 场景出现"加载题目 bundle" 和"加载 v86" 双层叠) */}
      {!bundleMounted && (
        <div className="absolute inset-0 grid place-items-center pointer-events-none">
          <div className="px-3 py-2 rounded bg-slate-800/80 text-slate-200 text-xs font-mono">
            加载题目 bundle…
          </div>
        </div>
      )}
    </div>
  )
})

export default StaticRunner
