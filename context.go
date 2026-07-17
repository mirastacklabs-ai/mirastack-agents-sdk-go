package mirastack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	pluginv1 "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/gen/pluginv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// Default config cache TTL.
const defaultConfigCacheTTL = 15 * time.Second

// EngineContext provides the plugin with engine services via gRPC callbacks.
// Plugins use this to access cache, publish results, request approvals,
// log events, and invoke other plugins. Plugins NEVER access Kine or Valkey
// directly — everything goes through the engine.
type EngineContext struct {
	engineAddr string
	pluginName string
	instanceID string
	tenantID   string // UUID5 of the tenant this plugin instance serves
	conn       *grpc.ClientConn
	client     pluginv1.EngineServiceClient

	// Config cache — avoids gRPC round-trip for every GetConfig call
	configCache    map[string]string
	configCachedAt time.Time
	configTTL      time.Duration
	configMu       sync.RWMutex
}

// InstanceID returns the unique instance identifier for this plugin process.
// Plugins use this to construct scoped Valkey keys (e.g. health:{name}:{instance_id}).
func (ec *EngineContext) InstanceID() string {
	return ec.instanceID
}

// PluginName returns the plugin's registered name.
func (ec *EngineContext) PluginName() string {
	return ec.pluginName
}

// TenantID returns the UUID5 tenant identifier this plugin instance serves.
// This is the value from MIRASTACK_PLUGIN_TENANT_ID, resolved at startup.
func (ec *EngineContext) TenantID() string {
	return ec.tenantID
}

// NewEngineContext creates a new EngineContext that connects back to the engine.
func NewEngineContext(engineAddr, pluginName, instanceID, tenantID string) (*EngineContext, error) {
	if engineAddr == "" {
		return nil, fmt.Errorf("engine address is required")
	}

	// Attach plugin identity metadata to every outgoing gRPC call so the
	// engine can enforce key scoping (e.g. CacheSet prefix validation).
	identityInterceptor := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-plugin-name", pluginName)
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	conn, err := grpc.NewClient(engineAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(pluginv1.JSONCodec{}.Name())),
		grpc.WithUnaryInterceptor(identityInterceptor),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to engine at %s: %w", engineAddr, err)
	}

	cacheTTL := defaultConfigCacheTTL
	if envTTL := os.Getenv("MIRASTACK_SDK_CONFIG_CACHE_TTL"); envTTL != "" {
		if secs, err := strconv.Atoi(envTTL); err == nil && secs > 0 {
			cacheTTL = time.Duration(secs) * time.Second
		}
	}

	return &EngineContext{
		engineAddr: engineAddr,
		pluginName: pluginName,
		instanceID: instanceID,
		tenantID:   tenantID,
		conn:       conn,
		client:     pluginv1.NewEngineServiceClient(conn),
		configTTL:  cacheTTL,
	}, nil
}

// GetConfig retrieves this plugin's configuration from the engine.
// Results are cached locally with a configurable TTL (default 15s, set via
// MIRASTACK_SDK_CONFIG_CACHE_TTL env var in seconds). On cache hit the gRPC
// round-trip is skipped entirely.
func (ec *EngineContext) GetConfig(ctx context.Context) (map[string]string, error) {
	ec.configMu.RLock()
	if ec.configCache != nil && time.Since(ec.configCachedAt) < ec.configTTL {
		result := make(map[string]string, len(ec.configCache))
		for k, v := range ec.configCache {
			result[k] = v
		}
		ec.configMu.RUnlock()
		return result, nil
	}
	ec.configMu.RUnlock()

	// Cache miss — call engine
	resp, err := ec.client.GetConfig(ctx, &pluginv1.GetConfigRequest{PluginName: ec.pluginName, TenantId: ec.tenantID})
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	config := resp.Config
	if config == nil {
		config = make(map[string]string)
	}

	ec.configMu.Lock()
	ec.configCache = config
	ec.configCachedAt = time.Now()
	ec.configMu.Unlock()

	// Return a copy so callers cannot mutate the cache.
	result := make(map[string]string, len(config))
	for k, v := range config {
		result[k] = v
	}
	return result, nil
}

// KPIFilter narrows ListKPIs requests for the plugin's tenant.
type KPIFilter struct {
	Kind  string
	Layer string
}

// KPIView is the SDK-side representation of an engine KPI definition.
type KPIView struct {
	ID            string
	TenantID      string
	Name          string
	Query         string
	IntegrationID string
	Kind          string
	Layer         string
	Sentiment     string
	Classifier    string
	Definition    string
	CreatedAt     int64
	UpdatedAt     int64
	CreatedBy     string
	UpdatedBy     string
}

func mapKPIView(v pluginv1.KPIView) KPIView {
	return KPIView{
		ID:            v.ID,
		TenantID:      v.TenantID,
		Name:          v.Name,
		Query:         v.Query,
		IntegrationID: v.IntegrationID,
		Kind:          v.Kind,
		Layer:         v.Layer,
		Sentiment:     v.Sentiment,
		Classifier:    v.Classifier,
		Definition:    v.Definition,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
		CreatedBy:     v.CreatedBy,
		UpdatedBy:     v.UpdatedBy,
	}
}

// ListKPIs retrieves KPI definitions in the plugin's tenant with optional filters.
func (ec *EngineContext) ListKPIs(ctx context.Context, filter KPIFilter) ([]KPIView, error) {
	resp, err := ec.client.ListKPIs(ctx, &pluginv1.ListKPIsRequest{
		TenantId: ec.tenantID,
		Kind:     filter.Kind,
		Layer:    filter.Layer,
	})
	if err != nil {
		return nil, fmt.Errorf("list kpis: %w", err)
	}
	out := make([]KPIView, len(resp.KPIs))
	for i := range resp.KPIs {
		out[i] = mapKPIView(resp.KPIs[i])
	}
	return out, nil
}

// GetKPI fetches a single KPI definition by ID in the plugin's tenant.
func (ec *EngineContext) GetKPI(ctx context.Context, kpiID string) (*KPIView, error) {
	resp, err := ec.client.GetKPI(ctx, &pluginv1.GetKPIRequest{
		TenantId: ec.tenantID,
		KPIID:    kpiID,
	})
	if err != nil {
		return nil, fmt.Errorf("get kpi %q: %w", kpiID, err)
	}
	if resp.KPI == nil {
		return nil, nil
	}
	k := mapKPIView(*resp.KPI)
	return &k, nil
}

// CacheGet retrieves a value from the engine's Valkey cache.
func (ec *EngineContext) CacheGet(ctx context.Context, key string) (string, error) {
	resp, err := ec.client.CacheGet(ctx, &pluginv1.CacheGetRequest{Key: key, TenantId: ec.tenantID})
	if err != nil {
		return "", fmt.Errorf("cache get: %w", err)
	}
	if !resp.Found {
		return "", nil
	}
	return string(resp.Value), nil
}

// CacheGetBatchEntry represents a single key result from CacheGetBatch.
type CacheGetBatchEntry struct {
	Key   string
	Value string
	Found bool
}

// CacheGetBatch retrieves multiple values from the engine's Valkey cache in a
// single round-trip. Returns one entry per key in the same order as the input.
func (ec *EngineContext) CacheGetBatch(ctx context.Context, keys []string) ([]CacheGetBatchEntry, error) {
	resp, err := ec.client.CacheGetBatch(ctx, &pluginv1.CacheGetBatchRequest{Keys: keys, TenantId: ec.tenantID})
	if err != nil {
		return nil, fmt.Errorf("cache get batch: %w", err)
	}
	entries := make([]CacheGetBatchEntry, len(resp.Entries))
	for i, e := range resp.Entries {
		entries[i] = CacheGetBatchEntry{
			Key:   e.Key,
			Value: string(e.Value),
			Found: e.Found,
		}
	}
	return entries, nil
}

// CacheSet stores a value in the engine's Valkey cache.
func (ec *EngineContext) CacheSet(ctx context.Context, key, value string, ttl time.Duration) error {
	_, err := ec.client.CacheSet(ctx, &pluginv1.CacheSetRequest{
		Key:        key,
		Value:      []byte(value),
		TtlSeconds: int64(ttl.Seconds()),
		TenantId:   ec.tenantID,
	})
	if err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// PublishResult sends execution output back to the engine.
func (ec *EngineContext) PublishResult(ctx context.Context, executionID string, output map[string]string) error {
	resultJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	_, err = ec.client.PublishResult(ctx, &pluginv1.PublishResultRequest{
		ExecutionId: executionID,
		ResultJson:  resultJSON,
		Success:     true,
		TenantId:    ec.tenantID,
	})
	if err != nil {
		return fmt.Errorf("publish result: %w", err)
	}
	return nil
}

// ApprovalRequestContext carries optional structured reviewer context for
// RequestApprovalWithContext. ContextJSON should be a compact JSON object with
// evidence relevant to the decision (target identifiers, dry-run output,
// policy checks, etc.).
type ApprovalRequestContext struct {
	ContextJSON []byte
	BlastRadius *pluginv1.BlastRadius
}

// RequestApproval pauses execution and waits for human approval for an action
// at the given permission level. The permission MUST reflect the side-effect
// class of the action being gated (PermissionModify for stateful changes,
// PermissionAdmin for destructive/privileged operations). The engine uses the
// permission to:
//
//   - choose the minimum approver role (engineer for MODIFY, admin for ADMIN);
//   - emit the correct human-readable label on the approval card surfaced to
//     the chat / web client; and
//   - run the approval-mode policy check (see internal/approval/modes.go).
//
// Passing PermissionRead is rejected by the engine — read actions never
// require human approval. Passing PermissionUnspecified (the zero value) is
// treated as PermissionModify (the conservative default) but emits a warning
// in the engine log; new plugin code MUST pass an explicit permission.
//
// StepID is sourced from the gRPC step id propagated by pluginmgr when
// available; pass an empty string when the caller does not know the step id.
func (ec *EngineContext) RequestApproval(ctx context.Context, executionID, reason string, permission Permission) (bool, error) {
	return ec.RequestApprovalWithContext(ctx, executionID, reason, permission, nil)
}

// RequestApprovalWithContext is the structured variant of RequestApproval. It
// lets agents attach machine-readable blast radius metadata and arbitrary JSON
// evidence for human reviewers.
func (ec *EngineContext) RequestApprovalWithContext(
	ctx context.Context,
	executionID, reason string,
	permission Permission,
	requestContext *ApprovalRequestContext,
) (bool, error) {
	var contextJSON []byte
	var blastRadius *pluginv1.BlastRadius
	if requestContext != nil {
		contextJSON = requestContext.ContextJSON
		blastRadius = requestContext.BlastRadius
	}

	resp, err := ec.client.RequestApproval(ctx, &pluginv1.RequestApprovalRequest{
		ExecutionId:        executionID,
		Description:        reason,
		TenantId:           ec.tenantID,
		RequiredPermission: permissionToProto(permission),
		ContextJson:        contextJSON,
		BlastRadius:        blastRadius,
	})
	if err != nil {
		return false, fmt.Errorf("request approval: %w", err)
	}
	return resp.Approved, nil
}

// permissionToProto converts the SDK-side Permission iota (Read=0, Modify=1,
// Admin=2, Write=3) to the proto-side pluginv1.Permission enum (Unspecified=0, Read=1,
// Modify=2, Admin=3, Write=4). The +1 offset matches the conversion used elsewhere in
// the SDK (see serve.go) and isolates plugin authors from the proto wire
// format.
func permissionToProto(p Permission) pluginv1.Permission {
	switch p {
	case PermissionRead:
		return pluginv1.PermissionRead
	case PermissionModify:
		return pluginv1.PermissionModify
	case PermissionAdmin:
		return pluginv1.PermissionAdmin
	case PermissionWrite:
		return pluginv1.PermissionWrite
	default:
		// Conservative default: anything unrecognised is treated as MODIFY
		// so the engine still routes it through the approval queue rather
		// than silently auto-approving.
		return pluginv1.PermissionModify
	}
}

// LogEvent sends a log entry to the engine's event stream.
func (ec *EngineContext) LogEvent(ctx context.Context, level, message string, fields map[string]string) error {
	dataJSON, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	_, err = ec.client.LogEvent(ctx, &pluginv1.LogEventRequest{
		PluginName: ec.pluginName,
		EventType:  message,
		DataJson:   dataJSON,
		Severity:   level,
		TenantId:   ec.tenantID,
	})
	if err != nil {
		return fmt.Errorf("log event: %w", err)
	}
	return nil
}

// CallPlugin invokes another plugin through the engine and returns its output.
// This enables composite plugins to orchestrate leaf plugins without direct connections.
func (ec *EngineContext) CallPlugin(ctx context.Context, targetPlugin string, params map[string]string) (map[string]string, error) {
	return ec.CallPluginWithTimeRange(ctx, targetPlugin, params, nil)
}

// CallPluginWithTimeRange invokes another plugin through the engine, propagating
// the given TimeRange so the target plugin receives the same absolute time
// boundaries as the original request. Use this when orchestrating agent-to-agent
// calls from within an Execute() handler to prevent time drift.
func (ec *EngineContext) CallPluginWithTimeRange(ctx context.Context, targetPlugin string, params map[string]string, timeRange *pluginv1.TimeRange) (map[string]string, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	resp, err := ec.client.CallPlugin(ctx, &pluginv1.CallPluginRequest{
		CallerPlugin: ec.pluginName,
		TargetPlugin: targetPlugin,
		TenantId:     ec.tenantID,
		ParamsJson:   paramsJSON,
		TimeRange:    timeRange,
	})
	if err != nil {
		return nil, fmt.Errorf("call plugin %q: %w", targetPlugin, err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("plugin %q returned error: %s", targetPlugin, resp.Error)
	}

	var output map[string]string
	if err := json.Unmarshal(resp.ResultJson, &output); err != nil {
		return nil, fmt.Errorf("unmarshal plugin %q response: %w", targetPlugin, err)
	}
	return output, nil
}

// Close cleans up the gRPC connection.
func (ec *EngineContext) Close() error {
	if ec.conn != nil {
		return ec.conn.Close()
	}
	return nil
}

// RegisterSelf announces this plugin to the engine so it becomes part of the
// active plugin registry without requiring an engine restart. The engine
// connects back to grpcAddr, calls Info()/GetSchema(), validates type
// boundaries, and ingests intents/templates/config schema.
func (ec *EngineContext) RegisterSelf(ctx context.Context, grpcAddr string, pluginType pluginv1.PluginType, version string) (string, error) {
	resp, err := ec.client.RegisterPlugin(ctx, &pluginv1.RegisterPluginRequest{
		Name:       ec.pluginName,
		Version:    version,
		GRPCAddr:   grpcAddr,
		PluginType: pluginType,
		InstanceID: ec.instanceID,
		TenantId:   ec.tenantID,
	})
	if err != nil {
		return "", fmt.Errorf("register plugin: %w", err)
	}
	if !resp.Success {
		return "", fmt.Errorf("register plugin rejected: %s", resp.Error)
	}
	return resp.PluginID, nil
}

// DeregisterSelf tells the engine this plugin is shutting down so it can be
// removed from the active registry immediately (rather than waiting for a
// health check timeout).
func (ec *EngineContext) DeregisterSelf(ctx context.Context) error {
	_, err := ec.client.DeregisterPlugin(ctx, &pluginv1.DeregisterPluginRequest{
		Name:       ec.pluginName,
		InstanceID: ec.instanceID,
	})
	if err != nil {
		return fmt.Errorf("deregister plugin: %w", err)
	}
	return nil
}

// Heartbeat sends a lightweight liveness signal to the engine.
// Returns the engine's response which may indicate re-registration is required
// (e.g. after an engine restart) and an optional recommended heartbeat interval.
func (ec *EngineContext) Heartbeat(ctx context.Context) (*pluginv1.HeartbeatResponse, error) {
	resp, err := ec.client.Heartbeat(ctx, &pluginv1.HeartbeatRequest{
		Name:       ec.pluginName,
		InstanceID: ec.instanceID,
	})
	if err != nil {
		return nil, fmt.Errorf("heartbeat: %w", err)
	}
	return resp, nil
}
