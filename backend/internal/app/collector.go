// 文件用途：SNMP/OPC UA 轮询采集器 app 装配（ROADMAP C6 SNMP/OPC UA 管理侧接入）。
// 配置门控：collectors.snmp.enabled / collectors.opcua.enabled（默认关闭，与 CoAP 网关同策略）。
// DB 或 uplink 服务未就绪时降级不启动（不阻断应用启动），与 WithCoAPGateway 语义一致。
// TB-45：OPC UA 采集回填路径挂载 Integration 上行转换执行器（绑定转换器才执行，失败丢弃）。
package app

import (
	"aetherlink-iot/backend/internal/collector"
	model "aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

// CollectorsWrapper 包装采集 Runner 组为 Service。
type CollectorsWrapper struct {
	cfg       collector.Config
	runners   []*collector.Runner
	isEnabled bool
	logger    *logrus.Logger
	// integrationResolver OPC UA 采集回填路径的 Integration 上行转换执行器（TB-45），
	// 保留引用以便后续诊断面读取 Transformed/Dropped 计数。
	integrationResolver *collector.IntegrationResolver
}

// Name 返回服务名称。
func (w *CollectorsWrapper) Name() string { return "SNMP/OPC UA 采集器" }

// Start 启动全部启用的采集 Runner（每个 Runner 独立 goroutine 常驻）。
func (w *CollectorsWrapper) Start() error {
	if !w.isEnabled {
		w.logger.Info("collectors is disabled, skipping...")
		return nil
	}
	for _, r := range w.runners {
		go r.Run()
	}
	w.logger.WithFields(logrus.Fields{
		"snmp":     w.cfg.SNMPEnabled,
		"opcua":    w.cfg.OpcuaEnabled,
		"interval": w.cfg.Interval,
	}).Info("collectors started")
	return nil
}

// Stop 停止全部 Runner（ServiceManager 反序停机由外层保证）。
func (w *CollectorsWrapper) Stop() error {
	if !w.isEnabled {
		return nil
	}
	for _, r := range w.runners {
		r.Stop()
	}
	w.logger.Info("collectors stopped")
	return nil
}

// WithCollectors 可选启动 SNMP/OPC UA 轮询采集器。
// 依赖：WithDatabase 与 WithFlowService 必须先于本选项注册。
func WithCollectors() Option {
	return func(a *Application) error {
		cfg := collector.DefaultConfig()
		wrapper := &CollectorsWrapper{cfg: cfg, isEnabled: false, logger: a.Logger}
		if !cfg.SNMPEnabled && !cfg.OpcuaEnabled {
			a.RegisterService(wrapper)
			return nil
		}
		if a.DB == nil || a.uplinkService == nil {
			a.Logger.Warn("collectors: DB/uplink 未就绪，采集器不启动（遥测汇入不可用）")
			a.RegisterService(wrapper)
			return nil
		}
		publisher := uplinkBusPublisher{bus: a.GetUplinkBus()}
		if cfg.SNMPEnabled {
			wrapper.runners = append(wrapper.runners,
				collector.NewRunner(a.DB, publisher, collector.SnmpPoller{}, cfg.Interval, cfg.Timeout, a.Logger))
		}
		if cfg.OpcuaEnabled {
			// TB-45：OPC UA 采集回填路径挂 Integration 上行转换（绑定 + 挂转换器才执行）。
			wrapper.integrationResolver = collector.NewIntegrationResolver(a.DB, model.IntegrationConnectorOpcua, a.Logger)
			poller := collector.NewOpcuaPoller(a.Logger)
			poller.Transform = wrapper.integrationResolver.Hook
			wrapper.runners = append(wrapper.runners,
				collector.NewRunner(a.DB, publisher, poller, cfg.Interval, cfg.Timeout, a.Logger))
		}
		wrapper.isEnabled = true
		a.RegisterService(wrapper)
		return nil
	}
}
