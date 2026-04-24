import { useNavigate } from 'react-router-dom'
import { CheckAttemptReply } from '../types'

// 按钮意图枚举:父页面用它决定后续动作
//   - close       : 关闭弹窗,保持当前容器 & 继续尝试(失败分支默认)
//   - enterReview : 通关后进入复盘 —— 前端倒计时持续,到期再 terminate
//   - restart     : 重新开始一次全新尝试 —— terminate 旧 + start 新
//   - giveUp      : 立即终止并回列表
//   - backHome    : 保持 attempt 状态,回列表(passed 状态下 backend 自然清理)
export type PassModalAction = 'close' | 'enterReview' | 'restart' | 'giveUp' | 'backHome'

interface Props {
  reply: CheckAttemptReply
  // 用户选择的后续动作 —— 由父页面实际执行
  onAction: (action: PassModalAction) => void
}

// PassModal:判题结果 + 分支选项
//
// passed 分支给三个路径:
//   - 进入 10 分钟复盘 → 顶部倒计时会切换为 review 计时
//   - 再做一次新环境   → terminate 当前 + start 新 attempt
//   - 返回列表        → 容器不立即销毁,由后端 PassedGrace 兜底
// failed 分支给两个路径:
//   - 继续做          → 关弹窗,保持当前容器
//   - 放弃退出        → terminate + 回列表
export default function PassModal({ reply, onAction }: Props) {
  const navigate = useNavigate()

  return (
    <div className="fixed inset-0 bg-slate-900/50 grid place-items-center z-50 backdrop-blur-sm">
      <div className="bg-white rounded-xl shadow-2xl w-[30rem] max-w-[92vw] overflow-hidden">
        {/* 头部条:通关彩条 vs 失败橘条 */}
        <div
          className={
            reply.passed
              ? 'h-1.5 bg-gradient-to-r from-emerald-400 to-brand-500'
              : 'h-1.5 bg-gradient-to-r from-amber-400 to-rose-400'
          }
        />

        <div className="p-6">
          <div className="flex items-start gap-3">
            <div
              className={
                reply.passed
                  ? 'w-10 h-10 rounded-full bg-emerald-100 grid place-items-center text-emerald-600 text-lg'
                  : 'w-10 h-10 rounded-full bg-amber-100 grid place-items-center text-amber-600 text-lg'
              }
            >
              {reply.passed ? '✓' : '!'}
            </div>
            <div className="flex-1 min-w-0">
              <h2 className="text-lg font-semibold text-slate-800">
                {reply.passed ? '恭喜通关' : '还没过'}
              </h2>
              <p className="mt-0.5 text-xs text-slate-500">
                {reply.passed
                  ? `用时 ${formatDuration(reply.durationSeconds)} · 检查 ${reply.checkCount} 次`
                  : `第 ${reply.checkCount} 次尝试 · 再调整一下看看`}
              </p>
            </div>
          </div>

          {/* 结果详情:成功用绿色 quote,失败用红色 pre */}
          {reply.passed ? (
            <div className="mt-4 rounded-lg bg-emerald-50 border border-emerald-100 px-4 py-3 text-sm text-emerald-800">
              {reply.message || '检查脚本输出正确,环境已达到目标状态。'}
            </div>
          ) : reply.message ? (
            <pre className="mt-4 max-h-40 overflow-auto rounded-lg bg-rose-50 border border-rose-100 px-4 py-3 text-xs text-rose-700 whitespace-pre-wrap break-all">
              {reply.message}
            </pre>
          ) : null}

          {/* 按钮区 */}
          {reply.passed ? (
            <div className="mt-6 grid grid-cols-3 gap-2">
              <button
                onClick={() => onAction('enterReview')}
                className="h-10 rounded-lg bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium transition"
              >
                进入复盘
              </button>
              <button
                onClick={() => onAction('restart')}
                className="h-10 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 text-sm transition"
              >
                再做一次
              </button>
              <button
                onClick={() => {
                  onAction('backHome')
                  navigate('/')
                }}
                className="h-10 rounded-lg border border-slate-200 hover:bg-slate-50 text-slate-600 text-sm transition"
              >
                返回列表
              </button>
            </div>
          ) : (
            <div className="mt-6 grid grid-cols-2 gap-2">
              <button
                onClick={() => onAction('close')}
                className="h-10 rounded-lg bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium transition"
              >
                继续做
              </button>
              <button
                onClick={() => {
                  onAction('giveUp')
                  navigate('/')
                }}
                className="h-10 rounded-lg bg-rose-50 hover:bg-rose-100 text-rose-700 text-sm transition"
              >
                放弃退出
              </button>
            </div>
          )}

          <p className="mt-4 text-[11px] text-slate-400 text-center">
            {reply.passed
              ? '提示:通关后容器默认保留 10 分钟复盘;选"再做一次"会销毁旧环境并启动新环境。'
              : '提示:选"继续做"不会销毁容器,可以立刻在左侧终端继续操作。'}
          </p>
        </div>
      </div>
    </div>
  )
}

function formatDuration(sec?: number): string {
  if (!sec || sec < 0) return '—'
  if (sec < 60) return `${sec}s`
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return s ? `${m}m${s}s` : `${m}m`
}
