package server

import (
	"log/slog"
	"net/http"
	"github.com/wuyuetianjian/kratos-template-for-all/internal/conf"

	kratosmetrics "github.com/go-kratos/kratos/contrib/otel/v3/metrics"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const defaultMetricsPath = "/metrics"

type Metrics struct {
	enabled    bool
	path       string
	middleware middleware.Middleware
}

func NewMetrics(data *conf.Data, logger *slog.Logger) *Metrics {
	api := data.GetApi()
	if !api.GetMetrics() {
		return &Metrics{}
	}

	exporter, err := prometheus.New()
	if err != nil {
		if logger != nil {
			logger.Error("create prometheus exporter failed", slog.Any("error", err))
		}
		return &Metrics{}
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(kratosmetrics.DefaultSecondsHistogramView(kratosmetrics.DefaultServerSecondsHistogramName)),
	)
	meter := provider.Meter("temperate")
	requests, err := kratosmetrics.DefaultRequestsCounter(meter, kratosmetrics.DefaultServerRequestsCounterName)
	if err != nil {
		if logger != nil {
			logger.Error("create metrics requests counter failed", slog.Any("error", err))
		}
		return &Metrics{}
	}
	seconds, err := kratosmetrics.DefaultSecondsHistogram(meter, kratosmetrics.DefaultServerSecondsHistogramName)
	if err != nil {
		if logger != nil {
			logger.Error("create metrics seconds histogram failed", slog.Any("error", err))
		}
		return &Metrics{}
	}

	path := api.GetMetricsPath()
	if path == "" {
		path = defaultMetricsPath
	}
	return &Metrics{
		enabled: true,
		path:    path,
		middleware: kratosmetrics.Server(
			kratosmetrics.WithRequests(requests),
			kratosmetrics.WithSeconds(seconds),
		),
	}
}

func (m *Metrics) Enabled() bool {
	return m != nil && m.enabled
}

func (m *Metrics) Path() string {
	if m == nil || m.path == "" {
		return defaultMetricsPath
	}
	return m.path
}

func (m *Metrics) Middleware() middleware.Middleware {
	if m == nil {
		return nil
	}
	return m.middleware
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
