import { useCallback, useEffect, useMemo, useRef, useState, type MutableRefObject } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useScenario } from '../api/scenario'
import {
  useAttempt,
  useCheckAttempt,
  useStartAttempt,
  useTerminateAttempt,
} from '../api/attempt'
import { useAttemptStore } from '../store/useAttemptStore'
import { useCountdown } from '../hooks/useCountdown'
import { useHeartbeat } from '../hooks/useHeartbeat'
import ScenarioMeta from '../components/ScenarioMeta'
import Terminal from '../components/Terminal'
import ActionBar from '../components/ActionBar'
import PassModal, { PassModalAction } from '../components/PassModal'
import BootSplash from '../components/BootSplash'
import CountdownBadge from '../components/CountdownBadge'
import StaticRunner, { StaticRunnerHandle } from '../components/runners/StaticRunner'
import WebContainerRunner, {
  WebContainerRunnerHandle,
} from '../components/runners/WebContainerRunner'
import { CheckAttemptReply, ClientCheckResult, ExecutionMode } from '../types'
import type { ApiError } from '../api/http'

// Scenario 页整体布局:
//   ┌─────────────────────────────────────────────┐
//   │ TopBar  ← 返回 · 标题 · 倒计时             │
//   ├────────┬────────────────────────────────────┤
//   │ Meta   │ Runner (BootSplash / Terminal / … )│
//   │ Hints  │                                    │
//   │        ├────────────────────────────────────┤
//   │        │ ActionBar (check / give up)        │
//   └────────┴────────────────────────────────────┘
//
// 四执行模式都共用这个壳:
//   - sandbox      : Runner = <Terminal/>(ttyd iframe)
//   - static       : Runner = <StaticRunner/>(bundle iframe + postMessage)
//   - wasm-linux   : 同上(bundle 里跑 v86)
//   - web-container: Runner = <WebContainerRunner/>(WebContainer.spawn)
//
// 倒计时/复盘/自动终止策略都在本层,与 Runner 解耦。

// 复盘窗口默认 10 分钟。后端 PassedGrace 也是 10 min,前端本地倒计时
// 到期会触发 terminate,保证跟后端兜底一致。
const DEFAULT_REVIEW_MS = 10 * 60 * 1000

export default function Scenario() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const { data: scenario, isLoading, error } = useScenario(slug)

  const start = useStartAttempt()
  const check = useCheckAttempt()
  const terminate = useTerminateAttempt()

  const current = useAttemptStore((s) => s.current)
  const patchCurrent = useAttemptStore((s) => s.patch)
  const resetStore = useAttemptStore((s) => s.reset)

  // 30s 轮询 attempt
  // NEEDS_RESTART 分支(后端 GetAttempt 发现 sandbox 容器已死):
  //   前端拿到 409 错误 → 清 store + 重新 Start,UX 表现"自动补一次,用户无感"
  //   避免用户盯着一个挂在"running"但 iframe 永远 refused 的死容器。
  const {
    data: attemptRemote,
    error: attemptErr,
  } = useAttempt(current?.attemptId, !!current?.attemptId)

  // 20s 心跳:告诉后端"用户还在",刷 LastActiveAt,避免 GC 把活着的容器当 idle 收走
  // hook 内部做了 tab 可见 + 最近交互的前置守卫,不活跃时静默
  useHeartbeat(current?.attemptId)

  // 判题结果弹窗状态
  const [checkInfo, setCheckInfo] = useState<{
    reply: CheckAttemptReply
    open: boolean
  } | null>(null)

  // 空闲警示 banner —— 第一次进入场景展示一次
  // 用 sessionStorage 防止每次切换都重弹
  const [showIdleTip, setShowIdleTip] = useState(() => {
    try {
      return window.sessionStorage.getItem('opslabs:idleTipSeen') !== '1'
    } catch {
      return true
    }
  })
  const dismissIdleTip = useCallback(() => {
    try {
      window.sessionStorage.setItem('opslabs:idleTipSeen', '1')
    } catch {
      /* ignore */
    }
    setShowIdleTip(false)
  }, [])

  // 本地 start 生命周期镜像(参见下方 useEffect 大段注释)
  const [startPhase, setStartPhase] = useState<'idle' | 'pending' | 'done' | 'error'>('idle')
  const [startErr, setStartErr] = useState<Error | null>(null)

  // 进入场景的 start 分派 —— 沿用上一轮 Task #18 的逻辑,结合 startingRef + phase 守卫
  const startingRef = useRef<string | null>(null)
  const fireStart = useCallback(
    (targetSlug: string) => {
      startingRef.current = targetSlug
      setStartPhase('pending')
      setStartErr(null)
      start.mutate(targetSlug, {
        onError: (err) => {
          setStartPhase('error')
          setStartErr(err instanceof Error ? err : new Error(String(err)))
        },
      })
    },
    // start 是 useMutation 返回对象,每次 render 都是新引用但内部稳定,
    // 这里不放依赖避免 callback 频繁重建
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  )

  useEffect(() => {
    if (!slug) return
    // ★★★ 去重守卫必须用同步 ref,不能用 startPhase state ★★★
    //
    // React 18 Strict Mode(dev)下 useEffect 会跑 effect→cleanup→effect 两轮,
    // 期间 setStartPhase('pending') 只是把 update 排队,state 在第二轮 effect 进来
    // 的时候还是 'idle'。守卫写成 phase 判定的话,第二轮会"以为没在跑"又发一次
    // mutation,两个 attempt 挤到后端,Docker 模式多起一个容器。
    //
    // 所以只用 ref 做重入守卫;fireStart 里第一行就把 ref 置成当前 slug,
    // 第二轮 effect 会命中这个守卫直接 return。error 态下用户点 retry 会走
    // onRetry→fireStart(slug),那条路径不经过这个 effect,不受守卫影响。
    if (startingRef.current === slug) return
    //
    // ───── 进入场景一律发一次 Start,让后端决定 reuse 还是 recreate ─────
    //
    // 以前这里走 localStorage 的 reusable 快路径:如果 store 里有同 slug
    // running/passed 的 attempt 就 skip start。问题是 Docker 容器会被外部
    // 操作干掉(docker rm、宿主 reboot、OOM…),前端无从察觉,Terminal iframe
    // 连 ttyd 直接 connect refused。现在把判定移回后端:Start 请求到 usecase,
    // usecase 先用 runner.Ping 校验容器真活着,活着就返回原 attempt,死了就
    // 新建,前端只负责幂等请求(startingRef 防 StrictMode 双发)。
    //
    // 切 slug 才清 store:同 slug 重入时保留老 current 给 UI 一个过渡态,
    // 新 Start 成功后 setCurrent 会覆盖。
    if (current && current.scenarioSlug !== slug) resetStore()
    fireStart(slug)
    // 依赖只放 slug,current 字段的变化由 fireStart/hook-level onSuccess 接管,
    // 不需要这里监听(否则每次 setCurrent 都重新跑一遍 effect,徒增噪声)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug])

  // store 同步信号
  useEffect(() => {
    if (startPhase !== 'pending') return
    if (current?.scenarioSlug === slug && current?.attemptId) {
      setStartPhase('done')
    }
  }, [startPhase, slug, current?.scenarioSlug, current?.attemptId])

  // 轮询结果更新 store
  //
  // ★ 走 patch 而不是 set:AttemptReply 里没有 expiresAt / idleTimeoutSeconds /
  //   passedGraceSeconds / reviewingUntil,直接 set 会把 Start 时落进 store 的
  //   这几个字段覆盖成 undefined —— 顶部倒计时依赖 startedAt + estimatedMinutes
  //   还稳,但 FinalTip 警示阈值(读 current.expiresAt)会失效,复盘剩余时间也丢。
  //   merge 语义只刷 server 权威字段(status / startedAt / lastActiveAt / ...),
  //   保留纯前端 / Start-only 的字段不被覆盖。
  useEffect(() => {
    if (attemptRemote) patchCurrent(attemptRemote)
  }, [attemptRemote, patchCurrent])

  // ★★★ 这里曾经有一段"卸载兜底 terminate"的代码,已永久删除 ★★★
  //
  // 历史:v1 为了防止"返回时 running 容器泄漏",useEffect 卸载时自动调 terminate。
  // 问题:用户点"返回" → Scenario 卸载 → terminate 发起(docker stop 需要秒级)
  //       用户立刻再次进入同一场景 → Start 命中 store 里还记着的 running 记录
  //       → Ping 成功但容器其实已经在 stop 中 → iframe 连 ttyd "refused"
  //
  // 现在靠以下四层保证"返回不泄漏":
  //   1. 用户显式"放弃"按钮 → onGiveUp → terminate (唯一主动销毁入口之一)
  //   2. GCServer idle 30min 未心跳 → 资源守护回收
  //   3. GCServer stale-container 周期扫 → 外部杀容器的自愈
  //   4. GetAttempt 轮询触发 Ping(后端 GetWithPing)→ 死容器 → NEEDS_RESTART → 前端清 store
  //
  // 这样"返回"恢复"暂时离开"语义,用户回到列表再点进来,容器照旧活着、做了一半的工作还在。
  // 详细见 backend/internal/biz/attempt/usecase.go 文件头的"容器销毁路径白名单"。

  // NEEDS_RESTART 自动兜底:轮询拿到 409 → 清 store + 重新 Start
  useEffect(() => {
    if (!attemptErr) return
    const reason = (attemptErr as ApiError)?.reason
    if (reason !== 'NEEDS_RESTART') return
    if (!slug) return
    console.warn('[scenario] attempt stale, auto restarting', reason)
    resetStore()
    // 重置 startingRef,让 fireStart 不被守卫拦住
    startingRef.current = null
    fireStart(slug)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attemptErr, slug])

  // ------------------------------------------------------------
  // 各 Runner 的 imperative handle
  // ------------------------------------------------------------
  const staticRunnerRef = useRef<StaticRunnerHandle | null>(null)
  const webContainerRunnerRef = useRef<WebContainerRunnerHandle | null>(null)

  // ------------------------------------------------------------
  // 倒计时:两种截止时间来回切
  // ------------------------------------------------------------
  //   - running          : deadline = attempt.startedAt + scenario.estimatedMinutes
  //                        ↑ 体验基线 —— 表达"推荐做题时长",和后端 GC(30min idle)解耦
  //                        超时后不会自动终止,badge 切"已超时 mm:ss"继续累计
  //   - passed + review  : deadline = current.reviewingUntil(本地 10 min 复盘)
  //   - 其它状态         : 不展示倒计时
  //
  // 为什么不用 current.expiresAt:那是后端给的"资源型超时"(idle 窗口),
  //   用户每次 check/heartbeat 会被刷新,数字会跳;体验上的"答题预估时长"
  //   应该固定在 startedAt + estimatedMinutes,让用户有稳定的时间感。
  const deadline = useMemo<string | undefined>(() => {
    if (!current) return undefined
    if (current.status === 'running') {
      const mins = scenario?.estimatedMinutes && scenario.estimatedMinutes > 0
        ? scenario.estimatedMinutes
        : 10 // registry 没填按 10min 兜底,和大多数场景量级接近
      const base = Date.parse(current.startedAt)
      if (!Number.isFinite(base)) return undefined
      return new Date(base + mins * 60_000).toISOString()
    }
    if (current.status === 'passed' && current.reviewingUntil) return current.reviewingUntil
    return undefined
  }, [current, scenario?.estimatedMinutes])

  // ------------------------------------------------------------
  // 接近结束的比例提醒:
  // ------------------------------------------------------------
  // 规则:
  //   - 剩余 <= max(场景 idle 总时长 * 0.2, 180s)、且 > 0 时显示
  //   - 场景 idle 越长,提醒阈值越大(60 分钟的场景会在最后 12 分钟提醒;
  //     5 分钟的场景会在最后 3 分钟提醒 —— 兜底 180s 防止过短场景没机会提醒)
  //   - 用户点关闭则本 attempt 不再弹(按 attemptId 维度记 session)
  //   - running 状态才弹;passed/review 不弹(已经通关,提醒没意义)
  //
  // 这里单独订阅 useCountdown 是因为 CountdownBadge 内部的值读不到外面,
  // 拿同一个 deadline 再读一次成本很低(useCountdown 里面 setInterval 是按 deadline 一份的)
  const remainingForWarning = useCountdown(
    current?.status === 'running' ? current?.expiresAt : undefined,
  )
  const warningThresholdSec = useMemo(() => {
    const idleSec = current?.idleTimeoutSeconds && current.idleTimeoutSeconds > 0
      ? current.idleTimeoutSeconds
      : 30 * 60
    return Math.max(Math.floor(idleSec * 0.2), 180)
  }, [current?.idleTimeoutSeconds])
  const [dismissedFinalTipFor, setDismissedFinalTipFor] = useState<string | null>(null)
  const showFinalTip =
    current?.status === 'running' &&
    !!current?.attemptId &&
    remainingForWarning > 0 &&
    remainingForWarning <= warningThresholdSec &&
    dismissedFinalTipFor !== current.attemptId

  // ------------------------------------------------------------
  // 到期自动行为
  // ------------------------------------------------------------
  //   - running 到期:
  //       ✗ 不再 auto-check
  //       ✗ 不再 60s 兜底 terminate
  //       ✓ 仅透传给 CountdownBadge 让它切"已超时 mm:ss"样式,用户继续做
  //       用户主动点"检查答案"仍正常判,点"放弃"仍 terminate。
  //       资源面:后端 GCServer 按 LastActiveAt + idle 30min 照常兜底,
  //       用户真走了容器还是会被清,只是前端层不再做额外销毁。
  //   - passed + review 到期:直接 terminate + 回列表(用户可能已走开)。
  //
  // 变更理由:把"预估时长"从"资源守护(可销毁)"降级为"体验提示(仅展示)"。
  // 以前到期立刻自动判 + 60s 切断,用户写得正起劲被一刀切很沮丧,
  // 资源浪费和用户沮丧两头不讨好;现在由 GC 兜底 + 前端只显示,干净。
  const runCheckRef = useRef<(opts?: { auto?: boolean }) => Promise<void>>(async () => {})

  const onExpire = useCallback(() => {
    const a = useAttemptStore.getState().current
    if (!a?.attemptId) return
    // 复盘倒计时到期 —— 用户已通关,纯超时清理(销毁路径 #3)
    if (a.status === 'passed') {
      terminate.mutate(a.attemptId, {
        onSuccess: (r) => {
          patchCurrent({ status: r.status, reviewingUntil: undefined })
          navigate('/')
        },
      })
      return
    }
    // running 到期:不做任何事,交给 CountdownBadge 切"已超时"展示
    // 用户仍可正常做题 / 提交;idle 窗口由后端 GC 兜底
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [navigate])

  // ------------------------------------------------------------
  // 检查 / 放弃 / 复盘 / 重开
  // ------------------------------------------------------------
  //
  // runCheck 是唯一一份"跑 check"的实现 —— 手动点按钮(onCheck)和到期自动
  // 判一次(onExpire)都经过这里。通过 opts.auto 标识差异化 PassModal 文案,
  // 让用户知道"这是系统到期自动判的,你还有机会操作"。
  const runCheck = async (opts?: { auto?: boolean }) => {
    if (!current?.attemptId) return
    const mode: ExecutionMode = current.executionMode ?? 'sandbox'

    let clientResult: ClientCheckResult | undefined
    if (mode !== 'sandbox') {
      const handle =
        mode === 'web-container' ? webContainerRunnerRef.current : staticRunnerRef.current
      if (!handle) {
        setCheckInfo({
          reply: {
            passed: false,
            message: '前端 Runner 尚未挂载,请稍候再试',
            checkCount: checkInfo?.reply.checkCount ?? 0,
          },
          open: true,
        })
        return
      }
      try {
        clientResult = await handle.triggerCheck()
      } catch (e) {
        setCheckInfo({
          reply: {
            passed: false,
            message: (e as Error).message || '前端判题失败',
            checkCount: checkInfo?.reply.checkCount ?? 0,
          },
          open: true,
        })
        return
      }
    }

    check.mutate(
      { id: current.attemptId, clientResult },
      {
        onSuccess: (r) => {
          const baseMsg = r.message ?? (r.passed ? '恭喜通关!' : '未通过,再接再厉')
          const finalMsg = opts?.auto
            ? `⏰ 容器已到期,系统自动判了一次 —— ${baseMsg}`
            : baseMsg
          setCheckInfo({ reply: { ...r, message: finalMsg }, open: true })
          if (r.passed) {
            patchCurrent({ status: 'passed' })
          }
        },
      },
    )
  }

  // 手动触发入口:按钮点击走这里,沿用外部 props 调用签名不变
  const onCheck = () => {
    void runCheck()
  }

  // 每次 render 把最新的 runCheck 写进 ref,给 onExpire 等稳定回调使用
  // —— 避免 onExpire 重新 memoize 触发 CountdownBadge 定时器重建
  runCheckRef.current = runCheck

  const onGiveUp = () => {
    if (!current?.attemptId) return
    terminate.mutate(current.attemptId, {
      onSuccess: (r) => {
        patchCurrent({ status: r.status })
      },
    })
  }

  // PassModal 按钮路由
  const onPassModalAction = (action: PassModalAction) => {
    // 注:曾经这里有 clearAutoExpireTimer(),配套 running 到期的 60s 兜底 terminate;
    // 现已取消(见 onExpire 注释),用户在 PassModal 上的操作只负责语义分发,
    // 不再需要阻断定时器。保留这条注释方便回溯。

    // 先关弹窗再执行动作 —— 避免用户等 API 响应期间 modal 一直挡着
    setCheckInfo((prev) => (prev ? { ...prev, open: false } : prev))
    const a = useAttemptStore.getState().current
    switch (action) {
      case 'close':
        // 失败路径:保持容器继续做
        break
      case 'enterReview': {
        // 通关后进入复盘:不 terminate 容器,纯前端写一个复盘截止时间
        // 到期后 onExpire 会触发 terminate + 回列表
        if (!a?.attemptId) return
        const until = new Date(Date.now() + DEFAULT_REVIEW_MS).toISOString()
        patchCurrent({ reviewingUntil: until })
        break
      }
      case 'restart': {
        // 重新来一遍:先 terminate 旧容器 (后端)+ 清 store + 触发 start
        if (!slug) return
        const attemptId = a?.attemptId
        if (attemptId) {
          terminate.mutate(attemptId)
        }
        resetStore()
        fireStart(slug)
        break
      }
      case 'giveUp': {
        if (a?.attemptId) {
          terminate.mutate(a.attemptId, {
            onSuccess: (r) => patchCurrent({ status: r.status }),
          })
        }
        break
      }
      case 'backHome':
        // 通关后返回:容器保留,让后端 PassedGrace 自然清理。不 terminate。
        break
    }
  }

  // ------------------------------------------------------------
  // render
  // ------------------------------------------------------------
  if (isLoading) return <div className="p-8 text-slate-400">加载场景…</div>
  if (error || !scenario) {
    return (
      <div className="p-8 text-rose-600">
        场景加载失败:{(error as Error)?.message ?? 'not found'}
      </div>
    )
  }

  const isFinished = !!current?.status && current.status !== 'running' && current.status !== 'passed'
  const execMode: ExecutionMode =
    current?.executionMode ?? scenario.executionMode ?? 'sandbox'

  // 顶部栏右侧的倒计时 label:running 显示"剩余",复盘显示"复盘剩余"
  const countdownLabel =
    current?.status === 'passed' && current.reviewingUntil ? '复盘剩余' : '剩余'

  // 空闲超时分钟数,用于 banner 文案。store 没有明确字段时按 30min 兜底
  const idleMinutes = current?.idleTimeoutSeconds
    ? Math.floor(current.idleTimeoutSeconds / 60)
    : 30

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* ==================== 顶部栏 ==================== */}
      <header className="h-12 shrink-0 border-b border-slate-200 bg-white flex items-center px-4 gap-3">
        <button
          onClick={() => navigate('/')}
          className="inline-flex items-center gap-1.5 h-8 px-2 rounded text-slate-600 hover:bg-slate-100 transition text-sm"
          title="返回场景列表"
        >
          <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
            <path
              fillRule="evenodd"
              d="M12.79 5.23a.75.75 0 010 1.06L9.06 10l3.73 3.71a.75.75 0 11-1.06 1.06l-4.26-4.24a.75.75 0 010-1.06l4.26-4.24a.75.75 0 011.06 0z"
              clipRule="evenodd"
            />
          </svg>
          <span>返回</span>
        </button>

        <div className="h-5 w-px bg-slate-200" />

        <div className="min-w-0 flex-1 flex items-center gap-2">
          <h1 className="text-sm font-medium text-slate-800 truncate">{scenario.title}</h1>
          <span className="text-[11px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-500 font-mono">
            {execMode}
          </span>
        </div>

        {/* 倒计时徽章:status 命中时自动出现 */}
        {/* running 时 allowOvertime=true,超出预估时长切"已超时 mm:ss"不终止;
            复盘阶段保持老行为,到时间走 onExpire → terminate → 回列表 */}
        <CountdownBadge
          deadline={deadline}
          label={countdownLabel}
          onExpire={onExpire}
          allowOvertime={current?.status === 'running'}
        />
      </header>

      {/* 空闲超时提示 banner —— 关闭一次后本 tab 不再弹 */}
      {showIdleTip && current?.status === 'running' && execMode === 'sandbox' && (
        <div className="flex items-center gap-2 px-4 py-2 bg-amber-50 border-b border-amber-200 text-xs text-amber-800">
          <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4 shrink-0">
            <path
              fillRule="evenodd"
              d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 6a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 6zm0 9a1 1 0 100-2 1 1 0 000 2z"
              clipRule="evenodd"
            />
          </svg>
          <span>
            容器将在 {idleMinutes} 分钟无操作后自动销毁,届时数据会丢失,需要手动重新开始。
          </span>
          <button
            onClick={dismissIdleTip}
            className="ml-auto text-amber-700 hover:text-amber-900 underline-offset-2 hover:underline"
          >
            我知道了
          </button>
        </div>
      )}

      {/* 接近结束 banner:按场景时长的比例触发,提醒用户快到期、可立刻提交 */}
      {showFinalTip && (
        <div className="flex items-center gap-2 px-4 py-2 bg-rose-50 border-b border-rose-200 text-xs text-rose-800">
          <span className="inline-block w-1.5 h-1.5 rounded-full bg-rose-500 animate-pulse" />
          <span>
            剩余不到 {Math.ceil(remainingForWarning / 60)} 分钟 —— 到时会自动销毁容器,
            建议尽快点"检查答案"提交,或立刻主动结束。
          </span>
          <button
            onClick={onCheck}
            className="ml-auto px-2 h-6 rounded bg-rose-600 text-white hover:bg-rose-700 text-[11px]"
          >
            立刻提交
          </button>
          <button
            onClick={() => current?.attemptId && setDismissedFinalTipFor(current.attemptId)}
            className="text-rose-600 hover:text-rose-800 underline-offset-2 hover:underline"
          >
            稍后再说
          </button>
        </div>
      )}

      {/* 复盘模式 banner:passed + reviewingUntil 有值时提醒用户容器仍在 */}
      {current?.status === 'passed' && current.reviewingUntil && (
        <div className="flex items-center gap-2 px-4 py-2 bg-emerald-50 border-b border-emerald-200 text-xs text-emerald-800">
          <span className="inline-block w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
          <span>已通关 · 正在复盘,容器会保留到计时结束后自动销毁。</span>
          <button
            onClick={() => {
              if (!current.attemptId) return
              terminate.mutate(current.attemptId, {
                onSuccess: (r) => {
                  patchCurrent({ status: r.status, reviewingUntil: undefined })
                  navigate('/')
                },
              })
            }}
            className="ml-auto underline underline-offset-2 text-emerald-700 hover:text-emerald-900"
          >
            立即结束并返回
          </button>
        </div>
      )}

      {/* ==================== 主体 ==================== */}
      <div className="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-[22rem_1fr] gap-0 overflow-hidden">
        <aside className="border-r border-slate-200 bg-white min-h-0 overflow-y-auto scroll-thin">
          <ScenarioMeta scenario={scenario} />
        </aside>
        <section className="min-h-0 flex flex-col overflow-hidden">
          <div className="flex-1 min-h-0 bg-slate-900 overflow-hidden">
            {renderByExecutionMode({
              mode: execMode,
              attemptId: current?.attemptId,
              terminalUrl: current?.terminalUrl,
              bundleUrl: current?.bundleUrl,
              startPending: startPhase === 'pending',
              startError: startErr,
              onRetry: () => slug && fireStart(slug),
              staticRunnerRef,
              webContainerRunnerRef,
              scenarioTitle: scenario.title,
              // 用 summary 的第一句作为 tagline(短简介不适合 splash 长描述)
              scenarioTagline: firstSentence(scenario.summary),
              // slug 带入 BootSplash,命中 STAGES_BY_SLUG 时显示场景专属阶段文案
              scenarioSlug: slug,
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
        <PassModal reply={checkInfo.reply} onAction={onPassModalAction} />
      )}
    </div>
  )
}

// ------------------------------------------------------------
// renderByExecutionMode
// ------------------------------------------------------------
// 四模式共用同一个 Runner 容器结构:
//   startError → 错误卡片(带 retry);startPending/无 attempt → BootSplash
//   都就绪 → 模式专属的 Runner 组件
function renderByExecutionMode(p: {
  mode: ExecutionMode
  attemptId?: string
  terminalUrl?: string
  bundleUrl?: string
  startPending: boolean
  startError?: Error | null
  onRetry?: () => void
  staticRunnerRef: MutableRefObject<StaticRunnerHandle | null>
  webContainerRunnerRef: MutableRefObject<WebContainerRunnerHandle | null>
  scenarioTitle?: string
  scenarioTagline?: string
  scenarioSlug?: string
}) {
  // 启动态:交给 BootSplash(含 error 分支)
  if (p.startError || p.startPending || !p.attemptId) {
    return (
      <BootSplash
        mode={p.mode}
        error={p.startError}
        onRetry={p.onRetry}
        scenarioTitle={p.scenarioTitle}
        scenarioTagline={p.scenarioTagline}
        scenarioSlug={p.scenarioSlug}
      />
    )
  }

  switch (p.mode) {
    case 'sandbox':
      return renderSandboxRunner(p.attemptId, p.terminalUrl)
    case 'static':
    case 'wasm-linux':
      return renderIframeBundleRunner(p.bundleUrl, p.mode, p.staticRunnerRef)
    case 'web-container':
      return renderWebContainerRunner(p.bundleUrl, p.webContainerRunnerRef)
    default: {
      const _exhaustive: never = p.mode
      return (
        <div className="h-full grid place-items-center text-rose-400 text-sm">
          unknown executionMode: {String(_exhaustive)}
        </div>
      )
    }
  }
}

function renderSandboxRunner(attemptId: string, terminalUrl?: string) {
  if (!terminalUrl) {
    // Mock 预览模式
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-slate-800/70 border border-slate-700 rounded-lg p-5 text-slate-200 text-sm leading-relaxed">
          <div className="text-base font-medium text-amber-300 mb-2">Mock 预览模式</div>
          <div className="text-slate-300">
            后端运行在 <code className="px-1 rounded bg-slate-900/80">driver: mock</code>,
            不会启动真实容器,也就没有终端可连。
          </div>
          <div className="mt-2 text-slate-400">
            想看真实终端,请把 <code>backend/configs/config.yaml</code> 里
            <code className="mx-1 px-1 rounded bg-slate-900/80">runtime.driver</code>
            改成 <code>docker</code>,再确保 Docker Desktop 在跑。
          </div>
          <div className="mt-3 text-slate-500 text-xs">attemptId = {attemptId}</div>
        </div>
      </div>
    )
  }
  return <Terminal src={terminalUrl} />
}

function renderIframeBundleRunner(
  bundleUrl: string | undefined,
  mode: 'static' | 'wasm-linux',
  runnerRef: MutableRefObject<StaticRunnerHandle | null>,
) {
  if (!bundleUrl) {
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-amber-900/30 border border-amber-700 rounded-lg p-5 text-slate-100 text-sm">
          <div className="text-base font-medium text-amber-300 mb-2">bundle_url 为空</div>
          <div className="text-slate-300">
            场景 execution_mode 为 {mode} 但后端没下发 bundle_url。检查
            <code className="mx-1 px-1 rounded bg-slate-900/80">internal/scenario/registry.go</code>
            是否注册了 slug,以及对应 bundles 目录下是否有 index.html。
          </div>
        </div>
      </div>
    )
  }
  return <StaticRunner ref={runnerRef} bundleUrl={bundleUrl} />
}

function renderWebContainerRunner(
  bundleUrl: string | undefined,
  runnerRef: MutableRefObject<WebContainerRunnerHandle | null>,
) {
  if (!bundleUrl) {
    return (
      <div className="h-full grid place-items-center p-6">
        <div className="max-w-md bg-amber-900/30 border border-amber-700 rounded-lg p-5 text-slate-100 text-sm">
          <div className="text-base font-medium text-amber-300 mb-2">bundle_url 为空</div>
          <div className="text-slate-300">
            web-container 模式需要后端下发 project.json URL,当前为空。
          </div>
        </div>
      </div>
    )
  }
  return <WebContainerRunner ref={runnerRef} bundleUrl={bundleUrl} />
}

// firstSentence 抽描述的第一句作为 BootSplash 底部 tagline
//   - 先按中文句号 / 英文句号 / 感叹号 / 换行 截断
//   - 不截时如果字数过长(>48 字)给省略号,避免撑破窄面板
//   - 空串 / undefined 直接返回 undefined,让 BootSplash 走默认文案
function firstSentence(desc?: string): string | undefined {
  if (!desc) return undefined
  const s = desc.trim()
  if (!s) return undefined
  const m = s.match(/^[^。.!?!?\n]+/)
  let out = (m ? m[0] : s).trim()
  if (out.length > 48) out = out.slice(0, 46) + '…'
  return out
}
