import { AttemptStatus } from '../types'

interface Props {
  status?: AttemptStatus
  checkCount: number
  onCheck: () => void
  onGiveUp: () => void
  checking: boolean
  disabled: boolean
}

// 底部动作条:check + give up + 状态显示
export default function ActionBar({
  status,
  checkCount,
  onCheck,
  onGiveUp,
  checking,
  disabled,
}: Props) {
  return (
    <div className="h-14 shrink-0 border-t border-slate-200 bg-white flex items-center justify-between px-5">
      <div className="text-xs text-slate-500">
        <StatusPill status={status} />
        <span className="ml-3">已检查 {checkCount} 次</span>
      </div>
      <div className="flex items-center gap-2">
        <button
          className="px-3 h-9 rounded border border-slate-300 text-slate-600 hover:bg-slate-50 disabled:opacity-50"
          onClick={onGiveUp}
          disabled={disabled}
        >
          放弃
        </button>
        <button
          className="px-4 h-9 rounded bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50"
          onClick={onCheck}
          disabled={disabled || checking}
        >
          {checking ? '判题中…' : '检查答案'}
        </button>
      </div>
    </div>
  )
}

function StatusPill({ status }: { status?: AttemptStatus }) {
  const map: Record<AttemptStatus, { label: string; cls: string }> = {
    running: { label: '进行中', cls: 'bg-emerald-100 text-emerald-700' },
    passed: { label: '已通关', cls: 'bg-brand-50 text-brand-700' },
    expired: { label: '已超时', cls: 'bg-slate-100 text-slate-500' },
    terminated: { label: '已结束', cls: 'bg-slate-100 text-slate-500' },
    failed: { label: '异常', cls: 'bg-rose-100 text-rose-700' },
  }
  const s = status ? map[status] : { label: '等待启动', cls: 'bg-slate-100 text-slate-400' }
  return <span className={`px-2 py-0.5 rounded text-xs ${s.cls}`}>{s.label}</span>
}
