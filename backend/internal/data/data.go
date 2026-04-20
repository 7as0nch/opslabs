package data

import (
	"github.com/example/aichat/backend/internal/biz/base/loginprovider"
	"github.com/example/aichat/backend/internal/db"
	"github.com/example/aichat/backend/pkg/auth"

	"github.com/google/wire"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewSysUserRepo,
	NewSysMenuRepo,
	auth.NewAuthRepo,
	db.NewData,
	NewTransaction,
	NewDictTypeRepo,
	NewDictDataRepo,
	NewTrackerRepo,
	db.NewRedisRepo,
	loginprovider.NewStateCache,
)
