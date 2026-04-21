import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useScenario } from '../api/scenario'
import {
  useAttempt,
  useCheckAttempt,
  useStartAttempt,
  useTerminateAttempt,
} from '../api/attempt'
import { useAttemptStore } from '../store/useAttemptStore'
import ScenarioMeta from '../components/ScenarioMeta'
import Terminal from '../components/Terminal'
import ActionBar from '../components/ActionBar'
import PassModal from '../components/PassModal'
import { CheckAttemptReply } from '../types'

// Scenario 页布局:
//   左边 ScenarioMeta(任务描述 + hints)
//   右上 Terminal(iframe 连 ttyd)
//   右下 ActionBar(check / give up)
//   判题结果用 PassModal 覆盖层
export default function Scenario() {
  const { slug } = useParams<{ slug: string }>()
  const { data: scenario, isLoading, error } = useScenario(slug)

  const start = useStartAttempt()
  const check = useCheckAttempt()
  const terminate = useTerminateAttempt()

  const current = useAttemptStore((s) => s.current)
  const setCurrent = useAttemptStore((s) => s.set)
  const patchCurrent = useAttemptStore((s) => s.patch)
  const resetStore = useAttemptStore((s) => s.reset)

  // attempt id 变动后轮询拉状态(兼作心跳)
  const { data: attemptRemote } = useAttempt(current?.attemptId, !!current?.attemptId)

  const [checkInfo, setCheckInfo] = useState<{
    reply: CheckAttemptReply
    open: boolean
  } | null>(null)

  // 进入场景页,先 reset,再 start 一个新 attempt
  // React 18 Strict Mode 下 useEffect 会被调两次,resetStore() 会清 store,
  // 原来的 `current?.scenarioSlug === slug` 守卫失效 → 会起两个容器。
  // 这里用 ref 记住「正在为哪个 slug 发 start」,天然穿过 strict mode 的双调
  const startingRef = useRef<string | null>(null)
  useEffect(() => {
    if (!slug) return
    if (current?.scenarioSlug === slug) return
    if (startingRef.current === slug) return
    startingRef.current = slug
    resetStore()
    start.mutate(slug, {
      onSuccess: (r) => {
        const now = new Date().toISOString()
        setCurrent({
          attemptId: r.attemptId,
          scenarioSlug: slug,
          status: 'running',
          terminalUrl: r.terminalUrl,
          startedAt: now,
          lastActiveAt: now,
        })
      },
      onError: () => {
        // 失败时放锁,允许用户重试触发
        startingRef.current = null
      },
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug])

  // 后端轮询结果覆盖 store,以保证 status 是最新的
  useEffect(() => {
    if (attemptRemote) setCurrent(attemptRemote)
  }, [attemptRemote, setCurrent])

  // 卸载时主动 terminate,防止空闲容器泄露
  useEffect(() => {
    return () => {
      const a = useAttemptStore.getState().current
      if (a?.attemptId && a.status === 'running') {
        terminate.mutate(a.attemptId)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const onCheck = () => {
    if (!current?.attemptId) return
    check.mutate(current.attemptId, {
      onSuccess: (r) => {
        setCheckInfo({ reply: r, open: true })
        if (r.passed) {
          patchCurrent({ status: 'passed' })
        }
      },
    })
  }

  const onGiveUp = () => {
    if (!current?.attemptId) return
    terminate.mutate(current.attemptId, {
      onSuccess: (r) => {
        patchCurrent({ status: r.status })
      },
    })
  }

  if (isLoading) return <div className="p-8 text-slate-400">加载场景…</div>
  if (error || !scenario) {
    return (
      <div className="p-8 text-rose-600">
        场景加载失败:{(error as Error)?.message ?? 'not found'}
      </div>
    )
  }

  const isFinished =
    !!current?.status && current.status !== 'running'

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-[22rem_1fr] gap-0 overflow-hidden">
        <aside className="border-r border-slate-200 bg-white min-h-0 overflow-y-auto scroll-thin">
          <ScenarioMeta scenario={scenario} />
        </aside>
        <section className="min-h-0 flex flex-col overflow-hidden">
          <div className="flex-1 min-h-0 bg-slate-900 overflow-hidden">
            {renderTerminalArea({
              attemptId: current?.attemptId,
              terminalUrl: current?.terminalUrl,
              startPending: start.isPending,
            })}
          </div>
          <ActionBar
            status={current?.status}
            checkCount={checkInfo?.reply.checkCount ?? 0}
            onCheck={onCheck}
            onGiveUp={onGiveUp}
            checking={check.isPending}
            disabled={!current || isFinished}
          />
        </section>
      </div>

      {checkInfo?.open && (
        <PassModal
          reply={checkInfo.reply}
          onClose={() => setCheckInfo({ ...checkInfo, open: false })}
        />
      )}
    </div>
  )
}

// renderTerminalArea 把"attemptId 有无 / terminalUrl 有无 / start 中"三种态归一化:
//   - start.isPending 且还没 attempt:正在分配容器…
//   - 有 attempt 但 terminalUrl 空:后端是 mock 模式,显示预览卡(不再尝试 iframe)
//   - 都有:正常挂 Terminal(组件内部自己做 ERR_CONNECTION 探测)
function renderTerminalArea(p: {
  attemptId?: string
  terminalUrl?: string
  startPending: boolean
}) {
  if (p.attemptId && !p.terminalUrl) {
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-slate-800/70 border border-slate-700 rounded-lg p-5 text-slate-200 text-sm leading-relaxed">
          <div className="text-base font-medium text-amber-300 mb-2">Mock 预览模式</div>
          <div className="text-slate-300">
            后端运行在 <code className="px-1 rounded bg-slate-900/80">driver: mock</code>,
            不会启动真实容器,也就没有终端可连。
          </div>
          <div className="mt-2 text-slate-400">
            这个模式只用来冒烟 API 契约(list / start / check / terminate)。
            想看真实终端,请把 <code>backend/configs/config.yaml</code> 里
            <code className="mx-1 px-1 rounded bg-slate-900/80">runtime.driver</code>
            改成 <code>docker</code>,再确保 Docker Desktop 在跑。
          </div>
          <div className="mt-3 text-slate-500 text-xs">
            你仍然可以点"检查答案",mock runtime 默认返回 OK,用于验证前后端打通。
          </div>
        </div>
      </div>
    )
  }
  if (p.attemptId && p.terminalUrl) {
    return <Terminal src={p.terminalUrl} />
  }
  return (
    <div className="h-full grid place-items-center text-slate-400">
      {p.startPending ? '正在分配容器…' : '等待 attempt 创建…'}
    </div>
  )
}
