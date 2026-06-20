package server

import (
	"log/slog"
	v1 "temperate/api/temperate/v1"
	"temperate/internal/biz"
	"temperate/internal/conf"
	"temperate/internal/service"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/logging"

	oteltracing "github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"

	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	c *conf.Server,
	data *conf.Data,
	metrics *Metrics,
	tracing *Tracing,
	auth *biz.UseCase,
	service *service.IncidentService,
	logger *slog.Logger,
) *http.Server {
	if logger == nil {
		logger = log.Default()
	}
	middlewares := []middleware.Middleware{
		recovery.Recovery(),
	}
	if tracing.Enabled() {
		middlewares = append(middlewares, oteltracing.Server())
	}
	middlewares = append(middlewares, logging.Server(logger))
	if metrics.Enabled() {
		middlewares = append(middlewares, metrics.Middleware())
	}
	api := data.GetApi()
	if api.GetRatelimit() {
		middlewares = append(middlewares, ratelimit.Server())
	}
	if api.GetAuth() {
		middlewares = append(middlewares, selectedAuthMiddleware(api.GetSigningMethod(), api.GetJwtKey(), auth, auth))
	}
	middlewares = append(middlewares,
		validate.Validator(func(req any) error {
			if msg, ok := req.(proto.Message); ok {
				if err := fieldbehavior.ValidateRequiredFields(msg); err != nil {
					return err
				}
			}
			return nil
		}),
	)
	var opts = []http.ServerOption{
		http.Middleware(middlewares...),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	if metrics.Enabled() {
		srv.Handle(metrics.Path(), metrics.Handler())
	}
	v1.RegisterTemperateServiceHTTPServer(srv, service)
	srv.HandleFunc("/v1/ws", newWSHandler(data, auth))
	return srv
}
