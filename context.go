package mirastack

import (
	"context"
	"fmt"
	"time"
)

// EngineContext provides the plugin with engine services via gRPC callbacks.
// Plugins use this to access cache, publish results, request approvals, and log events.
// Plugins NEVER access Kine or Valkey directly — everything goes through the engine.
type EngineContext struct {
	engineAddr string
	pluginName string
	// TODO(phase-2): Add gRPC client connection to engine
}

// NewEngineContext creates a new EngineContext that connects back to the engine.
func NewEngineContext(engineAddr, pluginName string) (*EngineContext, error) {
	if engineAddr == "" {
		return nil, fmt.Errorf("engine address is required")
	}
	return &EngineContext{
		engineAddr: engineAddr,
		pluginName: pluginName,
	}, nil
}

// CacheGet retrieves a value from the engine's Valkey cache.
func (ec *EngineContext) CacheGet(ctx context.Context, key string) (string, error) {
	_ = ctx
	_ = key
	// TODO(phase-2): Implement via EngineService.CacheGet gRPC call
	return "", fmt.Errorf("not yet implemented")
}

// CacheSet stores a value in the engine's Valkey cache.
func (ec *EngineContext) CacheSet(ctx context.Context, key, value string, ttl time.Duration) error {
	_ = ctx
	_ = key
	_ = value
	_ = ttl
	// TODO(phase-2): Implement via EngineService.CacheSet gRPC call
	return fmt.Errorf("not yet implemented")
}

// PublishResult sends execution output back to the engine.
func (ec *EngineContext) PublishResult(ctx context.Context, executionID string, output map[string]string) error {
	_ = ctx
	_ = executionID
	_ = output
	// TODO(phase-2): Implement via EngineService.PublishResult gRPC call
	return fmt.Errorf("not yet implemented")
}

// RequestApproval pauses execution and waits for human approval.
func (ec *EngineContext) RequestApproval(ctx context.Context, executionID, reason string) (bool, error) {
	_ = ctx
	_ = executionID
	_ = reason
	// TODO(phase-4): Implement via EngineService.RequestApproval gRPC call
	return false, fmt.Errorf("not yet implemented")
}

// LogEvent sends a log entry to the engine's event stream.
func (ec *EngineContext) LogEvent(ctx context.Context, level, message string, fields map[string]string) error {
	_ = ctx
	_ = level
	_ = message
	_ = fields
	// TODO(phase-2): Implement via EngineService.LogEvent gRPC call
	return fmt.Errorf("not yet implemented")
}

// Close cleans up the gRPC connection.
func (ec *EngineContext) Close() error {
	// TODO(phase-2): Close gRPC client connection
	return nil
}
