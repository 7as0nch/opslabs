//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/example/aichat/backend/internal/biz"
	"github.com/example/aichat/backend/internal/conf"
	"github.com/example/aichat/backend/internal/data"
	"github.com/example/aichat/backend/internal/server"
	"github.com/example/aichat/backend/internal/service"

	"go.uber.org/zap"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Bootstrap, *zap.Logger, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
