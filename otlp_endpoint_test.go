package mirastack

import "testing"

func TestNormalizeOTLPEndpoint(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"otel-collector:4317", "otel-collector:4317"},
		{"otel-collector:4317/", "otel-collector:4317"},
		{"http://otel-collector:4317", "otel-collector:4317"},
		{"https://otel-collector:4317", "otel-collector:4317"},
		{"grpc://otel-collector:4317", "otel-collector:4317"},
		{"grpcs://otel-collector:4317", "otel-collector:4317"},
		{"http://otel-collector:4317/", "otel-collector:4317"},
		{"http://otel-collector:4317/v1/traces", "otel-collector:4317/v1/traces"},
		{"otel-collector", "otel-collector"},
		{"  http://otel-collector:4317  ", "otel-collector:4317"},
	}
	for _, c := range cases {
		got := normalizeOTLPEndpoint(c.in)
		if got != c.want {
			t.Errorf("normalizeOTLPEndpoint(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTraceOTLPEndpointFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if ep, ok := traceOTLPEndpointFromEnv(); ok {
		t.Fatalf("expected unset, got %q", ep)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "grpc://otel-collector:4317")
	ep, ok := traceOTLPEndpointFromEnv()
	if !ok || ep != "otel-collector:4317" {
		t.Fatalf("global endpoint: got (%q,%v)", ep, ok)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "https://traces.example:4317")
	ep, ok = traceOTLPEndpointFromEnv()
	if !ok || ep != "traces.example:4317" {
		t.Fatalf("traces override: got (%q,%v)", ep, ok)
	}
}

func TestMetricsOTLPEndpointFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	ep, ok := metricsOTLPEndpointFromEnv()
	if !ok || ep != "otel-collector:4317" {
		t.Fatalf("global endpoint: got (%q,%v)", ep, ok)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "metrics.example:4317")
	ep, ok = metricsOTLPEndpointFromEnv()
	if !ok || ep != "metrics.example:4317" {
		t.Fatalf("metrics override: got (%q,%v)", ep, ok)
	}
}

func TestInsecureFromEnv(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"true", true}, {"TRUE", true}, {"1", true}, {"yes", true}, {"on", true},
		{"false", false}, {"0", false}, {"no", false}, {"off", false}, {"", false},
	}
	for _, c := range cases {
		t.Setenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE", "")
		t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", c.val)
		if got := traceOTLPInsecureFromEnv(); got != c.want {
			t.Errorf("traceOTLPInsecureFromEnv(%q)=%v, want %v", c.val, got, c.want)
		}
		t.Setenv("OTEL_EXPORTER_OTLP_METRICS_INSECURE", "")
		if got := metricsOTLPInsecureFromEnv(); got != c.want {
			t.Errorf("metricsOTLPInsecureFromEnv(%q)=%v, want %v", c.val, got, c.want)
		}
	}

	// Trace-specific override beats the global.
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "false")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE", "true")
	if !traceOTLPInsecureFromEnv() {
		t.Error("trace-specific override should win")
	}
}
