/* *
 * @Author: chengjiang
 * @Date: 2026-04-23
 * @Description: ClientID 是 V1 阶段的"匿名 owner"标识,由前端生成 uuid
 *               通过 X-Client-ID header 送到后端,service 层中间件塞进 ctx,
 *               biz 层从 ctx 读出来做 store 查询和落库字段填充
 *               登录接入后仍保留:clientID 可做"同一用户多设备区分"
**/
package attempt

import "context"

// clientIDCtxKey ctx 里挂 clientID 的 key;unexported 防止外部误 overwrite
type clientIDCtxKey struct{}

// AnonymousClientID header 缺失时的兜底 owner ID
// 约定:落库永远非空 —— 避免空串在 store 查询时踩到"匹配所有" bug
const AnonymousClientID = "anon"

// ClientIDFromContext 从 ctx 取 clientID,永远非空(缺失 fallback 到 anon)
func ClientIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientIDCtxKey{}).(string); ok && v != "" {
		return v
	}
	return AnonymousClientID
}

// WithClientID 把 clientID 塞进 ctx
//
// 使用方:
//   - service/opslabs.MiddlewareClientID 从 X-Client-ID header 读出来 → WithClientID
//   - 单测 / 契约 curl 脚本不走 HTTP 时直接 WithClientID 构造 ctx
func WithClientID(ctx context.Context, clientID string) context.Context {
	if clientID == "" {
		clientID = AnonymousClientID
	}
	return context.WithValue(ctx, clientIDCtxKey{}, clientID)
}
