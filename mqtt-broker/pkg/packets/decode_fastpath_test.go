// 文件用途：读侧合并分配快路径与通用 NewXxxPacket 解码的等价性测试。
package packets

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

// decodeBoth 用快路径（Reader.ReadPacket）与通用路径（NewPacket）各解码一次。
func decodeBoth(t *testing.T, raw []byte, v Version) (fast, slow Packet, fastErr, slowErr error) {
	t.Helper()
	r := NewReader(bufio.NewReader(bytes.NewReader(raw)))
	r.SetVersion(v)
	fast, fastErr = r.ReadPacket()

	br := bufio.NewReader(bytes.NewReader(raw))
	first, _ := br.ReadByte()
	length, err := EncodeRemainLength(br)
	if err != nil {
		t.Fatal(err)
	}
	slow, slowErr = NewPacket(&FixHeader{PacketType: first >> 4, Flags: first & 15, RemainLength: length}, v, br)
	return
}

func TestReadPacketFastPathEquivalence(t *testing.T) {
	cases := []struct {
		name string
		p    Packet
		v    Version
	}{
		{"publish-v311-qos0", &Publish{Version: Version311, TopicName: []byte("a/b"), Payload: []byte("x")}, Version311},
		{"publish-v311-qos1-retain", &Publish{Version: Version311, Qos: Qos1, Retain: true, PacketID: 9, TopicName: []byte("a/b"), Payload: []byte("xyz")}, Version311},
		{"publish-v311-qos2-dup", &Publish{Version: Version311, Qos: Qos2, Dup: true, PacketID: 10, TopicName: []byte("t")}, Version311},
		{"publish-v5-props", benchPublish(Version5), Version5},
		{"publish-v5-noprops", &Publish{Version: Version5, Qos: Qos1, PacketID: 3, TopicName: []byte("a"), Payload: []byte("p"), Properties: &Properties{}}, Version5},
		{"puback-v311", &Puback{Version: Version311, PacketID: 7}, Version311},
		{"puback-v5-success", &Puback{Version: Version5, PacketID: 7}, Version5},
		{"puback-v5-code", &Puback{Version: Version5, PacketID: 7, Code: codes.NotAuthorized, Properties: &Properties{}}, Version5},
		{"pubrec-v311", &Pubrec{Version: Version311, PacketID: 8}, Version311},
		{"pubrel-v311", &Pubrel{PacketID: 11}, Version311},
		{"pubcomp-v5", &Pubcomp{Version: Version5, PacketID: 12}, Version5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.p.Pack(&buf); err != nil {
				t.Fatal(err)
			}
			fast, slow, fe, se := decodeBoth(t, buf.Bytes(), tc.v)
			if fe != nil || se != nil {
				t.Fatalf("errors: fast=%v slow=%v", fe, se)
			}
			if !reflect.DeepEqual(fast, slow) {
				t.Fatalf("mismatch:\nfast=%#v\nslow=%#v", fast, slow)
			}
		})
	}
}

func TestReadPacketFastPathMalformed(t *testing.T) {
	cases := map[string][]byte{
		"puback-truncated":     {0x40, 0x02, 0x00},
		"publish-qos0-dup":     {0x38, 0x03, 0x00, 0x01, 'a'},
		"publish-qos3":         {0x36, 0x05, 0x00, 0x01, 'a', 0x00, 0x01},
		"publish-truncated":    {0x30, 0x05, 0x00, 0x01},
		"publish-wildcard":     {0x30, 0x03, 0x00, 0x01, '+'},
		"pubrel-empty-payload": {0x62, 0x02},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, fe, se := decodeBoth(t, raw, Version311)
			if fe == nil || se == nil || fe.Error() != se.Error() {
				t.Fatalf("want identical errors, fast=%v slow=%v", fe, se)
			}
		})
	}
}
