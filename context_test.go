package mirastack

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pluginv1 "github.com/mirastacklabs-ai/mirastack-agents-sdk-go/gen/pluginv1"
)

// ---------------------------------------------------------------------------
// Mock EngineServiceClient — only implements GetConfig for testing
// ---------------------------------------------------------------------------

type mockEngineClient struct {
	pluginv1.EngineServiceClient
	mu             sync.Mutex
	callCount      int64
	config         map[string]string
	err            error
	batchEntries   []pluginv1.CacheGetBatchEntry
	batchErr       error
}

func (m *mockEngineClient) GetConfig(_ context.Context, req *pluginv1.GetConfigRequest) (*pluginv1.GetConfigResponse, error) {
	atomic.AddInt64(&m.callCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	cfg := make(map[string]string, len(m.config))
	for k, v := range m.config {
		cfg[k] = v
	}
	return &pluginv1.GetConfigResponse{Config: cfg, Version: 1}, nil
}

func (m *mockEngineClient) CacheGetBatch(_ context.Context, req *pluginv1.CacheGetBatchRequest) (*pluginv1.CacheGetBatchResponse, error) {
	if m.batchErr != nil {
		return nil, m.batchErr
	}
	return &pluginv1.CacheGetBatchResponse{Entries: m.batchEntries}, nil
}

func (m *mockEngineClient) calls() int64 {
	return atomic.LoadInt64(&m.callCount)
}

func newTestEngineContext(mock *mockEngineClient) *EngineContext {
	return &EngineContext{
		engineAddr: "localhost:0",
		pluginName: "test-plugin",
		instanceID: "test-instance-001",
		client:     mock,
		configTTL:  defaultConfigCacheTTL,
	}
}

// ---------------------------------------------------------------------------
// Tests: GetConfig cache behavior
// ---------------------------------------------------------------------------

func TestGetConfig_CacheMiss_CallsGRPC(t *testing.T) {
	mock := &mockEngineClient{
		config: map[string]string{"url": "http://vm:8428", "timeout": "30s"},
	}
	ec := newTestEngineContext(mock)

	cfg, err := ec.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg["url"] != "http://vm:8428" {
		t.Errorf("expected url 'http://vm:8428', got %q", cfg["url"])
	}
	if cfg["timeout"] != "30s" {
		t.Errorf("expected timeout '30s', got %q", cfg["timeout"])
	}
	if mock.calls() != 1 {
		t.Errorf("expected 1 gRPC call on cache miss, got %d", mock.calls())
	}
}

func TestGetConfig_CacheHit_SkipsGRPC(t *testing.T) {
	mock := &mockEngineClient{
		config: map[string]string{"url": "http://vm:8428"},
	}
	ec := newTestEngineContext(mock)

	// First call — cache miss
	_, _ = ec.GetConfig(context.Background())

	// Second call — should be cached
	cfg, err := ec.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg["url"] != "http://vm:8428" {
		t.Errorf("expected url from cache, got %q", cfg["url"])
	}
	if mock.calls() != 1 {
		t.Errorf("expected 1 gRPC call (second should be cache hit), got %d", mock.calls())
	}
}

func TestGetConfig_CacheExpiry_ReCallsGRPC(t *testing.T) {
	mock := &mockEngineClient{
		config: map[string]string{"url": "http://vm:8428"},
	}
	ec := newTestEngineContext(mock)
	// Use a very short TTL for this test
	ec.configTTL = 10 * time.Millisecond

	// First call — cache miss
	_, _ = ec.GetConfig(context.Background())
	if mock.calls() != 1 {
		t.Fatalf("expected 1 call after first GetConfig, got %d", mock.calls())
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Update mock config
	mock.mu.Lock()
	mock.config["url"] = "http://new-vm:8428"
	mock.mu.Unlock()

	// Third call — cache expired, should call gRPC again
	cfg, err := ec.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg["url"] != "http://new-vm:8428" {
		t.Errorf("expected new url after cache expiry, got %q", cfg["url"])
	}
	if mock.calls() != 2 {
		t.Errorf("expected 2 gRPC calls after cache expiry, got %d", mock.calls())
	}
}

func TestGetConfig_ReturnsCopy_NotCacheReference(t *testing.T) {
	mock := &mockEngineClient{
		config: map[string]string{"url": "http://vm:8428"},
	}
	ec := newTestEngineContext(mock)

	cfg1, _ := ec.GetConfig(context.Background())
	cfg1["url"] = "mutated"

	// Second call should still return original value from cache
	cfg2, _ := ec.GetConfig(context.Background())
	if cfg2["url"] != "http://vm:8428" {
		t.Errorf("cache mutation leaked: expected 'http://vm:8428', got %q", cfg2["url"])
	}
}

func TestGetConfig_NilConfig_ReturnsEmptyMap(t *testing.T) {
	mock := &mockEngineClient{
		config: nil,
	}
	ec := newTestEngineContext(mock)

	cfg, err := ec.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil map, got nil")
	}
	if len(cfg) != 0 {
		t.Errorf("expected empty map, got %v", cfg)
	}
}

func TestGetConfig_GRPCError_ReturnsError(t *testing.T) {
	mock := &mockEngineClient{
		err: fmt.Errorf("connection refused"),
	}
	ec := newTestEngineContext(mock)

	_, err := ec.GetConfig(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetConfig_ThreadSafe(t *testing.T) {
	mock := &mockEngineClient{
		config: map[string]string{"url": "http://vm:8428"},
	}
	ec := newTestEngineContext(mock)

	var wg sync.WaitGroup
	errs := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg, err := ec.GetConfig(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if cfg["url"] != "http://vm:8428" {
				errs <- fmt.Errorf("unexpected url: %q", cfg["url"])
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent GetConfig error: %v", err)
	}
}

func TestGetConfig_CustomTTL(t *testing.T) {
	mock := &mockEngineClient{
		config: map[string]string{"key": "value"},
	}
	ec := newTestEngineContext(mock)
	ec.configTTL = 100 * time.Millisecond

	// First call
	_, _ = ec.GetConfig(context.Background())

	// Within TTL — should be cached
	time.Sleep(10 * time.Millisecond)
	_, _ = ec.GetConfig(context.Background())
	if mock.calls() != 1 {
		t.Errorf("expected 1 call within TTL, got %d", mock.calls())
	}

	// After TTL
	time.Sleep(100 * time.Millisecond)
	_, _ = ec.GetConfig(context.Background())
	if mock.calls() != 2 {
		t.Errorf("expected 2 calls after TTL expiry, got %d", mock.calls())
	}
}

func TestEngineContext_PluginName(t *testing.T) {
	ec := newTestEngineContext(&mockEngineClient{})
	if ec.PluginName() != "test-plugin" {
		t.Errorf("expected 'test-plugin', got %q", ec.PluginName())
	}
}

func TestEngineContext_InstanceID(t *testing.T) {
	ec := newTestEngineContext(&mockEngineClient{})
	if ec.InstanceID() != "test-instance-001" {
		t.Errorf("expected 'test-instance-001', got %q", ec.InstanceID())
	}
}

// ---------------------------------------------------------------------------
// Tests: CacheGetBatch
// ---------------------------------------------------------------------------

func TestCacheGetBatch_ReturnsEntries(t *testing.T) {
	mock := &mockEngineClient{
		batchEntries: []pluginv1.CacheGetBatchEntry{
			{Key: "k1", Value: []byte("v1"), Found: true},
			{Key: "k2", Value: nil, Found: false},
		},
	}
	ec := newTestEngineContext(mock)
	entries, err := ec.CacheGetBatch(context.Background(), []string{"k1", "k2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "k1" || entries[0].Value != "v1" || !entries[0].Found {
		t.Errorf("entry 0 mismatch: %+v", entries[0])
	}
	if entries[1].Key != "k2" || entries[1].Value != "" || entries[1].Found {
		t.Errorf("entry 1 mismatch: %+v", entries[1])
	}
}

func TestCacheGetBatch_Error(t *testing.T) {
	mock := &mockEngineClient{
		batchErr: fmt.Errorf("connection refused"),
	}
	ec := newTestEngineContext(mock)
	_, err := ec.CacheGetBatch(context.Background(), []string{"k1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
