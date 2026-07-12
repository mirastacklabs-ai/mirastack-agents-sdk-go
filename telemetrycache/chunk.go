package telemetrycache

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Chunk is a clock-aligned second-based range segment.
type Chunk struct {
	Start int64
	End   int64
}

// Bucket is a clock-aligned microsecond-based trace search segment.
type Bucket struct {
	StartUs int64
	EndUs   int64
}

// MatrixPoint is a timestamp-value tuple for matrix-style series.
type MatrixPoint struct {
	Ts    int64
	Value string
}

// MatrixSeries is a canonical matrix series representation.
type MatrixSeries struct {
	Metric map[string]string
	Values []MatrixPoint
}

// HitsSeries represents VictoriaLogs hits response rows.
type HitsSeries struct {
	Fields     any       `json:"fields"`
	Timestamps []string  `json:"timestamps"`
	Values     []float64 `json:"values"`
	Total      float64   `json:"total,omitempty"`
}

// ComputeChunks returns aligned chunks for [startSec, endSec).
func ComputeChunks(startSec, endSec int64) []Chunk {
	if endSec <= startSec {
		return nil
	}
	chunkSize := ChunkSizeForRange(endSec - startSec)
	if chunkSize <= 0 {
		return nil
	}
	alignedStart := (startSec / chunkSize) * chunkSize
	chunks := make([]Chunk, 0, 16)
	for cursor := alignedStart; cursor < endSec; cursor += chunkSize {
		chunks = append(chunks, Chunk{Start: cursor, End: cursor + chunkSize})
	}
	return chunks
}

// SeriesKey returns a stable label key for dedup/merge.
func SeriesKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, "|")
}

// MergeSeries merges matrix series and deduplicates values by timestamp.
func MergeSeries(sets ...[]MatrixSeries) []MatrixSeries {
	type acc struct {
		metric map[string]string
		points map[int64]string
	}
	byKey := make(map[string]*acc)
	for _, set := range sets {
		for _, series := range set {
			key := SeriesKey(series.Metric)
			cur, ok := byKey[key]
			if !ok {
				metricCopy := make(map[string]string, len(series.Metric))
				for k, v := range series.Metric {
					metricCopy[k] = v
				}
				cur = &acc{metric: metricCopy, points: make(map[int64]string, len(series.Values))}
				byKey[key] = cur
			}
			for _, p := range series.Values {
				cur.points[p.Ts] = p.Value
			}
		}
	}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]MatrixSeries, 0, len(keys))
	for _, k := range keys {
		cur := byKey[k]
		timestamps := make([]int64, 0, len(cur.points))
		for ts := range cur.points {
			timestamps = append(timestamps, ts)
		}
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
		values := make([]MatrixPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			values = append(values, MatrixPoint{Ts: ts, Value: cur.points[ts]})
		}
		out = append(out, MatrixSeries{
			Metric: cur.metric,
			Values: values,
		})
	}
	return out
}

// Downsample picks the first sample in each target step bucket.
func Downsample(series []MatrixSeries, targetStepSec int64) []MatrixSeries {
	if targetStepSec <= 0 {
		return series
	}
	out := make([]MatrixSeries, 0, len(series))
	for _, s := range series {
		if len(s.Values) == 0 {
			out = append(out, s)
			continue
		}
		nextTs := s.Values[0].Ts
		filtered := make([]MatrixPoint, 0, len(s.Values))
		for _, p := range s.Values {
			if p.Ts >= nextTs {
				filtered = append(filtered, p)
				nextTs = p.Ts + targetStepSec
			}
		}
		out = append(out, MatrixSeries{
			Metric: s.Metric,
			Values: filtered,
		})
	}
	return out
}

// MergeHits additively merges hit series by fields+timestamp.
func MergeHits(sets ...[]HitsSeries) []HitsSeries {
	type acc struct {
		fields any
		values map[int64]float64
	}
	byKey := make(map[string]*acc)
	for _, set := range sets {
		for _, s := range set {
			key := hitsSeriesKey(s.Fields)
			cur, ok := byKey[key]
			if !ok {
				cur = &acc{fields: s.Fields, values: map[int64]float64{}}
				byKey[key] = cur
			}
			n := len(s.Timestamps)
			if len(s.Values) < n {
				n = len(s.Values)
			}
			for i := 0; i < n; i++ {
				tsSec, ok := timestampToEpochSec(s.Timestamps[i])
				if !ok {
					continue
				}
				cur.values[tsSec] += s.Values[i]
			}
		}
	}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]HitsSeries, 0, len(keys))
	for _, k := range keys {
		cur := byKey[k]
		times := make([]int64, 0, len(cur.values))
		total := 0.0
		for ts, v := range cur.values {
			times = append(times, ts)
			total += v
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		tsOut := make([]string, 0, len(times))
		valOut := make([]float64, 0, len(times))
		for _, ts := range times {
			tsOut = append(tsOut, strconv.FormatInt(ts, 10))
			valOut = append(valOut, cur.values[ts])
		}
		out = append(out, HitsSeries{
			Fields:     cur.fields,
			Timestamps: tsOut,
			Values:     valOut,
			Total:      total,
		})
	}
	return out
}

// DownsampleHits additively downsamples hits into target buckets.
func DownsampleHits(series []HitsSeries, targetStepSec int64) []HitsSeries {
	if targetStepSec <= 0 {
		return series
	}
	out := make([]HitsSeries, 0, len(series))
	for _, s := range series {
		buckets := make(map[int64]float64, len(s.Timestamps))
		n := len(s.Timestamps)
		if len(s.Values) < n {
			n = len(s.Values)
		}
		for i := 0; i < n; i++ {
			tsSec, ok := timestampToEpochSec(s.Timestamps[i])
			if !ok {
				continue
			}
			bucket := (tsSec / targetStepSec) * targetStepSec
			buckets[bucket] += s.Values[i]
		}
		times := make([]int64, 0, len(buckets))
		total := 0.0
		for ts, v := range buckets {
			times = append(times, ts)
			total += v
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		tsOut := make([]string, 0, len(times))
		valOut := make([]float64, 0, len(times))
		for _, ts := range times {
			tsOut = append(tsOut, strconv.FormatInt(ts, 10))
			valOut = append(valOut, buckets[ts])
		}
		out = append(out, HitsSeries{
			Fields:     s.Fields,
			Timestamps: tsOut,
			Values:     valOut,
			Total:      total,
		})
	}
	return out
}

// MergeTracesByID deduplicates traces from multiple buckets by traceID.
func MergeTracesByID(sets ...[]map[string]any) []map[string]any {
	byID := make(map[string]map[string]any)
	order := make([]string, 0)
	for _, set := range sets {
		for _, trace := range set {
			id := traceID(trace)
			if id == "" {
				continue
			}
			if _, exists := byID[id]; exists {
				continue
			}
			byID[id] = trace
			order = append(order, id)
		}
	}
	out := make([]map[string]any, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

// ComputeTraceBuckets returns aligned microsecond buckets for [startUs,endUs).
func ComputeTraceBuckets(startUs, endUs, bucketMs int64) []Bucket {
	if endUs <= startUs {
		return nil
	}
	rangeMs := (endUs - startUs) / 1000
	if rangeMs <= 0 {
		return nil
	}
	effectiveBucketMs := bucketMs
	if effectiveBucketMs <= 0 {
		effectiveBucketMs = autoTraceBucketSizeMs(rangeMs)
	}
	count := int64(math.Ceil(float64(rangeMs) / float64(effectiveBucketMs)))
	if count > maxTraceBuckets {
		effectiveBucketMs = int64(math.Ceil(float64(rangeMs) / float64(maxTraceBuckets)))
	}
	bucketUs := effectiveBucketMs * 1000
	alignedStart := (startUs / bucketUs) * bucketUs
	out := make([]Bucket, 0, 16)
	for cursor := alignedStart; cursor < endUs; cursor += bucketUs {
		out = append(out, Bucket{StartUs: cursor, EndUs: cursor + bucketUs})
	}
	return out
}

func autoTraceBucketSizeMs(rangeMs int64) int64 {
	switch {
	case rangeMs <= 5*60*1000:
		return 60 * 1000
	case rangeMs <= 30*60*1000:
		return 5 * 60 * 1000
	case rangeMs <= 2*60*60*1000:
		return 15 * 60 * 1000
	case rangeMs <= 6*60*60*1000:
		return 30 * 60 * 1000
	case rangeMs <= 24*60*60*1000:
		return 2 * 60 * 60 * 1000
	default:
		return 6 * 60 * 60 * 1000
	}
}

func hitsSeriesKey(fields any) string {
	switch v := fields.(type) {
	case string:
		return v
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+toString(v[k]))
		}
		return strings.Join(parts, "|")
	default:
		b, _ := json.Marshal(fields)
		return string(b)
	}
}

func timestampToEpochSec(ts string) (int64, bool) {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(ts, 64); err == nil {
		return int64(n), true
	}
	if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return parsed.Unix(), true
	}
	return 0, false
}

func traceID(trace map[string]any) string {
	if v, ok := trace["traceID"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	if v, ok := trace["traceId"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return ""
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
