import { useMutation, useQuery } from '@tanstack/react-query'
import { request } from './http'
import {
  Attempt,
  CheckAttemptReply,
  StartScenarioReply,
  TerminateAttemptReply,
} from '../types'

// StartScenario 的返回是精简对象,页面层再合并出完整 Attempt
export function useStartAttempt() {
  return useMutation({
    mutationFn: async (slug: string) => {
      const r = await request<any>(`/v1/scenarios/${slug}/start`, {
        method: 'POST',
        json: {},
      })
      return {
        attemptId: r.attemptId || r.attempt_id,
        terminalUrl: r.terminalUrl || r.terminal_url,
        expiresAt: r.expiresAt || r.expires_at,
      } as StartScenarioReply
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
