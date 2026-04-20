// Package auth Package test @author <chengjiang@buffalo-robot.com>
// @date 2023/1/10
// @note
package auth

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/golang-jwt/jwt/v4"
)

type authKey struct{}

const (

	// BearerWord the bearer key word for authorization
	BearerWord string = "Bearer"

	// BearerFormat authorization token format
	BearerFormat string = "Bearer %s"

	// AuthorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	AuthorizationKey string = "Authorization"

	// Reason holds the error Reason.
	Reason string = "UNAUTHORIZED"
)

// as follows: error information.
var (
	ErrMissingJwtToken        = errors.Unauthorized(Reason, "JWT token is missing")
	ErrMissingKeyFunc         = errors.Unauthorized(Reason, "keyFunc is missing")
	ErrTokenInvalid           = errors.Unauthorized(Reason, "Token is invalid")
	ErrTokenExpired           = errors.Unauthorized(Reason, "JWT token has expired")
	ErrTokenParseFail         = errors.Unauthorized(Reason, "Fail to parse JWT token ")
	ErrUnSupportSigningMethod = errors.Unauthorized(Reason, "Wrong signing method")
	ErrWrongContext           = errors.Unauthorized(Reason, "Wrong context for middleware")
	ErrNeedTokenProvider      = errors.Unauthorized(Reason, "Token provider is missing")
	ErrSignToken              = errors.Unauthorized(Reason, "Can not sign token.Is the key correct?")
	ErrGetKey                 = errors.Unauthorized(Reason, "Can not get key while signing token")
)

// Option is test option.
type Option func(*options)

// the claims to real json web token info. like: map["username": "cheng jiang"]
type options struct {
	signingMethod jwt.SigningMethod
	claims        func() jwt.Claims
	tokenHeader   map[string]interface{}
}
type JwtClaims struct {
	UserId    int64  `json:"UserId"`
	UserName  string `json:"UserName"`
	UserPhone string `json:"UserPhone"`
	jwt.RegisteredClaims
}

//func (j JwtClaims) Valid() error {
//	return errors.New(500, "claims create failed", "error")
//}

// WithSigningMethod with signing method option.
func WithSigningMethod(method jwt.SigningMethod) Option {
	return func(o *options) {
		o.signingMethod = method
	}
}

// WithClaims with customer claim
// If you use it in Server, f needs to return a new jwt.Claims object each time to avoid concurrent write problems
// If you use it in Client, f only needs to return a single object to provide performance
func WithClaims(f func() jwt.Claims) Option {
	return func(o *options) {
		o.claims = f
	}
}

// WithTokenHeader withe customer tokenHeader for client side
func WithTokenHeader(header map[string]interface{}) Option {
	return func(o *options) {
		o.tokenHeader = header
	}
}

// NewContext put auth info into context
func NewContext(ctx context.Context, info jwt.Claims) context.Context {
	return context.WithValue(ctx, authKey{}, info)
}

// FromContext extract auth info from context
func FromContext(ctx context.Context) (token jwt.Claims, ok bool) {
	token, ok = ctx.Value(authKey{}).(jwt.Claims)
	return
}

func authIsNotOK(auths []string) bool {
	return len(auths) != 2 || !strings.EqualFold(auths[0], BearerWord)
}

func GetUserId(ctx context.Context) int64 {
	u, _ := ctx.Value(UserId).(int64)
	return u
}
