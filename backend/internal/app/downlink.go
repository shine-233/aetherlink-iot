// 文件用途：承载应用启动编排中 downlink 相关的服务或配置逻辑。
// 核心逻辑：把配置加载、服务注册、启动、停止和依赖注入封装为应用层可组合入口，主要围绕 type DownlinkServiceWrapper、func (s *DownlinkServiceWrapper) Name、func (s *DownlinkServiceWrapper) Start、func (s *DownlinkServiceWrapper) Stop 等声明展开。
// 关键注意事项：应用生命周期影响全局资源，修改需保持幂等、关闭顺序和错误回滚语义。
// 重构建议：后续可继续收敛服务接口边界，让启动编排与具体基础设施实现解耦。

package app

import (
	"context"
	"fmt"

	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/processor"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// DownlinkServiceWrapper 下行服务包装器
type DownlinkServiceWrapper struct {
	bus       *downlink.Bus
	handler   *downlink.Handler
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *logrus.Logger
	processor *processor.ScriptProcessor
}

// Name 服务名称
func (s *DownlinkServiceWrapper) Name() string {
	return "Downlink 下行服务"
}

// Start 启动服务
func (s *DownlinkServiceWrapper) Start() error {
	if !mqttServiceEnabled() {
		s.logger.Info("Downlink service disabled because MQTT service is disabled")
		return nil
	}

	// ✨ 在 Start 时才创建 Handler（此时 Adapter 已经初始化）
	adapter := GetGlobalMQTTAdapter()
	if adapter == nil {
		return fmt.Errorf("global message adapter not initialized")
	}

	// 创建 Handler（直接使用 Adapter 作为 MessagePublisher）
	s.handler = downlink.NewHandler(adapter, s.processor, s.logger)

	// 启动 Bus
	s.bus.Start(s.ctx, s.handler)
	s.logger.Info("Downlink service started successfully")
	return nil
}

// Stop 停止服务
func (s *DownlinkServiceWrapper) Stop() error {
	s.logger.Info("Stopping downlink service...")
	s.cancel()
	s.bus.Close()
	s.logger.Info("Downlink service stopped")
	return nil
}

// GetBus 获取消息总线（供 Service 层调用）
func (s *DownlinkServiceWrapper) GetBus() *downlink.Bus {
	return s.bus
}

// WithDownlinkService 创建下行服务
func WithDownlinkService() Option {
	return func(a *Application) error {
		// 1. 读取配置
		bufferSize := viper.GetInt("downlink.buffer_size")
		if bufferSize <= 0 {
			bufferSize = 1000 // 默认值
		}

		// 2. 创建消息总线
		bus := downlink.NewBus(bufferSize)

		// 3. 创建 Processor（提前创建，Handler 在 Start 时创建）
		dataProcessor := processor.NewScriptProcessor()

		// 4. 创建 context
		ctx, cancel := context.WithCancel(context.Background())

		// 5. 创建服务包装器（handler 在 Start 时初始化）
		wrapper := &DownlinkServiceWrapper{
			bus:       bus,
			handler:   nil, // Created during Start.
			ctx:       ctx,
			cancel:    cancel,
			logger:    a.Logger,
			processor: dataProcessor,
		}

		// 6. 注册到 ServiceManager
		a.RegisterService(wrapper)

		// 7. 保存到 Application
		a.downlinkService = wrapper

		// ✨ 8. 注入到 Service 层
		service.GroupApp.CommandData.SetDownlinkBus(bus)
		service.GroupApp.AttributeData.SetDownlinkBus(bus)
		service.GroupApp.TelemetryData.SetDownlinkBus(bus)

		a.Logger.WithFields(logrus.Fields{
			"module":      "downlink",
			"buffer_size": bufferSize,
		}).Info("Downlink service initialized and injected into services")

		return nil
	}
}
