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

export async function request<T>(
  path: string,
  init?: RequestInit & { json?: unknown },
): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('accept', 'application/json')

  let body = init?.body
  if (init?.json !== undefined) {
    headers.set('content-type', 'application/json')
    body = JSON.stringify(init.json)
  }

  const res = await fetch(path, { ...init, headers, body })
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
