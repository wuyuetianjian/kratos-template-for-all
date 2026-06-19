package server

import (
	"log/slog"
	v1 "temperate/api/temperate/v1"
	"temperate/internal/biz"
	"temperate/internal/conf"
	"temperate/internal/service"

	oteltracing "github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(
	c *conf.Server,
	data *conf.Data,
	metrics *Metrics,
	tracing *Tracing,
	auth *biz.UseCase,
	service *service.IncidentService,
	logger *slog.Logger,
) *grpc.Server {
	middlewares := []middleware.Middleware{
		recovery.Recovery(),
	}
	if tracing.Enabled() {
		middlewares = append(middlewares, oteltracing.Server())
	}
	if metrics.Enabled() {
		middlewares = append(middlewares, metrics.Middleware())
	}
	api := data.GetApi()
	if api.GetRatelimit() {
		middlewares = append(middlewares, ratelimit.Server())
	}
	if api.GetAuth() {
		middlewares = append(middlewares, selectedAuthMiddleware(api.GetSigningMethod(), api.GetJwtKey(), auth))
	}
	var opts = []grpc.ServerOption{
		grpc.Middleware(middlewares...),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterTemperateServiceServer(srv, service)
	return srv
}
