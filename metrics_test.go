package mirastack

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestInitMeterProvider_DisabledByDefault(t *testing.T) {
	t.Setenv("MIRASTACK_OTEL_ENABLED", "")
	shutdown, err := initMeterProvider(context.Background(), "test-agent", zap.NewNop())
	if err != nil {
		t.Fatalf("initMeterProvider should never return an error when disabled, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must be non-nil even when disabled")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown returned error: %v", err)
	}
}

func TestInitMeterProvider_NoopShutdownReusable(t *testing.T) {
	// Two consecutive disabled inits must both yield safe shutdowns —
	// guards against accidental global-state mutations between calls.
	for i := 0; i < 2; i++ {
		shutdown, _ := initMeterProvider(context.Background(), "test-agent", zap.NewNop())
		if err := shutdown(context.Background()); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
}
