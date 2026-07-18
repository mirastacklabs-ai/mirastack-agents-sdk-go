package mirastack

import (
	"context"
	"encoding/json"
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
	mu           sync.Mutex
	callCount    int64
	config       map[string]string
	err          error
	batchEntries []pluginv1.CacheGetBatchEntry
	batchErr     error
	listReq      *pluginv1.ListKPIsRequest
	listResp     *pluginv1.ListKPIsResponse
	listErr      error
	getReq       *pluginv1.GetKPIRequest
	getResp      *pluginv1.GetKPIResponse
	getErr       error
	approvalReq  *pluginv1.RequestApprovalRequest
	approvalResp *pluginv1.RequestApprovalResponse
	approvalErr  error
	callPluginReq  *pluginv1.CallPluginRequest
	callPluginResp *pluginv1.CallPluginResponse
	callPluginErr  error
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

func (m *mockEngineClient) ListKPIs(_ context.Context, req *pluginv1.ListKPIsRequest) (*pluginv1.ListKPIsResponse, error) {
	m.listReq = req
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.listResp == nil {
		return &pluginv1.ListKPIsResponse{}, nil
	}
	return m.listResp, nil
}

func (m *mockEngineClient) GetKPI(_ context.Context, req *pluginv1.GetKPIRequest) (*pluginv1.GetKPIResponse, error) {
	m.getReq = req
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResp == nil {
		return &pluginv1.GetKPIResponse{}, nil
	}
	return m.getResp, nil
}

func (m *mockEngineClient) RequestApproval(_ context.Context, req *pluginv1.RequestApprovalRequest) (*pluginv1.RequestApprovalResponse, error) {
	m.approvalReq = req
	if m.approvalErr != nil {
		return nil, m.approvalErr
	}
	if m.approvalResp == nil {
		return &pluginv1.RequestApprovalResponse{Approved: true}, nil
	}
	return m.approvalResp, nil
}

func (m *mockEngineClient) CallPlugin(_ context.Context, req *pluginv1.CallPluginRequest) (*pluginv1.CallPluginResponse, error) {
	m.callPluginReq = req
	if m.callPluginErr != nil {
		return nil, m.callPluginErr
	}
	if m.callPluginResp != nil {
		return m.callPluginResp, nil
	}
	return &pluginv1.CallPluginResponse{
		Success:    true,
		ResultJson: []byte(`{}`),
	}, nil
}

func (m *mockEngineClient) calls() int64 {
	return atomic.LoadInt64(&m.callCount)
}

func newTestEngineContext(mock *mockEngineClient) *EngineContext {
	return &EngineContext{
		engineAddr: "localhost:0",
		pluginName: "test-plugin",
		instanceID: "test-instance-001",
		tenantID:   "018ec9c7-5f3c-7f2e-a10f-111111111111",
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

func TestListKPIs_AutoStampsTenantAndMapsResponse(t *testing.T) {
	mock := &mockEngineClient{
		listResp: &pluginv1.ListKPIsResponse{KPIs: []pluginv1.KPIView{
			{
				ID:            "kpi-1",
				TenantID:      "018ec9c7-5f3c-7f2e-a10f-111111111111",
				Name:          "Error Rate",
				Query:         "sum(rate(http_requests_total{status=~\"5..\"}[5m]))",
				IntegrationID: "vm-main",
				Kind:          "business",
				Layer:         "gold",
				Sentiment:     "negative",
				Classifier:    "stability",
				Definition:    "5xx error rate",
				CreatedAt:     100,
				UpdatedAt:     101,
			},
		}},
	}
	ec := newTestEngineContext(mock)

	out, err := ec.ListKPIs(context.Background(), KPIFilter{Kind: "business", Layer: "gold"})
	if err != nil {
		t.Fatalf("ListKPIs: %v", err)
	}
	if mock.listReq == nil {
		t.Fatal("expected list request to be captured")
	}
	if mock.listReq.TenantId != ec.tenantID {
		t.Fatalf("expected tenant_id %q, got %q", ec.tenantID, mock.listReq.TenantId)
	}
	if mock.listReq.Kind != "business" || mock.listReq.Layer != "gold" {
		t.Fatalf("unexpected filter in request: %+v", mock.listReq)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 KPI, got %d", len(out))
	}
	if out[0].ID != "kpi-1" || out[0].Kind != "business" {
		t.Fatalf("unexpected KPI mapping: %+v", out[0])
	}
}

func TestGetKPI_AutoStampsTenantAndMapsResponse(t *testing.T) {
	mock := &mockEngineClient{
		getResp: &pluginv1.GetKPIResponse{KPI: &pluginv1.KPIView{
			ID:            "kpi-2",
			TenantID:      "018ec9c7-5f3c-7f2e-a10f-111111111111",
			Name:          "Latency",
			Query:         "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))",
			IntegrationID: "vm-main",
			Kind:          "technical",
			Layer:         "silver",
			Sentiment:     "negative",
			Classifier:    "latency",
			Definition:    "P95 latency",
		}},
	}
	ec := newTestEngineContext(mock)

	out, err := ec.GetKPI(context.Background(), "kpi-2")
	if err != nil {
		t.Fatalf("GetKPI: %v", err)
	}
	if mock.getReq == nil {
		t.Fatal("expected get request to be captured")
	}
	if mock.getReq.TenantId != ec.tenantID {
		t.Fatalf("expected tenant_id %q, got %q", ec.tenantID, mock.getReq.TenantId)
	}
	if mock.getReq.KPIID != "kpi-2" {
		t.Fatalf("expected kpi_id kpi-2, got %q", mock.getReq.KPIID)
	}
	if out == nil || out.ID != "kpi-2" || out.Kind != "technical" {
		t.Fatalf("unexpected KPI result: %+v", out)
	}
}

func TestRequestApprovalWithContext_PassesBlastRadiusAndEvidence(t *testing.T) {
	mock := &mockEngineClient{
		approvalResp: &pluginv1.RequestApprovalResponse{Approved: true},
	}
	ec := newTestEngineContext(mock)

	approved, err := ec.RequestApprovalWithContext(
		context.Background(),
		"exec-1",
		"Apply rollout",
		PermissionModify,
		&ApprovalRequestContext{
			ContextJSON: []byte(`{"change_id":"chg-7","plan":"rolling"}`),
			BlastRadius: &pluginv1.BlastRadius{
				ResourceType:   "kubernetes.deployment",
				Resources:      []string{"prod/payments-api"},
				Scope:          "namespace",
				Reversible:     true,
				EstimatedCount: 1,
				Regions:        []string{"us-east-1"},
				Summary:        "Rollout payments-api in prod namespace",
			},
		},
	)
	if err != nil {
		t.Fatalf("RequestApprovalWithContext: %v", err)
	}
	if !approved {
		t.Fatalf("expected approved=true")
	}
	if mock.approvalReq == nil {
		t.Fatalf("approval request was not captured")
	}
	if mock.approvalReq.TenantId != ec.tenantID {
		t.Fatalf("tenant_id=%q want %q", mock.approvalReq.TenantId, ec.tenantID)
	}
	if string(mock.approvalReq.ContextJson) != `{"change_id":"chg-7","plan":"rolling"}` {
		t.Fatalf("context_json=%s", string(mock.approvalReq.ContextJson))
	}
	if mock.approvalReq.BlastRadius == nil {
		t.Fatalf("expected blast_radius to be forwarded")
	}
	if mock.approvalReq.BlastRadius.Summary != "Rollout payments-api in prod namespace" {
		t.Fatalf("blast radius summary mismatch: %q", mock.approvalReq.BlastRadius.Summary)
	}
}

func TestPermissionToProto_IncludesWrite(t *testing.T) {
	if got := permissionToProto(PermissionWrite); got != pluginv1.PermissionWrite {
		t.Fatalf("permissionToProto(PermissionWrite)=%v want %v", got, pluginv1.PermissionWrite)
	}
}

func TestRequestApproval_DefaultMethodWithoutContext(t *testing.T) {
	mock := &mockEngineClient{
		approvalResp: &pluginv1.RequestApprovalResponse{Approved: true},
	}
	ec := newTestEngineContext(mock)

	approved, err := ec.RequestApproval(context.Background(), "exec-2", "Need gate", PermissionAdmin)
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if !approved {
		t.Fatalf("expected approved=true")
	}
	if mock.approvalReq == nil {
		t.Fatalf("approval request was not captured")
	}
	if len(mock.approvalReq.ContextJson) != 0 {
		t.Fatalf("expected empty context_json for default path")
	}
	if mock.approvalReq.BlastRadius != nil {
		t.Fatalf("expected nil blast_radius for default path")
	}
}

func TestValidateTargetPluginNameRejectsColon(t *testing.T) {
	err := validateTargetPluginName("query_vmetrics:range_query")
	if err == nil {
		t.Fatalf("expected validation error for colon-qualified target")
	}
}

func TestCallPluginRawWithTimeRange_RejectsColonTarget(t *testing.T) {
	mock := &mockEngineClient{}
	ec := newTestEngineContext(mock)
	_, err := ec.CallPluginRawWithTimeRange(context.Background(), "query_vmetrics:range_query", map[string]any{
		"action": "range_query",
		"query":  "up",
	}, nil)
	if err == nil {
		t.Fatalf("expected error for colon-qualified target")
	}
	if mock.callPluginReq != nil {
		t.Fatalf("expected no gRPC call when target validation fails")
	}
}

func TestCallPluginRawWithTimeRange_PreservesNativeJSONParams(t *testing.T) {
	mock := &mockEngineClient{
		callPluginResp: &pluginv1.CallPluginResponse{
			Success:    true,
			ResultJson: []byte(`{"status":"ok"}`),
		},
	}
	ec := newTestEngineContext(mock)
	params := map[string]any{
		"action": "range_query",
		"query":  "up",
		"flags":  []any{true, false},
		"meta":   map[string]any{"limit": 10, "live": true, "note": nil},
	}
	raw, err := ec.CallPluginRawWithTimeRange(context.Background(), "query_vmetrics", params, &pluginv1.TimeRange{
		StartEpochMs: 1,
		EndEpochMs:   2,
		Timezone:     "UTC",
	})
	if err != nil {
		t.Fatalf("CallPluginRawWithTimeRange: %v", err)
	}
	if string(raw) != `{"status":"ok"}` {
		t.Fatalf("unexpected response payload: %s", string(raw))
	}
	if mock.callPluginReq == nil {
		t.Fatalf("expected CallPlugin request capture")
	}
	if mock.callPluginReq.TargetPlugin != "query_vmetrics" {
		t.Fatalf("target_plugin=%q", mock.callPluginReq.TargetPlugin)
	}
	var decoded map[string]any
	if err := json.Unmarshal(mock.callPluginReq.ParamsJson, &decoded); err != nil {
		t.Fatalf("decode params_json: %v", err)
	}
	if decoded["action"] != "range_query" {
		t.Fatalf("action=%v", decoded["action"])
	}
	flags, ok := decoded["flags"].([]any)
	if !ok || len(flags) != 2 {
		t.Fatalf("flags=%#v", decoded["flags"])
	}
	meta, ok := decoded["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta=%#v", decoded["meta"])
	}
	if meta["limit"] != float64(10) {
		t.Fatalf("meta.limit=%v", meta["limit"])
	}
	if live, _ := meta["live"].(bool); !live {
		t.Fatalf("meta.live=%v", meta["live"])
	}
	if _, present := meta["note"]; !present {
		t.Fatalf("meta.note key missing")
	}
}
