// 与 backend/api/opslabs/v1/opslabs.proto 对齐
// protojson 默认输出 lowerCamelCase(例: attempt_id -> attemptId)

// ---------- Scenario ----------

// ExecutionMode 场景执行模式(V1 预留字段)
//   - sandbox      : 后端 Docker 容器 + ttyd,iframe 挂终端(V1 唯一实现)
//   - static       : 纯前端题,无后端运行时
//   - wasm-linux   : 前端 CheerpX / v86 跑 wasm Linux
//   - web-container: 前端 StackBlitz WebContainer
//
// V1 阶段后端 proto 里还没把这个字段下发,前端看到 undefined 时按 sandbox 处理;
// 等 V2 proto 打通后,相同的消费逻辑不用改。
export type ExecutionMode = 'sandbox' | 'static' | 'wasm-linux' | 'web-container'

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
  executionMode?: ExecutionMode
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
  executionMode?: ExecutionMode
  // bundleUrl 仅非 sandbox 模式有值,Runner 里用它加载 iframe
  bundleUrl?: string
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
//
// expiresAt / idleTimeoutSeconds / passedGraceSeconds 目前后端 AttemptReply 未下发,
// 前端把 StartScenarioReply 的 expiresAt 和默认 30min idle / 10min grace 写入 store,
// 后续从这里读。proto regen 之后补齐服务端字段即可无感切换。
export interface Attempt {
  attemptId: string
  scenarioSlug: string
  status: AttemptStatus
  terminalUrl?: string
  startedAt: string
  lastActiveAt: string
  executionMode?: ExecutionMode
  bundleUrl?: string
  expiresAt?: string
  idleTimeoutSeconds?: number
  passedGraceSeconds?: number
  // reviewingUntil:进入复盘后的纯前端倒计时基准(ISO)
  //   passed 之后用户按"进入复盘",前端写此字段到 store,CountdownBadge
  //   在 status=passed 时读它来显示复盘剩余时间。后端不感知这个字段。
  reviewingUntil?: string
}

export interface StartScenarioReply {
  attemptId: string
  terminalUrl: string
  expiresAt: string
  executionMode?: ExecutionMode
  bundleUrl?: string
}

// ClientCheckResult 非 sandbox 模式下前端 Runner 上报给后端的判题结果
// 字段与 proto 里的 ClientCheckResult 对齐
export interface ClientCheckResult {
  passed: boolean
  exitCode: number
  stdout?: string
  stderr?: string
}

// CheckAttemptRequest body;sandbox 模式下 clientResult 可省略
export interface CheckAttemptRequest {
  clientResult?: ClientCheckResult
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
