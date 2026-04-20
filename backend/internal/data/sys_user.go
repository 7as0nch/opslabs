package data

import (
	"context"
	"errors"
	"strings"

	"github.com/example/aichat/backend/internal/biz/base"
	"github.com/example/aichat/backend/internal/db"
	"github.com/example/aichat/backend/models/generator/model"
	"github.com/example/aichat/backend/models/generator/query"
	"gorm.io/gorm"
)

type sysUserRepo struct {
	db    db.DataRepo
	query *query.Query
}

func NewSysUserRepo(db db.DataRepo) base.SysUserRepo {
	// if err := db.GetDB().AutoMigrate(&model.SysUserAuth{}); err != nil {
	// 	panic(fmt.Sprintf("auto migrate userauth failed: %v", err))
	// }
	return &sysUserRepo{
		db:    db,
		query: query.Use(db.GetDB()),
	}
}

func (r *sysUserRepo) GetByAccount(ctx context.Context, account string) (*model.SysUser, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, nil
	}

	user, err := r.query.SysUser.WithContext(ctx).Where(r.query.SysUser.Account.Eq(account)).Or(r.query.SysUser.Name.Eq(account)).Or(r.query.SysUser.Email.Eq(account)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *sysUserRepo) GetById(ctx context.Context, id int64) (*model.SysUser, error) {
	user, err := r.query.SysUser.WithContext(ctx).Where(r.query.SysUser.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *sysUserRepo) GetByPhone(ctx context.Context, phone string) (*model.SysUser, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, nil
	}

	user, err := r.query.SysUser.WithContext(ctx).Where(r.query.SysUser.Phonenumber.Eq(phone)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *sysUserRepo) Create(ctx context.Context, user *model.SysUser) error {
	return r.db.DB(ctx).Create(user).Error
}

func (r *sysUserRepo) GetUserAuthByUserIDAndType(ctx context.Context, userID int64, authType model.AuthType) (*model.SysUserAuth, error) {
	var userAuth model.SysUserAuth
	err := r.db.DB(ctx).
		Where("user_id = ? AND auth_type = ?", userID, authType).
		First(&userAuth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &userAuth, nil
}

func (r *sysUserRepo) GetUserAuthByTypeAndIdentifier(ctx context.Context, authType model.AuthType, identifier string) (*model.SysUserAuth, error) {
	var userAuth model.SysUserAuth
	err := r.db.DB(ctx).
		Where("auth_type = ? AND identifier = ?", authType, strings.TrimSpace(identifier)).
		First(&userAuth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &userAuth, nil
}

func (r *sysUserRepo) CreateUserAuth(ctx context.Context, userAuth *model.SysUserAuth) error {
	return r.db.DB(ctx).Create(userAuth).Error
}

func (r *sysUserRepo) UpdateUserAuth(ctx context.Context, userAuth *model.SysUserAuth) error {
	return r.db.DB(ctx).Save(userAuth).Error
}


