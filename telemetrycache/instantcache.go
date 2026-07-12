package telemetrycache

import (
	"context"
	"time"

	mirastack "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

// InstantQueryCached caches instant queries in 30-second time buckets.
func InstantQueryCached(
	ctx context.Context,
	ec *mirastack.EngineContext,
	dsID, query string,
	evalSec int64,
	fetch func() ([]byte, error),
) ([]byte, error) {
	if fetch == nil {
		return nil, nil
	}
	if evalSec <= 0 {
		evalSec = time.Now().UTC().Unix()
	}
	query = SanitizePromQL(query)
	if ec == nil || tenantID(ec) == "" {
		return fetch()
	}
	tenant := tenantID(ec)
	quantizedSec := (evalSec / timeBucketWidthSec) * timeBucketWidthSec
	key := instantKey(tenant, dsID, query, quantizedSec)
	if raw, found, getErr := cacheGet(ctx, ec, key); getErr == nil && found {
		return []byte(raw), nil
	} else if getErr != nil {
		return fetch()
	}

	out, err := fetch()
	if err != nil {
		return nil, err
	}
	_ = cacheSet(ctx, ec, key, string(out), instantTTL(quantizedSec))
	return out, nil
}
