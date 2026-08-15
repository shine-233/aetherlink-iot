// uplink_manager.go 负责协调上行总线与各类处理器的生命周期。
// 主要职责：
// 1. 持有 Bus 与 Telemetry/Attribute/Event/Status/Response 五类处理器依赖。
// 2. 在 Start 阶段把总线订阅端连接给各处理器，形成完整消费链路。
// 3. 在 Stop 阶段先关闭总线发布闸门，待队列排空后再停止各处理器。
// 关键注意事项：
// 1. 该文件是上行子系统的装配层，不做具体业务处理，但会决定启动顺序与关闭顺序。
// 2. Start 若在中途失败，当前不会自动回滚已经启动的处理器；后续排障时要把“半启动”纳入考虑。
// 3. Stop 使用同一个 timeout context 等待 Bus 关闭与主处理器退出，超时会返回 context 错误。
// 静态审查建议：
// 1. 后续可补充配置校验，明确哪些依赖必须存在、哪些允许缺省。
// 2. 可把启动/停止结果汇总成结构化状态，方便运维侧定位具体是哪条处理链未成功上线或下线。

package uplink

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// UplinkManager 是上行子系统的生命周期协调器。
// 它将 Bus 视为输入枢纽，并把各类订阅通道挂接到对应处理器。
type UplinkManager struct {
	bus             *Bus
	telemetryUplink *TelemetryUplink
	attributeUplink *AttributeUplink
	eventUplink     *EventUplink
	statusUplink    *StatusUplink
	responseUplink  *ResponseUplink // ✨ 新增

	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc

	handlerDone []<-chan struct{}
}

// UplinkManagerConfig 定义管理器启动所需依赖。
// 大部分处理器允许为空，便于按部署形态裁剪消费链路。
type UplinkManagerConfig struct {
	Bus             *Bus
	TelemetryUplink *TelemetryUplink
	AttributeUplink *AttributeUplink
	EventUplink     *EventUplink
	StatusUplink    *StatusUplink
	ResponseUplink  *ResponseUplink // ✨ 新增
	Logger          *logrus.Logger
}

// NewUplinkManager 创建上行管理器。
// 参数说明：
// 1. Bus 为所有处理器共享的订阅来源，按当前实现应由外部保证非空。
// 2. Logger 为空时回退为标准 logger。
// 3. 其余处理器可按能力选择性注入，Start 时只会启动非 nil 的链路。
func NewUplinkManager(config UplinkManagerConfig) *UplinkManager {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.StandardLogger()
	}

	return &UplinkManager{
		bus:             config.Bus,
		telemetryUplink: config.TelemetryUplink,
		attributeUplink: config.AttributeUplink,
		eventUplink:     config.EventUplink,
		statusUplink:    config.StatusUplink,
		responseUplink:  config.ResponseUplink, // ✨ 新增
		logger:          config.Logger,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动所有已注入的上行处理器。
// 副作用：
// 1. 会立即从 Bus 订阅各类型 channel，并把 channel 交给处理器内部 goroutine 消费。
// 2. Status/Response 处理器启动失败会直接返回错误；已先启动的其他处理器不会在此处回滚。
func (m *UplinkManager) Start() error {
	m.logger.Info("UplinkManager starting...")
	m.handlerDone = m.handlerDone[:0]

	// 先启动高频数据链路，确保设备上报不被长期堆积在总线侧。
	if m.telemetryUplink != nil {
		telemetryChan := m.bus.SubscribeTelemetry()
		m.telemetryUplink.Start(telemetryChan)
		m.handlerDone = append(m.handlerDone, m.telemetryUplink.Done())
		m.logger.Info("TelemetryUplink started")
	}

	// 属性链路通常与遥测并行存在，但消费职责不同。
	if m.attributeUplink != nil {
		attributeChan := m.bus.SubscribeAttribute()
		m.attributeUplink.Start(attributeChan)
		m.handlerDone = append(m.handlerDone, m.attributeUplink.Done())
		m.logger.Info("AttributeUplink started")
	}

	// 事件链路承接告警、业务事件等异步副作用。
	if m.eventUplink != nil {
		eventChan := m.bus.SubscribeEvent()
		m.eventUplink.Start(eventChan)
		m.handlerDone = append(m.handlerDone, m.eventUplink.Done())
		m.logger.Info("EventUplink started")
	}

	// 状态链路会影响在线态、前端订阅与自动化触发，因此单独检查启动错误。
	if m.statusUplink != nil {
		statusChan := m.bus.SubscribeStatus()
		if err := m.statusUplink.Start(statusChan); err != nil {
			m.logger.WithError(err).Error("Failed to start StatusUplink")
			return err
		}
		m.handlerDone = append(m.handlerDone, m.statusUplink.Done())
		m.logger.Info("StatusUplink started")
	}

	// 响应链路承接下行指令回执，部署上可按需启用。
	if m.responseUplink != nil {
		responseChan := m.bus.SubscribeResponse()
		if err := m.responseUplink.Start(responseChan); err != nil {
			m.logger.WithError(err).Error("Failed to start ResponseUplink")
			return err
		}
		m.handlerDone = append(m.handlerDone, m.responseUplink.Done())
		m.logger.Info("ResponseUplink started")
	}

	m.logger.Info("UplinkManager started successfully")
	return nil
}

// Stop closes the bus after upstream producers have stopped, lets buffered
// messages drain through the main handlers, then cancels auxiliary workers.
func (m *UplinkManager) Stop(timeout time.Duration) error {
	m.logger.Info("UplinkManager stopping...")

	// 创建停止超时上下文，避免关闭流程无限等待。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Services are stopped in reverse registration order, so MQTT and heartbeat
	// producers have already been asked to stop before this bus is closed. The
	// bus uses the same deadline as handler draining and never closes a channel
	// while an admitted publisher is still inside Publish.
	if err := m.bus.CloseContext(ctx); err != nil {
		m.stopUplinkProcessors()
		m.logger.WithError(err).Warn("UplinkManager bus close timeout")
		return err
	}

	for _, done := range m.handlerDone {
		select {
		case <-done:
		case <-ctx.Done():
			m.stopUplinkProcessors()
			m.logger.WithError(ctx.Err()).Warn("UplinkManager stop timeout")
			return ctx.Err()
		}
	}

	m.stopUplinkProcessors()
	m.logger.Info("UplinkManager stopped successfully")
	return nil
}

func (m *UplinkManager) stopUplinkProcessors() {
	if m.telemetryUplink != nil {
		m.telemetryUplink.Stop()
	}
	if m.attributeUplink != nil {
		m.attributeUplink.Stop()
	}
	if m.eventUplink != nil {
		m.eventUplink.Stop()
	}
	if m.statusUplink != nil {
		m.statusUplink.Stop()
	}
	if m.responseUplink != nil {
		_ = m.responseUplink.Stop()
	}
}

// GetBusStats 透传 Bus 队列快照，便于统一从管理器层查看总线压力。
func (m *UplinkManager) GetBusStats() map[string]interface{} {
	return m.bus.GetChannelStats()
}
