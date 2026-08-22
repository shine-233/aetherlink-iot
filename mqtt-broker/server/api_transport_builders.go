// 文件用途：承接 broker 管理 API 的 HTTP/gRPC/TLS builder，集中 listener 侧传输装配。
// 核心逻辑：统一处理 endpoint 解析、TLS 配置加载、gRPC server 构造和 HTTP gateway server 构造。
// 使用注意：证书读取、监听地址解析、拦截器顺序和 TLS listener 包装都属于外部兼容边界，修改前需补 focused API 验证。
// 重构建议：后续可继续把 TLS 配置、gRPC 构造和 HTTP 构造拆成更细 helper，但不要改变当前启动和关闭语义。
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"strings"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	gcodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"

	"github.com/DrmagicE/gmqtt/config"
)

func splitEndpoint(endpoint string) (schema string, addr string) {
	epParts := strings.SplitN(endpoint, "://", 2)
	if len(epParts) == 1 && epParts[0] != "" {
		epParts = []string{"tcp", epParts[0]}
	}
	return epParts[0], epParts[1]
}

func buildTLSConfig(cfg *config.TLSOptions) (*tls.Config, error) {
	c, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if cfg.CACert != "" {
		b, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(b)
	}
	var cliAuthType tls.ClientAuthType
	if cfg.Verify {
		cliAuthType = tls.RequireAndVerifyClientCert
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{c},
		ClientCAs:    certPool,
		ClientAuth:   cliAuthType,
	}
	return tlsCfg, nil
}

func buildGRPCServer(endpoint *config.Endpoint) (*gRPCServer, error) {
	var cred credentials.TransportCredentials
	if cfg := endpoint.TLS; cfg != nil {
		tlsCfg, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		cred = credentials.NewTLS(tlsCfg)
	}
	gs := &gRPCServer{
		endpoint: endpoint.Address,
	}
	server := grpc.NewServer(
		grpc.Creds(cred),
		grpc.ChainUnaryInterceptor(
			grpc_zap.UnaryServerInterceptor(zaplog, grpc_zap.WithLevels(func(code gcodes.Code) zapcore.Level {
				if code == gcodes.OK {
					return zapcore.DebugLevel
				}
				return grpc_zap.DefaultClientCodeToLevel(code)
			})),
			grpc_prometheus.UnaryServerInterceptor,
			// 插件后置注册的一元拦截器统一经该分发器进入执行链，详见 dispatchUnary。
			gs.dispatchUnary,
		),
	)
	grpc_prometheus.Register(server)
	shutdown := func() {
		server.Stop()
	}
	serve := func(errChan chan error) error {
		schema, addr := splitEndpoint(endpoint.Address)
		l, err := net.Listen(schema, addr)
		if err != nil {
			return err
		}
		go func() {
			select {
			case errChan <- server.Serve(l):
			default:
			}
		}()
		return nil
	}

	gs.server = server
	gs.serve = serve
	gs.shutdown = shutdown
	return gs, nil
}

func buildHTTPServer(endpoint *config.Endpoint) (*httpServer, error) {
	var tlsCfg *tls.Config
	var err error
	if cfg := endpoint.TLS; cfg != nil {
		tlsCfg, err = buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
	}
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true}))
	server := &http.Server{
		Handler: mux,
	}
	shutdown := func() {
		server.Shutdown(context.Background())
	}
	serve := func(errChan chan error) error {
		schema, addr := splitEndpoint(endpoint.Address)
		l, err := net.Listen(schema, addr)
		if err != nil {
			return err
		}
		if tlsCfg != nil {
			l = tls.NewListener(l, tlsCfg)
		}
		go func() {
			select {
			case errChan <- server.Serve(l):
			default:
			}
		}()
		return nil
	}

	return &httpServer{
		gRPCEndpoint: endpoint.Map,
		mux:          mux,
		server:       server,
		serve:        serve,
		shutdown:     shutdown,
		endpoint:     endpoint.Address,
	}, nil
}
