package queue

import (
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func benchElem() *Elem {
	return &Elem{
		At:     time.Unix(1_700_000_000, 0),
		Expiry: time.Unix(1_700_003_600, 0),
		MessageWithID: &Publish{Message: &gmqtt.Message{
			QoS:           1,
			Topic:         "devices/telemetry/abc123/up",
			Payload:       []byte(`{"temperature":23.5,"humidity":61}`),
			PacketID:      7,
			PayloadFormat: packets.PayloadFormatString,
		}},
	}
}

// BenchmarkElemEncodePublish 是 redis 队列每次入队都会走的编码路径。
func BenchmarkElemEncodePublish(b *testing.B) {
	e := benchElem()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = e.Encode()
	}
}

// BenchmarkElemDecodePublish 是 redis 队列每次读取/重放都会走的解码路径。
func BenchmarkElemDecodePublish(b *testing.B) {
	data := benchElem().Encode()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var e Elem
		if err := e.Decode(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkElemEncodePubrel(b *testing.B) {
	e := &Elem{At: time.Unix(1_700_000_000, 0), MessageWithID: &Pubrel{PacketID: 9}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = e.Encode()
	}
}
