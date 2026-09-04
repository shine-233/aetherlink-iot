// 文件用途：C6 协议网关 app 装配（ROADMAP C6）——由配置门控启动 CoAP/LwM2M UDP 监听。
// P1-C 收尾：DB + uplink Bus 可用时一并装配遥测汇入桥（端点凭证映射 → UplinkMessage → Bus）。
package app

import (
	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/protocolgw"
	"aetherlink-iot/backend/internal/uplink"
)

// uplinkBusPublisher 把 *uplink.Bus 适配为 protocolgw.UplinkPublisher（保持窄类型发布面）。
type uplinkBusPublisher struct{ bus *uplink.Bus }

func (p uplinkBusPublisher) Publish(msg *mqttadapter.UplinkMessage) error {
	return p.bus.Publish(msg)
}

// WithCoAPGateway 可选启动 CoAP/LwM2M 协议网关（protocols.coap.enabled=true 时）。
// DB 或 uplink 服务未就绪时降级为纯接入层（仅注册/读写，不汇入遥测），不阻断启动。
func WithCoAPGateway() Option {
	return func(a *Application) error {
		cfg := protocolgw.DefaultConfig()
		var bridge *protocolgw.TelemetryBridge
		if a.DB != nil && a.uplinkService != nil {
			bridge = protocolgw.NewTelemetryBridge(
				protocolgw.NewDBNumberResolver(a.DB),
				uplinkBusPublisher{bus: a.GetUplinkBus()},
				a.Logger,
			)
		} else if a.Logger != nil {
			a.Logger.Warn("coap gateway: DB/uplink 未就绪，遥测汇入降级为纯接入层")
		}
		gw, err := protocolgw.Start(cfg, a.Logger, protocolgw.WithTelemetry(bridge))
		if err != nil {
			return err
		}
		a.CoAPGateway = gw
		return nil
	}
}
