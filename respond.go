package mirastack

import (
	"encoding/json"
	"fmt"
)

// RespondMap builds an ExecuteResponse from a flat key-value map.
// Values retain native Go types: int→JSON number, bool→JSON boolean.
//
//	return mirastack.RespondMap(map[string]any{
//	    "count": 42, "active": true, "name": "web-frontend",
//	})
//	// → {"count":42,"active":true,"name":"web-frontend"}
func RespondMap(m map[string]any) (*ExecuteResponse, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("mirastack.RespondMap: %w", err)
	}
	return &ExecuteResponse{Output: json.RawMessage(b)}, nil
}

// RespondJSON builds an ExecuteResponse from any JSON-serializable value.
// Use for structured output: nested objects, arrays, typed fields.
//
//	return mirastack.RespondJSON(RCAResult{
//	    RootCause:  "Memory leak",
//	    Confidence: 0.92,
//	})
func RespondJSON(v any) (*ExecuteResponse, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("mirastack.RespondJSON: %w", err)
	}
	return &ExecuteResponse{Output: json.RawMessage(b)}, nil
}

// RespondError is a convenience for returning a structured error.
// Produces {"error":"message"} with the given message.
func RespondError(msg string) (*ExecuteResponse, error) {
	return RespondMap(map[string]any{"error": msg})
}

// RespondRaw builds an ExecuteResponse from pre-serialized JSON bytes.
// Use when the output is already valid JSON (e.g. passthrough from a backend API).
// The caller is responsible for ensuring the bytes are valid JSON.
func RespondRaw(raw []byte) *ExecuteResponse {
	return &ExecuteResponse{Output: json.RawMessage(raw)}
}
