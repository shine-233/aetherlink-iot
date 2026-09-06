package grpcgateway

import (
	"context"

	"google.golang.org/grpc"
)

// PHASE-D-D9 BEGIN 手写服务描述（无 protoc 环境的 gRPC 装配）
//
// 服务名 aetherlink.plugin.v1.PluginGateway；编解码统一 aetherjson。
// Server 端：RegisterPluginService(s, gateway)；
// Client 端：GatewayClient（供插件 SDK/测试复用，bufconn 即插即用）。

// ServiceName gRPC 服务名。
const ServiceName = "aetherlink.plugin.v1.PluginGateway"

// PluginGatewayServer 服务端接口（gateway.Gateway 自身实现）。
type PluginGatewayServer interface {
	Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)
	Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error)
	Uplink(ctx context.Context, req *UplinkFrame) (*UplinkAck, error)
	// Attach 下行命令流；send 由描述符桥接到 ServerStream.SendMsg。
	Attach(ctx context.Context, req *AttachRequest, send func(*DownlinkCommand) error) error
}

func _registerHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginGatewayServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/aetherlink.plugin.v1.PluginGateway/Register"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginGatewayServer).Register(ctx, req.(*RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _heartbeatHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginGatewayServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/aetherlink.plugin.v1.PluginGateway/Heartbeat"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginGatewayServer).Heartbeat(ctx, req.(*HeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _uplinkHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UplinkFrame)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginGatewayServer).Uplink(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/aetherlink.plugin.v1.PluginGateway/Uplink"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginGatewayServer).Uplink(ctx, req.(*UplinkFrame))
	}
	return interceptor(ctx, in, info, handler)
}

func _attachHandler(srv interface{}, stream grpc.ServerStream) error {
	req := new(AttachRequest)
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	return srv.(PluginGatewayServer).Attach(stream.Context(), req, func(cmd *DownlinkCommand) error {
		return stream.SendMsg(cmd)
	})
}

// ServiceDesc 服务描述（方法顺序即契约顺序）。
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*PluginGatewayServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Register", Handler: _registerHandler},
		{MethodName: "Heartbeat", Handler: _heartbeatHandler},
		{MethodName: "Uplink", Handler: _uplinkHandler},
	},
	Streams: []grpc.StreamDesc{
		{StreamName: "Attach", Handler: _attachHandler, ServerStreams: true},
	},
}

// RegisterPluginService 把网关注册到 grpc.Server。
func RegisterPluginService(s *grpc.Server, gateway PluginGatewayServer) {
	s.RegisterService(&ServiceDesc, gateway)
}

// ServerOptions 服务端编解码选项（构造 grpc.Server 时必须带上）。
func ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.ForceServerCodec(jsonCodec{})}
}

// GatewayClient 插件侧客户端（SDK/集成测试复用）。
type GatewayClient struct {
	conn *grpc.ClientConn
}

// NewGatewayClient 建立客户端包装。
func NewGatewayClient(conn *grpc.ClientConn) *GatewayClient {
	return &GatewayClient{conn: conn}
}

// ClientOptions 客户端编解码选项（grpc.NewClient/dial 时必须带上）。
func ClientOptions() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{}))}
}

func (c *GatewayClient) invoke(ctx context.Context, method string, in, out any) error {
	return c.conn.Invoke(ctx, "/"+ServiceName+"/"+method, in, out)
}

// Register 注册。
func (c *GatewayClient) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	out := new(RegisterResponse)
	if err := c.invoke(ctx, "Register", req, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Heartbeat 心跳。
func (c *GatewayClient) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	out := new(HeartbeatResponse)
	if err := c.invoke(ctx, "Heartbeat", req, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Uplink 上行。
func (c *GatewayClient) Uplink(ctx context.Context, req *UplinkFrame) (*UplinkAck, error) {
	out := new(UplinkAck)
	if err := c.invoke(ctx, "Uplink", req, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Attach 建立下行流；返回接收函数（流式读取下行命令）。
func (c *GatewayClient) Attach(ctx context.Context, req *AttachRequest) (func() (*DownlinkCommand, error), error) {
	desc := &grpc.StreamDesc{StreamName: "Attach", ServerStreams: true}
	stream, err := c.conn.NewStream(ctx, desc, "/"+ServiceName+"/Attach")
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	recv := func() (*DownlinkCommand, error) {
		cmd := new(DownlinkCommand)
		if err := stream.RecvMsg(cmd); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	return recv, nil
}

// PHASE-D-D9 END
