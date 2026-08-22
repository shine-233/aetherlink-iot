// 文件用途：提供 broker 插件注册 HTTP/gRPC API 的统一入口和 server 构造逻辑。
// 核心逻辑：维护 APIRegistrar、HTTP handler、gRPC server、gateway 和 TLS 凭证装配。
// 使用注意：该文件影响插件暴露管理接口的方式，端口、TLS、handler 注册顺序都属于兼容边界。
// 重构建议：后续可按 HTTP、gRPC、gateway/TLS 三块拆分初始化逻辑，并补充启动失败路径说明。

package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

// HTTPMiddleware 包装 API HTTP server 的根 handler，用于插件级鉴权等横切处理。
type HTTPMiddleware = func(http.Handler) http.Handler

// HTTPMiddlewareSetter 是 APIRegistrar 的可选扩展能力：
// 允许插件在 broker 开始对外服务前为所有 API HTTP server 安装根级中间件。
type HTTPMiddlewareSetter interface {
	SetHTTPMiddleware(mw HTTPMiddleware)
}

// GRPCUnaryInterceptor 是 API gRPC server 的 unary 拦截器，用于插件级鉴权等横切处理。
type GRPCUnaryInterceptor = grpc.UnaryServerInterceptor

// GRPCUnaryInterceptorSetter 是 APIRegistrar 的可选扩展能力：
// 允许插件在 broker 开始对外服务前为所有 API gRPC server 追加一元拦截器。
type GRPCUnaryInterceptorSetter interface {
	AddGRPCUnaryInterceptor(ic GRPCUnaryInterceptor)
}

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

// AddGRPCUnaryInterceptor implements GRPCUnaryInterceptorSetter interface.
// 将插件提供的一元拦截器追加到所有 API gRPC server 的拦截器链尾部（先注册的先执行）；
// 必须在 broker 开始对外服务（Run）之前调用，nil 拦截器被忽略。
func (a *apiRegistrar) AddGRPCUnaryInterceptor(ic GRPCUnaryInterceptor) {
	if ic == nil {
		return
	}
	for _, v := range a.gRPCServers {
		v.appendUnaryInterceptor(ic)
	}
}

type gRPCServer struct {
	server   *grpc.Server
	serve    func(errChan chan error) error
	shutdown func()
	endpoint string

	unaryMu           sync.RWMutex
	unaryInterceptors []GRPCUnaryInterceptor
}

// appendUnaryInterceptor 追加一个插件注册的一元拦截器。
func (g *gRPCServer) appendUnaryInterceptor(ic GRPCUnaryInterceptor) {
	g.unaryMu.Lock()
	defer g.unaryMu.Unlock()
	g.unaryInterceptors = append(g.unaryInterceptors, ic)
}

// dispatchUnary 是构造期挂入 grpc.ChainUnaryInterceptor 的动态分发器：
// grpc.Server 的拦截器选项只能在构造时传入，插件后置注册的拦截器由它
// 在请求期按快照顺序执行。该分发器位于日志与监控拦截器之后，
// 被拒绝的调用仍会留下日志和指标。
func (g *gRPCServer) dispatchUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	g.unaryMu.RLock()
	chain := make([]GRPCUnaryInterceptor, len(g.unaryInterceptors))
	copy(chain, g.unaryInterceptors)
	g.unaryMu.RUnlock()
	for i := len(chain) - 1; i >= 0; i-- {
		next := handler
		handler = func(ctx context.Context, req interface{}) (interface{}, error) {
			return chain[i](ctx, req, info, next)
		}
	}
	return handler(ctx, req)
}

type httpServer struct {
	gRPCEndpoint string
	endpoint     string
	mux          *runtime.ServeMux
	server       *http.Server
	tlsCfg       *tls.Config
	middleware   HTTPMiddleware
	serve        func(errChan chan error) error
	shutdown     func()
}

// HTTPHandler is the http handler defined by gRPC-gateway.
type HTTPHandler = func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error)

// SetHTTPMiddleware implements HTTPMiddlewareSetter interface.
// 为所有 API HTTP server 安装根级中间件；必须在 broker 开始对外服务（Run）之前调用，
// 后安装的中间件覆盖先安装的。中间件作用于该 gateway 上的全部请求。
func (a *apiRegistrar) SetHTTPMiddleware(mw HTTPMiddleware) {
	if mw == nil {
		return
	}
	for _, v := range a.httpServers {
		v.middleware = mw
		v.server.Handler = mw(v.mux)
	}
}
