// 文件用途：TCP 入站协议网关 app 装配（TP-03）——protocols.tcp.enabled 门控启动 + 下行命令缝。
// 核心逻辑：网关复用 protocolgw 的 DBNumberResolver（device_number 凭证映射，租户守卫）与
// uplink 总线汇入遥测；下行经 tcpFallbackPublisher 包装原 MQTT 发布器——已注册 TCP 会话的设备
// 走 TCP 帧（离线入环形缓冲，重连续传），其余设备保持 MQTT 行为不变。
// 关键注意事项：DB/uplink 未就绪时降级不启动（不阻断应用启动，与 WithCoAPGateway/插件网关
// 语义一致）；全局实例在 Shutdown 时 Stop（区别于 CoAP 的进程级常驻，TCP 会话需显式回收）。
// 重构建议：如后续需要 per-tenant 端口或多实例监听，把全局单例改为 Application 持有切片。
package app

import (
	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/protocolgw"
	"aetherlink-iot/backend/internal/protocolgw/tcp"
)

// globalTCPGateway 全局 TCP 网关实例（与 globalMQTTAdapter 同款进程级单例口；
// 下行服务 Start 时按此判断是否包回退发布器，规避 Option 声明顺序问题）。
var globalTCPGateway *tcp.Gateway

// GetGlobalTCPGateway 获取全局 TCP 网关实例（未启用/未启动返回 nil）。
func GetGlobalTCPGateway() *tcp.Gateway {
	return globalTCPGateway
}

// tcpFallbackPublisher 下行回退发布器：TCP 注册过的设备走 TCP 通道（在线直投/离线入
// 环形缓冲），其余设备回落原 MQTT 发布器——保证非 TCP 设备零行为变化。
type tcpFallbackPublisher struct {
	base downlink.MessagePublisher
	gw   *tcp.Gateway
}

// 编译期断言：满足 downlink.Handler 所需的协议无关发布接口。
var _ downlink.MessagePublisher = tcpFallbackPublisher{}

// PublishMessage 命中 TCP 设备 → DeliverCommand（离线入缓冲返回 nil）；否则回落 MQTT。
func (p tcpFallbackPublisher) PublishMessage(
	deviceNumber string,
	msgType downlink.MessageType,
	deviceType string,
	topicPrefix string,
	messageID string,
	qos byte,
	payload []byte,
) error {
	if p.gw.HandlesDevice(deviceNumber) {
		return p.gw.DeliverCommand(deviceNumber, payload)
	}
	return p.base.PublishMessage(deviceNumber, msgType, deviceType, topicPrefix, messageID, qos, payload)
}

// WithTCPGateway 可选启动 TCP 入站协议网关（protocols.tcp.enabled=true 时）。
// 启用但 DB/uplink 未就绪时降级不启动（纯 TCP 监听无法完成凭证映射，比 CoAP 纯接入层更严格）。
func WithTCPGateway() Option {
	return func(a *Application) error {
		cfg := tcp.DefaultConfig()
		if !cfg.Enabled {
			a.Logger.Info("tcp gateway disabled (protocols.tcp.enabled=false)")
			return nil
		}
		if a.DB == nil || a.uplinkService == nil {
			a.Logger.Warn("tcp gateway: DB/uplink 未就绪，降级不启动")
			return nil
		}
		gw, err := tcp.Start(cfg, tcp.Dependencies{
			Resolver:  protocolgw.NewDBNumberResolver(a.DB),
			Publisher: uplinkBusPublisher{bus: a.GetUplinkBus()},
		}, a.Logger)
		if err != nil {
			return err
		}
		globalTCPGateway = gw
		a.TCPGateway = gw
		a.Logger.WithField("addr", cfg.Addr).Info("tcp gateway started")
		return nil
	}
}

// stopTCPGateway 应用关停时回收 TCP 网关（踢全部会话、关监听）。
func stopTCPGateway() {
	if globalTCPGateway == nil {
		return
	}
	globalTCPGateway.Stop()
	globalTCPGateway = nil
}
