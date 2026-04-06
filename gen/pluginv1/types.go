// Package pluginv1 provides the gRPC service types for the MIRASTACK plugin protocol.
// These types mirror the protobuf definitions in plugin.proto but are hand-written
// to avoid a protoc/buf dependency during early development. When buf generate is run,
// this package will be replaced by generated code.
package pluginv1

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

type Permission int32

const (
	PermissionUnspecified Permission = 0
	PermissionRead        Permission = 1
	PermissionModify      Permission = 2
	PermissionAdmin       Permission = 3
)

type DevOpsStage int32

const (
	DevOpsStageUnspecified DevOpsStage = 0
	DevOpsStagePlan       DevOpsStage = 1
	DevOpsStageCode       DevOpsStage = 2
	DevOpsStageBuild      DevOpsStage = 3
	DevOpsStageTest       DevOpsStage = 4
	DevOpsStageRelease    DevOpsStage = 5
	DevOpsStageDeploy     DevOpsStage = 6
	DevOpsStageOperate    DevOpsStage = 7
	DevOpsStageObserve    DevOpsStage = 8
)

type ExecutionMode int32

const (
	ExecutionModeUnspecified ExecutionMode = 0
	ExecutionModeManual     ExecutionMode = 1
	ExecutionModeGuided     ExecutionMode = 2
	ExecutionModeAutonomous ExecutionMode = 3
)

type PluginType int32

const (
	PluginTypeUnspecified PluginType = 0
	PluginTypeAgent      PluginType = 1
	PluginTypeProvider   PluginType = 2
	PluginTypeConnector  PluginType = 3
)

// ---------------------------------------------------------------------------
// PluginService Messages
// ---------------------------------------------------------------------------

type InfoRequest struct{}

type IntentPattern struct {
	Pattern     string  `json:"pattern"`
	Confidence  float32 `json:"confidence"`
	Description string  `json:"description,omitempty"`
	Priority    int32   `json:"priority,omitempty"`
}

// ActionDef describes a discrete operation a plugin can perform.
// Actions are first-class entities that the engine maps intents to.
type ActionDef struct {
	Id           string          `json:"id"`
	Description  string          `json:"description"`
	Permission   Permission      `json:"permission"`
	Stages       []DevOpsStage   `json:"stages"`
	Intents      []IntentPattern `json:"intents"`
	InputParams  []byte          `json:"input_params,omitempty"`  // JSON schema
	OutputParams []byte          `json:"output_params,omitempty"` // JSON schema
}

// ConfigParamSchema describes a configuration parameter declared by a plugin.
type ConfigParamSchema struct {
	Key         string `json:"key"`
	Type        string `json:"type"`        // "string", "int", "bool", "duration", "json"
	Required    bool   `json:"required"`
	Default     string `json:"default"`
	Description string `json:"description"`
	IsSecret    bool   `json:"is_secret"`
}

type InfoResponse struct {
	Name             string              `json:"name"`
	Version          string              `json:"version"`
	Description      string              `json:"description"`
	Type             PluginType          `json:"type"`
	Permission       Permission          `json:"permission"`
	DevopsStages     []DevOpsStage       `json:"devops_stages"`
	DefaultIntents   []IntentPattern     `json:"default_intents"`
	Actions          []ActionDef         `json:"actions,omitempty"`
	PromptTemplates  []PromptTemplateDef `json:"prompt_templates,omitempty"`
	ConfigSchema     []ConfigParamSchema `json:"config_schema,omitempty"`
	Metadata         map[string]string   `json:"metadata"`
	InstanceID       string              `json:"instance_id"`
}

// PromptTemplateDef describes a prompt template contributed by a plugin.
type PromptTemplateDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type GetSchemaRequest struct{}

type GetSchemaResponse struct {
	ParamsJsonSchema []byte      `json:"params_json_schema"`
	ResultJsonSchema []byte      `json:"result_json_schema"`
	Actions          []ActionDef `json:"actions,omitempty"`
}

// TimeRange represents a pre-parsed, absolute UTC time range.
// Mirrors the TimeRange message in plugin.proto.
type TimeRange struct {
	StartEpochMs       int64  `json:"start_epoch_ms"`
	EndEpochMs         int64  `json:"end_epoch_ms"`
	Timezone           string `json:"timezone,omitempty"`
	OriginalExpression string `json:"original_expression,omitempty"`
}

type ExecuteRequest struct {
	ExecutionId   string            `json:"execution_id"`
	StepId        string            `json:"step_id"`
	ActionId      string            `json:"action_id,omitempty"`
	ParamsJson    []byte            `json:"params_json"`
	ExecutionMode ExecutionMode     `json:"execution_mode"`
	Context       map[string]string `json:"context"`
	TimeRange     *TimeRange        `json:"time_range,omitempty"`
}

type ExecuteResponse struct {
	Success    bool   `json:"success"`
	ResultJson []byte `json:"result_json"`
	Error      string `json:"error"`
	DurationMs int64  `json:"duration_ms"`
}

type HealthCheckRequest struct{}

type HealthCheckResponse struct {
	Healthy bool              `json:"healthy"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

type ConfigUpdatedRequest struct {
	Config  map[string]string `json:"config"`
	Version int64             `json:"version"`
}

type ConfigUpdatedResponse struct {
	Acknowledged bool   `json:"acknowledged"`
	Error        string `json:"error"`
}

// ---------------------------------------------------------------------------
// EngineService Messages
// ---------------------------------------------------------------------------

type GetConfigRequest struct {
	PluginName string `json:"plugin_name"`
}

type GetConfigResponse struct {
	Config  map[string]string `json:"config"`
	Version int64             `json:"version"`
}

type CacheGetRequest struct {
	Key string `json:"key"`
}

type CacheGetResponse struct {
	Value []byte `json:"value"`
	Found bool   `json:"found"`
}

type CacheSetRequest struct {
	Key        string `json:"key"`
	Value      []byte `json:"value"`
	TtlSeconds int64  `json:"ttl_seconds"`
}

type CacheSetResponse struct {
	Success bool `json:"success"`
}

type PublishResultRequest struct {
	ExecutionId string `json:"execution_id"`
	StepId      string `json:"step_id"`
	ResultJson  []byte `json:"result_json"`
	Success     bool   `json:"success"`
	Error       string `json:"error"`
}

type PublishResultResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

type RequestApprovalRequest struct {
	ExecutionId        string     `json:"execution_id"`
	StepId             string     `json:"step_id"`
	Description        string     `json:"description"`
	RequiredPermission Permission `json:"required_permission"`
	ContextJson        []byte     `json:"context_json"`
	TimeoutSeconds     int32      `json:"timeout_seconds"`
}

type RequestApprovalResponse struct {
	Approved bool   `json:"approved"`
	TimedOut bool   `json:"timed_out"`
	Reviewer string `json:"reviewer"`
	Comment  string `json:"comment"`
}

type LogEventRequest struct {
	PluginName string `json:"plugin_name"`
	EventType  string `json:"event_type"`
	DataJson   []byte `json:"data_json"`
	Severity   string `json:"severity"`
}

type LogEventResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

type CallPluginRequest struct {
	CallerPlugin   string `json:"caller_plugin"`
	TargetPlugin   string `json:"target_plugin"`
	ActionId       string `json:"action_id,omitempty"`
	ParamsJson     []byte `json:"params_json"`
	TimeoutSeconds int32  `json:"timeout_seconds"`
}

type CallPluginResponse struct {
	Success    bool   `json:"success"`
	ResultJson []byte `json:"result_json"`
	Error      string `json:"error"`
	DurationMs int64  `json:"duration_ms"`
}
