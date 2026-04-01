package mirastack

import (
	"context"
)

// Plugin is the interface every MIRASTACK plugin must implement.
// The engine communicates with plugins via gRPC, but this interface
// abstracts the transport so plugin authors write plain Go.
type Plugin interface {
	// Info returns static metadata: name, version, description, permissions,
	// DevOps stages, and intent patterns.
	Info() *PluginInfo

	// Schema returns the plugin's input/output parameter schema.
	Schema() *PluginSchema

	// Execute runs the plugin's action with the given parameters.
	Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)

	// HealthCheck returns nil if the plugin is healthy.
	HealthCheck(ctx context.Context) error

	// ConfigUpdated is called when the engine pushes new configuration.
	ConfigUpdated(ctx context.Context, config map[string]string) error
}

// PluginInfo holds static plugin metadata.
type PluginInfo struct {
	Name         string
	Version      string
	Description  string
	Permissions  []Permission
	DevOpsStages []DevOpsStage
	Intents      []IntentPattern
}

// PluginSchema describes inputs and outputs.
type PluginSchema struct {
	InputParams  []ParamSchema
	OutputParams []ParamSchema
}

// ParamSchema describes a single parameter.
type ParamSchema struct {
	Name        string
	Type        string // "string", "number", "boolean", "json"
	Required    bool
	Description string
}

// ExecuteRequest contains the parameters for a plugin execution.
type ExecuteRequest struct {
	ExecutionID string
	WorkflowID  string
	StepID      string
	Params      map[string]string
	Mode        ExecutionMode
}

// ExecuteResponse is what a plugin returns after execution.
type ExecuteResponse struct {
	Output map[string]string
	Logs   []string
}

// IntentPattern declares a natural language pattern the plugin handles.
type IntentPattern struct {
	Pattern     string
	Description string
	Priority    int32
}

// Permission levels for plugin actions.
type Permission int

const (
	PermissionRead   Permission = iota
	PermissionModify
	PermissionAdmin
)

// DevOpsStage represents a step in the DevOps infinity loop.
type DevOpsStage int

const (
	StagePlan DevOpsStage = iota
	StageCode
	StageBuild
	StageTest
	StageRelease
	StageDeploy
	StageOperate
	StageObserve
)

// ExecutionMode controls human-in-the-loop behavior.
type ExecutionMode int

const (
	ModeManual ExecutionMode = iota
	ModeGuided
	ModeAutonomous
)
