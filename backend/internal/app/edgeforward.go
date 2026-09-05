// 文件用途：边缘网关遥测云转发 app 装配（ROADMAP 边缘计算 MVP）。
// 配置门控：edge.forward.enabled（默认关闭）。uplink 服务未就绪时降级不启动
// （转发依赖总线观察者，无总线即无源），与 WithCollectors 语义一致。
package app

import (
	"aetherlink-iot/backend/internal/edgeforward"

	"github.com/sirupsen/logrus"
)

// EdgeForwardWrapper 包装边缘转发器为 Service。
type EdgeForwardWrapper struct {
	forwarder *edgeforward.Forwarder
	logger    *logrus.Logger
}

// Name 返回服务名称。
func (w *EdgeForwardWrapper) Name() string { return "边缘遥测云转发" }

// Start 启动转发器（内部 goroutine 常驻）。
func (w *EdgeForwardWrapper) Start() error {
	w.forwarder.Start()
	w.logger.Info("edge forward started")
	return nil
}

// Stop 停止转发器。
func (w *EdgeForwardWrapper) Stop() error {
	w.forwarder.Stop()
	return nil
}

// WithEdgeForward 可选启动边缘网关遥测云转发。
// 依赖：WithFlowService 必须先于本选项注册（转发订阅 uplink 总线）。
// 配置：edge.forward.enabled/broker/topic-prefix/client-id/username/password/buffer-limit/qos。
func WithEdgeForward() Option {
	return func(a *Application) error {
		cfg := edgeforward.ConfigFromViper()
		if !cfg.Enabled {
			a.RegisterService(skipLogService{name: "边缘遥测云转发", reason: "edge.forward.enabled=false", logger: a.Logger})
			return nil
		}
		if a.DB == nil || a.uplinkService == nil {
			a.Logger.Warn("edge forward: DB/uplink 未就绪，转发器不启动（遥测源不可用）")
			a.RegisterService(skipLogService{name: "边缘遥测云转发", reason: "DB/uplink 未就绪", logger: a.Logger})
			return nil
		}
		wrapper := &EdgeForwardWrapper{forwarder: edgeforward.New(a.GetUplinkBus(), cfg, a.Logger), logger: a.Logger}
		a.RegisterService(wrapper)
		return nil
	}
}

// skipLogService 关闭态占位服务：Start 时打印跳过原因。
type skipLogService struct {
	name   string
	reason string
	logger *logrus.Logger
}

func (s skipLogService) Name() string { return s.name }

func (s skipLogService) Start() error {
	s.logger.Info(s.name + " disabled: " + s.reason)
	return nil
}

func (s skipLogService) Stop() error { return nil }
