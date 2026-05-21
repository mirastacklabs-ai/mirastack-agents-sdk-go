// metrics.go installs an OTLP/gRPC MeterProvider on the agent process.
// Gated by the same MIRASTACK_OTEL_ENABLED flag that controls tracing so a
// plugin author can disable the whole telemetry pipeline with a single env
// var.
//
// The exporter and reader are configured to mirror the engine
// (internal/observability/metrics.go) — same protocol, same default
// PeriodicReader interval, same resource enrichment from
// OTEL_RESOURCE_ATTRIBUTES.
package mirastack

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

const defaultMetricsInterval = 60 * time.Second

// initMeterProvider installs an OTLP/gRPC MeterProvider on the process when
// MIRASTACK_OTEL_ENABLED == "true". Returns a no-op shutdown otherwise.
//
// The function never returns an error for the disabled path — callers can
// always defer the returned shutdown without conditionals.
func initMeterProvider(ctx context.Context, pluginName string, logger *zap.Logger) (shutdown func(context.Context) error, err error) {
	if !otelEnabled() {
		logger.Debug("OTel metrics disabled for plugin")
		return noopMeterShutdown, nil
	}

	serviceName := defaultAgentServiceName
	if pluginName != "" {
		serviceName = pluginName
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(pluginBuildVersion()),
			attribute.String("mirastack.component_kind", componentKind),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		return noopMeterShutdown, fmt.Errorf("otel metrics resource: %w", err)
	}

	var metricOpts []otlpmetricgrpc.Option
	if ep, ok := metricsOTLPEndpointFromEnv(); ok {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithEndpoint(ep))
	}
	if metricsOTLPInsecureFromEnv() {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}
	exp, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		return noopMeterShutdown, fmt.Errorf("otel metrics exporter: %w", err)
	}

	reader := metric.NewPeriodicReader(exp, metric.WithInterval(defaultMetricsInterval))
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)
	otel.SetMeterProvider(mp)

	logger.Info("OTel metrics enabled for plugin",
		zap.String("service", serviceName),
		zap.Duration("interval", defaultMetricsInterval),
	)
	return mp.Shutdown, nil
}

func noopMeterShutdown(context.Context) error { return nil }
