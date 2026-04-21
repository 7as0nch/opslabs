// 与 backend/api/opslabs/v1/opslabs.proto 对齐
// protojson 默认输出 lowerCamelCase(例: attempt_id -> attemptId)

// ---------- Scenario ----------

export interface ScenarioBrief {
  slug: string
  title: string
  summary: string
  category: string
  difficulty: number
  estimatedMinutes: number
  targetPersonas?: string[]
  techStack?: string[]
  tags?: string[]
  isPremium?: boolean
}

export interface ScenarioDetail {
  slug: string
  version?: string
  title: string
  summary: string
  descriptionMd?: string
  category: string
  difficulty: number
  estimatedMinutes: number
  targetPersonas?: string[]
  experienceLevel?: string
  techStack?: string[]
  skills?: string[]
  commands?: string[]
  tags?: string[]
  hints?: ScenarioHint[]
  isPremium?: boolean
}

export interface ScenarioHint {
  level: number
  unlocked?: boolean
  content?: string
}

export interface ListScenariosReply {
  scenarios: ScenarioBrief[]
  total: number
}

export interface ScenarioReply {
  scenario: ScenarioDetail
}

// ---------- Attempt ----------

export type AttemptStatus =
  | 'running'
  | 'passed'
  | 'expired'
  | 'terminated'
  | 'failed'

// AttemptReply 与 proto 对齐(attempt_id 是 string,前端统一当 id 用)
export interface Attempt {
  attemptId: string
  scenarioSlug: string
  status: AttemptStatus
  terminalUrl?: string
  startedAt: string
  lastActiveAt: string
}

export interface StartScenarioReply {
  attemptId: string
  terminalUrl: string
  expiresAt: string
}

export interface CheckAttemptReply {
  passed: boolean
  message: string
  durationSeconds?: number
  checkCount: number
}

export interface TerminateAttemptReply {
  status: AttemptStatus
}
