package telemetrycache

import "time"

const (
	fullResultTTL      = 120 * time.Second
	liveChunkTTL       = 15 * time.Second
	recentChunkTTL     = 120 * time.Second
	historicalChunkTTL = 3600 * time.Second
	probeTTL           = 300 * time.Second
	maxConcurrentFetch = 6
	maxTraceBuckets    = 120
	maxLimitPerBucket  = 1000
	compressThreshold  = 1024
	maxCacheEntryBytes = 15 * 1024 * 1024
)

// TieredChunkTTL chooses cache TTL by chunk age for second-based timestamps.
func TieredChunkTTL(chunkEndSec int64) time.Duration {
	nowSec := time.Now().UTC().Unix()
	ageSec := nowSec - chunkEndSec
	switch {
	case ageSec < 60:
		return liveChunkTTL
	case ageSec < 300:
		return recentChunkTTL
	default:
		return historicalChunkTTL
	}
}

// TieredChunkTTLMicros chooses cache TTL by chunk age for microsecond timestamps.
func TieredChunkTTLMicros(chunkEndUs int64) time.Duration {
	nowUs := time.Now().UTC().UnixMicro()
	ageSec := (nowUs - chunkEndUs) / 1_000_000
	switch {
	case ageSec < 60:
		return liveChunkTTL
	case ageSec < 300:
		return recentChunkTTL
	default:
		return historicalChunkTTL
	}
}

func instantTTL(evalSec int64) time.Duration {
	if evalSec <= 0 {
		return recentChunkTTL
	}
	nowSec := time.Now().UTC().Unix()
	if nowSec-evalSec < 60 {
		return liveChunkTTL
	}
	return 300 * time.Second
}
