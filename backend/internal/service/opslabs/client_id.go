/* *
 * @Author: chengjiang
 * @Date: 2026-04-23
 * @Description: X-Client-ID header 中间件:从请求头把前端 uuid 塞进 ctx
 *               ctx key 和 helper 下沉到 biz/attempt 做领域概念,这里只做 HTTP 传输绑定
**/
package opslabs

import (
	"context"

	"github.com/7as0nch/backend/internal/biz/attempt"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// clientIDHeaderKey 前端 uuid 走这个 header 送过来
const clientIDHeaderKey = "X-Client-ID"

// MiddlewareClientID Kratos 中间件:从 transport 请求头读 X-Client-ID 塞 ctx
//
// 行为:
//   - 有 X-Client-ID: <v>            → ctx 里落 v(原样,不做格式校验)
//   - 无 header 或值为 ""           → 落 attempt.AnonymousClientID "anon"
//   - 非 transport 上下文(grpc/测试) → 落 AnonymousClientID
//
// 注:CORS 暴露这个 header 要在 auth.MiddlewareCors 的 Access-Control-Allow-Headers
// 里加上 "X-Client-ID";V1 前端走 vite 同源代理无 CORS,暂不处理,prod 分离部署时再补
func MiddlewareClientID() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			cid := attempt.AnonymousClientID
			if ts, ok := transport.FromServerContext(ctx); ok {
				if h := ts.RequestHeader().Get(clientIDHeaderKey); h != "" {
					cid = h
				}
			}
			ctx = attempt.WithClientID(ctx, cid)
			return handler(ctx, req)
		}
	}
}
