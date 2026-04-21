//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/7as0nch/backend/internal/biz"
	"github.com/7as0nch/backend/internal/conf"
	"github.com/7as0nch/backend/internal/data"
	"github.com/7as0nch/backend/internal/server"
	"github.com/7as0nch/backend/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Bootstrap, *zap.Logger, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
