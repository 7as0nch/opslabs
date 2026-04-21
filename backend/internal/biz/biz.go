package biz

/* *
 * @Author: chengjiang
 * @Date: 2025-10-02 15:21:17
 * @Description:
**/

import (
	"github.com/7as0nch/backend/internal/biz/base"
	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	base.NewSysUserUseCase,
	base.NewSysMenuUseCase,
	base.NewDictTypeUseCase,
	base.NewDictDataUseCase,
	base.NewTrackerUseCase,
)
