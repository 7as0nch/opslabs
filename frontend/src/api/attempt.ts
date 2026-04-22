import { useMutation, useQuery } from '@tanstack/react-query'
import { request } from './http'
import {
  Attempt,
  CheckAttemptReply,
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
      const now = new Date().toISOString()
      useAttemptStore.getState().set({
        attemptId: data.attemptId,
        scenarioSlug: slug,
        status: 'running',
        terminalUrl: data.terminalUrl || '',
        startedAt: now,
        lastActiveAt: now,
      })
      console.log('[opslabs/start] store written attemptId=', data.attemptId)
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
      } as Attempt
    },
    enabled: enabled && !!id,
    refetchInterval: 30_000,
  })
}

export function useCheckAttempt() {
  return useMutation({
    mutationFn: async (id: string) => {
      const r = await request<any>(`/v1/attempts/${id}/check`, {
        method: 'POST',
        json: {},
      })
      return {
        passed: r.passed,
        message: r.message,
        durationSeconds: r.durationSeconds || r.duration_seconds,
        checkCount: r.checkCount || r.check_count,
      } as CheckAttemptReply
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
