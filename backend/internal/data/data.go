package data

import (
	"github.com/7as0nch/backend/internal/biz/base/loginprovider"
	"github.com/7as0nch/backend/internal/db"
	"github.com/7as0nch/backend/pkg/auth"

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
