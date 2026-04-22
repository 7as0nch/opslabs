// 极简 fetch 封装
// 后端约定(Kratos 自定义中间件封装):
//   成功:{ code: 200, data: <真正的业务 body>, msg: "...", newToken: "", datetime: "..." }
//   失败:{ code: 4xx/5xx, reason: "XXX", message: "..." }  (Kratos errors 默认格式)
// 本函数统一:成功返回 data 字段,失败抛 ApiError

export interface ApiError extends Error {
  code: number
  reason: string
}

interface Envelope {
  code?: number
  data?: unknown
  msg?: string
  newToken?: string
  datetime?: string
  // 失败时出现
  reason?: string
  message?: string
}

// 默认 30s 超时 —— 后端 Start 场景涉及 docker run + DB 写入,15s 一般够,
// 留点冗余给 DB 抖动。超时会抛 TimeoutError,React Query 的 onError 能捕获
const DEFAULT_TIMEOUT_MS = 30_000

export async function request<T>(
  path: string,
  init?: RequestInit & { json?: unknown; timeoutMs?: number },
): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('accept', 'application/json')

  let body = init?.body
  if (init?.json !== undefined) {
    headers.set('content-type', 'application/json')
    body = JSON.stringify(init.json)
  }

  // AbortController 给请求加超时 —— fetch 自身没有超时语义
  const ctrl = new AbortController()
  const timer = setTimeout(
    () => ctrl.abort(new DOMException('request timeout', 'TimeoutError')),
    init?.timeoutMs ?? DEFAULT_TIMEOUT_MS,
  )
  let res: Response
  try {
    res = await fetch(path, { ...init, headers, body, signal: ctrl.signal })
  } catch (e) {
    clearTimeout(timer)
    if ((e as Error)?.name === 'TimeoutError' || (e as Error)?.name === 'AbortError') {
      const err = new Error('请求超时,请重试或检查后端是否存活') as ApiError
      err.code = 504
      err.reason = 'REQUEST_TIMEOUT'
      throw err
    }
    throw e
  }
  clearTimeout(timer)
  const text = await res.text()
  const parsed = text ? (safeParse(text) as Envelope | undefined) : undefined

  // HTTP 层失败:直接按 kratos errors 字段抛
  if (!res.ok) {
    const err = new Error(
      parsed?.message || parsed?.msg || res.statusText || 'request failed',
    ) as ApiError
    err.code = parsed?.code ?? res.status
    err.reason = parsed?.reason ?? 'UNKNOWN'
    throw err
  }

  // HTTP 200:可能是信封格式,也可能是直接 body
  if (isEnvelope(parsed)) {
    // 信封里 code 非 200 也当失败
    if (parsed.code !== undefined && parsed.code !== 200 && parsed.code !== 0) {
      const err = new Error(parsed.msg || parsed.message || 'request failed') as ApiError
      err.code = parsed.code
      err.reason = parsed.reason ?? 'UNKNOWN'
      throw err
    }
    return (parsed.data ?? {}) as T
  }
  return parsed as T
}

// 判断是否是后端统一信封。有 code + data/msg 字段才视为信封
function isEnvelope(v: unknown): v is Envelope {
  if (!v || typeof v !== 'object') return false
  const obj = v as Record<string, unknown>
  return 'code' in obj && ('data' in obj || 'msg' in obj)
}

function safeParse(t: string): unknown {
  try {
    return JSON.parse(t)
  } catch {
    return undefined
  }
}
