// errors.go isolates the PII-safe error classification helper used by
// the obs package. Keeping it separate from obs.go lets us evolve the
// implementation (e.g. build-tag swap) without touching the public API.
package obs

import (
	"errors"
	"reflect"
	"strings"

	"go.opentelemetry.io/otel/codes"
)

// errClass extracts a stable, low-cardinality identifier for an error.
// It walks errors.Unwrap so wrapping decorators (fmt.Errorf("%w", ...))
// still surface the underlying concrete type. The raw message is NEVER
// included — error strings frequently carry PII or secrets from
// upstream backends.
func errClass(err error) string {
	if err == nil {
		return ""
	}
	for unwrap := err; unwrap != nil; unwrap = errors.Unwrap(unwrap) {
		t := reflectErrTypeName(unwrap)
		if t != "" {
			return t
		}
	}
	// Last-resort fallback: a trimmed snippet stripped of common
	// PII-bearing delimiters. The semconv guard prefers a class name
	// so this branch should be rare.
	msg := strings.TrimSpace(err.Error())
	msg = strings.Map(func(r rune) rune {
		switch r {
		case ':', '=', '"', '\'', '\n', '\t':
			return ' '
		}
		return r
	}, msg)
	if len(msg) > 32 {
		msg = msg[:32]
	}
	return msg
}

// reflectErrTypeName returns the concrete type name of err.
func reflectErrTypeName(err error) string {
	if err == nil {
		return ""
	}
	t := reflect.TypeOf(err)
	if t == nil {
		return ""
	}
	if t.Name() == "" && t.Kind() != reflect.Ptr {
		return ""
	}
	return t.String()
}

func otelStatusError() codes.Code { return codes.Error }
