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
    mutationFn: (slug: string) =>
      request<StartScenarioReply>(`/v1/scenarios/${slug}/start`, {
        method: 'POST',
        json: {},
      }),
  })
}

// 查一次 attempt(30s 轮询充当心跳)
export function useAttempt(id: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ['attempt', id],
    queryFn: () => request<Attempt>(`/v1/attempts/${id}`),
    enabled: enabled && !!id,
    refetchInterval: 30_000,
  })
}

export function useCheckAttempt() {
  return useMutation({
    mutationFn: (id: string) =>
      request<CheckAttemptReply>(`/v1/attempts/${id}/check`, {
        method: 'POST',
        json: {},
      }),
  })
}

export function useTerminateAttempt() {
  return useMutation({
    mutationFn: (id: string) =>
      request<TerminateAttemptReply>(`/v1/attempts/${id}/terminate`, {
        method: 'POST',
        json: {},
      }),
  })
}
