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
	DevOpsStagePlan        DevOpsStage = 1
	DevOpsStageCode        DevOpsStage = 2
	DevOpsStageBuild       DevOpsStage = 3
	DevOpsStageTest        DevOpsStage = 4
	DevOpsStageRelease     DevOpsStage = 5
	DevOpsStageDeploy      DevOpsStage = 6
	DevOpsStageOperate     DevOpsStage = 7
	DevOpsStageObserve     DevOpsStage = 8
)

type ExecutionMode int32

const (
	ExecutionModeUnspecified ExecutionMode = 0
	ExecutionModeManual      ExecutionMode = 1
	ExecutionModeGuided      ExecutionMode = 2
	ExecutionModeAutonomous  ExecutionMode = 3
)

type PluginType int32

const (
	PluginTypeUnspecified PluginType = 0
	PluginTypeAgent       PluginType = 1
	PluginTypeProvider    PluginType = 2
	PluginTypeConnector   PluginType = 3
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
	Type        string `json:"type"` // "string", "int", "bool", "duration", "json"
	Required    bool   `json:"required"`
	Default     string `json:"default"`
	Description string `json:"description"`
	IsSecret    bool   `json:"is_secret"`
}

type InfoResponse struct {
	Name            string              `json:"name"`
	Version         string              `json:"version"`
	Description     string              `json:"description"`
	Type            PluginType          `json:"type"`
	Permission      Permission          `json:"permission"`
	DevopsStages    []DevOpsStage       `json:"devops_stages"`
	DefaultIntents  []IntentPattern     `json:"default_intents"`
	Actions         []ActionDef         `json:"actions,omitempty"`
	PromptTemplates []PromptTemplateDef `json:"prompt_templates,omitempty"`
	ConfigSchema    []ConfigParamSchema `json:"config_schema,omitempty"`
	Metadata        map[string]string   `json:"metadata"`
	InstanceID      string              `json:"instance_id"`
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
	// TenantId is the UUID5 tenant identifier resolved by the engine from the
	// calling Identity. Mandatory — plugins must treat this as authoritative
	// and never derive tenant from params or context.
	TenantId string `json:"tenant_id"`
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
	// TenantId is auto-stamped by the SDK; engine validates against the
	// registered plugin handle and refuses cross-tenant requests.
	TenantId string `json:"tenant_id"`
}

type GetConfigResponse struct {
	Config  map[string]string `json:"config"`
	Version int64             `json:"version"`
}

type CacheGetRequest struct {
	Key string `json:"key"`
	// TenantId is auto-stamped by the SDK; engine namespaces the key under
	// the tenant scope and rejects cross-tenant access.
	TenantId string `json:"tenant_id"`
}

type CacheGetResponse struct {
	Value []byte `json:"value"`
	Found bool   `json:"found"`
}

type CacheGetBatchRequest struct {
	Keys []string `json:"keys"`
	// TenantId is auto-stamped by the SDK; all keys are namespaced under the
	// tenant scope.
	TenantId string `json:"tenant_id"`
}

type CacheGetBatchResponse struct {
	Entries []CacheGetBatchEntry `json:"entries"`
}

type CacheGetBatchEntry struct {
	Key   string `json:"key"`
	Value []byte `json:"value"`
	Found bool   `json:"found"`
}

type CacheSetRequest struct {
	Key        string `json:"key"`
	Value      []byte `json:"value"`
	TtlSeconds int64  `json:"ttl_seconds"`
	// TenantId is auto-stamped by the SDK; engine namespaces the key under
	// the tenant scope and rejects cross-tenant writes.
	TenantId string `json:"tenant_id"`
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
	// TenantId is auto-stamped by the SDK; engine validates against the
	// execution's tenant scope.
	TenantId string `json:"tenant_id"`
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
	// TenantId is auto-stamped by the SDK; engine refuses approvals from
	// suspended/deleted tenants.
	TenantId string `json:"tenant_id"`
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
	// TenantId is auto-stamped by the SDK; audit events are persisted under
	// /mirastack/{tenant_id}/audit/{event_id}.
	TenantId string `json:"tenant_id"`
}

type LogEventResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

type CallPluginRequest struct {
	CallerPlugin   string     `json:"caller_plugin"`
	TargetPlugin   string     `json:"target_plugin"`
	ActionId       string     `json:"action_id,omitempty"`
	ParamsJson     []byte     `json:"params_json"`
	TimeoutSeconds int32      `json:"timeout_seconds"`
	TimeRange      *TimeRange `json:"time_range,omitempty"`
	// TenantId is the UUID5 of the calling plugin's tenant. The engine
	// refuses any CallPlugin where target_plugin is not registered for the
	// same tenant_id (PERMISSION_DENIED). Plugins must NOT set this from
	// user input — the SDK auto-stamps it from MIRASTACK_PLUGIN_TENANT_ID.
	TenantId string `json:"tenant_id"`
}

type CallPluginResponse struct {
	Success    bool   `json:"success"`
	ResultJson []byte `json:"result_json"`
	Error      string `json:"error"`
	DurationMs int64  `json:"duration_ms"`
}

// ---------------------------------------------------------------------------
// Plugin Self-Registration Messages
// ---------------------------------------------------------------------------

// RegisterPluginRequest is sent by a plugin to the engine to announce itself.
// The engine connects back to the plugin's gRPC address and performs the full
// registration handshake (Info, GetSchema, validate, ingest intents/templates).
type RegisterPluginRequest struct {
	Name       string     `json:"name"`
	Version    string     `json:"version"`
	GRPCAddr   string     `json:"grpc_addr"` // Externally reachable gRPC address (e.g., "plugin-host:50051")
	PluginType PluginType `json:"plugin_type"`
	InstanceID string     `json:"instance_id"`
	// TenantId is the UUID5 of the tenant this plugin process serves.
	// Mandatory: empty value is rejected (INVALID_ARGUMENT). The engine
	// validates the tenant exists and is active and stores the registration
	// under /mirastack/{tenant_id}/plugins/{name}.
	TenantId string `json:"tenant_id"`
}

type RegisterPluginResponse struct {
	Success  bool   `json:"success"`
	PluginID string `json:"plugin_id"` // Stable PluginID assigned by the engine
	Error    string `json:"error"`
	// License is the engine's licensing snapshot at registration time.
	// SDK consumers MUST treat License.Active=false as a directive to
	// abort the registration loop — the engine has already refused the
	// plugin via PermissionDenied/FailedPrecondition; this field is a
	// human-readable hint for diagnostics. When the engine is licensed
	// with an unlimited tier (ULTRA), the embedded quotas may report
	// negative values which mean "unlimited".
	License *LicenseContext `json:"license,omitempty"`
}

// DeregisterPluginRequest is sent by a plugin during graceful shutdown
// so the engine can immediately remove it from the active registry.
type DeregisterPluginRequest struct {
	Name       string `json:"name"`
	InstanceID string `json:"instance_id"`
}

type DeregisterPluginResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// HeartbeatRequest is a lightweight liveness signal sent periodically by plugins.
// Unlike RegisterPlugin, it does NOT trigger a full registration handshake.
type HeartbeatRequest struct {
	Name       string `json:"name"`
	InstanceID string `json:"instance_id"`
}

// HeartbeatResponse tells the plugin whether the engine recognizes it.
// If ReRegisterRequired is true, the plugin should perform a full RegisterPlugin.
type HeartbeatResponse struct {
	Acknowledged             bool  `json:"acknowledged"`
	ReRegisterRequired       bool  `json:"re_register_required"`
	HeartbeatIntervalSeconds int32 `json:"heartbeat_interval_seconds,omitempty"`
	// License piggybacks on heartbeat so SDK consumers can react to
	// expiry / tier changes without polling a separate endpoint. When
	// License.Active flips to false (e.g. license expired between
	// heartbeats), the SDK SHOULD stop accepting new ExecuteRequests.
	// Optional: nil when the engine cannot resolve the active license
	// (boot race) — SDK consumers should keep using the last known
	// snapshot from RegisterPluginResponse.
	License *LicenseContext `json:"license,omitempty"`
}

// LicenseContext is the engine's licensing snapshot served to plugins
// on registration handshake (RegisterPluginResponse) and on every
// Heartbeat. SDKs use this to short-circuit work the engine would
// reject anyway and to surface the active tier in diagnostic logs.
//
// Field semantics:
//
//   - Active: true iff the engine considers the license currently
//     enforceable (signed, not revoked, not past expiry).
//   - EffectiveTier: the tier the engine is currently honouring. During
//     the post-expiry grace period this degrades to "neo" while the
//     payload still carries the originally-issued tier.
//   - GraceMode: true when the license has expired and the engine is
//     serving from grace; the SDK should warn at startup.
//   - Quotas: distilled hard caps the SDK may use to choose between
//     paths (e.g. skip a feature a "neo" install cannot run). -1 in
//     a quota means unlimited.
type LicenseContext struct {
	Active        bool          `json:"active"`
	EffectiveTier string        `json:"effective_tier"`
	IssuedTier    string        `json:"issued_tier,omitempty"`
	GraceMode     bool          `json:"grace_mode,omitempty"`
	ExpiresAt     int64         `json:"expires_at,omitempty"` // epoch ms
	OrgID         string        `json:"org_id,omitempty"`
	SiteID        string        `json:"site_id,omitempty"`
	Region        string        `json:"region,omitempty"`
	RegionKind    string        `json:"region_kind,omitempty"`
	Quotas        LicenseQuotas `json:"quotas"`
}

// LicenseQuotas mirrors the engine's enforced caps. -1 means unlimited.
//
// AI Box counts are deliberately omitted: per AGENTS.md rule 14, those
// are marketing labels and never enforced at runtime. The fields here
// are limited to caps the engine actively meters.
type LicenseQuotas struct {
	MaxTenants               int `json:"max_tenants"`
	MaxIntegrationTypes       int `json:"max_integration_types"`
	MaxAgenticSessionsPerDay int `json:"max_agentic_sessions_per_day,omitempty"`
}
