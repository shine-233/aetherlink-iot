// 文件用途：维护 persistence\queue\elem_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func assertElemEqual(a *assert.Assertions, expected, actual *Elem) {
	expected.At = time.Unix(expected.At.Unix(), 0)
	expected.Expiry = time.Unix(expected.Expiry.Unix(), 0)
	actual.At = time.Unix(actual.At.Unix(), 0)
	actual.Expiry = time.Unix(actual.Expiry.Unix(), 0)
	a.Equal(expected, actual)
}

func TestElem_Encode_Publish(t *testing.T) {
	a := assert.New(t)
	e := &Elem{
		At: time.Now(),
		MessageWithID: &Publish{
			Message: &gmqtt.Message{
				Dup:                    false,
				QoS:                    2,
				Retained:               false,
				Topic:                  "/mytopic",
				Payload:                []byte("payload"),
				PacketID:               2,
				ContentType:            "type",
				CorrelationData:        nil,
				MessageExpiry:          1,
				PayloadFormat:          packets.PayloadFormatString,
				ResponseTopic:          "",
				SubscriptionIdentifier: []uint32{1, 2},
				UserProperties: []packets.UserProperty{
					{
						K: []byte("1"),
						V: []byte("2"),
					}, {
						K: []byte("3"),
						V: []byte("4"),
					},
				},
			},
		},
	}
	rs := e.Encode()
	de := &Elem{}
	err := de.Decode(rs)
	a.Nil(err)
	assertElemEqual(a, e, de)
}
func TestElem_Encode_Pubrel(t *testing.T) {
	a := assert.New(t)
	e := &Elem{
		At: time.Unix(time.Now().Unix(), 0),
		MessageWithID: &Pubrel{
			PacketID: 2,
		},
	}
	rs := e.Encode()
	de := &Elem{}
	err := de.Decode(rs)
	a.Nil(err)
	assertElemEqual(a, e, de)
}

func TestElem_Encode_PreservesSignedTimestamps(t *testing.T) {
	expected := &Elem{
		At:            time.Unix(-123, 0),
		Expiry:        time.Unix(-456, 0),
		MessageWithID: &Pubrel{PacketID: 9},
	}
	actual := &Elem{}
	if err := actual.Decode(expected.Encode()); err != nil {
		t.Fatalf("decode encoded element: %v", err)
	}
	if actual.At.Unix() != expected.At.Unix() {
		t.Fatalf("entry timestamp = %d, want %d", actual.At.Unix(), expected.At.Unix())
	}
	if actual.Expiry.Unix() != expected.Expiry.Unix() {
		t.Fatalf("expiry timestamp = %d, want %d", actual.Expiry.Unix(), expected.Expiry.Unix())
	}
}

func Benchmark_Encode_Publish(b *testing.B) {
	for i := 0; i < b.N; i++ {
		e := &Elem{
			At: time.Unix(time.Now().Unix(), 0),
			MessageWithID: &Publish{
				Message: &gmqtt.Message{
					Dup:                    false,
					QoS:                    2,
					Retained:               false,
					Topic:                  "/mytopic",
					Payload:                []byte("payload"),
					PacketID:               2,
					ContentType:            "type",
					CorrelationData:        nil,
					MessageExpiry:          1,
					PayloadFormat:          packets.PayloadFormatString,
					ResponseTopic:          "",
					SubscriptionIdentifier: []uint32{1, 2},
					UserProperties: []packets.UserProperty{
						{
							K: []byte("1"),
							V: []byte("2"),
						}, {
							K: []byte("3"),
							V: []byte("4"),
						},
					},
				},
			},
		}
		e.Encode()
	}
}
