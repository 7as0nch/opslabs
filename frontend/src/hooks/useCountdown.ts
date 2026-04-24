import { useEffect, useRef, useState } from 'react'

/**
 * useCountdown
 * ---------------------------------------------------------------
 * 传入一个 ISO 字符串的截止时间,按秒刷新剩余秒数。
 * 适合 Scenario 顶部的倒计时徽章、PassModal 复盘倒计时。
 *
 * 特性:
 *   - deadline 变化(续期 / 切场景)后内部重新对齐,不累计漂移
 *   - deadline 为空 返回 0
 *   - 一秒一次 setState,组件重渲染成本可控
 *   - 到期自动触发 onExpire(保证只触发一次)
 *   - tab 切到后台 React 仍会更新 state,回来显示无需手工刷新
 *
 * allowOvertime=false(默认兼容老行为):
 *   - 过期后返回 0,内部 clearInterval 停掉定时器
 *   - 调用方读到负值的风险为零
 *
 * allowOvertime=true(V1 新增,Scenario 的 estimatedMinutes 模式用):
 *   - 过期后继续每秒 tick,返回"有符号秒数"(负数表示已超时多少秒)
 *   - onExpire 仍然只触发一次(在跨过 0 的那一瞬),之后 badge 展示累计超时
 *   - 需要 Scenario 页显式告诉用户"超时不会销毁,可以继续做"
 */
export function useCountdown(
  deadline: string | undefined,
  onExpire?: () => void,
  allowOvertime = false,
): number {
  const [remaining, setRemaining] = useState<number>(() => calcRemaining(deadline, allowOvertime))
  // 只触发一次 onExpire —— 组件内部 useState 闭包 + ref 守卫防止重复
  const firedRef = useRef(false)

  useEffect(() => {
    firedRef.current = false
    // 立即同步一次,避免从非目标 deadline 切换来的陈旧 state
    setRemaining(calcRemaining(deadline, allowOvertime))
    if (!deadline) return
    const id = window.setInterval(() => {
      const r = calcRemaining(deadline, allowOvertime)
      setRemaining(r)
      if (r <= 0 && !firedRef.current) {
        firedRef.current = true
        onExpire?.()
        if (!allowOvertime) {
          // 到期后不再定时刷新,节省资源(老行为)
          window.clearInterval(id)
        }
        // allowOvertime=true 时继续 tick,让 badge 展示累计超时
      }
    }, 1000)
    return () => window.clearInterval(id)
    // onExpire 故意不进依赖 —— 调用方一般传匿名函数,每次 re-render 都是新引用,
    // 进依赖会导致每秒清定时器再建一遍。改成 ref 写入也可,但那种做法对读者更不直觉,
    // 这里显式地用 eslint-disable 注明意图,更清楚。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deadline, allowOvertime])

  return remaining
}

function calcRemaining(deadline: string | undefined, allowOvertime = false): number {
  if (!deadline) return 0
  const t = Date.parse(deadline)
  if (!Number.isFinite(t)) return 0
  const diff = Math.floor((t - Date.now()) / 1000)
  if (allowOvertime) return diff // 可能为负,表示已超时
  return diff > 0 ? diff : 0
}

/**
 * formatCountdown 秒 → "mm:ss" / "hh:mm:ss"
 * 超过 1 小时补小时位;< 1 分钟也展示为 00:ss 保持列宽一致
 * 负值(超时)取绝对值后同样格式化 —— 调用方决定要不要加"已超时"前缀
 */
export function formatCountdown(sec: number): string {
  if (sec === 0) return '00:00'
  const abs = Math.abs(sec)
  const h = Math.floor(abs / 3600)
  const m = Math.floor((abs % 3600) / 60)
  const s = abs % 60
  const pad = (n: number) => (n < 10 ? '0' + n : String(n))
  if (h > 0) return `${pad(h)}:${pad(m)}:${pad(s)}`
  return `${pad(m)}:${pad(s)}`
}
