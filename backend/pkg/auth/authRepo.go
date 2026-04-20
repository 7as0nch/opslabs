// Package auth @author <cheng jiang>
// @date 2023/1/10
// @note
package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/example/aichat/backend/tools"
	myStrings "github.com/example/aichat/backend/tools/strings"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/metadata"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v4"
	"strconv"
	"strings"
	"time"
)


type authRepo struct {
	signingKey []byte
}

type AuthRepo interface {
	CheckToken(ctx context.Context, token string) (*JwtClaims, error)
	GetToken(ctx context.Context) (string, error)
	NewToken(ctx context.Context, userId int64, username, phone string) (string, error)
	//Server the middleware
	Server() func(handler middleware.Handler) middleware.Handler
}

func NewAuthRepo() AuthRepo {
	return &authRepo{
		signingKey: []byte("2025/ai/chat|author:chengjiang@stu.cdu.edu.cn"),
	}
}

func (a *authRepo) CheckToken(ctx context.Context, tokenString string) (*JwtClaims, error) {
	var claims *JwtClaims
	var err error
	// 除掉Bear
	tokenStr := strings.SplitN(tokenString, " ", 2)
	jwtToken := tokenStr[1]
	if authIsNotOK(tokenStr) {
		return nil, ErrMissingJwtToken
	}
	//
	//at(time.Unix(0, 0), func() {
	//
	//})
	token, err := jwt.ParseWithClaims(jwtToken, &JwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		return a.signingKey, nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			claims = nil
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				err = ErrSignToken
				//return
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				// Token is expired
				err = ErrTokenExpired
				//return
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				err = ErrTokenInvalid
				//return
			} else {
				err = ErrTokenInvalid
				//return
			}
		}
		return nil, err
	}
	if claimsInner, ok := token.Claims.(*JwtClaims); ok && token.Valid {
		claims = claimsInner
		err = nil
		//return
		return claims, err
	}
	claims = nil
	err = ErrTokenInvalid
	return claims, err
}

func (a *authRepo) GetToken(ctx context.Context) (string, error) {
	var token string
	if header, ok := transport.FromServerContext(ctx); ok {
		token = header.RequestHeader().Get(AuthorizationKey)
	}
	if myStrings.IsEmpty(token) {
		return myStrings.EmptyStr, errors.New("in GetToken, token is nil")
	}
	return token, nil
}

func (a *authRepo) NewToken(ctx context.Context, userId int64, username, phone string) (string, error) {
	// ExpiredTime Expired time, 失效时间：十分钟。
	ExpiredTime := &jwt.NumericDate{Time: time.Now().Add(time.Hour * 24)}
	claims := JwtClaims{
		UserId:    userId,
		UserName:  username,
		UserPhone: phone,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        strconv.FormatInt(tools.GetSnowID(), 10),
			ExpiresAt: ExpiredTime,
			//Issuer:    conf.Config.Jwt.Issuer,
		},
	}
	// the method
	newWithClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := newWithClaims.SignedString(a.signingKey)
	if err != nil {
		return "", fmt.Errorf("创建token失败，%v", err)
	}
	// token = fmt.Sprintf(BearerFormat, token)
	fmt.Println(token)
	return token, err
}

func (a *authRepo) Server() func(handler middleware.Handler) middleware.Handler {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			var token string
			if header, ok := transport.FromServerContext(ctx); ok {
				token = header.RequestHeader().Get(AuthorizationKey)
			}
			if myStrings.IsEmpty(token) {
				return myStrings.EmptyStr, errors.New("Token is missing")
			}
			//
			claims, err := a.CheckToken(ctx, token)
			if err != nil || claims == nil {
				return nil, kerrors.New(401, " PHMToken is expired ", " PHMToken is expired ")
			}
			ctx = context.WithValue(ctx, UserId, int64(claims.UserId))
			ctx = context.WithValue(ctx, UserName, claims.UserName)
			ctx = context.WithValue(ctx, UserPhone, claims.UserPhone)
			reply, err = handler(ctx, req)
			return
		}
	}
}

// at 时间的刷新。
func at(t time.Time, f func()) {
	jwt.TimeFunc = func() time.Time {
		return t
	}
	f()
	jwt.TimeFunc = time.Now
}

func NewHeaderServer() func(handler middleware.Handler) middleware.Handler {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			var clientID string
			if md, ok := metadata.FromServerContext(ctx); ok {
				extra := md.Get("U-OrGniZaTiOn")
				fmt.Println(extra)
			}
			if header, ok := transport.FromServerContext(ctx); ok {
				clientID = header.RequestHeader().Get("Clientid")
				ctx = context.WithValue(ctx, ClientID, clientID)
			}
			reply, err = handler(ctx, req)
			return
		}
	}
}
