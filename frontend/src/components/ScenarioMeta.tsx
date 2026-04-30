import { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { ScenarioDetail } from '../types'
import { useAttemptStore } from '../store/useAttemptStore'
import FeedbackPanel from './FeedbackPanel'

// 左侧:元信息 + 任务详情 + 三档提示
// Week 1 后端不下发 hint.content,这里点"解锁"只改前端 state
// Week 2 接真实解锁接口后,hintLevel 配合远程已解锁记录一起决定是否展示 content
export default function ScenarioMeta({
  scenario,
  onCollapse,
}: {
  scenario: ScenarioDetail
  onCollapse?: () => void
}) {
  const hintLevel = useAttemptStore((s) => s.hintLevel)
  const unlockHint = useAttemptStore((s) => s.unlockHint)
  const [descOpen, setDescOpen] = useState(true)

  return (
    <div className="p-5 text-sm">
      <div className="flex items-center gap-2 text-xs">
        <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600">
          {scenario.category}
        </span>
        <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600">
          L{scenario.difficulty}
        </span>
        <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600">
          ~{scenario.estimatedMinutes}min
        </span>
        {onCollapse && (
          <button
            type="button"
            title="收起任务详情面板"
            onClick={onCollapse}
            className="ml-auto px-1.5 py-0.5 rounded text-slate-400 hover:text-slate-700 hover:bg-slate-100"
          >
            ‹
          </button>
        )}
      </div>
      <h1 className="mt-2 text-lg font-semibold text-slate-800">{scenario.title}</h1>
      <p className="mt-1 text-slate-600">{scenario.summary}</p>

      {scenario.techStack?.length ? (
        <MetaLine label="技术栈" values={scenario.techStack} />
      ) : null}
      {scenario.skills?.length ? (
        <MetaLine label="技能" values={scenario.skills} />
      ) : null}
      {scenario.commands?.length ? (
        <MetaLine label="常用命令" values={scenario.commands} mono />
      ) : null}

      {scenario.descriptionMd && (
        <section className="mt-5">
          <button
            className="flex items-center gap-1 text-xs text-slate-500 hover:text-slate-800"
            onClick={() => setDescOpen((v) => !v)}
          >
            {descOpen ? '▼' : '▶'} 任务详情
          </button>
          {descOpen && (
            <div className="mt-2 prose prose-sm max-w-none prose-slate prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-code:before:content-none prose-code:after:content-none">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {scenario.descriptionMd}
              </ReactMarkdown>
            </div>
          )}
        </section>
      )}

      {scenario.hints?.length ? (
        <section className="mt-6 border-t pt-4 border-slate-100">
          <div className="text-xs font-medium text-slate-500 mb-2">提示</div>
          <ul className="space-y-2">
            {scenario.hints.map((h) => {
              const unlocked = h.level <= hintLevel
              return (
                <li
                  key={h.level}
                  className="rounded border border-slate-200 bg-slate-50 px-3 py-2"
                >
                  <div className="text-xs text-slate-500">Lv.{h.level}</div>
                  {unlocked ? (
                    <div className="mt-1 text-slate-700">
                      {h.content || '(服务端暂未下发提示内容)'}
                    </div>
                  ) : (
                    <button
                      className="mt-1 text-brand-700 hover:underline text-xs"
                      onClick={() => unlockHint(h.level)}
                    >
                      解锁第 {h.level} 档提示
                    </button>
                  )}
                </li>
              )
            })}
          </ul>
        </section>
      ) : null}

      <FeedbackPanel scenarioSlug={scenario.slug} />
    </div>
  )
}

function MetaLine({
  label,
  values,
  mono,
}: {
  label: string
  values: string[]
  mono?: boolean
}) {
  return (
    <div className="mt-3 flex gap-2 text-xs">
      <span className="text-slate-400 w-16 shrink-0">{label}</span>
      <div className="flex flex-wrap gap-1">
        {values.map((v) => (
          <span
            key={v}
            className={`px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 ${
              mono ? 'font-mono' : ''
            }`}
          >
            {v}
          </span>
        ))}
      </div>
    </div>
  )
}
