import { useMutation, useQuery } from '@tanstack/react-query'
import { request } from './http'
import {
  Attempt,
  CheckAttemptReply,
  ClientCheckResult,
  StartScenarioReply,
  TerminateAttemptReply,
} from '../types'
import { useAttemptStore } from '../store/useAttemptStore'

// StartScenario 的返回是精简对象,页面层再合并出完整 Attempt
//
// **关键设计决策**:store 写入放在 hook-level `onSuccess`(下方),而不是 call-site 的
// `mutate(vars, {onSuccess})`。原因是 React 18 Strict Mode 下 call-site 回调会
// 被 observer 重建吞掉(实测过),只有挂在 MutationCache 里的 Mutation 实例上的
// hook-level 回调能稳定触发 —— 即使 observer 换了一个新的,Mutation 本身走完
// 生命周期时一定会把 hook-level 回调跑完。
//
// hook-level 回调直接读 zustand,看似把 API 层和 store 耦合了,但对于这种
// "请求成功就必须落一份到 store" 的强关联场景,比绕 ref/context 干净得多。
export function useStartAttempt() {
  // useAttemptStore.getState() 每次调用拿当前值,不订阅 —— 避免 hook 层产生多余渲染
  return useMutation({
    mutationFn: async (slug: string) => {
      console.log('[opslabs/start] mutationFn enter slug=', slug)
      let r: any
      try {
        r = await request<any>(`/v1/scenarios/${slug}/start`, {
          method: 'POST',
          json: {},
        })
      } catch (e) {
        console.error('[opslabs/start] request threw', e)
        throw e
      }
      console.log('[opslabs/start] request resolved keys=', r && Object.keys(r), 'r=', r)
      const reply: StartScenarioReply = {
        attemptId: r?.attemptId || r?.attempt_id,
        terminalUrl: r?.terminalUrl || r?.terminal_url,
        expiresAt: r?.expiresAt || r?.expires_at,
        executionMode: r?.executionMode || r?.execution_mode,
        bundleUrl: r?.bundleUrl || r?.bundle_url,
      }
      console.log('[opslabs/start] mutationFn return', reply)
      return reply
    },
    onMutate: (slug) => {
      console.log('[opslabs/start] onMutate slug=', slug)
    },
    // hook-level onSuccess:直接把结果落进 zustand。
    // 这是唯一能跨 Strict Mode observer 重建稳定触发的写入点
    onSuccess: (data, slug) => {
      console.log('[opslabs/start] onSuccess slug=', slug, data)
      if (!data?.attemptId) {
        console.error('[opslabs/start] reply missing attemptId', data)
        return
      }
      // ───── 复用判定:同 attemptId + 同 slug 视为"后端 reuse 分支" ─────
      //
      // 背景:刷新 / 重新进入场景时 Scenario.tsx 一律会再发一次 Start,
      // 后端走 reuse 分支返回 **相同** attemptId 的同一条 attempt。
      // 老写法无脑把 startedAt 写成 now,导致:
      //   - 顶部倒计时每次刷新都"回到满",用户观感"计时器被重置"
      //   - localStorage 里持久化的老 startedAt 被活活覆盖掉
      //
      // 修复:在 onSuccess 里先读 store 当前值,如果 attemptId / scenarioSlug
      // 都对得上,就保留原 startedAt(单调真源),只刷新 lastActiveAt / expiresAt
      // 这类"后端这次 UpdateLastActive 之后的新值"。
      //
      // 不走后端 Reply 带 started_at:StartScenarioReply 刻意没有这字段
      //   (proto 注释里写明"避开 regen 依赖"),GetAttempt 轮询会在几百 ms 内
      //   把权威 startedAt 补回来;这里只需要保证"闪烁期"的 startedAt 不抖。
      const prev = useAttemptStore.getState().current
      const isReuse =
        !!prev?.attemptId &&
        prev.attemptId === data.attemptId &&
        prev.scenarioSlug === slug
      const now = new Date().toISOString()
      const startedAt = isReuse && prev?.startedAt ? prev.startedAt : now
      useAttemptStore.getState().set({
        attemptId: data.attemptId,
        scenarioSlug: slug,
        status: 'running',
        terminalUrl: data.terminalUrl || '',
        startedAt,
        // 后端 reuse 会 UpdateLastActive(now),前端对齐刷新成 now;
        // 新建也自然是 now。两条路径同值,不需要分叉。
        lastActiveAt: now,
        executionMode: data.executionMode,
        bundleUrl: data.bundleUrl,
        // expiresAt 直接灌进 store —— Scenario 顶部倒计时依赖这个字段
        // 后端 proto 未在 AttemptReply 里带 expires_at,所以 polling 不刷新它,
        // Start 这一次的值就是 running 阶段的基准(idle 期间不会自动刷新倒计时,
        // 用户每次 check 后 lastActiveAt 推进会由 check mutation 的上层负责对齐)
        expiresAt: data.expiresAt,
        // 复用时保留 idleTimeoutSeconds 等上一轮 GetAttempt 填充过的字段,
        // 否则 CheckAttempt 的外推 expiresAt / Scenario 的警示阈值会退化到默认
        ...(isReuse
          ? {
              idleTimeoutSeconds: prev?.idleTimeoutSeconds,
              passedGraceSeconds: prev?.passedGraceSeconds,
              reviewingUntil: prev?.reviewingUntil,
            }
          : {}),
      })
      console.log(
        '[opslabs/start] store written attemptId=',
        data.attemptId,
        'isReuse=',
        isReuse,
        'startedAt=',
        startedAt,
      )
    },
    onError: (err) => {
      console.error('[opslabs/start] onError', err)
    },
    onSettled: (data, err) => {
      console.log('[opslabs/start] onSettled data=', data, 'err=', err)
    },
  })
}

// 查一次 attempt(30s 轮询充当心跳)
export function useAttempt(id: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ['attempt', id],
    queryFn: async () => {
      const r = await request<any>(`/v1/attempts/${id}`)
      return {
        attemptId: r.attemptId || r.attempt_id,
        scenarioSlug: r.scenarioSlug || r.scenario_slug,
        status: r.status,
        terminalUrl: r.terminalUrl || r.terminal_url,
        startedAt: r.startedAt || r.started_at,
        lastActiveAt: r.lastActiveAt || r.last_active_at,
        executionMode: r.executionMode || r.execution_mode,
        bundleUrl: r.bundleUrl || r.bundle_url,
      } as Attempt
    },
    enabled: enabled && !!id,
    refetchInterval: 30_000,
  })
}

// useCheckAttempt 触发一次判题
//
// 非 sandbox 模式(static / wasm-linux / web-container)必须传 clientResult —— 后端
// 对这种模式下 client_result 缺失会直接 400。sandbox 模式可以不传,后端忽略。
//
// mutate 参数改为对象 { id, clientResult? },相比原来的裸 string id,调用方更清楚。
export interface CheckAttemptVars {
  id: string
  clientResult?: ClientCheckResult
}

export function useCheckAttempt() {
  return useMutation({
    mutationFn: async (vars: CheckAttemptVars) => {
      // 只有 clientResult 存在才放进 body,让 sandbox 请求体保持最小
      const body: Record<string, unknown> = {}
      if (vars.clientResult) {
        body.clientResult = {
          passed: vars.clientResult.passed,
          exitCode: vars.clientResult.exitCode,
          stdout: vars.clientResult.stdout || '',
          stderr: vars.clientResult.stderr || '',
        }
      }
      const r = await request<any>(`/v1/attempts/${vars.id}/check`, {
        method: 'POST',
        json: body,
      })
      return {
        passed: r.passed,
        message: r.message,
        durationSeconds: r.durationSeconds || r.duration_seconds,
        checkCount: r.checkCount || r.check_count,
      } as CheckAttemptReply
    },
    // 每次成功 check 后客户端推断 expiresAt = now + idleMinutes,
    // 对齐后端 checkSandbox 里 LastActiveAt = now 的刷新逻辑。
    // 后端没在 reply 里回 expiresAt,所以这里默认 30min(和 service.DefaultIdleTimeout 一致)。
    // 未来 AttemptReply 加了 expires_at 字段,直接用服务端值替代这段外推。
    onSuccess: () => {
      const a = useAttemptStore.getState().current
      if (!a?.attemptId) return
      const idleSec = a.idleTimeoutSeconds && a.idleTimeoutSeconds > 0
        ? a.idleTimeoutSeconds
        : 30 * 60
      const newExpires = new Date(Date.now() + idleSec * 1000).toISOString()
      useAttemptStore.getState().patch({
        lastActiveAt: new Date().toISOString(),
        expiresAt: newExpires,
      })
    },
  })
}

export function useTerminateAttempt() {
  return useMutation({
    mutationFn: async (id: string) => {
      const r = await request<any>(`/v1/attempts/${id}/terminate`, {
        method: 'POST',
        json: {},
      })
      return {
        status: r.status,
      } as TerminateAttemptReply
    },
  })
}
