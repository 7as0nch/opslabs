import { useCountdown, formatCountdown } from '../hooks/useCountdown'

interface Props {
  // ISO 截止时间
  deadline: string | undefined
  // label 左侧标签,例如"剩余"、"复盘剩余"
  label?: string
  // 到期回调 —— 被 useCountdown 守卫过,只触发一次
  onExpire?: () => void
  // compact 时隐去 label,只显示 mm:ss,用于空间紧张场景
  compact?: boolean
  // allowOvertime: 超时不停,改为累计展示"已超时 mm:ss"
  //   - true  : 达到 0 后继续每秒 tick,label 切成"已超时",红色+脉动提醒
  //             适用于基于 scenario.estimatedMinutes 的体验型倒计时(用户还能继续做)
  //   - false : 老行为(复盘、硬到期等资源型倒计时用)
  allowOvertime?: boolean
}

/**
 * CountdownBadge
 * ---------------------------------------------------------------
 * Scenario 顶部常驻的倒计时徽章。
 *
 * 颜色规则(剩余时间越少越突出):
 *   > 5 min  : 灰(常态)
 *   1 - 5 min: 琥珀(提醒)
 *   < 1 min  : 红 + 轻微脉动(警示 + 注意视觉)
 *   已超时   : 深红 + 脉动(allowOvertime=true 才会出现)
 *
 * 过期:
 *   - allowOvertime=false: 徽章变灰,显示 00:00 并触发 onExpire
 *   - allowOvertime=true : 徽章变红脉动,显示"已超时 mm:ss",onExpire 仅第一次过线时触发
 */
export default function CountdownBadge({
  deadline,
  label = '剩余',
  onExpire,
  compact,
  allowOvertime = false,
}: Props) {
  // allowOvertime=true 时 useCountdown 返回有符号秒数(负数即超时)
  const signed = useCountdown(deadline, onExpire, allowOvertime)
  const overtime = signed < 0
  const tier = classifyRemaining(signed, allowOvertime)

  // 没有 deadline 时完全不渲染(父级来控制展示时机)
  if (!deadline) return null

  // 超时时把 label 切成"已超时",不拼在数字后面避免换行
  const effectiveLabel = overtime ? '已超时' : label
  const title = overtime
    ? `已超出预估时长 ${formatCountdown(signed)} —— 可继续做,超时后容器仍会按资源策略自动回收`
    : `将在 ${formatCountdown(signed)} 后到期`

  return (
    <div
      className={[
        'inline-flex items-center gap-1.5 rounded-full px-3 h-7 text-xs font-mono tabular-nums',
        'border transition-colors select-none',
        tier.cls,
        tier.pulse ? 'animate-pulse' : '',
      ].join(' ')}
      title={title}
    >
      {/* 小圆点指示灯,颜色跟随 tier */}
      <span className={`w-1.5 h-1.5 rounded-full ${tier.dot}`} />
      {!compact && <span className="text-[11px]">{effectiveLabel}</span>}
      <span className="tracking-wide">{formatCountdown(signed)}</span>
    </div>
  )
}

function classifyRemaining(
  sec: number,
  allowOvertime: boolean,
): { cls: string; dot: string; pulse: boolean } {
  if (sec < 0 && allowOvertime) {
    // 超时态:深红脉动,提醒用户已超出预估时长(但仍能做)
    return {
      cls: 'bg-rose-100 text-rose-800 border-rose-300',
      dot: 'bg-rose-600',
      pulse: true,
    }
  }
  if (sec <= 0) {
    return {
      cls: 'bg-slate-200 text-slate-600 border-slate-300',
      dot: 'bg-slate-500',
      pulse: false,
    }
  }
  if (sec < 60) {
    return {
      cls: 'bg-rose-50 text-rose-700 border-rose-200',
      dot: 'bg-rose-500',
      pulse: true,
    }
  }
  if (sec < 5 * 60) {
    return {
      cls: 'bg-amber-50 text-amber-700 border-amber-200',
      dot: 'bg-amber-500',
      pulse: false,
    }
  }
  return {
    cls: 'bg-slate-50 text-slate-700 border-slate-200',
    dot: 'bg-emerald-500',
    pulse: false,
  }
}
