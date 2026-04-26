import { useState } from 'react'
import { submitFeedback } from '../api/feedback'
import { useAttemptStore } from '../store/useAttemptStore'

/**
 * FeedbackPanel
 *
 * 详情页底部小反馈入口 —— V1 内测前必需,但不等登录系统接入。
 * 点击"发送反馈"折叠链接展开 textarea + 可选星级 + 提交按钮。
 * 提交走 /v1/feedback(后端 srv.HandleFunc 直写日志,不落 DB)。
 *
 * 设计选择:
 *   - 不用 modal —— 在 ScenarioMeta 底部 inline 展开,不抢视线
 *   - 不做富文本 / 截图上传 —— 纯文字 + 星级,保持 V1 最小面
 *   - 不展示提交历史 —— 用户写一条送一条,记录在服务端日志里
 */
export default function FeedbackPanel({ scenarioSlug }: { scenarioSlug?: string }) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const [rating, setRating] = useState(0)
  const [status, setStatus] = useState<'idle' | 'sending' | 'ok' | 'err'>('idle')
  const [errMsg, setErrMsg] = useState('')
  const attemptId = useAttemptStore((s) => s.current?.attemptId)

  const canSubmit = text.trim().length > 0 && status !== 'sending'

  const onSubmit = async () => {
    if (!canSubmit) return
    setStatus('sending')
    setErrMsg('')
    try {
      await submitFeedback({
        text: text.trim(),
        scenarioSlug,
        attemptId,
        rating: rating || undefined,
      })
      setStatus('ok')
      setText('')
      setRating(0)
      // 2 秒后自动关闭面板,给用户"收到了"的视觉反馈
      setTimeout(() => {
        setStatus('idle')
        setOpen(false)
      }, 2000)
    } catch (e) {
      setStatus('err')
      setErrMsg((e as Error).message || '发送失败')
    }
  }

  if (!open) {
    return (
      <section className="mt-6 border-t pt-4 border-slate-100">
        <button
          type="button"
          className="text-xs text-slate-500 hover:text-slate-800"
          onClick={() => setOpen(true)}
        >
          💬 发送反馈(题目太难 / 文案不清 / 判题不准 / 其他)
        </button>
      </section>
    )
  }

  return (
    <section className="mt-6 border-t pt-4 border-slate-100">
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium text-slate-600">发送反馈</span>
        <button
          type="button"
          className="text-xs text-slate-400 hover:text-slate-600"
          onClick={() => {
            setOpen(false)
            setStatus('idle')
            setErrMsg('')
          }}
        >
          收起
        </button>
      </div>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value.slice(0, 2000))}
        placeholder="告诉我们哪里卡壳、哪里不准、你希望怎么改。中文就可以。"
        rows={4}
        className="w-full rounded border border-slate-300 px-2 py-1.5 text-xs text-slate-700 focus:outline-none focus:ring-1 focus:ring-brand-500 resize-y"
      />
      <div className="mt-2 flex items-center gap-1 text-xs">
        <span className="text-slate-500 mr-1">评分(可选):</span>
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            onClick={() => setRating((r) => (r === n ? 0 : n))}
            className={`w-6 text-center transition ${
              n <= rating ? 'text-amber-500' : 'text-slate-300 hover:text-slate-400'
            }`}
            aria-label={`${n} 星`}
            title={`${n} 星`}
          >
            ★
          </button>
        ))}
        <span className="text-slate-400 ml-2">{text.length}/2000</span>
      </div>
      <div className="mt-3 flex items-center gap-2">
        <button
          type="button"
          onClick={onSubmit}
          disabled={!canSubmit}
          className="px-3 h-8 rounded bg-brand-600 text-white hover:bg-brand-700 text-xs disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {status === 'sending' ? '发送中…' : '发送反馈'}
        </button>
        {status === 'ok' && <span className="text-xs text-emerald-600">已收到,谢谢!</span>}
        {status === 'err' && <span className="text-xs text-rose-500">{errMsg}</span>}
      </div>
      <p className="mt-2 text-[11px] text-slate-400 leading-snug">
        反馈会附带当前场景 slug 和 attempt id,方便我们定位问题。不要填真实密码 / 手机号 / 公司机密。
      </p>
    </section>
  )
}
