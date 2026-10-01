// 文件用途：TCP 上行摄取（TP-03）——注册帧解析 + 遥测帧汇入 uplink 总线。
// 核心逻辑：连接首帧即注册帧（纯文本 device_number 或 {"device_number": "..."} JSON），
// 解析后经 DeviceResolver 做凭证映射（复用 protocolgw.DBNumberResolver，is_enabled 设备才放行，
// 租户取自 DB 不信任客户端）；后续帧为遥测，必须是 JSON 对象，原样透传进 UplinkMessage。
// 关键注意事项：fail-closed——未知/禁用设备连接直接拒绝，非对象遥测丢弃并计数，绝不阻塞读循环；
// 设备号白名单字符集防控制字符/换行注入（line 模式下设备号会进入帧流）。
// 重构建议：注册帧如需携带鉴权凭证（token/PSK），扩展 ParseRegistrationFrame 的信封结构即可，
// 会话注册与遥测链路无需改动。
package tcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/protocolgw"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// SourceProtocolTCP uplink 元数据里的协议标记（与 mqtt/coap/plugin 同口径的小写协议名）。
const SourceProtocolTCP = "tcp"

// maxDeviceNumberLength 设备号长度上限（注册帧防滥用）。
const maxDeviceNumberLength = 128

// RegistrationWindow 注册窗口：连接建立后必须在该时限内发出合法注册帧，否则关闭。
const RegistrationWindow = 30 * time.Second

// DefaultIdleTimeoutSeconds 注册后读空闲超时默认值（秒；0 表示不启用空闲踢除）。
const DefaultIdleTimeoutSeconds = 300

// DefaultCommandBufferLimit 单设备下行命令环形缓冲默认上限（条，同 edgeforward 丢最旧语义）。
const DefaultCommandBufferLimit = 1000

// DefaultTCPPort TCP 网关默认端口（避开平台已占端口：9999/1883/8080/8082/5683/18881）。
const DefaultTCPPort = 9877

// ParseRegistrationFrame 解析注册帧 payload：纯文本设备号或 JSON 信封
// {"device_number":"..."}（JSON 便于未来携带凭证/版本而不破坏纯文本设备）。
// 返回错误表示注册帧非法（连接必须关闭，fail-closed）。
func ParseRegistrationFrame(payload []byte) (string, error) {
	raw := strings.TrimSpace(string(payload))
	if raw == "" {
		return "", fmt.Errorf("tcp: 注册帧为空")
	}
	if len(raw) > maxDeviceNumberLength {
		return "", fmt.Errorf("tcp: 注册帧超过 %d 字节上限", maxDeviceNumberLength)
	}
	number := raw
	if raw[0] == '{' {
		var envelope struct {
			DeviceNumber string `json:"device_number"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			return "", fmt.Errorf("tcp: 注册帧 JSON 非法: %w", err)
		}
		number = strings.TrimSpace(envelope.DeviceNumber)
		if number == "" {
			return "", fmt.Errorf("tcp: 注册帧缺少 device_number")
		}
	}
	if len(number) > maxDeviceNumberLength {
		return "", fmt.Errorf("tcp: 设备号超过 %d 字节上限", maxDeviceNumberLength)
	}
	if !validDeviceNumber(number) {
		return "", fmt.Errorf("tcp: 设备号含非法字符（仅允许字母数字与 . _ : @ -）")
	}
	return number, nil
}

// validDeviceNumber 设备号字符白名单：字母/数字/. _ : @ -（防换行、控制字符与空白注入）。
func validDeviceNumber(number string) bool {
	for i := 0; i < len(number); i++ {
		c := number[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == ':' || c == '@' || c == '-':
		default:
			return false
		}
	}
	return true
}

// Ingestor 遥测摄取器：注册后的遥测帧 → UplinkMessage → uplink 总线。
// 会话身份在注册时解析并固写（session.identity），本结构只负责帧到消息的转换与计数。
type Ingestor struct {
	publisher protocolgw.UplinkPublisher
	log       *logrus.Logger

	published atomic.Uint64
	dropped   atomic.Uint64 // 非法遥测帧（非 JSON 对象/空对象/发布失败）
}

// NewIngestor 构造摄取器；publisher 必填。
func NewIngestor(publisher protocolgw.UplinkPublisher, log *logrus.Logger) *Ingestor {
	if log == nil {
		log = logrus.New()
	}
	return &Ingestor{publisher: publisher, log: log}
}

// ingest 把一条遥测帧汇入总线；payload 必须是非空 JSON 对象（键值集），原样透传。
// identity 来自会话（注册时解析固写），租户归属不信任帧内容。
func (g *Ingestor) ingest(number string, identity *protocolgw.DeviceIdentity, payload []byte) {
	if len(payload) == 0 || identity == nil {
		g.dropped.Add(1)
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		g.dropped.Add(1)
		g.log.WithField("device_number", utils.SanitizeForLog(number)).Warn("tcp 遥测丢弃：payload 不是 JSON 对象")
		return
	}
	if len(obj) == 0 {
		g.dropped.Add(1)
		g.log.WithField("device_number", utils.SanitizeForLog(number)).Warn("tcp 遥测丢弃：空对象")
		return
	}
	msg := &mqttadapter.UplinkMessage{
		Type:      "telemetry",
		DeviceID:  identity.DeviceID,
		TenantID:  identity.TenantID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
		Metadata: map[string]interface{}{
			"device_id":       identity.DeviceID,
			"device_number":   number,
			"source_protocol": SourceProtocolTCP,
		},
	}
	if err := g.publisher.Publish(msg); err != nil {
		g.dropped.Add(1)
		g.log.WithFields(logrus.Fields{"device_id": identity.DeviceID, "error": err}).
			Error("tcp 遥测发布失败")
		return
	}
	g.published.Add(1)
}
