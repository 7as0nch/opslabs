// Package auth @author <cheng jiang>
// @date 2023/1/10
// @note
package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v4"
)

// NewWhiteListMatcher 白名单, is the white list for url request.
func NewWhiteListMatcher(whiteList map[string]bool) selector.MatchFunc {
	return func(ctx context.Context, optUrl string) bool {
		if _, ok := whiteList[optUrl]; ok {
			return false
		}
		return true
	}
}

// 需要开放跨域的接口路径
var allowedPaths = map[string]bool{
	"/tracker/batch": true,
}
// PHMMiddlewareCors 对跨域做出过滤，只对特定接口开放
func MiddlewareCors() middleware.Middleware {

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if ts, ok := transport.FromServerContext(ctx); ok {
				if ht, ok := ts.(http.Transporter); ok {
					// 获取请求路径
					path := ht.RequestHeader().Get(":path")
					// 对于OPTIONS预检请求，也需要设置跨域头
					method := ht.RequestHeader().Get("x-method")
					isOptions := method == "OPTIONS"

					// 如果是允许的路径或者OPTIONS预检请求（针对允许的路径），则设置跨域头
					if allowedPaths[path] || (isOptions && allowedPaths[ht.RequestHeader().Get("x-path")]) {
						ht.ReplyHeader().Set("Access-Control-Allow-Origin", "*")
						ht.ReplyHeader().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS,PUT,PATCH,DELETE")
						ht.ReplyHeader().Set("Access-Control-Allow-Credentials", "true")
						ht.ReplyHeader().Set("Access-Control-Allow-Headers", "Content-Type,Token,"+
							"X-Requested-With,Access-Control-Allow-Credentials,User-Agent,Content-Length,Authorization,Accept,Accept-Language,Content-Language,Origin")
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

// Server is a server auth middleware. Check the token and extract the info from token.
func Server(keyFunc jwt.Keyfunc, opts ...Option) middleware.Middleware {
	// set the test method. the options are the test options.
	o := &options{
		signingMethod: jwt.SigningMethodHS256,
	}
	for _, opt := range opts {
		opt(o)
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if header, ok := transport.FromServerContext(ctx); ok {
				if keyFunc == nil {
					return nil, ErrMissingKeyFunc
				}
				auths := strings.SplitN(header.RequestHeader().Get(AuthorizationKey), " ", 2)

				if authIsNotOK(auths) {
					return nil, ErrMissingJwtToken
				}
				jwtToken := auths[1]
				var (
					tokenInfo *jwt.Token
					err       error
				)

				if o.claims != nil {
					// 解析
					tokenInfo, err = jwt.ParseWithClaims(jwtToken, o.claims(), keyFunc)
				} else {
					tokenInfo, err = jwt.Parse(jwtToken, keyFunc)
				}
				if err != nil {
					ve, ok := err.(*jwt.ValidationError)
					if !ok {
						return nil, errors.Unauthorized(Reason, err.Error())
					}
					if ve.Errors&jwt.ValidationErrorMalformed != 0 {
						return nil, ErrTokenInvalid
					}
					if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
						return nil, ErrTokenExpired
					}
					return nil, ErrTokenParseFail
				}
				if !tokenInfo.Valid {
					return nil, ErrTokenInvalid
				}
				if tokenInfo.Method != o.signingMethod {
					return nil, ErrUnSupportSigningMethod
				}
				ctx = NewContext(ctx, tokenInfo.Claims)
				return handler(ctx, req)
			}
			return nil, ErrWrongContext
		}
	}
}

// Client is a client test middleware.
func Client(keyProvider jwt.Keyfunc, opts ...Option) middleware.Middleware {
	claims := jwt.RegisteredClaims{}
	o := &options{
		signingMethod: jwt.SigningMethodHS256,
		claims:        func() jwt.Claims { return claims },
	}
	for _, opt := range opts {
		opt(o)
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if keyProvider == nil {
				return nil, ErrNeedTokenProvider
			}
			token := jwt.NewWithClaims(o.signingMethod, o.claims())
			if o.tokenHeader != nil {
				for k, v := range o.tokenHeader {
					token.Header[k] = v
				}
			}
			key, err := keyProvider(token)
			if err != nil {
				return nil, ErrGetKey
			}
			tokenStr, err := token.SignedString(key)
			if err != nil {
				return nil, ErrSignToken
			}
			if clientContext, ok := transport.FromClientContext(ctx); ok {
				clientContext.RequestHeader().Set(AuthorizationKey, fmt.Sprintf(BearerFormat, tokenStr))
				return handler(ctx, req)
			}
			return nil, ErrWrongContext
		}
	}
}
