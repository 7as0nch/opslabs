package model

import (
	"strings"

	"github.com/example/aichat/backend/models"
)

const TableNameSysUserAuth = "sys_user_auth"

type AuthType int

const (
	AuthTypePassword AuthType = 1
	AuthTypePhone    AuthType = 2
	AuthTypeQQ       AuthType = 3
	AuthTypeWechat   AuthType = 4
	AuthTypeGithub   AuthType = 5
)

func (t AuthType) String() string {
	switch t {
	case AuthTypePassword:
		return "password"
	case AuthTypePhone:
		return "phone"
	case AuthTypeQQ:
		return "qq"
	case AuthTypeWechat:
		return "wechat"
	case AuthTypeGithub:
		return "github"
	default:
		return "unknown"
	}
}

func ParseAuthType(value string) (AuthType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "password":
		return AuthTypePassword, true
	case "phone":
		return AuthTypePhone, true
	case "qq":
		return AuthTypeQQ, true
	case "wechat":
		return AuthTypeWechat, true
	case "github":
		return AuthTypeGithub, true
	default:
		return 0, false
	}
}

// SysUserAuth stores credentials for different login providers.
type SysUserAuth struct {
	models.Model
	UserID     int64    `gorm:"column:user_id;type:bigint;index:idx_sys_user_auth_user_id;uniqueIndex:uk_sys_user_auth_user_type,priority:1" json:"user_id"`
	AuthType   AuthType `gorm:"column:auth_type;type:smallint;uniqueIndex:uk_sys_user_auth_type_identifier,priority:1;uniqueIndex:uk_sys_user_auth_user_type,priority:2" json:"auth_type"`
	Identifier string   `gorm:"column:identifier;type:varchar(128);uniqueIndex:uk_sys_user_auth_type_identifier,priority:2" json:"identifier"`
	Secret     string   `gorm:"column:secret;type:varchar(255)" json:"secret"`
}

func (*SysUserAuth) TableName() string {
	return TableNameSysUserAuth
}

func (m *SysUserAuth) New() {
	m.Model.New()
}

