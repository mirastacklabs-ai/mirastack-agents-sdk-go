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
	Name            string
	Version         string
	Description     string
	Permissions     []Permission
	DevOpsStages    []DevOpsStage
	Intents         []IntentPattern
	Actions         []Action
	PromptTemplates []PromptTemplate
	ConfigParams    []ConfigParam // Config schema — declares what config keys the plugin accepts
}

// ConfigParam declares a configuration parameter the plugin accepts.
// The engine reads these from Info() during registration and seeds them
// into the unified settings store at plugin.{name}.{key}.
type ConfigParam struct {
	Key         string
	Type        string // "string", "int", "bool", "duration", "json"
	Required    bool
	Default     string
	Description string
	IsSecret    bool
}

// Action describes a discrete operation a plugin can perform.
// Actions are first-class entities that the engine maps intents to.
// A plugin may declare zero or more actions. When actions are declared,
// the engine registers their intents and routes matching user messages
// directly to the plugin with the action_id set on ExecuteRequest.
type Action struct {
	ID           string
	Description  string
	Permission   Permission
	Stages       []DevOpsStage
	Intents      []IntentPattern
	InputParams  []ParamSchema
	OutputParams []ParamSchema
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

// TimeRange represents a pre-parsed, absolute UTC time range delivered by the engine.
// Plugins receive this on ExecuteRequest and use SDK datetimeutils to format epochs
// for their specific backends (Prometheus, Jaeger, VictoriaLogs, etc.).
type TimeRange struct {
	StartEpochMs       int64  `json:"start_epoch_ms"`
	EndEpochMs         int64  `json:"end_epoch_ms"`
	Timezone           string `json:"timezone,omitempty"`
	OriginalExpression string `json:"original_expression,omitempty"`
}

// ExecuteRequest contains the parameters for a plugin execution.
type ExecuteRequest struct {
	ExecutionID string
	WorkflowID  string
	StepID      string
	ActionID    string
	Params      map[string]string
	Mode        ExecutionMode
	TimeRange   *TimeRange
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

// PromptTemplate is a prompt template contributed by a plugin.
// The engine auto-ingests these during plugin registration and makes them
// available through the Prompt Template Store for LLM interactions.
type PromptTemplate struct {
	Name        string // Unique template name (e.g. "query_metrics_context")
	Description string // Human-readable description
	Content     string // Go text/template content
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

// EngineAware is an optional interface plugins implement to receive the EngineContext.
// Plugins that need engine callbacks (cache, call other plugins, etc.) should implement this.
// Serve() will call SetEngineContext before any Execute calls.
type EngineAware interface {
	SetEngineContext(ec *EngineContext)
}
