// Package obs is the agent-side typed helper surface for OpenTelemetry
// instrumentation. Agent authors should call StartAction from every
// action handler so spans + metrics are emitted in a uniform shape
// across the entire MIRASTACK plugin ecosystem.
//
// The helpers own the semantic-convention vocabulary (attribute keys and
// instrument names) declared in
// developer/engine-agents-connectors-providers/notes/mirastack-observability-semconv.md
// §3.2 and §4.2. They are safe to use when MIRASTACK_OTEL_ENABLED is
// unset: the underlying TracerProvider / MeterProvider degrade to no-ops
// and every helper returns harmlessly.
package obs

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
	meterName  = "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

// ActionSpan wraps a started "agent.action" span. The zero value is
// unusable — callers must obtain one from StartAction.
type ActionSpan struct {
	span        trace.Span
	start       time.Time
	plugin      string
	action      string
	permission  string
	inputBytes  int64
	outputBytes int64
	counter     metric.Int64Counter
	latency     metric.Float64Histogram
}

// StartAction opens a span named "agent.action" with the canonical
// agent.* attributes stamped on it. The returned ActionSpan MUST be
// finalised exactly once via End or EndWithError.
//
//	ctx, span := obs.StartAction(ctx, "victoriametrics-query", "query", "READ")
//	defer span.End(ctx)
//	// … do work …
//	if err != nil { span.EndWithError(ctx, err); return … }
func StartAction(ctx context.Context, pluginName, actionID, permission string) (context.Context, *ActionSpan) {
	tr := otel.Tracer(tracerName)
	mt := otel.Meter(meterName)

	ctx, sp := tr.Start(ctx, "agent.action",
		trace.WithAttributes(
			attribute.String("agent.plugin", pluginName),
			attribute.String("agent.action", actionID),
			attribute.String("agent.permission", permission),
		),
	)

	counter, _ := mt.Int64Counter("mirastack_agent_actions_total",
		metric.WithDescription("Count of agent action invocations broken down by plugin/action/outcome."),
	)
	latency, _ := mt.Float64Histogram("mirastack_agent_action_latency_seconds",
		metric.WithDescription("Wall-clock latency of agent action invocations."),
		metric.WithUnit("s"),
	)

	return ctx, &ActionSpan{
		span:       sp,
		start:      time.Now(),
		plugin:     pluginName,
		action:     actionID,
		permission: permission,
		counter:    counter,
		latency:    latency,
	}
}

// SetAttribute stamps an arbitrary attribute on the underlying span.
// Use the canonical agent.* / llm.* keys from the semconv spec.
func (a *ActionSpan) SetAttribute(kv ...attribute.KeyValue) {
	if a == nil || a.span == nil {
		return
	}
	a.span.SetAttributes(kv...)
}

// SetIOBytes records the request/response payload sizes for stamping on
// the agent.action span. Either value may be zero when not yet known.
func (a *ActionSpan) SetIOBytes(input, output int64) {
	if a == nil {
		return
	}
	a.inputBytes = input
	a.outputBytes = output
}

// End finalises the span as a successful action.
func (a *ActionSpan) End(ctx context.Context) {
	a.finish(ctx, "ok", nil)
}

// EndWithError finalises the span as a failed action.
func (a *ActionSpan) EndWithError(ctx context.Context, err error) {
	if err == nil {
		a.finish(ctx, "ok", nil)
		return
	}
	a.finish(ctx, "error", err)
}

func (a *ActionSpan) finish(ctx context.Context, outcome string, err error) {
	if a == nil || a.span == nil {
		return
	}
	elapsedSec := time.Since(a.start).Seconds()
	a.span.SetAttributes(
		attribute.String("agent.outcome", outcome),
		attribute.Int64("agent.input_bytes", a.inputBytes),
		attribute.Int64("agent.output_bytes", a.outputBytes),
		attribute.Int64("agent.latency_ms", int64(elapsedSec*1000)),
	)
	mAttrs := metric.WithAttributes(
		attribute.String("plugin", a.plugin),
		attribute.String("action", a.action),
		attribute.String("outcome", outcome),
	)
	if a.counter != nil {
		a.counter.Add(ctx, 1, mAttrs)
	}
	if a.latency != nil {
		a.latency.Record(ctx, elapsedSec, mAttrs)
	}
	if err != nil {
		a.span.RecordError(err)
		// Use only error CLASS, never the raw message — error strings
		// frequently leak PII / secrets from upstream backends.
		a.span.SetStatus(otelStatusError(), errClass(err))
	}
	a.span.End()
}
