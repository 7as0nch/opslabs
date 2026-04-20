package loginprovider

import (
	"context"
	"strings"

	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/pkg/auth"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type PasswordProvider struct {
	userRepo UserRepo
	authRepo auth.AuthRepo
}

func NewPasswordProvider(userRepo UserRepo, authRepo auth.AuthRepo) *PasswordProvider {
	return &PasswordProvider{
		userRepo: userRepo,
		authRepo: authRepo,
	}
}

func (p *PasswordProvider) Type() model.AuthType {
	return model.AuthTypePassword
}

func (p *PasswordProvider) Login(ctx context.Context, req *LoginRequest) (*LoginResult, error) {
	account := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if account == "" || password == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "username or password is empty")
	}

	user, err := p.userRepo.GetByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if user == nil || user.ID == 0 {
		return nil, kerrors.Unauthorized(auth.Reason, "invalid credentials")
	}

	matched := false
	needUpgrade := false

	authRecord, err := p.userRepo.GetUserAuthByUserIDAndType(ctx, user.ID, model.AuthTypePassword)
	if err != nil {
		return nil, err
	}
	if authRecord != nil {
		matched, needUpgrade = VerifyPassword(password, authRecord.Secret)
		if matched && needUpgrade {
			if upgradeErr := UpsertPasswordAuth(ctx, p.userRepo, user, account, password); upgradeErr != nil {
				log.Warnf("upgrade password hash failed for user=%d: %v", user.ID, upgradeErr)
			}
		}
	} else {
		matched, needUpgrade = VerifyPassword(password, user.Password)
		if matched {
			if upgradeErr := UpsertPasswordAuth(ctx, p.userRepo, user, account, password); upgradeErr != nil {
				log.Warnf("migrate legacy password failed for user=%d: %v", user.ID, upgradeErr)
			}
			if needUpgrade {
				log.Infof("legacy password upgraded for user=%d", user.ID)
			}
		}
	}

	if !matched {
		return nil, kerrors.Unauthorized(auth.Reason, "invalid credentials")
	}

	token, err := IssueToken(ctx, p.authRepo, user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}
