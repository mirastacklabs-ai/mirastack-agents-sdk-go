// Package mirastack provides OpenTelemetry auto-instrumentation for agent plugins.
// When MIRASTACK_OTEL_ENABLED="true", the SDK automatically initializes a
// TracerProvider and wires gRPC interceptors — plugin authors get distributed
// tracing for FREE with zero code changes.
package mirastack

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

const (
	defaultAgentServiceName = "mirastack-agent"
	tracerName              = "mirastack.plugin"
)

// otelEnabled returns true when MIRASTACK_OTEL_ENABLED is "true".
func otelEnabled() bool {
	return os.Getenv("MIRASTACK_OTEL_ENABLED") == "true"
}

// initOTel initializes OpenTelemetry TracerProvider for the plugin.
// Returns a shutdown function to flush pending spans.
// When MIRASTACK_OTEL_ENABLED != "true", returns a no-op shutdown.
func initOTel(ctx context.Context, pluginName string, logger *zap.Logger) (shutdown func(context.Context) error, err error) {
	if !otelEnabled() {
		logger.Debug("OTel tracing disabled for plugin")
		return noopOTelShutdown, nil
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = defaultAgentServiceName
		if pluginName != "" {
			serviceName = pluginName
		}
	}

	serviceVersion := pluginBuildVersion()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return noopOTelShutdown, fmt.Errorf("otel resource: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return noopOTelShutdown, fmt.Errorf("otel exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(otelSamplerRatio()))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OTel tracing enabled for plugin",
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
	)

	return tp.Shutdown, nil
}

func noopOTelShutdown(context.Context) error { return nil }

func pluginBuildVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 8 {
			return s.Value[:8]
		}
	}
	return "dev"
}

func otelSamplerRatio() float64 {
	v := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if v == "" {
		return 1.0
	}
	var ratio float64
	if _, err := fmt.Sscanf(v, "%f", &ratio); err != nil {
		return 1.0
	}
	if ratio < 0 || ratio > 1 {
		return 1.0
	}
	return ratio
}
