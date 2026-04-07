package mirastack

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	pluginv1 "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/gen/pluginv1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
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

	server := grpc.NewServer()

	// Register the PluginService adapter that delegates to the Plugin interface
	adapter := &pluginServiceAdapter{plugin: plugin, instanceID: instanceID, logger: logger}
	pluginv1.RegisterPluginServiceServer(server, adapter)

	// Connect to the engine if address provided (for EngineContext callbacks)
	engineAddr := os.Getenv("MIRASTACK_ENGINE_ADDR")
	if engineAddr != "" {
		engineCtx, err := NewEngineContext(engineAddr, info.Name, instanceID)
		if err != nil {
			logger.Warn("failed to connect to engine, callbacks unavailable", zap.Error(err))
		} else {
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
		zap.String("addr", lis.Addr().String()),
	)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down plugin")
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
		return &pluginv1.ExecuteResponse{
			Success:    false,
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

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
