package telemetrycache

import (
	"context"
	"fmt"
	"sync"

	mirastack "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

// TraceSearchOptions captures the full trace search context for cache keys.
type TraceSearchOptions struct {
	Service      string
	Operation    string
	Tags         string
	MinDuration  string
	MaxDuration  string
	StartUs      int64
	EndUs        int64
	Limit        int
	BucketSizeMs int64
}

// BucketedSearchCached caches trace searches in microsecond buckets.
func BucketedSearchCached(
	ctx context.Context,
	ec *mirastack.EngineContext,
	dsID string,
	opts TraceSearchOptions,
	fetch func(startUs, endUs int64, limit int) ([]byte, error),
) ([]byte, error) {
	if fetch == nil {
		return nil, fmt.Errorf("trace fetch callback is required")
	}
	if opts.EndUs <= opts.StartUs {
		return fetch(opts.StartUs, opts.EndUs, opts.Limit)
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if ec == nil || tenantID(ec) == "" {
		return fetch(opts.StartUs, opts.EndUs, opts.Limit)
	}
	tenant := tenantID(ec)
	fullKey := tracesBucketResultKey(tenant, dsID, opts)
	if raw, found, getErr := cacheGet(ctx, ec, fullKey); getErr == nil && found {
		return []byte(raw), nil
	} else if getErr != nil {
		return fetch(opts.StartUs, opts.EndUs, opts.Limit)
	}

	buckets := ComputeTraceBuckets(opts.StartUs, opts.EndUs, opts.BucketSizeMs)
	if len(buckets) == 0 {
		raw, err := fetch(opts.StartUs, opts.EndUs, opts.Limit)
		if err != nil {
			return nil, err
		}
		_ = cacheSet(ctx, ec, fullKey, string(raw), fullResultTTL)
		return raw, nil
	}

	keys := make([]string, len(buckets))
	for i, b := range buckets {
		keys[i] = tracesBucketChunkKey(tenant, dsID, opts, b.StartUs, b.EndUs)
	}
	cached, batchErr := cacheGetBatch(ctx, ec, keys)
	if batchErr != nil {
		return fetch(opts.StartUs, opts.EndUs, opts.Limit)
	}

	traceSets := make([][]map[string]any, len(buckets))
	missing := make([]int, 0)
	for i, key := range keys {
		if raw, ok := cached[key]; ok {
			parsed, parseErr := parseTracesResponse([]byte(raw))
			if parseErr != nil {
				missing = append(missing, i)
				continue
			}
			traceSets[i] = parsed
			continue
		}
		missing = append(missing, i)
	}

	var fetchErr error
	var fetchErrMu sync.Mutex
	perBucketLimit := opts.Limit
	if perBucketLimit > maxLimitPerBucket {
		perBucketLimit = maxLimitPerBucket
	}
	runWithConcurrency(len(missing), maxConcurrentFetch, func(i int) {
		idx := missing[i]
		b := buckets[idx]
		raw, err := fetch(b.StartUs, b.EndUs, perBucketLimit)
		if err != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = err
			}
			fetchErrMu.Unlock()
			return
		}
		parsed, parseErr := parseTracesResponse(raw)
		if parseErr != nil {
			fetchErrMu.Lock()
			if fetchErr == nil {
				fetchErr = parseErr
			}
			fetchErrMu.Unlock()
			return
		}
		traceSets[idx] = parsed
		_ = cacheSet(ctx, ec, keys[idx], string(raw), TieredChunkTTLMicros(b.EndUs))
	})
	if fetchErr != nil {
		return fetch(opts.StartUs, opts.EndUs, opts.Limit)
	}

	merged := MergeTracesByID(traceSets...)
	finalRaw, marshalErr := buildTracesResponse(merged)
	if marshalErr != nil {
		return fetch(opts.StartUs, opts.EndUs, opts.Limit)
	}
	_ = cacheSet(ctx, ec, fullKey, string(finalRaw), fullResultTTL)
	return finalRaw, nil
}
