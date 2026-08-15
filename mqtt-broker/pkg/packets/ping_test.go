// 文件用途: 覆盖 MQTT Ping 控制包的编解码行为。
// 核心逻辑: 校验仅含固定头的 PINGREQ 和 PINGRESP 样例。
// 关键注意事项: 为保障 keepalive 安全，应保留畸形剩余长度用例。
// 重构建议: 将空负载控制包的通用样例 helper 与其他简单控制包合并。
package packets

import (
	"bytes"
	"reflect"
	"testing"
)

func TestReadPingreq(t *testing.T) {
	b := []byte{0xc0, 0}
	buf := bytes.NewBuffer(b)
	packet, err := NewReader(buf).ReadPacket()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if _, ok := packet.(*Pingreq); !ok {
		t.Fatalf("Packet Type error,want %v,got %v", reflect.TypeOf(&Pingreq{}), reflect.TypeOf(packet))
	}
}

func TestWritePingreq(t *testing.T) {
	req := &Pingreq{}
	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	err := NewWriter(buf).WriteAndFlush(req)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	want := []byte{0xc0, 0}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("write error,want %v, got %v", want, buf.Bytes())
	}
}

func TestReadPingresp(t *testing.T) {
	b := []byte{0xd0, 0}
	buf := bytes.NewBuffer(b)
	packet, err := NewReader(buf).ReadPacket()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if _, ok := packet.(*Pingresp); !ok {
		t.Fatalf("Packet Type error,want %v,got %v", reflect.TypeOf(&Pingresp{}), reflect.TypeOf(packet))
	}
}

func TestWritePingresp(t *testing.T) {

	resp := &Pingresp{}
	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	err := NewWriter(buf).WriteAndFlush(resp)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	want := []byte{0xd0, 0}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("write error,want %v, got %v", want, buf.Bytes())
	}
}
