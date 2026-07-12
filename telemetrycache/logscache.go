package telemetrycache

import (
	"context"
	"fmt"
	"strings"
	"sync"

	mirastack "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

// StatsRangeCached caches VictoriaLogs stats_query_range responses.
func StatsRangeCached(
	ctx context.Context,
	ec *mirastack.EngineContext,
	dsID, query string,
	startSec, endSec int64,
	userStep string,
	fetch func(cStart, cEnd int64, step string) ([]byte, error),
) ([]byte, error) {
	if fetch == nil {
		return nil, fmt.Errorf("stats range fetch callback is required")
	}
	query = SanitizePromQL(query)
	if ec == nil || tenantID(ec) == "" {
		return fetch(startSec, endSec, userStep)
	}
	tenant := tenantID(ec)
	rangeSec := endSec - startSec
	chunkSizeSec := ChunkSizeForRange(rangeSec)
	canonicalStep := CanonicalStepForChunkSize(chunkSizeSec)
	effectiveStep, effectiveStepSec, err := parseStepOrDefault(userStep, canonicalStep)
	if err != nil {
		effectiveStep = canonicalStep
		effectiveStepSec = stepToSeconds(canonicalStep)
	}
	fetchStep := effectiveStep
	fetchStepSec := effectiveStepSec
	canonicalSec := stepToSeconds(canonicalStep)
	if effectiveStepSec > canonicalSec {
		fetchStep = canonicalStep
		fetchStepSec = canonicalSec
	}

	aStart, aEnd := alignRangeForKey(startSec, endSec, chunkSizeSec)
	fullKey := statsRangeResultKey(tenant, dsID, query, aStart, aEnd, effectiveStep)
	if raw, found, getErr := cacheGet(ctx, ec, fullKey); getErr == nil && found {
		return []byte(raw), nil
	} else if getErr != nil {
		return fetch(startSec, endSec, effectiveStep)
	}

	if rangeSec <= shortRangeThresholdSec {
		raw, fetchErr := fetch(startSec, endSec, effectiveStep)
		if fetchErr != nil {
			return nil, fetchErr
		}
		_ = cacheSet(ctx, ec, fullKey, string(raw), fullResultTTL)
		return raw, nil
	}

	chunks := ComputeChunks(startSec, endSec)
	keys := make([]string, len(chunks))
	for i, c := range chunks {
		keys[i] = statsRangeChunkKey(tenant, dsID, query, c.Start, c.End, fetchStep)
	}
	cached, batchErr := cacheGetBatch(ctx, ec, keys)
	if batchErr != nil {
		return fetch(startSec, endSec, effectiveStep)
	}

	seriesSets := make([][]MatrixSeries, len(chunks))
	missing := make([]int, 0)
	for i, key := range keys {
		if raw, ok := cached[key]; ok {
			parsed, parseErr := parseStatsRangeResponse([]byte(raw))
			if parseErr != nil {
				missing = append(missing, i)
				continue
			}
			seriesSets[i] = parsed
			continue
		}
		missing = append(missing, i)
	}

	var fetchErr error
	var fetchErrMu sync.Mutex
	runWithConcurrency(len(missing), maxConcurrentFetch, func(i int) {
		idx := missing[i]
		chunk := chunks[idx]
		raw, err := fetch(chunk.Start, chunk.End, fetchStep)
		if err != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = err
			}
			fetchErrMu.Unlock()
			return
		}
		parsed, parseErr := parseStatsRangeResponse(raw)
		if parseErr != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = parseErr
			}
			fetchErrMu.Unlock()
			return
		}
		seriesSets[idx] = parsed
		_ = cacheSet(ctx, ec, keys[idx], string(raw), TieredChunkTTL(chunk.End))
	})
	if fetchErr != nil {
		return fetch(startSec, endSec, effectiveStep)
	}

	merged := MergeSeries(seriesSets...)
	if effectiveStepSec > fetchStepSec {
		merged = Downsample(merged, effectiveStepSec)
	}
	finalRaw, marshalErr := buildMatrixResponse(merged)
	if marshalErr != nil {
		return fetch(startSec, endSec, effectiveStep)
	}
	_ = cacheSet(ctx, ec, fullKey, string(finalRaw), fullResultTTL)
	return finalRaw, nil
}

// HitsCached caches VictoriaLogs hits responses with additive merge/downsample.
func HitsCached(
	ctx context.Context,
	ec *mirastack.EngineContext,
	dsID, query, fields string,
	startSec, endSec int64,
	userStep string,
	fetch func(filteredQuery string, cStart, cEnd int64, step string) ([]byte, error),
) ([]byte, error) {
	if fetch == nil {
		return nil, fmt.Errorf("hits fetch callback is required")
	}
	query = SanitizePromQL(query)
	if query == "" {
		query = "*"
	}
	if ec == nil || tenantID(ec) == "" {
		return fetch(withTimeFilter(query, startSec, endSec), startSec, endSec, userStep)
	}
	tenant := tenantID(ec)
	rangeSec := endSec - startSec
	chunkSizeSec := ChunkSizeForRange(rangeSec)
	canonicalStep := CanonicalStepForChunkSize(chunkSizeSec)
	effectiveStep, effectiveStepSec, err := parseStepOrDefault(userStep, canonicalStep)
	if err != nil {
		effectiveStep = canonicalStep
		effectiveStepSec = stepToSeconds(canonicalStep)
	}
	fetchStep := effectiveStep
	fetchStepSec := effectiveStepSec
	canonicalSec := stepToSeconds(canonicalStep)
	if effectiveStepSec > canonicalSec {
		fetchStep = canonicalStep
		fetchStepSec = canonicalSec
	}

	aStart, aEnd := alignRangeForKey(startSec, endSec, chunkSizeSec)
	fullKey := hitsResultKey(tenant, dsID, query, fields, aStart, aEnd, effectiveStep)
	if raw, found, getErr := cacheGet(ctx, ec, fullKey); getErr == nil && found {
		return []byte(raw), nil
	} else if getErr != nil {
		return fetch(withTimeFilter(query, startSec, endSec), startSec, endSec, effectiveStep)
	}

	chunks := ComputeChunks(startSec, endSec)
	keys := make([]string, len(chunks))
	for i, c := range chunks {
		keys[i] = hitsChunkKey(tenant, dsID, query, fields, c.Start, c.End, fetchStep)
	}
	cached, batchErr := cacheGetBatch(ctx, ec, keys)
	if batchErr != nil {
		return fetch(withTimeFilter(query, startSec, endSec), startSec, endSec, effectiveStep)
	}

	hitSets := make([][]HitsSeries, len(chunks))
	missing := make([]int, 0)
	for i, key := range keys {
		if raw, ok := cached[key]; ok {
			parsed, parseErr := parseHitsResponse([]byte(raw))
			if parseErr != nil {
				missing = append(missing, i)
				continue
			}
			hitSets[i] = parsed
			continue
		}
		missing = append(missing, i)
	}

	var fetchErr error
	var fetchErrMu sync.Mutex
	runWithConcurrency(len(missing), maxConcurrentFetch, func(i int) {
		idx := missing[i]
		chunk := chunks[idx]
		filteredQuery := withTimeFilter(query, chunk.Start, chunk.End)
		raw, err := fetch(filteredQuery, chunk.Start, chunk.End, fetchStep)
		if err != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = err
			}
			fetchErrMu.Unlock()
			return
		}
		parsed, parseErr := parseHitsResponse(raw)
		if parseErr != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = parseErr
			}
			fetchErrMu.Unlock()
			return
		}
		hitSets[idx] = parsed
		_ = cacheSet(ctx, ec, keys[idx], string(raw), TieredChunkTTL(chunk.End))
	})
	if fetchErr != nil {
		return fetch(withTimeFilter(query, startSec, endSec), startSec, endSec, effectiveStep)
	}

	merged := MergeHits(hitSets...)
	if effectiveStepSec > fetchStepSec {
		merged = DownsampleHits(merged, effectiveStepSec)
	}
	finalRaw, marshalErr := buildHitsResponse(merged)
	if marshalErr != nil {
		return fetch(withTimeFilter(query, startSec, endSec), startSec, endSec, effectiveStep)
	}
	_ = cacheSet(ctx, ec, fullKey, string(finalRaw), fullResultTTL)
	return finalRaw, nil
}

func withTimeFilter(query string, startSec, endSec int64) string {
	base := strings.TrimSpace(query)
	if base == "" {
		base = "*"
	}
	return fmt.Sprintf("(%s) AND _time:[%d,%d]", base, startSec, endSec)
}
