package telemetrycache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSanitizePromQL(t *testing.T) {
	in := "  \"rate(http_requests_total[5m])\",  "
	got := SanitizePromQL(in)
	if got != "rate(http_requests_total[5m])" {
		t.Fatalf("sanitize mismatch: %q", got)
	}
}

func TestAdaptiveStepTiers(t *testing.T) {
	start := int64(0)
	tests := []struct {
		name  string
		endMs int64
		want  string
	}{
		{name: "5m", endMs: 5 * 60 * 1000, want: "15s"},
		{name: "30m", endMs: 30 * 60 * 1000, want: "30s"},
		{name: "2h", endMs: 2 * 60 * 60 * 1000, want: "1m"},
		{name: "24h", endMs: 24 * 60 * 60 * 1000, want: "5m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AdaptiveStep(start, tt.endMs)
			if got != tt.want {
				t.Fatalf("AdaptiveStep() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputeChunksClockAligned(t *testing.T) {
	// 0..1800 sec => chunk size 300 sec.
	chunks := ComputeChunks(120, 1780)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	if chunks[0].Start%300 != 0 {
		t.Fatalf("expected aligned chunk start, got %d", chunks[0].Start)
	}
}

func TestComputeTraceBucketsAutoWiden(t *testing.T) {
	startUs := int64(0)
	endUs := int64(10 * 24 * 60 * 60 * 1_000_000)         // 10 days
	buckets := ComputeTraceBuckets(startUs, endUs, 1_000) // intentionally too fine
	if len(buckets) == 0 {
		t.Fatal("expected buckets")
	}
	if len(buckets) > maxTraceBuckets {
		t.Fatalf("expected <= %d buckets, got %d", maxTraceBuckets, len(buckets))
	}
}

func TestMergeSeriesDedupByTimestamp(t *testing.T) {
	a := []MatrixSeries{{
		Metric: map[string]string{"service": "checkout"},
		Values: []MatrixPoint{{Ts: 10, Value: "1"}, {Ts: 20, Value: "2"}},
	}}
	b := []MatrixSeries{{
		Metric: map[string]string{"service": "checkout"},
		Values: []MatrixPoint{{Ts: 20, Value: "2"}, {Ts: 30, Value: "3"}},
	}}
	merged := MergeSeries(a, b)
	if len(merged) != 1 {
		t.Fatalf("expected one merged series, got %d", len(merged))
	}
	if len(merged[0].Values) != 3 {
		t.Fatalf("expected 3 points, got %d", len(merged[0].Values))
	}
}

func TestDownsamplePickFirst(t *testing.T) {
	in := []MatrixSeries{{
		Metric: map[string]string{"service": "checkout"},
		Values: []MatrixPoint{
			{Ts: 0, Value: "1"},
			{Ts: 10, Value: "2"},
			{Ts: 20, Value: "3"},
			{Ts: 30, Value: "4"},
		},
	}}
	out := Downsample(in, 20)
	if len(out[0].Values) != 2 {
		t.Fatalf("expected 2 points after downsample, got %d", len(out[0].Values))
	}
	if out[0].Values[0].Ts != 0 || out[0].Values[1].Ts != 20 {
		t.Fatalf("unexpected sampled timestamps: %+v", out[0].Values)
	}
}

func TestMergeHitsAndDownsampleHitsAdditive(t *testing.T) {
	a := []HitsSeries{{
		Fields:     "service=checkout",
		Timestamps: []string{"100", "110"},
		Values:     []float64{1, 2},
	}}
	b := []HitsSeries{{
		Fields:     "service=checkout",
		Timestamps: []string{"110", "120"},
		Values:     []float64{3, 4},
	}}
	merged := MergeHits(a, b)
	if len(merged) != 1 {
		t.Fatalf("expected one merged hits series, got %d", len(merged))
	}
	if len(merged[0].Values) != 3 {
		t.Fatalf("expected 3 merged points, got %d", len(merged[0].Values))
	}
	down := DownsampleHits(merged, 20)
	if len(down[0].Values) != 2 {
		t.Fatalf("expected 2 downsampled buckets, got %d", len(down[0].Values))
	}
}

func TestMergeTracesByIDDedup(t *testing.T) {
	a := []map[string]any{{"traceID": "t1"}, {"traceID": "t2"}}
	b := []map[string]any{{"traceID": "t2"}, {"traceID": "t3"}}
	out := MergeTracesByID(a, b)
	if len(out) != 3 {
		t.Fatalf("expected 3 deduped traces, got %d", len(out))
	}
}

func TestTieredChunkTTLSeconds(t *testing.T) {
	now := time.Now().UTC().Unix()
	if got := TieredChunkTTL(now - 10); got != liveChunkTTL {
		t.Fatalf("expected live ttl, got %v", got)
	}
	if got := TieredChunkTTL(now - 120); got != recentChunkTTL {
		t.Fatalf("expected recent ttl, got %v", got)
	}
	if got := TieredChunkTTL(now - 900); got != historicalChunkTTL {
		t.Fatalf("expected historical ttl, got %v", got)
	}
}

func TestTieredChunkTTLMicros(t *testing.T) {
	nowUs := time.Now().UTC().UnixMicro()
	if got := TieredChunkTTLMicros(nowUs - 10_000_000); got != liveChunkTTL {
		t.Fatalf("expected live ttl, got %v", got)
	}
	if got := TieredChunkTTLMicros(nowUs - 120_000_000); got != recentChunkTTL {
		t.Fatalf("expected recent ttl, got %v", got)
	}
	if got := TieredChunkTTLMicros(nowUs - 900_000_000); got != historicalChunkTTL {
		t.Fatalf("expected historical ttl, got %v", got)
	}
}

func TestCompressRoundTrip(t *testing.T) {
	src := strings.Repeat("abcdefgh", 500)
	compressed, ok := CompressValue(src)
	if !ok {
		t.Fatal("expected compress ok")
	}
	got, err := DecompressValue(compressed)
	if err != nil {
		t.Fatalf("decompress error: %v", err)
	}
	if got != src {
		t.Fatal("round-trip mismatch")
	}
}

func TestWithStepRetry_422PointsRetriesOnce(t *testing.T) {
	calls := 0
	out, err := WithStepRetry(0, 3600, "15s", func(step string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, &HTTPStatusError{Code: 422, Body: "too many points for query"}
		}
		if step != "30s" {
			t.Fatalf("expected retry with 30s, got %q", step)
		}
		return []byte(`{"status":"ok"}`), nil
	})
	if err != nil {
		t.Fatalf("unexpected retry error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls, got %d", calls)
	}
	if string(out) == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestWithStepRetry_422ParseNoRetry(t *testing.T) {
	calls := 0
	_, err := WithStepRetry(0, 3600, "15s", func(step string) ([]byte, error) {
		calls++
		return nil, &HTTPStatusError{Code: 422, Body: "invalid expression syntax error"}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("parse error must not retry; calls=%d", calls)
	}
}

func TestRangeQueryCached_ECNilPassthrough(t *testing.T) {
	calls := 0
	raw, err := RangeQueryCached(
		context.Background(),
		nil,
		"metrics",
		"ds-1",
		"up",
		0,
		600,
		"15s",
		func(cStart, cEnd int64, step string) ([]byte, error) {
			calls++
			if step != "15s" {
				t.Fatalf("unexpected step: %q", step)
			}
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`), nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 passthrough call, got %d", calls)
	}
	if !json.Valid(raw) {
		t.Fatalf("expected valid json, got %q", string(raw))
	}
}

func TestRangeQueryCached_ECNilPropagatesBackendError(t *testing.T) {
	wantErr := errors.New("backend unavailable")
	_, err := RangeQueryCached(
		context.Background(),
		nil,
		"metrics",
		"ds-1",
		"up",
		0,
		600,
		"15s",
		func(cStart, cEnd int64, step string) ([]byte, error) {
			return nil, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected backend error propagation, got %v", err)
	}
}
