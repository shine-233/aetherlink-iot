package encoding

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

// benchMessage 是持久化队列/保留消息中典型的 MQTT v5 消息：带 topic、payload、
// 订阅标识与用户属性，覆盖编码器的全部分支。
func benchMessage() *gmqtt.Message {
	return &gmqtt.Message{
		QoS:                    1,
		Topic:                  "devices/telemetry/abc123/up",
		Payload:                []byte(`{"temperature":23.5,"humidity":61}`),
		PacketID:               42,
		ContentType:            "application/json",
		CorrelationData:        []byte("corr-1"),
		MessageExpiry:          3600,
		PayloadFormat:          packets.PayloadFormatString,
		ResponseTopic:          "devices/telemetry/abc123/resp",
		SubscriptionIdentifier: []uint32{1, 300},
		UserProperties: []packets.UserProperty{
			{K: []byte("k1"), V: []byte("v1")},
			{K: []byte("k2"), V: []byte("v2")},
		},
	}
}

func TestEncodeDecodeMessageRoundTrip(t *testing.T) {
	want := benchMessage()
	var buf bytes.Buffer
	EncodeMessage(want, &buf)
	got, err := DecodeMessageFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestReadStringShortInput(t *testing.T) {
	if _, err := ReadString(bytes.NewBuffer([]byte{0})); err == nil {
		t.Fatal("expected error on truncated length prefix")
	}
	if _, err := ReadString(bytes.NewBuffer([]byte{0, 5, 'a'})); err == nil {
		t.Fatal("expected error on truncated body")
	}
}

func BenchmarkEncodeMessage(b *testing.B) {
	msg := benchMessage()
	var buf bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		EncodeMessage(msg, &buf)
	}
}

func BenchmarkDecodeMessage(b *testing.B) {
	var buf bytes.Buffer
	EncodeMessage(benchMessage(), &buf)
	data := buf.Bytes()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeMessageFromBytes(data); err != nil {
			b.Fatal(err)
		}
	}
}
