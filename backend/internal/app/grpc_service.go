// 文件用途：承载应用启动编排中 grpc service 相关的服务或配置逻辑。
// 核心逻辑：把配置加载、服务注册、启动、停止和依赖注入封装为应用层可组合入口，主要围绕 type GRPCService、func NewGRPCService、func (s *GRPCService) Name、func (s *GRPCService) Start 等声明展开。
// 关键注意事项：应用生命周期影响全局资源，修改需保持幂等、关闭顺序和错误回滚语义。
// 重构建议：后续可继续收敛服务接口边界，让启动编排与具体基础设施实现解耦。

package app

import (
	"strings"

	tptodb "aetherlink-iot/backend/third_party/grpc/tptodb_client"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// GRPCService 实现gRPC客户端服务
type GRPCService struct {
	initialized bool
}

// NewGRPCService 创建gRPC服务实例
func NewGRPCService() *GRPCService {
	return &GRPCService{
		initialized: false,
	}
}

// Name 返回服务名称
func (s *GRPCService) Name() string {
	return "gRPC客户端服务"
}

func externalTelemetryGRPCEnabled() bool {
	switch strings.ToUpper(strings.TrimSpace(viper.GetString("grpc.tptodb_type"))) {
	case "TSDB", "KINGBASE", "POLARDB":
		return true
	default:
		return false
	}
}

// Start initializes the compatibility client only for explicitly selected external stores.
// NONE and local database modes keep telemetry queries on the built-in persistence path.
func (s *GRPCService) Start() error {
	if !externalTelemetryGRPCEnabled() {
		logrus.Info("external telemetry gRPC integration is disabled; using local storage")
		return nil
	}

	logrus.Info("正在初始化外部遥测 gRPC 客户端...")
	if err := tptodb.GrpcTptodbInit(); err != nil {
		return err
	}

	s.initialized = true
	logrus.Info("外部遥测 gRPC 客户端初始化完成")
	return nil
}

// Stop 停止gRPC服务
func (s *GRPCService) Stop() error {
	if !s.initialized {
		return nil
	}

	logrus.Info("正在停止gRPC客户端...")
	// 关闭 gRPC 客户端连接，释放底层资源
	tptodb.Close()
	s.initialized = false
	logrus.Info("gRPC客户端已停止")
	return nil
}

// WithGRPCService 将gRPC服务添加到应用
func WithGRPCService() Option {
	return func(app *Application) error {
		service := NewGRPCService()
		app.RegisterService(service)
		return nil
	}
}
