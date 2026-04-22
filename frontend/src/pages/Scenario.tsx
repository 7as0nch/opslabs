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

  // 本地镜像 start 的生命周期 —— 不依赖 useMutation 的 observer state。
  // Strict Mode 下 observer 会被重建,observer state 永远停在 pending,
  // 所以 UI 只信 call-site 回调里设进来的本地值
  const [startPhase, setStartPhase] = useState<'idle' | 'pending' | 'done' | 'error'>('idle')
  const [startErr, setStartErr] = useState<Error | null>(null)

  // 进入场景页:
  //   - 如果 store 里已经有当前 slug 的 attempt(刷新后从 localStorage 读回来的),直接复用
  //   - 否则 start 一个新 attempt
  //
  // **注意**:不能用 `start.data` / `start.isPending` 作为 source of truth。
  // React 18 Strict Mode 下 `useMutation` 的 observer 会在模拟卸载时被销毁重建,
  // `mutate()` 已经把请求发出去了,mutation 在 MutationCache 里继续跑到 success,
  // 但新 observer 从没调用过 mutate,因此 observer state 永远停在 pending。
  // hook 级和 call-site 回调挂在 mutation 自己身上,跟 observer 无关,所以可靠。
  const startingRef = useRef<string | null>(null)
  useEffect(() => {
    if (!slug) return
    if (current?.scenarioSlug === slug && current?.attemptId) return
    if (startingRef.current === slug) return
    startingRef.current = slug
    if (current && current.scenarioSlug !== slug) resetStore()
    setStartPhase('pending')
    setStartErr(null)
    // 不传 call-site callbacks —— Strict Mode 下 observer 重建会吞掉它们
    // store 写入由 useStartAttempt 的 hook-level onSuccess 负责,组件只订阅 store 变化
    start.mutate(slug)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug])

  // store 里 current 变到跟当前 slug 一致时,说明 hook-level onSuccess 已经把数据灌进来
  // 这是我们给 UI 的"success 信号",用来把 startPhase 从 pending 切到 done
  // (observer state 和 call-site 回调都不可靠,只好绕 store)
  useEffect(() => {
    if (startPhase !== 'pending') return
    if (current?.scenarioSlug === slug && current?.attemptId) {
      setStartPhase('done')
    }
  }, [startPhase, slug, current?.scenarioSlug, current?.attemptId])

  // 生命周期追踪:每次 status 变化打一条,配合 attempt.ts 里的日志可以还原时间线
  // 正常路径应当依次看到:idle -> pending -> success
  // 如果 status 一直停在 "pending" —— mutation Promise 没落地
  // 如果看到 status 切到 success 但 isPending 仍被 UI 读为 true —— React Query 订阅/观察者问题
  useEffect(() => {
    console.log(
      '[opslabs/Scenario] start status=',
      start.status,
      'isPending=',
      start.isPending,
      'hasData=',
      !!start.data,
      'hasError=',
      !!start.error,
    )
  }, [start.status, start.isPending, start.data, start.error])

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
              startPending: startPhase === 'pending',
              startError: startErr,
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
//   - 兜底文案会把关键状态打出来,卡住时一眼能看出是哪一步没走通
function renderTerminalArea(p: {
  attemptId?: string
  terminalUrl?: string
  startPending: boolean
  startError?: Error | null
}) {
  if (p.startError) {
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-rose-900/30 border border-rose-700 rounded-lg p-5 text-slate-100 text-sm leading-relaxed">
          <div className="text-base font-medium text-rose-300 mb-2">启动场景失败</div>
          <div className="text-slate-200 break-all">{p.startError.message}</div>
          <div className="mt-3 text-slate-400 text-xs">
            可能原因:Docker Desktop 未启动 / 镜像未构建 / 后端 DB 写入超时。
            打开浏览器控制台看 <code>[opslabs]</code> 开头的日志。
          </div>
        </div>
      </div>
    )
  }
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
      <div className="text-center">
        <div>{p.startPending ? '正在分配容器…' : '等待 attempt 创建…'}</div>
        {/* 诊断行:UI 和后端状态有出入时,这里会露出来 */}
        <div className="text-xs text-slate-600 mt-2 font-mono">
          isPending={String(p.startPending)} · attemptId={p.attemptId || '—'} · terminalUrl={p.terminalUrl || '—'}
        </div>
      </div>
    </div>
  )
}
