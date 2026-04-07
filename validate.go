package mirastack

import "fmt"

// validatePlugin checks that the plugin's Info and Schema comply with the
// type boundaries for an Agent plugin. Called by Serve() before the gRPC
// server starts listening — a failing validation causes a fatal log + exit.
func validatePlugin(info *PluginInfo) error {
	if info.Name == "" {
		return fmt.Errorf("mirastack: plugin name must not be empty")
	}
	if info.Version == "" {
		return fmt.Errorf("mirastack: plugin version must not be empty")
	}

	// Validate actions have required fields.
	for i, act := range info.Actions {
		if act.ID == "" {
			return fmt.Errorf("mirastack: action[%d] must have a non-empty ID", i)
		}
		if act.Description == "" {
			return fmt.Errorf("mirastack: action[%d] (%s) must have a description", i, act.ID)
		}
	}

	return nil
}
