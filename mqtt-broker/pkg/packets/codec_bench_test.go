// 文件用途：控制包编解码热路径基准（PUBLISH v3/v5 编码与解码、PUBACK、主题校验）。
// 方法学：编码写入可复用的 bufio.Writer(io.Discard)，解码从预编码字节经 bufio.Reader 读回，
// 与 broker 读写协程使用 Reader/Writer 的方式一致；统计 ns/op、B/op、allocs/op。
package packets

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

var benchPayload = bytes.Repeat([]byte("x"), 256)

func benchPublish(v Version) *Publish {
	p := &Publish{
		Version:   v,
		Qos:       Qos1,
		TopicName: []byte("devices/telemetry/control/device-0001"),
		PacketID:  7,
		Payload:   benchPayload,
	}
	if v == Version5 {
		p.Properties = &Properties{
			PayloadFormat: ptrByte(1),
			MessageExpiry: ptrUint32(60),
			ContentType:   []byte("application/json"),
		}
	}
	return p
}

func ptrByte(b byte) *byte       { return &b }
func ptrUint32(u uint32) *uint32 { return &u }

func benchEncode(b *testing.B, p Packet) {
	w := NewWriter(bufio.NewWriterSize(io.Discard, 4096))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.WritePacket(p); err != nil {
			b.Fatal(err)
		}
	}
}

func benchDecode(b *testing.B, p Packet, v Version) {
	var buf bytes.Buffer
	if err := p.Pack(&buf); err != nil {
		b.Fatal(err)
	}
	raw := buf.Bytes()
	src := bytes.NewReader(raw)
	bufr := bufio.NewReaderSize(src, 4096)
	r := NewReader(bufr)
	r.SetVersion(v)
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.Reset(raw)
		bufr.Reset(src)
		if _, err := r.ReadPacket(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishEncodeV311(b *testing.B) { benchEncode(b, benchPublish(Version311)) }
func BenchmarkPublishEncodeV5(b *testing.B)   { benchEncode(b, benchPublish(Version5)) }
func BenchmarkPublishDecodeV311(b *testing.B) {
	benchDecode(b, benchPublish(Version311), Version311)
}
func BenchmarkPublishDecodeV5(b *testing.B) { benchDecode(b, benchPublish(Version5), Version5) }
func BenchmarkPubackEncodeV311(b *testing.B) {
	benchEncode(b, &Puback{Version: Version311, PacketID: 7})
}
func BenchmarkPubackDecodeV311(b *testing.B) {
	benchDecode(b, &Puback{Version: Version311, PacketID: 7}, Version311)
}

func BenchmarkValidTopicName(b *testing.B) {
	t := []byte("devices/telemetry/control/device-0001/sensors/temperature")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !ValidTopicName(true, t) {
			b.Fatal("invalid")
		}
	}
}

func BenchmarkValidUTF8ASCII(b *testing.B) {
	t := []byte("devices/telemetry/control/device-0001/sensors/temperature")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !ValidUTF8(t) {
			b.Fatal("invalid")
		}
	}
}
