package pluginv1

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
)

// ---------------------------------------------------------------------------
// EngineService — Server (engine process implements this)
// ---------------------------------------------------------------------------

// EngineServiceServer is the interface the engine serves for plugin callbacks.
type EngineServiceServer interface {
	GetConfig(context.Context, *GetConfigRequest) (*GetConfigResponse, error)
	CacheGet(context.Context, *CacheGetRequest) (*CacheGetResponse, error)
	CacheSet(context.Context, *CacheSetRequest) (*CacheSetResponse, error)
	PublishResult(context.Context, *PublishResultRequest) (*PublishResultResponse, error)
	RequestApproval(context.Context, *RequestApprovalRequest) (*RequestApprovalResponse, error)
	LogEvent(context.Context, *LogEventRequest) (*LogEventResponse, error)
	CallPlugin(context.Context, *CallPluginRequest) (*CallPluginResponse, error)
}

// UnimplementedEngineServiceServer provides forward compatibility.
type UnimplementedEngineServiceServer struct{}

func (UnimplementedEngineServiceServer) GetConfig(context.Context, *GetConfigRequest) (*GetConfigResponse, error) {
	return nil, fmt.Errorf("GetConfig not implemented")
}
func (UnimplementedEngineServiceServer) CacheGet(context.Context, *CacheGetRequest) (*CacheGetResponse, error) {
	return nil, fmt.Errorf("CacheGet not implemented")
}
func (UnimplementedEngineServiceServer) CacheSet(context.Context, *CacheSetRequest) (*CacheSetResponse, error) {
	return nil, fmt.Errorf("CacheSet not implemented")
}
func (UnimplementedEngineServiceServer) PublishResult(context.Context, *PublishResultRequest) (*PublishResultResponse, error) {
	return nil, fmt.Errorf("PublishResult not implemented")
}
func (UnimplementedEngineServiceServer) RequestApproval(context.Context, *RequestApprovalRequest) (*RequestApprovalResponse, error) {
	return nil, fmt.Errorf("RequestApproval not implemented")
}
func (UnimplementedEngineServiceServer) LogEvent(context.Context, *LogEventRequest) (*LogEventResponse, error) {
	return nil, fmt.Errorf("LogEvent not implemented")
}
func (UnimplementedEngineServiceServer) CallPlugin(context.Context, *CallPluginRequest) (*CallPluginResponse, error) {
	return nil, fmt.Errorf("CallPlugin not implemented")
}

// EngineService_ServiceDesc is the grpc.ServiceDesc for EngineService.
var EngineService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "mirastack.plugin.v1.EngineService",
	HandlerType: (*EngineServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "GetConfig", Handler: _EngineService_GetConfig_Handler},
		{MethodName: "CacheGet", Handler: _EngineService_CacheGet_Handler},
		{MethodName: "CacheSet", Handler: _EngineService_CacheSet_Handler},
		{MethodName: "PublishResult", Handler: _EngineService_PublishResult_Handler},
		{MethodName: "RequestApproval", Handler: _EngineService_RequestApproval_Handler},
		{MethodName: "LogEvent", Handler: _EngineService_LogEvent_Handler},
		{MethodName: "CallPlugin", Handler: _EngineService_CallPlugin_Handler},
	},
	Streams: []grpc.StreamDesc{},
}

// RegisterEngineServiceServer registers the EngineService server with a gRPC server.
func RegisterEngineServiceServer(s *grpc.Server, srv EngineServiceServer) {
	s.RegisterService(&EngineService_ServiceDesc, srv)
}

// --- Server-side handlers ---

func _EngineService_GetConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(GetConfigRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).GetConfig(ctx, req)
}

func _EngineService_CacheGet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(CacheGetRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).CacheGet(ctx, req)
}

func _EngineService_CacheSet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(CacheSetRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).CacheSet(ctx, req)
}

func _EngineService_PublishResult_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(PublishResultRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).PublishResult(ctx, req)
}

func _EngineService_RequestApproval_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(RequestApprovalRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).RequestApproval(ctx, req)
}

func _EngineService_LogEvent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(LogEventRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).LogEvent(ctx, req)
}

func _EngineService_CallPlugin_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(CallPluginRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(EngineServiceServer).CallPlugin(ctx, req)
}

// ---------------------------------------------------------------------------
// EngineService — Client (plugins use this to call back to the engine)
// ---------------------------------------------------------------------------

// EngineServiceClient provides typed access to engine callbacks.
type EngineServiceClient interface {
	GetConfig(ctx context.Context, req *GetConfigRequest) (*GetConfigResponse, error)
	CacheGet(ctx context.Context, req *CacheGetRequest) (*CacheGetResponse, error)
	CacheSet(ctx context.Context, req *CacheSetRequest) (*CacheSetResponse, error)
	PublishResult(ctx context.Context, req *PublishResultRequest) (*PublishResultResponse, error)
	RequestApproval(ctx context.Context, req *RequestApprovalRequest) (*RequestApprovalResponse, error)
	LogEvent(ctx context.Context, req *LogEventRequest) (*LogEventResponse, error)
	CallPlugin(ctx context.Context, req *CallPluginRequest) (*CallPluginResponse, error)
}

type engineServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewEngineServiceClient creates a new EngineService client.
func NewEngineServiceClient(cc grpc.ClientConnInterface) EngineServiceClient {
	return &engineServiceClient{cc: cc}
}

func (c *engineServiceClient) GetConfig(ctx context.Context, req *GetConfigRequest) (*GetConfigResponse, error) {
	out := new(GetConfigResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/GetConfig", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *engineServiceClient) CacheGet(ctx context.Context, req *CacheGetRequest) (*CacheGetResponse, error) {
	out := new(CacheGetResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/CacheGet", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *engineServiceClient) CacheSet(ctx context.Context, req *CacheSetRequest) (*CacheSetResponse, error) {
	out := new(CacheSetResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/CacheSet", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *engineServiceClient) PublishResult(ctx context.Context, req *PublishResultRequest) (*PublishResultResponse, error) {
	out := new(PublishResultResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/PublishResult", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *engineServiceClient) RequestApproval(ctx context.Context, req *RequestApprovalRequest) (*RequestApprovalResponse, error) {
	out := new(RequestApprovalResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/RequestApproval", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *engineServiceClient) LogEvent(ctx context.Context, req *LogEventRequest) (*LogEventResponse, error) {
	out := new(LogEventResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/LogEvent", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *engineServiceClient) CallPlugin(ctx context.Context, req *CallPluginRequest) (*CallPluginResponse, error) {
	out := new(CallPluginResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.EngineService/CallPlugin", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
