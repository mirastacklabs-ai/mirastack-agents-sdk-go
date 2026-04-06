package pluginv1

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
)

// ---------------------------------------------------------------------------
// PluginService — Server (plugin binary implements this)
// ---------------------------------------------------------------------------

// PluginServiceServer is the interface a plugin process must serve.
type PluginServiceServer interface {
	Info(context.Context, *InfoRequest) (*InfoResponse, error)
	GetSchema(context.Context, *GetSchemaRequest) (*GetSchemaResponse, error)
	Execute(context.Context, *ExecuteRequest) (*ExecuteResponse, error)
	HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error)
	ConfigUpdated(context.Context, *ConfigUpdatedRequest) (*ConfigUpdatedResponse, error)
}

// UnimplementedPluginServiceServer provides forward compatibility.
type UnimplementedPluginServiceServer struct{}

func (UnimplementedPluginServiceServer) Info(context.Context, *InfoRequest) (*InfoResponse, error) {
	return nil, fmt.Errorf("Info not implemented")
}
func (UnimplementedPluginServiceServer) GetSchema(context.Context, *GetSchemaRequest) (*GetSchemaResponse, error) {
	return nil, fmt.Errorf("GetSchema not implemented")
}
func (UnimplementedPluginServiceServer) Execute(context.Context, *ExecuteRequest) (*ExecuteResponse, error) {
	return nil, fmt.Errorf("Execute not implemented")
}
func (UnimplementedPluginServiceServer) HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error) {
	return nil, fmt.Errorf("HealthCheck not implemented")
}
func (UnimplementedPluginServiceServer) ConfigUpdated(context.Context, *ConfigUpdatedRequest) (*ConfigUpdatedResponse, error) {
	return nil, fmt.Errorf("ConfigUpdated not implemented")
}

// PluginService_ServiceDesc is the grpc.ServiceDesc for PluginService.
var PluginService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "mirastack.plugin.v1.PluginService",
	HandlerType: (*PluginServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Info", Handler: _PluginService_Info_Handler},
		{MethodName: "GetSchema", Handler: _PluginService_GetSchema_Handler},
		{MethodName: "Execute", Handler: _PluginService_Execute_Handler},
		{MethodName: "HealthCheck", Handler: _PluginService_HealthCheck_Handler},
		{MethodName: "ConfigUpdated", Handler: _PluginService_ConfigUpdated_Handler},
	},
	Streams: []grpc.StreamDesc{},
}

// RegisterPluginServiceServer registers the PluginService server with a gRPC server.
func RegisterPluginServiceServer(s *grpc.Server, srv PluginServiceServer) {
	s.RegisterService(&PluginService_ServiceDesc, srv)
}

// --- Server-side handlers (JSON codec) ---

func _PluginService_Info_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(InfoRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(PluginServiceServer).Info(ctx, req)
}

func _PluginService_GetSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(GetSchemaRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(PluginServiceServer).GetSchema(ctx, req)
}

func _PluginService_Execute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(ExecuteRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(PluginServiceServer).Execute(ctx, req)
}

func _PluginService_HealthCheck_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(HealthCheckRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(PluginServiceServer).HealthCheck(ctx, req)
}

func _PluginService_ConfigUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	req := new(ConfigUpdatedRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	return srv.(PluginServiceServer).ConfigUpdated(ctx, req)
}

// ---------------------------------------------------------------------------
// PluginService — Client (engine uses this to call plugins)
// ---------------------------------------------------------------------------

// PluginServiceClient provides typed access to a running plugin.
type PluginServiceClient interface {
	Info(ctx context.Context) (*InfoResponse, error)
	GetSchema(ctx context.Context) (*GetSchemaResponse, error)
	Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)
	HealthCheck(ctx context.Context) (*HealthCheckResponse, error)
	ConfigUpdated(ctx context.Context, req *ConfigUpdatedRequest) (*ConfigUpdatedResponse, error)
}

type pluginServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewPluginServiceClient creates a new PluginService client.
func NewPluginServiceClient(cc grpc.ClientConnInterface) PluginServiceClient {
	return &pluginServiceClient{cc: cc}
}

func (c *pluginServiceClient) Info(ctx context.Context) (*InfoResponse, error) {
	req := &InfoRequest{}
	out := new(InfoResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.PluginService/Info", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) GetSchema(ctx context.Context) (*GetSchemaResponse, error) {
	req := &GetSchemaRequest{}
	out := new(GetSchemaResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.PluginService/GetSchema", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error) {
	out := new(ExecuteResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.PluginService/Execute", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) HealthCheck(ctx context.Context) (*HealthCheckResponse, error) {
	req := &HealthCheckRequest{}
	out := new(HealthCheckResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.PluginService/HealthCheck", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) ConfigUpdated(ctx context.Context, req *ConfigUpdatedRequest) (*ConfigUpdatedResponse, error) {
	out := new(ConfigUpdatedResponse)
	err := c.cc.Invoke(ctx, "/mirastack.plugin.v1.PluginService/ConfigUpdated", req, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// JSON codec for gRPC — used instead of protobuf wire format.
// Implements the encoding.CodecV2 interface for gRPC v1.65+.
// ---------------------------------------------------------------------------

// JSONCodec implements gRPC encoding using JSON instead of protobuf.
// Register with encoding.RegisterCodec or pass to grpc.ForceCodecV2.
type JSONCodec struct{}

func (JSONCodec) Name() string { return "json" }

func (JSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (JSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
