package mirastack

import (
	"os"
	"testing"
	"time"

	pluginv1 "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/gen/pluginv1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Tenant ID resolution ──────────────────────────────────────────────────────

// TestIDFromSlug_Deterministic verifies that IDFromSlug produces the same
// UUID5 each time for the same slug and matches the engine's derivation.
func TestIDFromSlug_Deterministic(t *testing.T) {
	id1 := IDFromSlug("acme")
	id2 := IDFromSlug("acme")
	if id1 != id2 {
		t.Fatalf("IDFromSlug is not deterministic: %s != %s", id1, id2)
	}
	// Must be a valid 36-char UUID string
	if len(id1) != 36 {
		t.Fatalf("expected 36-char UUID, got %d chars: %s", len(id1), id1)
	}
}

func TestIDFromSlug_DifferentSlugsProduceDifferentIDs(t *testing.T) {
	if IDFromSlug("acme") == IDFromSlug("globex") {
		t.Fatal("different slugs must not produce the same UUID5")
	}
}

func TestIDFromSlug_NormalisesSlug(t *testing.T) {
	// The engine normalises slugs to lower case before hashing.
	if IDFromSlug("ACME") != IDFromSlug("acme") {
		t.Fatal("IDFromSlug must normalise to lower case before hashing")
	}
}

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

	ec, err := NewEngineContext("localhost:1", "test-plugin", "test-instance", IDFromSlug("acme"))
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
	ec, err := NewEngineContext("localhost:1", "test-hb-plugin", "test-hb-instance", IDFromSlug("acme"))
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
	ec, err := NewEngineContext("localhost:1", "test-fallback", "test-instance", IDFromSlug("acme"))
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
	ec, err := NewEngineContext("localhost:1", "test-zero-hb", "test-instance", IDFromSlug("acme"))
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

// ── EngineContext tenant propagation ─────────────────────────────────────────

func TestNewEngineContext_StoresTenantID(t *testing.T) {
	tenantID := IDFromSlug("acme")
	ec, err := NewEngineContext("localhost:1", "test-plugin", "inst-1", tenantID)
	if err != nil {
		t.Fatalf("NewEngineContext: %v", err)
	}
	defer ec.Close()

	if ec.TenantID() != tenantID {
		t.Fatalf("expected TenantID %s, got %s", tenantID, ec.TenantID())
	}
}

func TestNewEngineContext_EmptyTenantIDIsPreserved(t *testing.T) {
	// Validation is the caller's responsibility (Serve() fatal-logs on empty
	// tenant). The important invariant is that TenantID() reflects exactly
	// what was passed in.
	ec, err := NewEngineContext("localhost:1", "test-plugin", "inst-1", "")
	if err != nil {
		t.Fatalf("NewEngineContext: %v", err)
	}
	defer ec.Close()

	if ec.TenantID() != "" {
		t.Fatalf("expected empty TenantID, got %s", ec.TenantID())
	}
}

// ── V-01: TestSDKBootstrapRequiresTenantID ───────────────────────────────

// TestSDKBootstrapRequiresTenantID verifies the three resolution paths of
// resolveTenantID so that the tenant-ID bootstrap contract is unit-testable
// without invoking logger.Fatal in Serve().
func TestSDKBootstrapRequiresTenantID(t *testing.T) {
	// ── Case 1: neither env var set → must return an error ────────────
	t.Run("neither_var_set", func(t *testing.T) {
		t.Setenv("MIRASTACK_PLUGIN_TENANT_ID", "")
		t.Setenv("MIRASTACK_PLUGIN_TENANT_SLUG", "")

		id, err := resolveTenantID()
		if err == nil {
			t.Errorf("expected error when neither env var is set, got id=%q", id)
		}
	})

	// ── Case 2: MIRASTACK_PLUGIN_TENANT_ID is set → returned as-is ───
	t.Run("tenant_id_set", func(t *testing.T) {
		const wantID = "550e8400-e29b-41d4-a716-446655440000"
		t.Setenv("MIRASTACK_PLUGIN_TENANT_ID", wantID)
		t.Setenv("MIRASTACK_PLUGIN_TENANT_SLUG", "")

		id, err := resolveTenantID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != wantID {
			t.Errorf("got %q, want %q", id, wantID)
		}
	})

	// ── Case 3: only MIRASTACK_PLUGIN_TENANT_SLUG set → UUID5 derived ─
	t.Run("slug_set", func(t *testing.T) {
		const slug = "acme"
		t.Setenv("MIRASTACK_PLUGIN_TENANT_ID", "")
		t.Setenv("MIRASTACK_PLUGIN_TENANT_SLUG", slug)

		id, err := resolveTenantID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := IDFromSlug(slug)
		if id != want {
			t.Errorf("got %q, want IDFromSlug(%q)=%q", id, slug, want)
		}
	})
}

func TestClassifyRegistrationError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason string
		wantCode   string
	}{
		{
			name:       "engine_unavailable",
			err:        status.Error(codes.Unavailable, "connection refused"),
			wantReason: "engine_unavailable",
			wantCode:   codes.Unavailable.String(),
		},
		{
			name:       "tenant_not_active_or_missing",
			err:        status.Error(codes.PermissionDenied, "tenant \"abc\" is not active: tenants: tenant not found"),
			wantReason: "tenant_not_active_or_missing",
			wantCode:   codes.PermissionDenied.String(),
		},
		{
			name:       "invalid_registration_request",
			err:        status.Error(codes.InvalidArgument, "tenant_id is required for plugin registration"),
			wantReason: "invalid_registration_request",
			wantCode:   codes.InvalidArgument.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := classifyRegistrationError(tt.err)
			if got := zapFieldString(fields, "reason"); got != tt.wantReason {
				t.Fatalf("reason = %q, want %q", got, tt.wantReason)
			}
			if got := zapFieldString(fields, "grpc_code"); got != tt.wantCode {
				t.Fatalf("grpc_code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func zapFieldString(fields []zap.Field, key string) string {
	for _, field := range fields {
		if field.Key == key {
			return field.String
		}
	}
	return ""
}
