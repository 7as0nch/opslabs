package base

import (
	"context"
	"strings"

	"github.com/example/aichat/backend/internal/biz/base/loginprovider"
	"github.com/example/aichat/backend/internal/conf"
	"github.com/example/aichat/backend/models"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/pkg/auth"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

type SysUserRepo interface {
	GetByAccount(ctx context.Context, account string) (*model.SysUser, error)
	GetById(ctx context.Context, id int64) (*model.SysUser, error)
	GetByPhone(ctx context.Context, phone string) (*model.SysUser, error)
	Create(ctx context.Context, user *model.SysUser) error
	GetUserAuthByUserIDAndType(ctx context.Context, userID int64, authType model.AuthType) (*model.SysUserAuth, error)
	GetUserAuthByTypeAndIdentifier(ctx context.Context, authType model.AuthType, identifier string) (*model.SysUserAuth, error)
	CreateUserAuth(ctx context.Context, userAuth *model.SysUserAuth) error
	UpdateUserAuth(ctx context.Context, userAuth *model.SysUserAuth) error
}

type SysUserUseCase struct {
	user      SysUserRepo
	auth      auth.AuthRepo
	providers map[model.AuthType]loginprovider.LoginProvider
	qq        loginprovider.OAuthProvider
}

func NewSysUserUseCase(user SysUserRepo, auth auth.AuthRepo, redisRepo loginprovider.StateCache, bootstrap *conf.Bootstrap) *SysUserUseCase {
	passwordProvider := loginprovider.NewPasswordProvider(user, auth)
	phoneProvider := loginprovider.NewPhoneProvider(user, auth)
	var qqConf *conf.Auth_QQ
	if bootstrap != nil && bootstrap.GetAuth() != nil {
		qqConf = bootstrap.GetAuth().GetQq()
	}
	qqProvider := loginprovider.NewQQProvider(user, auth, redisRepo, qqConf)

	return &SysUserUseCase{
		user: user,
		auth: auth,
		providers: map[model.AuthType]loginprovider.LoginProvider{
			passwordProvider.Type(): passwordProvider,
			phoneProvider.Type():    phoneProvider,
			qqProvider.Type():       qqProvider,
		},
		qq: qqProvider,
	}
}

func (s *SysUserUseCase) Login(ctx context.Context, req *loginprovider.LoginRequest) (*loginprovider.LoginResult, error) {
	if req == nil {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "login request is nil")
	}

	provider, ok := s.providers[req.AuthType]
	if !ok {
		return nil, kerrors.BadRequest("UNSUPPORTED_LOGIN_TYPE", "unsupported login type")
	}
	return provider.Login(ctx, req)
}

func (s *SysUserUseCase) BuildQQLoginURL(ctx context.Context, redirectURL string) (string, error) {
	if s.qq == nil {
		return "", kerrors.InternalServer("QQ_PROVIDER_NOT_READY", "qq provider is not ready")
	}
	return s.qq.BuildLoginURL(ctx, redirectURL)
}

func (s *SysUserUseCase) PopQQRedirectURL(state string) string {
	if s.qq == nil {
		return ""
	}
	return s.qq.PopRedirectURL(state)
}

func (s *SysUserUseCase) QQDefaultRedirectURL() string {
	if s.qq == nil {
		return "/"
	}
	return s.qq.DefaultRedirectURL()
}

// Register creates a new local account and password credential.
func (s *SysUserUseCase) Register(ctx context.Context, account, password, email, phone, sex string) (string, error) {
	account = strings.TrimSpace(account)
	password = strings.TrimSpace(password)
	if account == "" || password == "" {
		return "", kerrors.BadRequest("INVALID_ARGUMENT", "account or password is empty")
	}

	existing, err := s.user.GetByAccount(ctx, account)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return "", kerrors.BadRequest("ACCOUNT_EXISTS", "account already exists")
	}

	newUser := &model.SysUser{
		Type:        model.SysUserType_Guest,
		Account:     account,
		Name:        account,
		Email:       strings.TrimSpace(email),
		Phonenumber: strings.TrimSpace(phone),
		Sex:         strings.TrimSpace(sex),
		Status:      models.Status_Enabled,
	}
	newUser.New()
	if err = s.user.Create(ctx, newUser); err != nil {
		return "", err
	}

	if err = loginprovider.UpsertPasswordAuth(ctx, s.user, newUser, account, password); err != nil {
		return "", err
	}

	return loginprovider.IssueToken(ctx, s.auth, newUser)
}

// GetInfo returns profile from token context.
func (s *SysUserUseCase) GetInfo(ctx context.Context) (*model.SysUser, error) {
	uid, _ := ctx.Value(auth.UserId).(int64)
	user, err := s.user.GetById(ctx, uid)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Logout keeps current stateless behavior.
func (s *SysUserUseCase) Logout(ctx context.Context) error {
	return nil
}

