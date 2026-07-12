package telemetrycache

import (
	"context"
	"strings"
	"time"

	mirastack "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

func tenantID(ec *mirastack.EngineContext) string {
	if ec == nil {
		return ""
	}
	return strings.TrimSpace(ec.TenantID())
}

func cacheGet(ctx context.Context, ec *mirastack.EngineContext, key string) (string, bool, error) {
	if ec == nil {
		return "", false, nil
	}
	raw, err := ec.CacheGet(ctx, key)
	if err != nil {
		return "", false, err
	}
	if raw == "" {
		return "", false, nil
	}
	value, err := DecompressValue(raw)
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func cacheGetBatch(ctx context.Context, ec *mirastack.EngineContext, keys []string) (map[string]string, error) {
	out := map[string]string{}
	if ec == nil || len(keys) == 0 {
		return out, nil
	}
	entries, err := ec.CacheGetBatch(ctx, keys)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.Found || e.Value == "" {
			continue
		}
		value, decErr := DecompressValue(e.Value)
		if decErr != nil {
			continue
		}
		out[e.Key] = value
	}
	return out, nil
}

func cacheSet(ctx context.Context, ec *mirastack.EngineContext, key, value string, ttl time.Duration) error {
	if ec == nil {
		return nil
	}
	compressed, ok := CompressValue(value)
	if !ok {
		return nil
	}
	return ec.CacheSet(ctx, key, compressed, ttl)
}
