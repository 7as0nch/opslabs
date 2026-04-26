import { getClientId } from '../lib/clientId'

// submitFeedback:最简反馈上报,绕开 api/http.ts 的信封约束
//
// 后端 /v1/feedback 走 srv.HandleFunc 直接注册,不走 proto,
// 响应是朴素的 {ok:true} / {ok:false,error:"..."},不是 kratos 信封。
// 用户侧只关心"成功/失败 + 简短提示",因此这里手写极简 fetch,不引入复杂错误分类。
export interface FeedbackPayload {
  text: string
  scenarioSlug?: string
  attemptId?: string
  /** 1..5 星;0 或 undefined 表示未评分 */
  rating?: number
}

export async function submitFeedback(payload: FeedbackPayload): Promise<void> {
  const resp = await fetch('/v1/feedback', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      accept: 'application/json',
      'X-Client-ID': getClientId(),
    },
    body: JSON.stringify(payload),
  })
  if (!resp.ok) {
    let msg = resp.statusText || 'submit failed'
    try {
      const body = (await resp.json()) as { error?: string }
      if (body?.error) msg = body.error
    } catch {
      /* ignore */
    }
    throw new Error(msg)
  }
}
