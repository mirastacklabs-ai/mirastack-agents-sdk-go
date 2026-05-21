// otlp_endpoint.go provides robust parsing of the OTLP exporter endpoint and
// insecure-flag environment variables.
//
// The upstream OpenTelemetry Go SDK feeds OTEL_EXPORTER_OTLP_ENDPOINT into
// net/url.Parse. A bare "host:port" value (e.g. "otel-collector:4317") is
// parsed as scheme=host / opaque=port with an empty Host, which collapses the
// gRPC dial target to "" and surfaces as
//
//	traces export: failed to exit idle mode: dns resolver: missing address
//
// Conversely, prefixing the endpoint with "https://" or "grpc://" trips the
// SDK's default-secure path; it works only if OTEL_EXPORTER_OTLP_INSECURE=true
// is processed *after* the URL-derived secure option, which is fragile.
//
// We side-step both pitfalls by parsing the endpoint ourselves and feeding
// host:port directly to otlptracegrpc.WithEndpoint / otlpmetricgrpc.WithEndpoint
// (their canonical input form), and toggling WithInsecure() explicitly from
// the dedicated env var. The upstream env reader still runs and supplies any
// other config (headers, timeouts, compression).
package mirastack

import (
	"net/url"
	"os"
	"strings"
)

// traceOTLPEndpointFromEnv returns the gRPC dial target for OTLP traces in
// host:port form, normalised from any of the supported env vars. The second
// return is false when no endpoint is configured (caller leaves it to the
// upstream env reader / SDK default).
func traceOTLPEndpointFromEnv() (string, bool) {
	return firstNormalizedEndpointFromEnv(
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	)
}

// metricsOTLPEndpointFromEnv returns the gRPC dial target for OTLP metrics in
// host:port form. Mirrors traceOTLPEndpointFromEnv with the metrics-specific
// override env var taking precedence.
func metricsOTLPEndpointFromEnv() (string, bool) {
	return firstNormalizedEndpointFromEnv(
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	)
}

// traceOTLPInsecureFromEnv reports whether the OTLP trace exporter should use
// a plaintext gRPC connection. Trace-specific override wins over the global.
func traceOTLPInsecureFromEnv() bool {
	return firstTruthyEnv(
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE",
		"OTEL_EXPORTER_OTLP_INSECURE",
	)
}

// metricsOTLPInsecureFromEnv mirrors traceOTLPInsecureFromEnv for metrics.
func metricsOTLPInsecureFromEnv() bool {
	return firstTruthyEnv(
		"OTEL_EXPORTER_OTLP_METRICS_INSECURE",
		"OTEL_EXPORTER_OTLP_INSECURE",
	)
}

// firstNormalizedEndpointFromEnv walks the supplied env-var keys in order and
// returns the first non-empty value, normalised to host:port form.
func firstNormalizedEndpointFromEnv(keys ...string) (string, bool) {
	for _, k := range keys {
		raw, ok := lookupNonEmptyEnv(k)
		if !ok {
			continue
		}
		ep := normalizeOTLPEndpoint(raw)
		if ep == "" {
			continue
		}
		return ep, true
	}
	return "", false
}

// normalizeOTLPEndpoint strips any URL scheme (http://, https://, grpc://,
// grpcs://, unix://) and returns a bare host:port string suitable for
// otlptracegrpc.WithEndpoint / otlpmetricgrpc.WithEndpoint.
//
// Inputs already in host:port form are returned as-is. Surrounding whitespace
// is trimmed. Trailing slashes are removed.
func normalizeOTLPEndpoint(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// If the value parses as a URL with an explicit scheme, prefer the
	// Host field — that yields the canonical host:port even when the
	// caller wrote https:// or grpc://.
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			host := u.Host
			if u.Path != "" && u.Path != "/" {
				host = host + strings.TrimRight(u.Path, "/")
			}
			return host
		}
	}
	// Fall through: bare host:port (or host) — strip a trailing slash and
	// return.
	return strings.TrimRight(s, "/")
}

// lookupNonEmptyEnv returns (value, true) only when the env var is set AND
// non-empty after trimming whitespace.
func lookupNonEmptyEnv(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	if strings.TrimSpace(v) == "" {
		return "", false
	}
	return v, true
}

// firstTruthyEnv returns true when any of the supplied env vars is set to a
// recognised truthy value ("true", "1", "yes", "on" — case-insensitive).
func firstTruthyEnv(keys ...string) bool {
	for _, k := range keys {
		v, ok := lookupNonEmptyEnv(k)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return false
}
