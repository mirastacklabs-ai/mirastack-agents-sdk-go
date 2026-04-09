package mirastack

import (
	"os"
	"testing"
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
