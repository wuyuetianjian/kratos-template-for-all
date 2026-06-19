package server

import (
	"context"
	"log/slog"
	"temperate/internal/conf"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	defaultTracingEndpoint = "localhost:4318"
	tracingServiceName     = "temperate"
)

type Tracing struct {
	enabled bool
}

func NewTracing(data *conf.Data, logger *slog.Logger) (*Tracing, func(), error) {
	api := data.GetApi()
	if !api.GetTracing() {
		return &Tracing{}, func() {}, nil
	}

	endpoint := api.GetTracingEndpoint()
	if endpoint == "" {
		endpoint = defaultTracingEndpoint
	}

	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(tracingServiceName),
		)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil && logger != nil {
			logger.Error("shutdown tracer provider failed", slog.Any("error", err))
		}
	}
	return &Tracing{enabled: true}, cleanup, nil
}

func (t *Tracing) Enabled() bool {
	return t != nil && t.enabled
}
