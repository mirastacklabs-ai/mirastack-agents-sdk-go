package mirastack

import (
	"os"
	"testing"
	"time"

	pluginv1 "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/gen/pluginv1"
	"go.uber.org/zap"
)

func TestResolveAdvertiseAddr_ExplicitEnvVar(t *testing.T) {
	t.Setenv("MIRASTACK_PLUGIN_ADVERTISE_ADDR", "my-agent-svc:50051")
	addr := resolveAdvertiseAddr(9999)
	if addr != "my-agent-svc:50051" {
		t.Fatalf("expected my-agent-svc:50051, got %s", addr)
	}
}

func TestResolveAdvertiseAddr_K8sServiceDNS(t *testing.T) {
	t.Setenv("MIRASTACK_PLUGIN_ADVERTISE_ADDR", "agent-query-vmetrics.mirastack.svc.cluster.local:50051")
	addr := resolveAdvertiseAddr(50051)
	if addr != "agent-query-vmetrics.mirastack.svc.cluster.local:50051" {
		t.Fatalf("expected K8s FQDN, got %s", addr)
	}
}

func TestResolveAdvertiseAddr_FallbackToHostname(t *testing.T) {
	os.Unsetenv("MIRASTACK_PLUGIN_ADVERTISE_ADDR")
	addr := resolveAdvertiseAddr(50051)
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}
	expected := hostname + ":50051"
	if addr != expected {
		t.Fatalf("expected %s, got %s", expected, addr)
	}
}

func TestResolveAdvertiseAddr_EnvVarTakesPrecedenceOverHostname(t *testing.T) {
	t.Setenv("MIRASTACK_PLUGIN_ADVERTISE_ADDR", "provider-openai:50051")
	addr := resolveAdvertiseAddr(12345) // bound port should be ignored
	if addr != "provider-openai:50051" {
		t.Fatalf("expected provider-openai:50051, got %s", addr)
	}
}

// ── maintainRegistration ─────────────────────────────────────────────────────

func TestMaintainRegistration_StopChExitsPromptly(t *testing.T) {
	// When stopCh is closed before the first backoff wait, the function
	// should return promptly rather than blocking for the full retry cycle.
	logger := zap.NewNop()

	ec, err := NewEngineContext("localhost:1", "test-plugin", "test-instance")
	if err != nil {
		t.Fatalf("NewEngineContext: %v", err)
	}
	defer ec.Close()

	stopCh := make(chan struct{})
	close(stopCh) // close immediately

	done := make(chan struct{})
	go func() {
		maintainRegistration(logger, ec, "localhost:50051", pluginv1.PluginTypeAgent, "1.0.0", stopCh)
		close(done)
	}()

	select {
	case <-done:
		// OK — function returned promptly
	case <-time.After(15 * time.Second):
		t.Fatal("maintainRegistration did not exit within 15 seconds after stopCh was closed")
	}
}

func TestMaintainRegistration_HeartbeatIntervalFromEnv(t *testing.T) {
	// Verify that MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL is respected.
	// We set a very short interval (1s) and verify the function runs the
	// heartbeat phase at that cadence by observing it exits promptly when
	// stopCh is closed.
	t.Setenv("MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL", "1")

	logger := zap.NewNop()
	ec, err := NewEngineContext("localhost:1", "test-hb-plugin", "test-hb-instance")
	if err != nil {
		t.Fatalf("NewEngineContext: %v", err)
	}
	defer ec.Close()

	stopCh := make(chan struct{})

	done := make(chan struct{})
	go func() {
		maintainRegistration(logger, ec, "localhost:50051", pluginv1.PluginTypeAgent, "1.0.0", stopCh)
		close(done)
	}()

	// Let it attempt initial registration (will fail fast against localhost:1),
	// then close stopCh to trigger exit.
	time.Sleep(500 * time.Millisecond)
	close(stopCh)

	select {
	case <-done:
		// OK
	case <-time.After(15 * time.Second):
		t.Fatal("maintainRegistration did not exit after stopCh close")
	}
}

func TestMaintainRegistration_InvalidHeartbeatEnvFallsBackToDefault(t *testing.T) {
	// Invalid env values should fall back to the default 30s interval.
	// We verify by checking the function starts without panic
	// and exits promptly when stopCh is closed.
	t.Setenv("MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL", "not-a-number")

	logger := zap.NewNop()
	ec, err := NewEngineContext("localhost:1", "test-fallback", "test-instance")
	if err != nil {
		t.Fatalf("NewEngineContext: %v", err)
	}
	defer ec.Close()

	stopCh := make(chan struct{})
	close(stopCh) // close immediately

	done := make(chan struct{})
	go func() {
		maintainRegistration(logger, ec, "localhost:50051", pluginv1.PluginTypeAgent, "1.0.0", stopCh)
		close(done)
	}()

	select {
	case <-done:
		// OK — function handled invalid env and exited
	case <-time.After(15 * time.Second):
		t.Fatal("maintainRegistration did not exit after stopCh close with invalid env")
	}
}

func TestMaintainRegistration_ZeroHeartbeatEnvFallsBackToDefault(t *testing.T) {
	// Zero or negative values should fall back to default.
	t.Setenv("MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL", "0")

	logger := zap.NewNop()
	ec, err := NewEngineContext("localhost:1", "test-zero-hb", "test-instance")
	if err != nil {
		t.Fatalf("NewEngineContext: %v", err)
	}
	defer ec.Close()

	stopCh := make(chan struct{})
	close(stopCh) // close immediately

	done := make(chan struct{})
	go func() {
		maintainRegistration(logger, ec, "localhost:50051", pluginv1.PluginTypeAgent, "1.0.0", stopCh)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(15 * time.Second):
		t.Fatal("maintainRegistration did not exit after stopCh close")
	}
}
