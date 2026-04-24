/**
 * clientId: 未登录阶段的"匿名 owner"标识
 *
 * 流程:
 *   - 首次进入站点生成一个 UUID,写入 localStorage('opslabs:clientId')
 *   - 之后每次请求读这个 UUID,通过 X-Client-ID header 送到后端
 *   - 后端 middleware 塞到 ctx 里,AttemptStore 按 (clientID, slug) 找"我的"attempt
 *
 * 为什么不放在 http.ts:
 *   - clientId 的值未来还会被 heartbeat hook / 埋点等非 request 流程用到
 *   - 单独一个模块方便 Scenario 页在 console 上肉眼打印排查
 *
 * 注意:
 *   - localStorage 被用户清掉 / 换浏览器会拿到新 UUID,等于"新的匿名用户"
 *   - 这对用户没损失 —— 后端会按新 clientID 给一条新 attempt,不会串号
 *   - 登录接入后仍然保留 clientID,和 userID 共存做"多设备区分"
 */

const STORAGE_KEY = 'opslabs:clientId'

// 生成一个 RFC4122 v4 UUID,优先走 crypto.randomUUID,老浏览器降级到 Math.random
function genUUID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  // 降级:用 Math.random 拼 v4 —— 仅 fallback 用,v4 quality 足够做匿名 owner id
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

let cached: string | null = null

/**
 * getClientId 读取 / 初始化 clientId
 *
 * - 命中 localStorage: 直接返回(热路径,请求前调用零开销)
 * - 未命中: 生成一个新 UUID,写回 localStorage,返回
 * - localStorage 被禁用(Safari 隐私模式、SSR): 生成内存版本,session 级别有效
 */
export function getClientId(): string {
  if (cached) return cached
  try {
    const v = window.localStorage.getItem(STORAGE_KEY)
    if (v) {
      cached = v
      return v
    }
    const next = genUUID()
    window.localStorage.setItem(STORAGE_KEY, next)
    cached = next
    return next
  } catch {
    // localStorage 不可用 —— 内存缓存,刷新页面会换新 id(能接受)
    if (!cached) cached = genUUID()
    return cached
  }
}
