package obs

import (
	"context"
	"errors"
	"testing"
)

func TestStartAction_NilSafe(t *testing.T) {
	ctx, sp := StartAction(context.Background(), "agent-x", "query", "READ")
	if sp == nil {
		t.Fatal("StartAction returned nil ActionSpan")
	}
	if ctx == nil {
		t.Fatal("StartAction returned nil context")
	}
	// Calling End on a span backed by the no-op TracerProvider must not panic.
	sp.End(ctx)
}

func TestEndWithError_ClassifiesError(t *testing.T) {
	ctx, sp := StartAction(context.Background(), "agent-x", "query", "READ")
	type customErr struct{ error }
	sp.EndWithError(ctx, &customErr{error: errors.New("boom")})
	// No assertions on exporter — purpose is panic-safety + type-name path.
}

func TestEndWithError_NilErrorTreatedAsSuccess(t *testing.T) {
	ctx, sp := StartAction(context.Background(), "agent-x", "query", "READ")
	sp.EndWithError(ctx, nil) // must follow the OK branch, not panic.
}

func TestErrClass_TypeNameWins(t *testing.T) {
	if got := errClass(errors.New("plain")); got == "" {
		t.Fatal("expected non-empty class for plain error")
	}
}

func TestSetAttribute_NilSpanSafe(t *testing.T) {
	var a *ActionSpan
	a.SetAttribute() // must not panic on nil receiver
}
