package mirastack

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestOtelEnabled_False(t *testing.T) {
	os.Unsetenv("MIRASTACK_OTEL_ENABLED")
	if otelEnabled() {
		t.Error("expected otelEnabled() = false when env not set")
	}
}

func TestOtelEnabled_True(t *testing.T) {
	t.Setenv("MIRASTACK_OTEL_ENABLED", "true")
	if !otelEnabled() {
		t.Error("expected otelEnabled() = true")
	}
}

func TestInitOTel_Disabled(t *testing.T) {
	os.Unsetenv("MIRASTACK_OTEL_ENABLED")
	shutdown, err := initOTel(context.Background(), "test-plugin", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestInitOTel_Enabled(t *testing.T) {
	t.Setenv("MIRASTACK_OTEL_ENABLED", "true")
	// Use a non-routable endpoint so exporter creation succeeds but won't actually export
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")

	shutdown, err := initOTel(context.Background(), "test-plugin", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown")
	}
	// Shutdown should not error even with no spans exported
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPluginBuildVersion_NonEmpty(t *testing.T) {
	v := pluginBuildVersion()
	if v == "" {
		t.Error("expected non-empty build version")
	}
}

func TestOtelSamplerRatio_Default(t *testing.T) {
	os.Unsetenv("OTEL_TRACES_SAMPLER_ARG")
	if ratio := otelSamplerRatio(); ratio != 1.0 {
		t.Errorf("expected 1.0, got %f", ratio)
	}
}

func TestOtelSamplerRatio_Valid(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.5")
	if ratio := otelSamplerRatio(); ratio != 0.5 {
		t.Errorf("expected 0.5, got %f", ratio)
	}
}

func TestOtelSamplerRatio_Invalid(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "abc")
	if ratio := otelSamplerRatio(); ratio != 1.0 {
		t.Errorf("expected 1.0 for invalid, got %f", ratio)
	}
}

func TestNoopOTelShutdown(t *testing.T) {
	if err := noopOTelShutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
