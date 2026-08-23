// 文件用途：为设备遗嘱消息（will message）补齐与普通上行一致的授权边界。
// 核心逻辑：复用发布主题白名单校验 willTopic；按认证阶段写入 Redis 的 clientID 绑定
// 区分系统账号与设备，并把设备 will payload 与标准上行一致包裹 device_id 后再投递。
// 关键注意事项：未注册 OnWillPublish 时，任意认证设备可在 CONNECT 中声明跨租户主题的
// will，在异常断线时绕过 OnMsgArrived 白名单/schema 门禁注入原始 payload；该钩子是安全边界。
// will 触发时客户端可能已从连接注册表注销，因此必须使用认证期落库的 Redis 绑定而非在线回查。
// 重构建议：后续把 will 校验与 publish 校验收敛到同一个策略入口，避免两处白名单漂移。
package aetherlink

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DrmagicE/gmqtt/plugin/aetherlink/util"
	"github.com/DrmagicE/gmqtt/server"
	"go.uber.org/zap"
)

const (
	// mqttClientDeviceBindingKeyPrefix 复用 rememberMQTTAuthenticatedDevice 写入的既有绑定键。
	mqttClientDeviceBindingKeyPrefix = "mqtt_client_id_"
	// mqttClientUserBindingKeyPrefix 认证成功时记录 username，供 will 钩子区分系统账号与设备。
	mqttClientUserBindingKeyPrefix = "mqtt_client_user_"
	// mqttClientBindingTTL 与设备绑定键保持同寿命。
	mqttClientBindingTTL = 48 * time.Hour
)

// OnWillPublishWrapper 让 will message 与普通 PUBLISH 经过同一主题白名单：
// 1. 主题不在发布白名单内的 will 直接丢弃；
// 2. 设备 will 的 payload 与标准上行一致包裹 device_id，避免原始 payload 直入平台链路；
// 3. 系统内部账号（root/plugin）保持原有行为；
// 4. 查不到认证绑定的 will 一律丢弃（默认拒绝）。
func (t *AetherLinkPlugin) OnWillPublishWrapper(pre server.OnWillPublish) server.OnWillPublish {
	return func(ctx context.Context, clientID string, req *server.WillMsgRequest) {
		if pre != nil {
			pre(ctx, clientID, req)
		}
		if req.Message == nil {
			return
		}

		msg := req.Message
		if !util.ValidateTopic(msg.Topic) {
			Log.Warn("mqtt will message dropped by publish topic whitelist",
				zap.String("topic", msg.Topic),
				zap.String("client_id", clientID))
			req.Message = nil
			return
		}

		if isMQTTSystemUser(lookupMQTTClientUsername(clientID)) {
			return
		}

		deviceID, ok := lookupMQTTClientDeviceID(clientID)
		if !ok {
			Log.Warn("mqtt will message dropped because authenticated binding is missing",
				zap.String("topic", msg.Topic),
				zap.String("client_id", clientID))
			req.Message = nil
			return
		}
		msg.Payload = buildMQTTUplinkPayload(deviceID, msg.Payload)
	}
}

func lookupMQTTClientUsername(clientID string) string {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return ""
	}
	username, err := GetStr(mqttClientUserBindingKeyPrefix + clientID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(username)
}

// rememberMQTTClientUsername 在认证成功路径写入 clientID→username 绑定，
// 供 will 钩子区分系统账号（root/plugin）与设备客户端；仅成功路径调用，
// 失败的认证尝试不得留下任何绑定痕迹。绑定随 TTL 过期或被会话撤销清理覆盖。
func rememberMQTTClientUsername(clientID string, username string) error {
	clientID = strings.TrimSpace(clientID)
	username = strings.TrimSpace(username)
	if clientID == "" || username == "" {
		return errors.New("mqtt client username binding requires client id and username")
	}
	return SetStr(mqttClientUserBindingKeyPrefix+clientID, username, mqttClientBindingTTL)
}

func lookupMQTTClientDeviceID(clientID string) (string, bool) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", false
	}
	deviceID, err := GetStr(mqttClientDeviceBindingKeyPrefix + clientID)
	if err != nil {
		return "", false
	}
	deviceID = strings.TrimSpace(deviceID)
	return deviceID, deviceID != ""
}

func forgetMQTTClientUsername(clientID string) {
	if strings.TrimSpace(clientID) != "" {
		_ = DelKey(mqttClientUserBindingKeyPrefix + clientID)
	}
}
