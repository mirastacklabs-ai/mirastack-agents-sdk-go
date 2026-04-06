// Package datetimeutils provides time format conversion functions for MIRASTACK plugins.
//
// The engine parses user time expressions and delivers pre-resolved UTC epoch
// milliseconds in the TimeRange proto message. Plugins use this package to
// convert those epochs into the format their backend expects.
//
// This package is purely a formatter — it never parses user input.
// Parsing is the engine's responsibility (internal/datetimeutils).
//
// Usage in a plugin:
//
//	import "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/datetimeutils"
//
//	// From ExecuteRequest.TimeRange.StartEpochMs
//	promQL := datetimeutils.FormatEpochSeconds(startMs)     // → "1743580800"
//	rfc3339 := datetimeutils.FormatRFC3339(startMs)         // → "2026-04-02T00:00:00Z"
//	jaeger := datetimeutils.FormatEpochMicros(startMs)      // → "1743580800000000"
package datetimeutils

import (
	"fmt"
	"strconv"
	"time"
)

// FormatEpochSeconds converts UTC epoch milliseconds to epoch seconds string.
// Used by: Prometheus, VictoriaMetrics (query_range start/end).
func FormatEpochSeconds(epochMs int64) string {
	return strconv.FormatFloat(float64(epochMs)/1000.0, 'f', 3, 64)
}

// FormatEpochMillis converts UTC epoch milliseconds to epoch milliseconds string.
// Used by: Jaeger dependencies endpoint (endTs parameter).
func FormatEpochMillis(epochMs int64) string {
	return strconv.FormatInt(epochMs, 10)
}

// FormatEpochMicros converts UTC epoch milliseconds to epoch microseconds string.
// Used by: Jaeger trace search (start/end parameters).
func FormatEpochMicros(epochMs int64) string {
	return strconv.FormatInt(epochMs*1000, 10)
}

// FormatEpochNanos converts UTC epoch milliseconds to epoch nanoseconds string.
// Used by: OpenTelemetry native formats.
func FormatEpochNanos(epochMs int64) string {
	return strconv.FormatInt(epochMs*1_000_000, 10)
}

// FormatRFC3339 converts UTC epoch milliseconds to an RFC3339 string.
// Used by: REST APIs, JSON responses, general-purpose interchange.
func FormatRFC3339(epochMs int64) string {
	return time.UnixMilli(epochMs).UTC().Format(time.RFC3339)
}

// FormatRFC3339Nano converts UTC epoch milliseconds to an RFC3339Nano string.
// Used by: High-precision timestamp logging.
func FormatRFC3339Nano(epochMs int64) string {
	return time.UnixMilli(epochMs).UTC().Format(time.RFC3339Nano)
}

// FormatDate converts UTC epoch milliseconds to a date string "2006-01-02".
// Used by: Date-only queries, partition keys.
func FormatDate(epochMs int64) string {
	return time.UnixMilli(epochMs).UTC().Format("2006-01-02")
}

// FormatDateTime converts UTC epoch milliseconds to "2006-01-02 15:04:05".
// Used by: Human-readable logs, audit trails.
func FormatDateTime(epochMs int64) string {
	return time.UnixMilli(epochMs).UTC().Format("2006-01-02 15:04:05")
}

// FormatCustom converts UTC epoch milliseconds using a Go time layout string.
// Used by: Plugin-specific formats not covered by the standard functions.
func FormatCustom(epochMs int64, layout string) string {
	return time.UnixMilli(epochMs).UTC().Format(layout)
}

// FormatInTimezone converts UTC epoch milliseconds to RFC3339 in a specific timezone.
// The tz parameter must be a valid IANA timezone name (e.g., "Asia/Kolkata").
// Returns an error if the timezone is invalid.
func FormatInTimezone(epochMs int64, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	return time.UnixMilli(epochMs).In(loc).Format(time.RFC3339), nil
}

// FormatRelativeDuration converts epoch ms to a relative lookback string for backends
// that support relative time expressions.
// Returns: duration in milliseconds as string (e.g., "3600000" for 1 hour lookback).
func FormatLookbackMillis(startMs, endMs int64) string {
	return strconv.FormatInt(endMs-startMs, 10)
}

// ToTime converts UTC epoch milliseconds to a Go time.Time (UTC).
func ToTime(epochMs int64) time.Time {
	return time.UnixMilli(epochMs).UTC()
}

// FromTime converts a Go time.Time to UTC epoch milliseconds.
func FromTime(t time.Time) int64 {
	return t.UTC().UnixMilli()
}
