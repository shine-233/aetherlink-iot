// 文件用途：提供 broker 插件注册 HTTP/gRPC API 的统一入口和 server 构造逻辑。
// 核心逻辑：维护 APIRegistrar、HTTP handler、gRPC server、gateway 和 TLS 凭证装配。
// 使用注意：该文件影响插件暴露管理接口的方式，端口、TLS、handler 注册顺序都属于兼容边界。
// 重构建议：后续可按 HTTP、gRPC、gateway/TLS 三块拆分初始化逻辑，并补充启动失败路径说明。

package server

import (
	"context"
	"crypto/tls"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

// APIRegistrar is the registrar for all gRPC servers and HTTP servers.
// It provides the ability for plugins to register gRPC and HTTP handler.
type APIRegistrar interface {
	// RegisterHTTPHandler registers the handler to all http servers.
	RegisterHTTPHandler(fn HTTPHandler) error
	// RegisterService registers a service and its implementation to all gRPC servers.
	RegisterService(desc *grpc.ServiceDesc, impl interface{})
}

type apiRegistrar struct {
	gRPCServers []*gRPCServer
	httpServers []*httpServer
}

// RegisterService implements APIRegistrar interface
func (a *apiRegistrar) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	for _, v := range a.gRPCServers {
		v.server.RegisterService(desc, impl)
	}
}

// RegisterHTTPHandler implements APIRegistrar interface
func (a *apiRegistrar) RegisterHTTPHandler(fn HTTPHandler) error {
	var err error
	for _, v := range a.httpServers {
		schema, addr := splitEndpoint(v.gRPCEndpoint)
		if schema == "unix" {
			err = fn(context.Background(), v.mux, v.gRPCEndpoint, []grpc.DialOption{grpc.WithInsecure()})
			if err != nil {
				return err
			}
			continue
		}
		err = fn(context.Background(), v.mux, addr, []grpc.DialOption{grpc.WithInsecure()})
		if err != nil {
			return err
		}
	}
	return nil
}

type gRPCServer struct {
	server   *grpc.Server
	serve    func(errChan chan error) error
	shutdown func()
	endpoint string
}

type httpServer struct {
	gRPCEndpoint string
	endpoint     string
	mux          *runtime.ServeMux
	tlsCfg       *tls.Config
	serve        func(errChan chan error) error
	shutdown     func()
}

// HTTPHandler is the http handler defined by gRPC-gateway.
type HTTPHandler = func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error)
