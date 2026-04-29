package mirastack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	pluginv1 "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/gen/pluginv1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/keepalive"
)

func init() {
	encoding.RegisterCodec(pluginv1.JSONCodec{})
}

// Serve starts the plugin gRPC server and blocks until shutdown.
// This is the main entry point for plugin binaries.
//
//	func main() {
//	    mirastack.Serve(&MyPlugin{})
//	}
func Serve(plugin Plugin) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	info := plugin.Info()
	if info == nil {
		logger.Fatal("plugin.Info() must not return nil")
	}

	// Validate plugin metadata before starting the gRPC server.
	if err := validatePlugin(info); err != nil {
		logger.Fatal("plugin validation failed", zap.Error(err))
	}

	// Resolve the mandatory tenant identifier for this plugin process.
	// MIRASTACK_PLUGIN_TENANT_ID must be set to the UUID5 of the tenant.
	// MIRASTACK_PLUGIN_TENANT_SLUG may be provided instead; the SDK derives
	// the UUID5 deterministically so operators need not compute it manually.
	tenantID, err := resolveTenantID()
	if err != nil {
		logger.Fatal("tenant ID resolution failed", zap.Error(err))
	}

	// Generate a unique instance ID for this process. Every plugin instance
	// gets its own UUID so the engine can distinguish horizontally-scaled
	// replicas and scope Valkey health keys per instance.
	instanceID := uuid.New().String()

	listenAddr := os.Getenv("MIRASTACK_PLUGIN_ADDR")
	if listenAddr == "" {
		listenAddr = ":0" // OS-assigned port
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Fatal("failed to listen", zap.Error(err))
	}

	// Initialize OpenTelemetry (no-op when MIRASTACK_OTEL_ENABLED != "true")
	otelShutdown, otelErr := initOTel(context.Background(), info.Name, logger)
	if otelErr != nil {
		logger.Warn("OTel initialization failed, continuing without tracing", zap.Error(otelErr))
		otelShutdown = noopOTelShutdown
	}

	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             20 * time.Second, // allow client pings every 20s
			PermitWithoutStream: true,             // allow pings with no active RPCs
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second, // server-side keepalive ping interval
			Timeout: 10 * time.Second, // wait for ping ack
		}),
	)

	// Register the PluginService adapter that delegates to the Plugin interface
	adapter := &pluginServiceAdapter{plugin: plugin, instanceID: instanceID, logger: logger}
	pluginv1.RegisterPluginServiceServer(server, adapter)

	// Connect to the engine if address provided (for EngineContext callbacks)
	engineAddr := os.Getenv("MIRASTACK_ENGINE_ADDR")
	var engineCtx *EngineContext
	if engineAddr != "" {
		ec, err := NewEngineContext(engineAddr, info.Name, instanceID, tenantID)
		if err != nil {
			logger.Warn("failed to connect to engine, callbacks unavailable", zap.Error(err))
		} else {
			engineCtx = ec
			defer engineCtx.Close()
			logger.Info("connected to engine for callbacks", zap.String("engine_addr", engineAddr))

			// Inject EngineContext into plugins that implement EngineAware
			if aware, ok := plugin.(EngineAware); ok {
				aware.SetEngineContext(engineCtx)
				logger.Info("injected EngineContext into plugin")
			}
		}
	}

	// Write the actual port to stdout for the engine to discover
	addr := lis.Addr().(*net.TCPAddr)
	fmt.Fprintf(os.Stdout, "MIRASTACK_PLUGIN_PORT=%d\n", addr.Port)

	logger.Info("plugin serving",
		zap.String("name", info.Name),
		zap.String("version", info.Version),
		zap.String("instance_id", instanceID),
		zap.String("tenant_id", tenantID),
		zap.String("addr", lis.Addr().String()),
	)

	// Maintain persistent registration with the engine in a background
	// goroutine. Registration must not block the gRPC server — the plugin
	// must be ready to accept Execute / HealthCheck calls immediately.
	// After initial registration, the goroutine enters a heartbeat loop
	// that periodically re-registers. This ensures the plugin survives
	// engine restarts: when the engine comes back, the next heartbeat
	// re-establishes the registration.
	// In container and Kubernetes environments every replica should set
	// MIRASTACK_PLUGIN_ADVERTISE_ADDR to the Service name
	// (e.g. "agent-query-vmetrics:50051") so the engine dials the
	// infrastructure load-balancer, not an ephemeral pod/container address.
	stopCh := make(chan struct{})
	if engineCtx != nil {
		go maintainRegistration(logger, engineCtx, resolveAdvertiseAddr(addr.Port), pluginv1.PluginTypeAgent, info.Version, stopCh)
	}

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down plugin")

		// Stop the registration heartbeat loop.
		close(stopCh)

		// Deregister from engine before stopping gRPC server
		if engineCtx != nil {
			deregCtx, deregCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := engineCtx.DeregisterSelf(deregCtx); err != nil {
				logger.Warn("deregistration from engine failed", zap.Error(err))
			} else {
				logger.Info("deregistered from engine")
			}
			deregCancel()
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			logger.Warn("OTel shutdown error", zap.Error(err))
		}
		server.GracefulStop()
	}()

	if err := server.Serve(lis); err != nil {
		logger.Fatal("gRPC serve error", zap.Error(err))
	}
}

// ---------------------------------------------------------------------------
// pluginServiceAdapter bridges the Plugin interface to PluginServiceServer.
// ---------------------------------------------------------------------------

type pluginServiceAdapter struct {
	pluginv1.UnimplementedPluginServiceServer
	plugin     Plugin
	instanceID string
	logger     *zap.Logger
}

func (a *pluginServiceAdapter) Info(_ context.Context, _ *pluginv1.InfoRequest) (*pluginv1.InfoResponse, error) {
	info := a.plugin.Info()

	// Convert SDK types to gRPC types
	stages := make([]pluginv1.DevOpsStage, len(info.DevOpsStages))
	for i, s := range info.DevOpsStages {
		stages[i] = pluginv1.DevOpsStage(s + 1) // SDK is 0-indexed, proto is 1-indexed
	}

	intents := make([]pluginv1.IntentPattern, len(info.Intents))
	for i, in := range info.Intents {
		intents[i] = pluginv1.IntentPattern{
			Pattern:     in.Pattern,
			Confidence:  float32(in.Priority) / 10.0,
			Description: in.Description,
			Priority:    in.Priority,
		}
	}

	// Convert Actions to proto ActionDef
	actions := actionsToProto(info.Actions)

	perm := pluginv1.PermissionRead
	if len(info.Permissions) > 0 {
		perm = pluginv1.Permission(info.Permissions[0] + 1)
	}

	// Convert PromptTemplates to proto PromptTemplateDef
	promptTemplates := make([]pluginv1.PromptTemplateDef, len(info.PromptTemplates))
	for i, pt := range info.PromptTemplates {
		promptTemplates[i] = pluginv1.PromptTemplateDef{
			Name:        pt.Name,
			Description: pt.Description,
			Content:     pt.Content,
		}
	}

	// Convert ConfigParams to proto ConfigParamSchema
	configSchema := make([]pluginv1.ConfigParamSchema, len(info.ConfigParams))
	for i, cp := range info.ConfigParams {
		configSchema[i] = pluginv1.ConfigParamSchema{
			Key:         cp.Key,
			Type:        cp.Type,
			Required:    cp.Required,
			Default:     cp.Default,
			Description: cp.Description,
			IsSecret:    cp.IsSecret,
		}
	}

	return &pluginv1.InfoResponse{
		Name:            info.Name,
		Version:         info.Version,
		Description:     info.Description,
		Type:            pluginv1.PluginTypeAgent,
		Permission:      perm,
		DevopsStages:    stages,
		DefaultIntents:  intents,
		Actions:         actions,
		PromptTemplates: promptTemplates,
		ConfigSchema:    configSchema,
		InstanceID:      a.instanceID,
	}, nil
}

func (a *pluginServiceAdapter) GetSchema(_ context.Context, _ *pluginv1.GetSchemaRequest) (*pluginv1.GetSchemaResponse, error) {
	schema := a.plugin.Schema()

	paramsJSON, _ := json.Marshal(schema.InputParams)
	resultJSON, _ := json.Marshal(schema.OutputParams)

	return &pluginv1.GetSchemaResponse{
		ParamsJsonSchema: paramsJSON,
		ResultJsonSchema: resultJSON,
		Actions:          actionsToProto(schema.Actions),
	}, nil
}

func (a *pluginServiceAdapter) Execute(ctx context.Context, req *pluginv1.ExecuteRequest) (*pluginv1.ExecuteResponse, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "plugin.execute",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	span.SetAttributes(
		attribute.String("plugin.action", req.ActionId),
		attribute.String("plugin.execution_id", req.ExecutionId),
	)

	start := time.Now()

	// Decode params from JSON
	var params map[string]string
	if len(req.ParamsJson) > 0 {
		if err := json.Unmarshal(req.ParamsJson, &params); err != nil {
			return &pluginv1.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("invalid params JSON: %v", err),
			}, nil
		}
	}

	sdkReq := &ExecuteRequest{
		ExecutionID: req.ExecutionId,
		StepID:      req.StepId,
		ActionID:    req.ActionId,
		Params:      params,
		Mode:        ExecutionMode(req.ExecutionMode),
		TenantID:    req.TenantId,
	}

	// Map proto TimeRange to SDK TimeRange
	if req.TimeRange != nil {
		sdkReq.TimeRange = &TimeRange{
			StartEpochMs:       req.TimeRange.StartEpochMs,
			EndEpochMs:         req.TimeRange.EndEpochMs,
			Timezone:           req.TimeRange.Timezone,
			OriginalExpression: req.TimeRange.OriginalExpression,
		}
	}

	resp, err := a.plugin.Execute(ctx, sdkReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return &pluginv1.ExecuteResponse{
			Success:    false,
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	span.SetAttributes(attribute.Bool("plugin.success", true))

	// Output is json.RawMessage — direct passthrough to gRPC ResultJson
	return &pluginv1.ExecuteResponse{
		Success:    true,
		ResultJson: []byte(resp.Output),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func (a *pluginServiceAdapter) HealthCheck(ctx context.Context, _ *pluginv1.HealthCheckRequest) (*pluginv1.HealthCheckResponse, error) {
	err := a.plugin.HealthCheck(ctx)
	if err != nil {
		return &pluginv1.HealthCheckResponse{
			Healthy: false,
			Message: err.Error(),
		}, nil
	}
	return &pluginv1.HealthCheckResponse{
		Healthy: true,
		Message: "ok",
	}, nil
}

func (a *pluginServiceAdapter) ConfigUpdated(ctx context.Context, req *pluginv1.ConfigUpdatedRequest) (*pluginv1.ConfigUpdatedResponse, error) {
	err := a.plugin.ConfigUpdated(ctx, req.Config)
	if err != nil {
		return &pluginv1.ConfigUpdatedResponse{
			Acknowledged: false,
			Error:        err.Error(),
		}, nil
	}
	return &pluginv1.ConfigUpdatedResponse{
		Acknowledged: true,
	}, nil
}

// actionsToProto converts SDK Action structs to proto ActionDef messages.
// Used by both Info() and GetSchema() adapters.
func actionsToProto(actions []Action) []pluginv1.ActionDef {
	defs := make([]pluginv1.ActionDef, len(actions))
	for i, act := range actions {
		stages := make([]pluginv1.DevOpsStage, len(act.Stages))
		for j, s := range act.Stages {
			stages[j] = pluginv1.DevOpsStage(s + 1)
		}
		intents := make([]pluginv1.IntentPattern, len(act.Intents))
		for j, in := range act.Intents {
			intents[j] = pluginv1.IntentPattern{
				Pattern:     in.Pattern,
				Confidence:  float32(in.Priority) / 10.0,
				Description: in.Description,
				Priority:    in.Priority,
			}
		}
		var inputJSON, outputJSON []byte
		if len(act.InputParams) > 0 {
			inputJSON, _ = json.Marshal(act.InputParams)
		}
		if len(act.OutputParams) > 0 {
			outputJSON, _ = json.Marshal(act.OutputParams)
		}
		defs[i] = pluginv1.ActionDef{
			Id:           act.ID,
			Description:  act.Description,
			Permission:   pluginv1.Permission(act.Permission + 1),
			Stages:       stages,
			Intents:      intents,
			InputParams:  inputJSON,
			OutputParams: outputJSON,
		}
	}
	return defs
}

// resolveAdvertiseAddr determines the address the engine should use to reach
// this plugin instance via gRPC.
//
// Order of precedence:
//
//  1. MIRASTACK_PLUGIN_ADVERTISE_ADDR — explicit, always wins.
//     In containerized (Docker/Podman) and Kubernetes deployments this MUST be
//     set to the Service DNS name (e.g. "agent-query-vmetrics:50051" for
//     Compose or "agent-query-vmetrics.ns.svc.cluster.local:50051" for K8s).
//     For horizontal scaling every replica advertises the same Service address;
//     the infrastructure (kube-proxy, Compose DNS round-robin) handles
//     load-balancing across pods/containers.
//  2. os.Hostname() + bound port — suitable for native (bare-metal / VM)
//     installs where the OS hostname is DNS-resolvable.
func resolveAdvertiseAddr(boundPort int) string {
	if addr := os.Getenv("MIRASTACK_PLUGIN_ADVERTISE_ADDR"); addr != "" {
		return addr
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}
	return fmt.Sprintf("%s:%d", hostname, boundPort)
}

// maintainRegistration performs initial registration with exponential backoff,
// then enters a persistent heartbeat loop that periodically re-registers with
// the engine. This ensures the plugin survives engine restarts: when the engine
// comes back, the next heartbeat re-establishes the registration.
//
// The heartbeat interval defaults to 30 seconds and can be overridden via
// MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL (in seconds).
//
// This function blocks until stopCh is closed (shutdown signal).
func maintainRegistration(logger *zap.Logger, ec *EngineContext, advertiseAddr string, pluginType pluginv1.PluginType, version string, stopCh <-chan struct{}) {
	const (
		initialMaxAttempts = 10
		maxBackoff         = 30 * time.Second
	)

	heartbeatInterval := 30 * time.Second
	if envSecs := os.Getenv("MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL"); envSecs != "" {
		if secs, err := strconv.Atoi(envSecs); err == nil && secs > 0 {
			heartbeatInterval = time.Duration(secs) * time.Second
		}
	}

	// Phase 1: Initial registration with bounded retries and backoff.
	backoff := 2 * time.Second
	registered := false
	for attempt := 1; attempt <= initialMaxAttempts; attempt++ {
		regCtx, regCancel := context.WithTimeout(context.Background(), 10*time.Second)
		pluginID, err := ec.RegisterSelf(regCtx, advertiseAddr, pluginType, version)
		regCancel()

		if err == nil {
			logger.Info("self-registered with engine",
				zap.String("plugin_id", pluginID),
				zap.String("advertise_addr", advertiseAddr),
			)
			registered = true
			break
		}

		if attempt == initialMaxAttempts {
			logger.Error("initial registration exhausted all retries — entering heartbeat mode, will keep trying",
				zap.Int("attempts", initialMaxAttempts),
				zap.Error(err),
			)
			break
		}

		logger.Warn("self-registration attempt failed, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		select {
		case <-stopCh:
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	if registered {
		logger.Info("entering registration heartbeat loop",
			zap.Duration("interval", heartbeatInterval),
		)
	}

	// Phase 2: Persistent heartbeat loop.
	// Sends a lightweight Heartbeat RPC instead of full RegisterPlugin.
	// If the engine responds with re_register_required (e.g. engine restarted
	// and lost in-memory state), the SDK performs a full RegisterSelf to
	// re-establish the plugin in the engine's registry.
	// If the engine returns a non-zero heartbeat_interval_seconds, the SDK
	// adopts it (unless overridden by env var).
	envOverride := os.Getenv("MIRASTACK_PLUGIN_HEARTBEAT_INTERVAL") != ""
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			hbCtx, hbCancel := context.WithTimeout(context.Background(), 10*time.Second)
			resp, err := ec.Heartbeat(hbCtx)
			hbCancel()

			if err != nil {
				consecutiveFailures++
				if consecutiveFailures == 1 || consecutiveFailures%10 == 0 {
					logger.Warn("heartbeat failed",
						zap.Int("consecutive_failures", consecutiveFailures),
						zap.Error(err),
					)
				}
				continue
			}

			// Adopt engine-recommended heartbeat interval (if no env override).
			if !envOverride && resp.HeartbeatIntervalSeconds > 0 {
				newInterval := time.Duration(resp.HeartbeatIntervalSeconds) * time.Second
				if newInterval != heartbeatInterval {
					heartbeatInterval = newInterval
					ticker.Reset(heartbeatInterval)
					logger.Info("adopted engine-recommended heartbeat interval",
						zap.Duration("interval", heartbeatInterval),
					)
				}
			}

			if resp.ReRegisterRequired {
				logger.Info("engine requested re-registration, performing full RegisterPlugin")
				regCtx, regCancel := context.WithTimeout(context.Background(), 10*time.Second)
				pluginID, regErr := ec.RegisterSelf(regCtx, advertiseAddr, pluginType, version)
				regCancel()
				if regErr != nil {
					consecutiveFailures++
					logger.Warn("re-registration after heartbeat failed",
						zap.Error(regErr),
					)
					continue
				}
				logger.Info("re-registered with engine after heartbeat",
					zap.String("plugin_id", pluginID),
				)
			}

			if consecutiveFailures > 0 {
				logger.Info("heartbeat recovered after failures",
					zap.Int("previous_failures", consecutiveFailures),
				)
			}
			consecutiveFailures = 0
		}
	}
}

// resolveTenantID returns the tenant UUID5 this plugin process is bound to.
//
// Resolution order:
//  1. MIRASTACK_PLUGIN_TENANT_ID — used as-is if set.
//  2. MIRASTACK_PLUGIN_TENANT_SLUG — UUID5 is derived deterministically via
//     IDFromSlug so operators need not pre-compute the identifier.
//
// Returns an error when neither variable is set; the caller (Serve) treats
// this as a fatal startup failure.
func resolveTenantID() (string, error) {
	if id := os.Getenv("MIRASTACK_PLUGIN_TENANT_ID"); id != "" {
		return id, nil
	}
	if slug := os.Getenv("MIRASTACK_PLUGIN_TENANT_SLUG"); slug != "" {
		return IDFromSlug(slug), nil
	}
	return "", errors.New("MIRASTACK_PLUGIN_TENANT_ID is required — set it to the UUID5 of the tenant this plugin serves, or set MIRASTACK_PLUGIN_TENANT_SLUG to derive it")
}
