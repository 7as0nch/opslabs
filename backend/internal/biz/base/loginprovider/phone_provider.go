package loginprovider

import (
	"context"
	"strings"

	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/pkg/auth"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

const MockPhoneCode = "666666"

type PhoneProvider struct {
	userRepo UserRepo
	authRepo auth.AuthRepo
}

func NewPhoneProvider(userRepo UserRepo, authRepo auth.AuthRepo) *PhoneProvider {
	return &PhoneProvider{
		userRepo: userRepo,
		authRepo: authRepo,
	}
}

func (p *PhoneProvider) Type() model.AuthType {
	return model.AuthTypePhone
}

func (p *PhoneProvider) Login(ctx context.Context, req *LoginRequest) (*LoginResult, error) {
	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)
	if phone == "" || code == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "phone or code is empty")
	}
	if code != MockPhoneCode {
		return nil, kerrors.Unauthorized(auth.Reason, "invalid phone code")
	}

	authRecord, err := p.userRepo.GetUserAuthByTypeAndIdentifier(ctx, model.AuthTypePhone, phone)
	if err != nil {
		return nil, err
	}

	var user *model.SysUser
	if authRecord != nil {
		user, err = p.userRepo.GetById(ctx, authRecord.UserID)
		if err != nil {
			return nil, err
		}
	}

	if user == nil {
		user, err = p.userRepo.GetByPhone(ctx, phone)
		if err != nil {
			return nil, err
		}
	}

	if user == nil {
		user = BuildPhoneUser(phone)
		if err = p.userRepo.Create(ctx, user); err != nil {
			return nil, err
		}
	}

	if authRecord == nil {
		authRecord = &model.SysUserAuth{
			UserID:     user.ID,
			AuthType:   model.AuthTypePhone,
			Identifier: phone,
			Secret:     "",
		}
		authRecord.New()
		if err = p.userRepo.CreateUserAuth(ctx, authRecord); err != nil {
			return nil, err
		}
	}

	token, err := IssueToken(ctx, p.authRepo, user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}
