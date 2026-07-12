package telemetrycache

import (
	"strconv"
	"strings"
	"time"
)

const (
	shortRangeThresholdSec = int64(600)
	timeBucketWidthSec     = int64(30)
)

var canonicalStepOrder = []string{"15s", "30s", "1m", "5m"}

// ChunkSizeForRange maps a query range to a chunk size in seconds.
func ChunkSizeForRange(rangeSec int64) int64 {
	switch {
	case rangeSec <= 600:
		return 60
	case rangeSec <= 3600:
		return 1800
	case rangeSec <= 21600:
		return 7200
	case rangeSec <= 86400:
		return 21600
	case rangeSec <= 172800:
		return 43200
	default:
		return 86400
	}
}

// CanonicalStepForChunkSize maps chunk size tiers to canonical query steps.
func CanonicalStepForChunkSize(chunkSec int64) string {
	switch {
	case chunkSec <= 300:
		return "15s"
	case chunkSec <= 1800:
		return "30s"
	case chunkSec <= 7200:
		return "1m"
	default:
		return "5m"
	}
}

// AdaptiveStep picks a default step for the provided range in epoch ms.
func AdaptiveStep(startMs, endMs int64) string {
	if endMs <= startMs {
		return "1m"
	}
	rangeSec := (endMs - startMs) / 1000
	if rangeSec <= 0 {
		return "1m"
	}
	return CanonicalStepForChunkSize(ChunkSizeForRange(rangeSec))
}

func stepToSeconds(step string) int64 {
	step = strings.TrimSpace(step)
	if step == "" {
		return 0
	}
	if secs, err := strconv.ParseInt(step, 10, 64); err == nil && secs > 0 {
		return secs
	}
	d, err := time.ParseDuration(step)
	if err != nil || d <= 0 {
		return 0
	}
	return int64(d.Seconds())
}

func normalizeCanonicalStep(step string) string {
	secs := stepToSeconds(step)
	switch {
	case secs <= 0:
		return ""
	case secs <= 15:
		return "15s"
	case secs <= 30:
		return "30s"
	case secs <= 60:
		return "1m"
	default:
		return "5m"
	}
}

func nextCoarserCanonicalStep(step string) string {
	cur := normalizeCanonicalStep(step)
	for i, s := range canonicalStepOrder {
		if s == cur && i+1 < len(canonicalStepOrder) {
			return canonicalStepOrder[i+1]
		}
	}
	return ""
}
