package service

/* *
 * @Author: chengjiang
 * @Date: 2025-10-02 15:21:17
 * @Description:
**/

import (
	"github.com/7as0nch/backend/internal/service/base"
	"github.com/7as0nch/backend/internal/service/opslabs"
	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	base.NewAuthService,
	base.NewSystemService,
	base.NewTrackerService,
	opslabs.NewScenarioService,
	opslabs.NewAttemptService,
)
