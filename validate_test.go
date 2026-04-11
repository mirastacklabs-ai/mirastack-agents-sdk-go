package mirastack

import (
	"strings"
	"testing"
)

// validAgentInfo returns a fully-populated PluginInfo that passes all quality gates.
func validAgentInfo() *PluginInfo {
	return &PluginInfo{
		Name:         "query_vmetrics",
		Version:      "1.0.0",
		Description:  "Query VictoriaMetrics TSDB for instant and range PromQL queries",
		DevOpsStages: []DevOpsStage{StageObserve},
		Actions: []Action{
			{
				ID:          "query_instant",
				Description: "Run an instant PromQL query",
				Permission:  PermissionRead,
				Stages:      []DevOpsStage{StageObserve},
			},
		},
	}
}

func TestValidatePlugin_ValidAgent(t *testing.T) {
	if err := validatePlugin(validAgentInfo()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlugin_ValidAgent_MultipleActions(t *testing.T) {
	info := validAgentInfo()
	info.Actions = append(info.Actions, Action{
		ID:          "query_range",
		Description: "Run a range PromQL query",
		Permission:  PermissionRead,
		Stages:      []DevOpsStage{StageObserve},
	})
	if err := validatePlugin(info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlugin_ValidAgent_WithConfigParams(t *testing.T) {
	info := validAgentInfo()
	info.ConfigParams = []ConfigParam{
		{Key: "url", Description: "VictoriaMetrics URL"},
		{Key: "api_key", Description: "API key for auth", IsSecret: true},
	}
	if err := validatePlugin(info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Plugin-level gates ────────────────────────────────────────────────────

func TestValidatePlugin_EmptyName(t *testing.T) {
	info := validAgentInfo()
	info.Name = ""
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "plugin name must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_EmptyVersion(t *testing.T) {
	info := validAgentInfo()
	info.Version = ""
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for empty version")
	}
	if !strings.Contains(err.Error(), "plugin version must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_EmptyDescription(t *testing.T) {
	info := validAgentInfo()
	info.Description = ""
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for empty description")
	}
	if !strings.Contains(err.Error(), "plugin description must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_WhitespaceDescription(t *testing.T) {
	info := validAgentInfo()
	info.Description = "   "
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for whitespace-only description")
	}
	if !strings.Contains(err.Error(), "plugin description must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_NoDevOpsStages(t *testing.T) {
	info := validAgentInfo()
	info.DevOpsStages = nil
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for missing DevOps stages")
	}
	if !strings.Contains(err.Error(), "at least one DevOps stage") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_NoActions(t *testing.T) {
	info := validAgentInfo()
	info.Actions = nil
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for zero actions")
	}
	if !strings.Contains(err.Error(), "at least one action") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ── Per-action gates ──────────────────────────────────────────────────────

func TestValidatePlugin_ActionMissingID(t *testing.T) {
	info := validAgentInfo()
	info.Actions = []Action{{ID: "", Description: "Something", Stages: []DevOpsStage{StageObserve}}}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for action with empty ID")
	}
	if !strings.Contains(err.Error(), "ID must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_ActionDuplicateID(t *testing.T) {
	info := validAgentInfo()
	info.Actions = []Action{
		{ID: "query", Description: "Query A", Stages: []DevOpsStage{StageObserve}},
		{ID: "query", Description: "Query B", Stages: []DevOpsStage{StageObserve}},
	}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for duplicate action ID")
	}
	if !strings.Contains(err.Error(), "duplicate action ID") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_ActionMissingDescription(t *testing.T) {
	info := validAgentInfo()
	info.Actions = []Action{{ID: "query_instant", Description: "", Stages: []DevOpsStage{StageObserve}}}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for action with empty description")
	}
	if !strings.Contains(err.Error(), "description must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_ActionMissingStages(t *testing.T) {
	info := validAgentInfo()
	info.Actions = []Action{{ID: "query_instant", Description: "Query metrics"}}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for action with no stages")
	}
	if !strings.Contains(err.Error(), "at least one DevOps stage") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ── ConfigParam gates ─────────────────────────────────────────────────────

func TestValidatePlugin_ConfigParamEmptyKey(t *testing.T) {
	info := validAgentInfo()
	info.ConfigParams = []ConfigParam{{Key: "", Description: "some param"}}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for config param with empty key")
	}
	if !strings.Contains(err.Error(), "key must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_ConfigParamEmptyDescription(t *testing.T) {
	info := validAgentInfo()
	info.ConfigParams = []ConfigParam{{Key: "url", Description: ""}}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for config param with empty description")
	}
	if !strings.Contains(err.Error(), "config_param[0] (url): description must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ── Multiple errors ───────────────────────────────────────────────────────

func TestValidatePlugin_MultipleErrors(t *testing.T) {
	info := &PluginInfo{
		Name:    "",
		Version: "",
	}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for multiple violations")
	}
	s := err.Error()
	if !strings.Contains(s, "plugin name") {
		t.Error("expected name error in multi-error output")
	}
	if !strings.Contains(s, "plugin version") {
		t.Error("expected version error in multi-error output")
	}
	if !strings.Contains(s, "at least one action") {
		t.Error("expected actions error in multi-error output")
	}
}
