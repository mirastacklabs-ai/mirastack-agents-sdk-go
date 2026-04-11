package mirastack

import (
	"fmt"
	"strings"
)

// validatePlugin checks that the plugin's Info complies with the quality gates
// for an Agent plugin.  Called by Serve() before the gRPC server starts
// listening — a failing validation causes a fatal log + exit.
//
// Quality gates guarantee that every registered agent carries enough metadata
// for the engine's tool catalog, console, approval gates, and Lane 3 stage
// filtering to work correctly.
func validatePlugin(info *PluginInfo) error {
	var errs []string

	// ── Plugin-level gates ────────────────────────────────────────────────
	if info.Name == "" {
		errs = append(errs, "plugin name must not be empty")
	}
	if info.Version == "" {
		errs = append(errs, "plugin version must not be empty")
	}
	if strings.TrimSpace(info.Description) == "" {
		errs = append(errs, "plugin description must not be empty")
	}
	if len(info.DevOpsStages) == 0 {
		errs = append(errs, "plugin must declare at least one DevOps stage")
	}
	if len(info.Actions) == 0 {
		errs = append(errs, "agent must declare at least one action")
	}

	// ── Per-action gates ──────────────────────────────────────────────────
	seen := make(map[string]bool, len(info.Actions))
	for i, act := range info.Actions {
		if act.ID == "" {
			errs = append(errs, fmt.Sprintf("action[%d]: ID must not be empty", i))
			continue
		}
		if seen[act.ID] {
			errs = append(errs, fmt.Sprintf("action[%d]: duplicate action ID %q", i, act.ID))
		}
		seen[act.ID] = true

		if strings.TrimSpace(act.Description) == "" {
			errs = append(errs, fmt.Sprintf("action[%d] (%s): description must not be empty", i, act.ID))
		}
		if len(act.Stages) == 0 {
			errs = append(errs, fmt.Sprintf("action[%d] (%s): must declare at least one DevOps stage", i, act.ID))
		}
	}

	// ── ConfigParam gates (when declared) ─────────────────────────────────
	for i, cp := range info.ConfigParams {
		if cp.Key == "" {
			errs = append(errs, fmt.Sprintf("config_param[%d]: key must not be empty", i))
			continue
		}
		if strings.TrimSpace(cp.Description) == "" {
			errs = append(errs, fmt.Sprintf("config_param[%d] (%s): description must not be empty", i, cp.Key))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("mirastack: quality gate failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
