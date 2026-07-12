package telemetrycache

import (
	"context"
	"fmt"
	"sync"

	mirastack "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

// RangeQueryCached executes a chunked range query with cache-aside semantics.
// On cache callback failures, it degrades to a single direct fetch.
func RangeQueryCached(
	ctx context.Context,
	ec *mirastack.EngineContext,
	backend, dsID, query string,
	startSec, endSec int64,
	userStep string,
	fetch func(cStart, cEnd int64, step string) ([]byte, error),
) ([]byte, error) {
	return rangeQueryCached(ctx, ec, backend, dsID, query, startSec, endSec, userStep, fetch)
}

func rangeQueryCached(
	ctx context.Context,
	engineCtx *mirastack.EngineContext,
	backend, dsID, query string,
	startSec, endSec int64,
	userStep string,
	fetch func(cStart, cEnd int64, step string) ([]byte, error),
) ([]byte, error) {
	if fetch == nil {
		return nil, fmt.Errorf("range fetch callback is required")
	}
	if endSec <= startSec {
		return fetch(startSec, endSec, userStep)
	}

	query = SanitizePromQL(query)
	if engineCtx == nil {
		return fetch(startSec, endSec, userStep)
	}
	tenant := tenantID(engineCtx)
	if tenant == "" {
		return fetch(startSec, endSec, userStep)
	}

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
	fullKey := rangeResultKey(backend, tenant, dsID, query, aStart, aEnd, effectiveStep)
	if raw, found, getErr := cacheGet(ctx, engineCtx, fullKey); getErr == nil && found {
		return []byte(raw), nil
	} else if getErr != nil {
		return fetch(startSec, endSec, effectiveStep)
	}

	// For short windows, bypass chunk fan-out but still keep full-result cache.
	if rangeSec <= shortRangeThresholdSec {
		raw, fetchErr := fetch(startSec, endSec, effectiveStep)
		if fetchErr != nil {
			return nil, fetchErr
		}
		_ = cacheSet(ctx, engineCtx, fullKey, string(raw), fullResultTTL)
		return raw, nil
	}

	chunks := ComputeChunks(startSec, endSec)
	keys := make([]string, len(chunks))
	for i, c := range chunks {
		keys[i] = rangeChunkKey(backend, tenant, dsID, query, c.Start, c.End, fetchStep)
	}

	cached, batchErr := cacheGetBatch(ctx, engineCtx, keys)
	if batchErr != nil {
		return fetch(startSec, endSec, effectiveStep)
	}

	seriesSets := make([][]MatrixSeries, len(chunks))
	missing := make([]int, 0)
	for i, key := range keys {
		if raw, ok := cached[key]; ok {
			parsed, parseErr := parseMatrixResponse([]byte(raw))
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
		parsed, parseErr := parseMatrixResponse(raw)
		if parseErr != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = parseErr
			}
			fetchErrMu.Unlock()
			return
		}
		seriesSets[idx] = parsed
		_ = cacheSet(ctx, engineCtx, keys[idx], string(raw), TieredChunkTTL(chunk.End))
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
	_ = cacheSet(ctx, engineCtx, fullKey, string(finalRaw), fullResultTTL)
	return finalRaw, nil
}
