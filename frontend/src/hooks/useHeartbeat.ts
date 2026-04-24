import { useEffect, useRef } from 'react'

/**
 * useHeartbeat: 周期向后端 POST /v1/attempts/{id}/heartbeat,刷 LastActiveAt
 *
 * 为什么存在:
 *   - 后端 GCServer 按 LastActiveAt 30min 没刷新就回收容器(资源守护)
 *   - 前端倒计时按 scenario.estimatedMinutes 展示,和 GC 解耦(用户体验)
 *   - 两者独立后,必须由前端主动告诉后端"用户还在",否则 GC 会在用户还在做题时
 *     把容器 kill 掉(经典"我刚思考了 30min 容器就没了")
 *
 * 触发条件(隔 20s 发一次):
 *   - attemptId 存在
 *   - document.visibilityState === 'visible'(tab 在前台)
 *   - 最近 idleWindowMs 内有用户交互(键盘 / 鼠标 / 触屏)
 *     → 防止"tab 开着人走开"的场景一直续命,违背资源守护初衷
 *
 * 失败策略:
 *   - 响应 4xx: stop 后续心跳(attempt 可能已终态,继续心跳无意义)
 *   - 响应 5xx / 网络错误: 只 console.warn,下一轮继续尝试(可能只是瞬态抖)
 *
 * 清理:
 *   - attemptId 改变 / 组件卸载时,清定时器 + 移除 event listener
 */

const HEARTBEAT_INTERVAL_MS = 20_000
const IDLE_WINDOW_MS = 60_000 // 最近 60s 有交互才发心跳(2-3 个间隔)

export function useHeartbeat(attemptId?: string) {
  const lastInteractAt = useRef<number>(Date.now())
  // stopped 为 true 后不再 fetch,直到 attemptId 变化重建整个 effect
  const stopped = useRef(false)

  useEffect(() => {
    if (!attemptId) return
    stopped.current = false
    lastInteractAt.current = Date.now() // 进页面视为一次交互

    const markInteract = () => {
      lastInteractAt.current = Date.now()
    }

    // 监听一组"证明用户在操作"的事件。passive 防止误伤原生滚动 / 触屏性能。
    const events: Array<keyof WindowEventMap> = [
      'keydown',
      'mousedown',
      'mousemove',
      'touchstart',
      'wheel',
    ]
    events.forEach((ev) => window.addEventListener(ev, markInteract, { passive: true }))

    const tick = async () => {
      if (stopped.current) return
      if (document.visibilityState !== 'visible') return
      if (Date.now() - lastInteractAt.current > IDLE_WINDOW_MS) return

      try {
        const res = await fetch(`/v1/attempts/${attemptId}/heartbeat`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: '{}',
        })
        if (!res.ok) {
          // 4xx 基本都是 attempt 终态 / id 非法 / 路径不对,没必要硬刚,stop
          if (res.status >= 400 && res.status < 500) {
            console.warn('[heartbeat] stopping due to 4xx', res.status)
            stopped.current = true
          } else {
            console.warn('[heartbeat] 5xx, will retry', res.status)
          }
        }
      } catch (e) {
        // 网络抖动不认为是致命错,下一轮继续尝试
        console.warn('[heartbeat] network error, will retry', e)
      }
    }

    const timer = window.setInterval(tick, HEARTBEAT_INTERVAL_MS)

    return () => {
      window.clearInterval(timer)
      events.forEach((ev) => window.removeEventListener(ev, markInteract))
      stopped.current = true
    }
  }, [attemptId])
}
