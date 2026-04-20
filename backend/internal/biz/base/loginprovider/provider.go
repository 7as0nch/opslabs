package loginprovider

import (
	"context"
	"time"

	"github.com/example/aichat/backend/models/generator/model"
)

type StateCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, key string) error
	GetDel(ctx context.Context, key string) (string, error)
}

type UserRepo interface {
	GetByAccount(ctx context.Context, account string) (*model.SysUser, error)
	GetById(ctx context.Context, id int64) (*model.SysUser, error)
	GetByPhone(ctx context.Context, phone string) (*model.SysUser, error)
	Create(ctx context.Context, user *model.SysUser) error
	GetUserAuthByUserIDAndType(ctx context.Context, userID int64, authType model.AuthType) (*model.SysUserAuth, error)
	GetUserAuthByTypeAndIdentifier(ctx context.Context, authType model.AuthType, identifier string) (*model.SysUserAuth, error)
	CreateUserAuth(ctx context.Context, userAuth *model.SysUserAuth) error
	UpdateUserAuth(ctx context.Context, userAuth *model.SysUserAuth) error
}

type LoginRequest struct {
	AuthType model.AuthType
	Username string
	Password string
	Phone    string
	Code     string
	AuthCode string
	State    string
}

type LoginResult struct {
	Token string
	User  *model.SysUser
}

type LoginProvider interface {
	Type() model.AuthType
	Login(ctx context.Context, req *LoginRequest) (*LoginResult, error)
}

type OAuthProvider interface {
	LoginProvider
	BuildLoginURL(ctx context.Context, redirectURL string) (string, error)
	PopRedirectURL(state string) string
	DefaultRedirectURL() string
}