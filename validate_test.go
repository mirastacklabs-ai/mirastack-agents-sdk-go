package mirastack

import (
	"strings"
	"testing"
)

func TestValidatePlugin_ValidAgent(t *testing.T) {
	info := &PluginInfo{
		Name:    "query_vmetrics",
		Version: "1.0.0",
		Actions: []Action{
			{
				ID:          "query_instant",
				Description: "Run an instant PromQL query",
				Permission:  PermissionRead,
			},
		},
	}
	if err := validatePlugin(info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlugin_EmptyName(t *testing.T) {
	info := &PluginInfo{
		Name:    "",
		Version: "1.0.0",
	}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "name must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_EmptyVersion(t *testing.T) {
	info := &PluginInfo{
		Name:    "query_vmetrics",
		Version: "",
	}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for empty version")
	}
	if !strings.Contains(err.Error(), "version must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_ActionMissingID(t *testing.T) {
	info := &PluginInfo{
		Name:    "query_vmetrics",
		Version: "1.0.0",
		Actions: []Action{
			{ID: "", Description: "Something"},
		},
	}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for action with empty ID")
	}
	if !strings.Contains(err.Error(), "non-empty ID") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_ActionMissingDescription(t *testing.T) {
	info := &PluginInfo{
		Name:    "query_vmetrics",
		Version: "1.0.0",
		Actions: []Action{
			{ID: "query_instant", Description: ""},
		},
	}
	err := validatePlugin(info)
	if err == nil {
		t.Fatal("expected error for action with empty description")
	}
	if !strings.Contains(err.Error(), "must have a description") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePlugin_NoActions(t *testing.T) {
	info := &PluginInfo{
		Name:    "service_graph",
		Version: "1.0.0",
	}
	// Agents with zero actions are valid (backward compat with intent-only plugins)
	if err := validatePlugin(info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePlugin_MultipleActions(t *testing.T) {
	info := &PluginInfo{
		Name:    "query_vmetrics",
		Version: "1.0.0",
		Actions: []Action{
			{ID: "query_instant", Description: "Instant query", Permission: PermissionRead},
			{ID: "query_range", Description: "Range query", Permission: PermissionRead},
		},
	}
	if err := validatePlugin(info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
