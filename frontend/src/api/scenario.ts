import { useQuery } from '@tanstack/react-query'
import { request } from './http'
import { ListScenariosReply, ScenarioBrief, ScenarioDetail, ScenarioReply } from '../types'

// http path 对照 backend/api/opslabs/v1/opslabs.proto

export function useScenarios() {
  return useQuery({
    queryKey: ['scenarios'],
    queryFn: () => request<ListScenariosReply>('/v1/scenarios'),
    select: (d): ScenarioBrief[] => d.scenarios ?? [],
  })
}

export function useScenario(slug: string | undefined) {
  return useQuery({
    queryKey: ['scenario', slug],
    queryFn: () => request<ScenarioReply>(`/v1/scenarios/${slug}`),
    enabled: !!slug,
    select: (d): ScenarioDetail => d.scenario,
  })
}
