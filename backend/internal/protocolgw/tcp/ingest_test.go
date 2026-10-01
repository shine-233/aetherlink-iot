// 文件用途：注册帧解析与遥测摄取单测（TP-03）——首帧 device_number 映射与 fail-closed 丢弃。
// 核心逻辑：ParseRegistrationFrame 表驱动钉死纯文本/JSON 信封/非法输入全边界；
// Ingestor 用假 resolver+publisher 验证 UplinkMessage 字段与租户归属不信任帧内容。
// 关键注意事项：发布失败也必须计入 dropped（调用方据此观测总线上游故障）。
// 重构建议：注册信封扩展鉴权字段时，把"凭证校验"分支加入此处表驱动。
package tcp

import (
	"errors"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/protocolgw"

	"github.com/sirupsen/logrus"
)

// fakeResolver 设备号映射假实现（未知/禁用即报错，fail-closed 语义与 DBNumberResolver 一致）。
type fakeResolver struct {
	byNumber map[string]*protocolgw.DeviceIdentity
}

func (f *fakeResolver) ResolveByNumber(number string) (*protocolgw.DeviceIdentity, error) {
	if id, ok := f.byNumber[number]; ok {
		return id, nil
	}
	return nil, errors.New("no such enabled device")
}

// fakePublisher 捕获发布的 UplinkMessage。
type fakePublisher struct {
	msgs []*mqttadapter.UplinkMessage
	err  error
}

func (f *fakePublisher) Publish(msg *mqttadapter.UplinkMessage) error {
	if f.err != nil {
		return f.err
	}
	f.msgs = append(f.msgs, msg)
	return nil
}

func TestParseRegistrationFramePlainText(t *testing.T) {
	for input, want := range map[string]string{
		"dev-01":       "dev-01",
		"  dev-02 \n":  "dev-02", // 首尾空白容忍（帧内容裁剪）
		"GW_1:a.b@x-y": "GW_1:a.b@x-y",
	} {
		got, err := ParseRegistrationFrame([]byte(input))
		if err != nil || got != want {
			t.Fatalf("ParseRegistrationFrame(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestParseRegistrationFrameJSONEnvelope(t *testing.T) {
	got, err := ParseRegistrationFrame([]byte(`{"device_number":"dev-9"}`))
	if err != nil || got != "dev-9" {
		t.Fatalf("got %q, %v; want dev-9", got, err)
	}
}

func TestParseRegistrationFrameInvalid(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"empty", "   "},
		{"too-long", strings.Repeat("a", maxDeviceNumberLength+1)},
		{"json-no-field", `{"other":"x"}`},
		{"json-bad", `{"device_number":`},
		{"invalid-chars", "dev 01"},    // 空白注入
		{"control-chars", "dev\x0101"}, // 控制字符
		{"newline-inside", "dev\n01"},  // line 模式帧边界注入
		{"json-inner-invalid", `{"device_number":"de v"}`},
	}
	for _, c := range cases {
		if got, err := ParseRegistrationFrame([]byte(c.payload)); err == nil {
			t.Fatalf("%s: expected error, got %q", c.name, got)
		}
	}
}

func TestIngestorPublishesTelemetry(t *testing.T) {
	pub := &fakePublisher{}
	ing := NewIngestor(pub, logrus.New())
	identity := &protocolgw.DeviceIdentity{DeviceID: "uuid-1", TenantID: "tenant-1", DeviceNumber: "dev-1"}

	ing.ingest("dev-1", identity, []byte(`{"temperature":25.5}`))

	if len(pub.msgs) != 1 {
		t.Fatalf("published %d messages, want 1", len(pub.msgs))
	}
	msg := pub.msgs[0]
	if msg.Type != "telemetry" || msg.DeviceID != "uuid-1" || msg.TenantID != "tenant-1" {
		t.Fatalf("message header mismatch: %+v", msg)
	}
	if string(msg.Payload) != `{"temperature":25.5}` {
		t.Fatalf("payload = %s, want passthrough", msg.Payload)
	}
	if msg.Metadata["source_protocol"] != SourceProtocolTCP || msg.Metadata["device_number"] != "dev-1" {
		t.Fatalf("metadata = %v", msg.Metadata)
	}
	if ing.published.Load() != 1 || ing.dropped.Load() != 0 {
		t.Fatalf("counters published=%d dropped=%d", ing.published.Load(), ing.dropped.Load())
	}
}

func TestIngestorFailClosed(t *testing.T) {
	cases := []struct {
		name     string
		payload  []byte
		identity *protocolgw.DeviceIdentity
		pubErr   bool
	}{
		{"empty payload", []byte{}, &protocolgw.DeviceIdentity{}, false},
		{"nil identity", []byte(`{"a":1}`), nil, false},
		{"not an object", []byte(`[1,2]`), &protocolgw.DeviceIdentity{}, false},
		{"scalar", []byte(`"x"`), &protocolgw.DeviceIdentity{}, false},
		{"broken json", []byte(`{"a":`), &protocolgw.DeviceIdentity{}, false},
		{"empty object", []byte(`{}`), &protocolgw.DeviceIdentity{}, false},
		{"publish error", []byte(`{"a":1}`), &protocolgw.DeviceIdentity{}, true},
	}
	for _, c := range cases {
		pub := &fakePublisher{}
		if c.pubErr {
			pub.err = errors.New("bus down")
		}
		ing := NewIngestor(pub, logrus.New())
		ing.ingest("dev-1", c.identity, c.payload)
		if ing.published.Load() != 0 || ing.dropped.Load() != 1 {
			t.Fatalf("%s: published=%d dropped=%d, want 0/1", c.name, ing.published.Load(), ing.dropped.Load())
		}
		if len(pub.msgs) != 0 {
			t.Fatalf("%s: messages should not be published", c.name)
		}
	}
}
