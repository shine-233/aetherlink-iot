// 文件用途：MQTT 适配器的 Sparkplug B 接入契约测试（ROADMAP TB-10）。
//
// 核心逻辑：只覆盖**在任何外部依赖之前就该拒绝**的路径——话题非法、载荷畸形、
// 设备编号解析失败。这三条都不碰 bus 与数据库，因此可以在无基础设施的情况下真实执行，
// 而不是"跳过并声称已验证"。
//
// 关键注意事项：**刻意不测"成功投递"**——那需要真实 MQTT broker、Redis 与 PostgreSQL。
// 把不可执行的成功路径写成 skip，等于用绿灯掩盖"从未跑过"。
// 成功路径的证据应在活栈联调时补（见 docs/validation/2026-09-16-tb10-sparkplug-decoder-evidence.md）。
package mqttadapter

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// newSparkplugTestAdapter 构造一个只用于**拒绝路径**的适配器：bus 与 mqtt 客户端均为 nil。
// 只要被测路径在触达它们之前就返回，这个适配器就是有效的。
func newSparkplugTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	return NewAdapter(nil, nil, logger)
}

func TestHandleSparkplugMessageRejectsInvalidTopic(t *testing.T) {
	adapter := newSparkplugTestAdapter(t)

	cases := []string{
		"",
		"devices/telemetry",         // 不是 Sparkplug 命名空间
		"spBv2.0/g/NDATA/e",         // 命名空间错误
		"spBv1.0/g/NFOO/e",          // 未知消息类型
		"spBv1.0/g/NDATA",           // 段数过少
		"spBv1.0/g/NDATA/e/d/extra", // 段数过多
		"spBv1.0//NDATA/e",          // 空段
		"spBv1.0/g/ndata/e",         // 小写不做归一
	}
	for _, topic := range cases {
		err := adapter.HandleSparkplugMessage([]byte{0x08, 0x01}, topic)
		if err == nil {
			t.Fatalf("话题 %q 必须在触达 bus 之前被拒绝", topic)
		}
	}
}

func TestHandleSparkplugMessageRejectsMalformedPayloadBeforeBus(t *testing.T) {
	adapter := newSparkplugTestAdapter(t)

	// 话题合法（DDATA），载荷畸形 → 必须在解析设备之前失败。
	err := adapter.HandleSparkplugMessage([]byte{0x12, 0x7F, 0x0A}, "spBv1.0/g/DDATA/edge-1/plc-7")
	if err == nil {
		t.Fatal("畸形载荷必须被拒绝")
	}
	if !strings.Contains(err.Error(), "malformed") && !strings.Contains(err.Error(), "sparkplug") {
		t.Fatalf("错误信息应指向载荷问题，实际：%v", err)
	}
}

func TestHandleSparkplugMessageIgnoresSessionMessagesWithoutError(t *testing.T) {
	adapter := newSparkplugTestAdapter(t)

	// BIRTH / DEATH / CMD / STATE 本版不处理，但**不能报错**：
	// 设备在 birth 之后持续发 data 是常态，把 birth 当失败会刷满错误日志。
	topics := []string{
		"spBv1.0/g/NBIRTH/edge-1",
		"spBv1.0/g/DBIRTH/edge-1/plc-7",
		"spBv1.0/g/NDEATH/edge-1",
		"spBv1.0/g/DDEATH/edge-1/plc-7",
		"spBv1.0/g/NCMD/edge-1",
		"spBv1.0/g/DCMD/edge-1/plc-7",
		"spBv1.0/g/STATE/edge-1",
	}
	for _, topic := range topics {
		// 载荷刻意给畸形值：会话类消息应在解码之前就被放行忽略。
		if err := adapter.HandleSparkplugMessage([]byte{0xFF}, topic); err != nil {
			t.Fatalf("话题 %q 属会话类消息，应被忽略而非报错，实际：%v", topic, err)
		}
	}
}

func TestHandleSparkplugMessageFailsClosedWhenDeviceLookupFails(t *testing.T) {
	adapter := newSparkplugTestAdapter(t)

	// 话题与载荷都合法，但 DAL 未初始化 → 设备解析必然失败。
	// 该路径必须在触达 bus 之前返回错误，而不是把无主消息投进上行链路。
	payload := []byte{0x08, 0x01, 0x12, 0x07, 0x0A, 0x01, 0x61, 0x20, 0x03, 0x50, 0x07}
	err := adapter.HandleSparkplugMessage(payload, "spBv1.0/g/DDATA/edge-1/plc-7")
	if err == nil {
		t.Fatal("设备解析失败时必须返回错误，绝不能把无主消息投进上行链路")
	}
}

func TestSparkplugTopicPatternCoversBothNodeAndDeviceLevel(t *testing.T) {
	// `spBv1.0/+/+/+/#` 必须同时覆盖节点级（4 段）与设备级（5 段）话题。
	// `#` 在 MQTT 中可匹配零层，这正是选择它而不是 `+` 的原因——
	// 写成 `spBv1.0/+/+/+/+` 会漏掉全部节点级消息。
	if TopicPatternSparkplug != "spBv1.0/+/+/+/#" {
		t.Fatalf("话题模式被改动：%q。改动前请确认仍同时覆盖 4 段与 5 段结构", TopicPatternSparkplug)
	}
	if !strings.HasSuffix(TopicPatternSparkplug, "/#") {
		t.Fatal("必须以 /# 结尾，否则节点级（4 段）消息会全部漏订")
	}
}
