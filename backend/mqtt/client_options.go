// 文件用途：集中构造后端 Paho MQTT 客户端选项。
// 核心逻辑：统一设置 broker、认证、client id、会话、订阅恢复、重连和消息顺序参数。
// 关键注意事项：该 helper 被发布/订阅客户端复用，修改默认值会影响连接稳定性和消息投递顺序。
// 重构建议：建议为不同客户端类型补表驱动测试，并把生产 broker 默认值限制在配置层。
package mqtt

import (
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-basic/uuid"
)

type PahoClientOptionsConfig struct {
	Broker               string
	Username             string
	Password             string
	ClientID             string
	CleanSession         bool
	ResumeSubs           bool
	AutoReconnect        bool
	ConnectRetryInterval time.Duration
	MaxReconnectInterval time.Duration
	OrderMatters         bool
}

func NewShortClientID(prefix string) string {
	return prefix + uuid.New()[0:8]
}

func NewPahoClientOptions(config PahoClientOptionsConfig) *paho.ClientOptions {
	opts := paho.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetClientID(config.ClientID)
	opts.SetCleanSession(config.CleanSession)
	opts.SetResumeSubs(config.ResumeSubs)
	opts.SetAutoReconnect(config.AutoReconnect)
	if config.ConnectRetryInterval > 0 {
		opts.SetConnectRetryInterval(config.ConnectRetryInterval)
	}
	if config.MaxReconnectInterval > 0 {
		opts.SetMaxReconnectInterval(config.MaxReconnectInterval)
	}
	opts.SetOrderMatters(config.OrderMatters)
	return opts
}
