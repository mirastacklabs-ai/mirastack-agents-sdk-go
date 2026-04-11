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

// NewEngineContext creates a new EngineContext that connects back to the engine.
func NewEngineContext(engineAddr, pluginName, instanceID string) (*EngineContext, error) {
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
	resp, err := ec.client.GetConfig(ctx, &pluginv1.GetConfigRequest{PluginName: ec.pluginName})
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

// CacheGet retrieves a value from the engine's Valkey cache.
func (ec *EngineContext) CacheGet(ctx context.Context, key string) (string, error) {
	resp, err := ec.client.CacheGet(ctx, &pluginv1.CacheGetRequest{Key: key})
	if err != nil {
		return "", fmt.Errorf("cache get: %w", err)
	}
	if !resp.Found {
		return "", nil
	}
	return string(resp.Value), nil
}

// CacheSet stores a value in the engine's Valkey cache.
func (ec *EngineContext) CacheSet(ctx context.Context, key, value string, ttl time.Duration) error {
	_, err := ec.client.CacheSet(ctx, &pluginv1.CacheSetRequest{
		Key:        key,
		Value:      []byte(value),
		TtlSeconds: int64(ttl.Seconds()),
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
	})
	if err != nil {
		return fmt.Errorf("publish result: %w", err)
	}
	return nil
}

// RequestApproval pauses execution and waits for human approval.
func (ec *EngineContext) RequestApproval(ctx context.Context, executionID, reason string) (bool, error) {
	resp, err := ec.client.RequestApproval(ctx, &pluginv1.RequestApprovalRequest{
		ExecutionId: executionID,
		Description: reason,
	})
	if err != nil {
		return false, fmt.Errorf("request approval: %w", err)
	}
	return resp.Approved, nil
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
	})
	if err != nil {
		return fmt.Errorf("log event: %w", err)
	}
	return nil
}

// CallPlugin invokes another plugin through the engine and returns its output.
// This enables composite plugins to orchestrate leaf plugins without direct connections.
func (ec *EngineContext) CallPlugin(ctx context.Context, targetPlugin string, params map[string]string) (map[string]string, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	resp, err := ec.client.CallPlugin(ctx, &pluginv1.CallPluginRequest{
		CallerPlugin: ec.pluginName,
		TargetPlugin: targetPlugin,
		ParamsJson:   paramsJSON,
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
