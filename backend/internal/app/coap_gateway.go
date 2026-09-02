// 文件用途：C6 协议网关 app 装配（ROADMAP C6）——由配置门控启动 CoAP/LwM2M UDP 监听。
package app

import (
	"aetherlink-iot/backend/internal/protocolgw"
)

// WithCoAPGateway 可选启动 CoAP/LwM2M 协议网关（protocols.coap.enabled=true 时）。
func WithCoAPGateway() Option {
	return func(a *Application) error {
		cfg := protocolgw.DefaultConfig()
		gw, err := protocolgw.Start(cfg, a.Logger)
		if err != nil {
			return err
		}
		a.CoAPGateway = gw
		return nil
	}
}
