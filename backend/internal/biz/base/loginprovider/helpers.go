package loginprovider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/example/aichat/backend/models"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/pkg/auth"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/crypto/bcrypt"
)

func IssueToken(ctx context.Context, authRepo auth.AuthRepo, user *model.SysUser) (string, error) {
	token, err := authRepo.NewToken(ctx, user.ID, user.Account, user.Phonenumber)
	if err != nil {
		log.Errorf("create token failed: %v", err)
		return "", kerrors.InternalServer("TOKEN_CREATE_FAILED", "create token failed")
	}
	return token, nil
}

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(input, stored string) (matched bool, needUpgrade bool) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return false, false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(input)); err == nil {
		return true, false
	}
	if input == stored {
		return true, true
	}
	return false, false
}

func UpsertPasswordAuth(ctx context.Context, repo UserRepo, user *model.SysUser, identifier, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = user.Account
	}

	authRecord, err := repo.GetUserAuthByUserIDAndType(ctx, user.ID, model.AuthTypePassword)
	if err != nil {
		return err
	}

	if authRecord == nil {
		authRecord = &model.SysUserAuth{
			UserID:     user.ID,
			AuthType:   model.AuthTypePassword,
			Identifier: identifier,
			Secret:     hash,
		}
		authRecord.New()
		return repo.CreateUserAuth(ctx, authRecord)
	}

	authRecord.Identifier = identifier
	authRecord.Secret = hash
	return repo.UpdateUserAuth(ctx, authRecord)
}

func BuildQQAccount(ctx context.Context, repo UserRepo, openID string) (string, error) {
	suffix := openID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	base := "qq_" + suffix
	account := base

	for i := 0; i < 5; i++ {
		existing, err := repo.GetByAccount(ctx, account)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return account, nil
		}
		randSuffix, err := RandomHex(3)
		if err != nil {
			return "", err
		}
		account = base + "_" + randSuffix
	}
	return "", kerrors.InternalServer("QQ_ACCOUNT_CREATE_FAILED", "failed to allocate qq account")
}

func RandomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func BuildPhoneUser(phone string) *model.SysUser {
	phone = strings.TrimSpace(phone)
	user := &model.SysUser{
		Type:        model.SysUserType_Guest,
		Account:     phone,
		Name:        "用户" + suffixForDisplay(phone),
		Phonenumber: phone,
		Status:      models.Status_Enabled,
	}
	user.New()
	return user
}

func suffixForDisplay(input string) string {
	if len(input) <= 4 {
		return input
	}
	return input[len(input)-4:]
}
