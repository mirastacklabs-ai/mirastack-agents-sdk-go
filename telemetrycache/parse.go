package telemetrycache

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type matrixEnvelope struct {
	Status string `json:"status,omitempty"`
	Data   struct {
		ResultType string `json:"resultType,omitempty"`
		Result     []struct {
			Metric map[string]any `json:"metric"`
			Values [][]any        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func parseMatrixResponse(raw []byte) ([]MatrixSeries, error) {
	var env matrixEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return parseMatrixEntries(env.Data.Result), nil
}

func parseStatsRangeResponse(raw []byte) ([]MatrixSeries, error) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	data, ok := top["data"]
	if !ok || data == nil {
		return nil, nil
	}
	switch t := data.(type) {
	case []any:
		return parseStatsSeriesArray(t), nil
	case map[string]any:
		if result, ok := t["result"].([]any); ok {
			return parseStatsSeriesArray(result), nil
		}
	}
	return nil, nil
}

func parseStatsSeriesArray(input []any) []MatrixSeries {
	out := make([]MatrixSeries, 0, len(input))
	for _, item := range input {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		metric := map[string]string{}
		if m, ok := row["metric"].(map[string]any); ok {
			for k, v := range m {
				metric[k] = toString(v)
			}
		}
		values := make([]MatrixPoint, 0)
		if arr, ok := row["values"].([]any); ok {
			for _, p := range arr {
				pair, ok := p.([]any)
				if !ok || len(pair) < 2 {
					continue
				}
				ts, ok := toEpochSec(pair[0])
				if !ok {
					continue
				}
				values = append(values, MatrixPoint{
					Ts:    ts,
					Value: toString(pair[1]),
				})
			}
		}
		out = append(out, MatrixSeries{Metric: metric, Values: values})
	}
	return out
}

func parseHitsResponse(raw []byte) ([]HitsSeries, error) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	src := []any{}
	if hits, ok := top["hits"].([]any); ok {
		src = hits
	} else if data, ok := top["data"].([]any); ok {
		src = data
	} else {
		return nil, nil
	}
	out := make([]HitsSeries, 0, len(src))
	for _, item := range src {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var hs HitsSeries
		hs.Fields = row["fields"]
		if timestamps, ok := row["timestamps"].([]any); ok {
			for _, ts := range timestamps {
				hs.Timestamps = append(hs.Timestamps, toString(ts))
			}
		}
		if values, ok := row["values"].([]any); ok {
			for _, v := range values {
				f, _ := strconv.ParseFloat(toString(v), 64)
				hs.Values = append(hs.Values, f)
			}
		}
		out = append(out, hs)
	}
	return out, nil
}

func parseTracesResponse(raw []byte) ([]map[string]any, error) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	data, ok := top["data"].([]any)
	if !ok {
		return nil, nil
	}
	out := make([]map[string]any, 0, len(data))
	for _, d := range data {
		row, ok := d.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func buildMatrixResponse(series []MatrixSeries) ([]byte, error) {
	type wireSeries struct {
		Metric map[string]string `json:"metric"`
		Values [][2]any          `json:"values"`
	}
	wire := make([]wireSeries, 0, len(series))
	for _, s := range series {
		vals := make([][2]any, 0, len(s.Values))
		for _, p := range s.Values {
			vals = append(vals, [2]any{p.Ts, p.Value})
		}
		wire = append(wire, wireSeries{
			Metric: s.Metric,
			Values: vals,
		})
	}
	resp := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "matrix",
			"result":     wire,
		},
	}
	return json.Marshal(resp)
}

func buildHitsResponse(series []HitsSeries) ([]byte, error) {
	resp := map[string]any{
		"hits": series,
	}
	return json.Marshal(resp)
}

func buildTracesResponse(traces []map[string]any) ([]byte, error) {
	resp := map[string]any{
		"data":  traces,
		"total": len(traces),
	}
	return json.Marshal(resp)
}

func parseMatrixEntries(input []struct {
	Metric map[string]any `json:"metric"`
	Values [][]any        `json:"values"`
}) []MatrixSeries {
	out := make([]MatrixSeries, 0, len(input))
	for _, item := range input {
		metric := make(map[string]string, len(item.Metric))
		for k, v := range item.Metric {
			metric[k] = toString(v)
		}
		values := make([]MatrixPoint, 0, len(item.Values))
		for _, pair := range item.Values {
			if len(pair) < 2 {
				continue
			}
			ts, ok := toEpochSec(pair[0])
			if !ok {
				continue
			}
			values = append(values, MatrixPoint{Ts: ts, Value: toString(pair[1])})
		}
		out = append(out, MatrixSeries{
			Metric: metric,
			Values: values,
		})
	}
	return out
}

func toEpochSec(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return n, true
		}
		f, err := t.Float64()
		if err != nil {
			return 0, false
		}
		return int64(f), true
	case string:
		t = strings.TrimSpace(t)
		if t == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n, true
		}
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		return int64(f), true
	default:
		return 0, false
	}
}

func alignRangeForKey(startSec, endSec int64, chunkSizeSec int64) (int64, int64) {
	if chunkSizeSec <= 0 {
		return startSec, endSec
	}
	aStart := (startSec / chunkSizeSec) * chunkSizeSec
	aEnd := endSec
	if endSec%chunkSizeSec != 0 {
		aEnd = ((endSec / chunkSizeSec) + 1) * chunkSizeSec
	}
	return aStart, aEnd
}

func parseStepOrDefault(step, fallback string) (string, int64, error) {
	s := strings.TrimSpace(step)
	if s == "" {
		s = fallback
	}
	sec := stepToSeconds(s)
	if sec <= 0 {
		return "", 0, fmt.Errorf("invalid step %q", s)
	}
	return s, sec, nil
}
