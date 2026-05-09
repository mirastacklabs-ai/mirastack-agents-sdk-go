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
	// ChatStream is the server-streaming RPC that emits LLM token deltas as
	// they arrive from the upstream model. Implementations that do not support
	// real streaming should embed UnimplementedPluginServiceServer to receive
	// an Unimplemented status code; the engine's grpcProvider falls back to
	// buffered Complete in that case.
	ChatStream(*ChatStreamRequest, PluginService_ChatStreamServer) error
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
func (UnimplementedPluginServiceServer) ChatStream(*ChatStreamRequest, PluginService_ChatStreamServer) error {
	return fmt.Errorf("ChatStream not implemented")
}

// ─── ChatStream — server-streaming interfaces ─────────────────────────────────

// PluginService_ChatStreamServer is the server-side streaming interface for
// ChatStream. The plugin calls Send once per token delta and implicitly closes
// the stream when its ChatStream method returns.
type PluginService_ChatStreamServer interface {
	Send(*ChatTokenResponse) error
	grpc.ServerStream
}

type pluginServiceChatStreamServer struct{ grpc.ServerStream }

func (x *pluginServiceChatStreamServer) Send(m *ChatTokenResponse) error {
	return x.ServerStream.SendMsg(m)
}

// PluginService_ChatStreamClient is the client-side streaming interface for
// ChatStream. The engine calls Recv in a loop until io.EOF.
type PluginService_ChatStreamClient interface {
	Recv() (*ChatTokenResponse, error)
	grpc.ClientStream
}

type pluginServiceChatStreamClient struct{ grpc.ClientStream }

func (x *pluginServiceChatStreamClient) Recv() (*ChatTokenResponse, error) {
	m := new(ChatTokenResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
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
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ChatStream",
			Handler:       _PluginService_ChatStream_Handler,
			ServerStreams: true,
		},
	},
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
	// ChatStream opens a server-streaming RPC. The caller iterates Recv() until
	// io.EOF or a ChatTokenResponse with a non-empty Error field is received.
	ChatStream(ctx context.Context, req *ChatStreamRequest, opts ...grpc.CallOption) (PluginService_ChatStreamClient, error)
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

func (c *pluginServiceClient) ChatStream(ctx context.Context, req *ChatStreamRequest, opts ...grpc.CallOption) (PluginService_ChatStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &PluginService_ServiceDesc.Streams[0], "/mirastack.plugin.v1.PluginService/ChatStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &pluginServiceChatStreamClient{stream}
	if err := x.ClientStream.SendMsg(req); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// --- Server-side ChatStream handler ---

func _PluginService_ChatStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	req := new(ChatStreamRequest)
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	return srv.(PluginServiceServer).ChatStream(req, &pluginServiceChatStreamServer{stream})
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
