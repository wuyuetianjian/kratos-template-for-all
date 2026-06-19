//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"log/slog"

	"temperate/internal/biz"
	"temperate/internal/conf"
	"temperate/internal/data"
	registrar "temperate/internal/registry"
	"temperate/internal/server"
	"temperate/internal/service"

	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *conf.Registry, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(registrar.ProviderSet, server.ProviderSet, biz.ProviderSet, service.ProviderSet, data.ProviderSet, newApp))
}
