import { Link } from 'react-router-dom'
import { CheckAttemptReply } from '../types'

// 判题结果弹窗:通关 / 未通关两条路径
// 后端当前只回传 message + passed + durationSeconds + checkCount
export default function PassModal({
  reply,
  onClose,
}: {
  reply: CheckAttemptReply
  onClose: () => void
}) {
  return (
    <div className="fixed inset-0 bg-slate-900/40 grid place-items-center z-50">
      <div className="bg-white rounded-lg shadow-lg w-[28rem] max-w-[92vw] p-6">
        <div className="flex items-start justify-between">
          <h2 className="text-lg font-semibold">
            {reply.passed ? '🎉 通关' : '还没过'}
          </h2>
          <button
            className="text-slate-400 hover:text-slate-600"
            onClick={onClose}
          >
            ×
          </button>
        </div>

        {reply.passed ? (
          <div className="mt-3 text-sm text-slate-600 space-y-1">
            <p>{reply.message || '恭喜通关,容器会保留一段时间方便你复盘。'}</p>
            {reply.durationSeconds ? (
              <p className="text-xs text-slate-500">
                用时 {formatDuration(reply.durationSeconds)} · 检查 {reply.checkCount} 次
              </p>
            ) : null}
          </div>
        ) : (
          <div className="mt-3 text-sm">
            <div className="text-slate-500">尝试第 {reply.checkCount} 次 · 判题未通过</div>
            {reply.message && (
              <pre className="mt-1 max-h-32 overflow-auto bg-rose-50 text-rose-700 rounded p-2 text-xs whitespace-pre-wrap">
                {reply.message}
              </pre>
            )}
          </div>
        )}

        <div className="mt-5 flex justify-end gap-2">
          {reply.passed && (
            <Link
              to="/"
              className="px-3 h-9 inline-flex items-center rounded border border-slate-300 text-slate-600 hover:bg-slate-50"
            >
              返回场景列表
            </Link>
          )}
          <button
            className="px-4 h-9 rounded bg-brand-600 text-white hover:bg-brand-700"
            onClick={onClose}
          >
            {reply.passed ? '继续复盘' : '继续尝试'}
          </button>
        </div>
      </div>
    </div>
  )
}

function formatDuration(sec: number): string {
  if (sec < 60) return `${sec}s`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s ? `${m}m${s}s` : `${m}m`
}
