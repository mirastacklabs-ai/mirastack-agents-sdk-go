package telemetrycache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Sha256_16 returns the first 16 hex chars of SHA-256(input).
func Sha256_16(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:16]
}

func rangeResultKey(backend, tenant, dsID, query string, alignedStart, alignedEnd int64, step string) string {
	payload := strings.Join([]string{
		query,
		strconv.FormatInt(alignedStart, 10),
		strconv.FormatInt(alignedEnd, 10),
		step,
	}, "|")
	return fmt.Sprintf("%s:%s:result:%s:%s", backend, tenant, dsID, Sha256_16(payload))
}

func rangeChunkKey(backend, tenant, dsID, query string, chunkStart, chunkEnd int64, step string) string {
	return fmt.Sprintf(
		"%s:%s:chunk:%s:%s:%d:%d:%s",
		backend,
		tenant,
		dsID,
		Sha256_16(query),
		chunkStart,
		chunkEnd,
		step,
	)
}

func instantKey(tenant, dsID, query string, quantizedSec int64) string {
	payload := query + "|" + strconv.FormatInt(quantizedSec, 10)
	return fmt.Sprintf("metrics:%s:instant:%s:%s", tenant, dsID, Sha256_16(payload))
}

func statsRangeResultKey(tenant, dsID, query string, alignedStart, alignedEnd int64, step string) string {
	payload := strings.Join([]string{
		query,
		strconv.FormatInt(alignedStart, 10),
		strconv.FormatInt(alignedEnd, 10),
		step,
	}, "|")
	return fmt.Sprintf("logsql:%s:range:result:%s:%s", tenant, dsID, Sha256_16(payload))
}

func statsRangeChunkKey(tenant, dsID, query string, chunkStart, chunkEnd int64, step string) string {
	return fmt.Sprintf(
		"logsql:%s:range:chunk:%s:%s:%d:%d:%s",
		tenant,
		dsID,
		Sha256_16(query),
		chunkStart,
		chunkEnd,
		step,
	)
}

func hitsResultKey(tenant, dsID, query, fields string, alignedStart, alignedEnd int64, step string) string {
	payload := strings.Join([]string{
		query,
		strconv.FormatInt(alignedStart, 10),
		strconv.FormatInt(alignedEnd, 10),
		step,
		fields,
	}, "|")
	return fmt.Sprintf("logsql:%s:hits:result:%s:%s", tenant, dsID, Sha256_16(payload))
}

func hitsChunkKey(tenant, dsID, query, fields string, chunkStart, chunkEnd int64, step string) string {
	return fmt.Sprintf(
		"logsql:%s:hits:chunk:%s:%s:%d:%d:%s",
		tenant,
		dsID,
		Sha256_16(query+"|"+fields),
		chunkStart,
		chunkEnd,
		step,
	)
}

func tracesBucketResultKey(tenant, dsID string, opts TraceSearchOptions) string {
	payload := strings.Join([]string{
		opts.Service,
		opts.Operation,
		opts.Tags,
		opts.MinDuration,
		opts.MaxDuration,
		strconv.FormatInt(opts.StartUs, 10),
		strconv.FormatInt(opts.EndUs, 10),
		strconv.Itoa(opts.Limit),
		strconv.FormatInt(opts.BucketSizeMs, 10),
	}, "|")
	return fmt.Sprintf("traces:%s:bucketed:result:%s:%s", tenant, dsID, Sha256_16(payload))
}

func tracesBucketChunkKey(tenant, dsID string, opts TraceSearchOptions, bucketStartUs, bucketEndUs int64) string {
	payload := strings.Join([]string{
		opts.Service,
		opts.Operation,
		opts.Tags,
		opts.MinDuration,
		opts.MaxDuration,
		strconv.Itoa(opts.Limit),
	}, "|")
	return fmt.Sprintf(
		"traces:%s:bucketed:chunk:%s:%s:%d:%d",
		tenant,
		dsID,
		Sha256_16(payload),
		bucketStartUs,
		bucketEndUs,
	)
}
