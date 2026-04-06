package mirastack

import (
	"encoding/json"
	"testing"
)

func TestRespondMap_BasicTypes(t *testing.T) {
	resp, err := RespondMap(map[string]any{
		"count":  42,
		"active": true,
		"name":   "web",
	})
	if err != nil {
		t.Fatalf("RespondMap error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(resp.Output, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["count"].(float64) != 42 {
		t.Errorf("count: got %v, want 42", out["count"])
	}
	if out["active"].(bool) != true {
		t.Errorf("active: got %v, want true", out["active"])
	}
	if out["name"].(string) != "web" {
		t.Errorf("name: got %v, want web", out["name"])
	}
}

func TestRespondJSON_Struct(t *testing.T) {
	type result struct {
		Message string `json:"message"`
		Score   float64 `json:"score"`
	}
	resp, err := RespondJSON(result{Message: "ok", Score: 0.95})
	if err != nil {
		t.Fatalf("RespondJSON error: %v", err)
	}

	var out result
	if err := json.Unmarshal(resp.Output, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Message != "ok" {
		t.Errorf("message: got %q, want %q", out.Message, "ok")
	}
	if out.Score != 0.95 {
		t.Errorf("score: got %v, want 0.95", out.Score)
	}
}

func TestRespondError_Format(t *testing.T) {
	resp, err := RespondError("something went wrong")
	if err != nil {
		t.Fatalf("RespondError error: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal(resp.Output, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["error"] != "something went wrong" {
		t.Errorf("error: got %q, want %q", out["error"], "something went wrong")
	}
}

func TestRespondRaw_Passthrough(t *testing.T) {
	raw := []byte(`{"already":"serialized","count":7}`)
	resp := RespondRaw(raw)
	if string(resp.Output) != string(raw) {
		t.Errorf("raw output mismatch: got %q, want %q", resp.Output, raw)
	}
}

func TestRespondMap_EmptyMap(t *testing.T) {
	resp, err := RespondMap(map[string]any{})
	if err != nil {
		t.Fatalf("RespondMap error: %v", err)
	}
	if string(resp.Output) != "{}" {
		t.Errorf("expected {}, got %q", resp.Output)
	}
}

func TestRespondJSON_Array(t *testing.T) {
	resp, err := RespondJSON([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("RespondJSON error: %v", err)
	}
	if string(resp.Output) != `["a","b","c"]` {
		t.Errorf("expected array, got %q", resp.Output)
	}
}
