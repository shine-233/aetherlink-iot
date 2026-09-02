// 文件用途：ObserverHub 单测——注册/注销幂等、Notify 逐观察者发送（fake sender）、
// Observe 选项递增、Token 透传、空订阅零发送。
package lwm2m

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/coap"
)

type sentMsg struct {
	key string
	raw []byte
}

func TestObserverRegisterNotifyDeregister(t *testing.T) {
	hub := NewObserverHub()
	var sent []sentMsg
	send := func(key string, data []byte) error {
		sent = append(sent, sentMsg{key, append([]byte{}, data...)})
		return nil
	}

	hub.Register("/19/0/0", "devA", []byte{1, 2})
	hub.Register("/19/0/0", "devB", []byte{3})
	hub.Register("/19/0/1", "devA", []byte{9})

	n, err := hub.Notify("/19/0/0", 1, []byte("v=1"), send)
	if err != nil || n != 2 || len(sent) != 2 {
		t.Fatalf("Notify n=%d sent=%d err=%v", n, len(sent), err)
	}
	// Observe seq=1
	for _, s := range sent {
		msg, err := coap.Decode(s.raw)
		if err != nil {
			t.Fatalf("解码通知失败: %v", err)
		}
		if msg.Code != coap.CodeContent || msg.Type != coap.TypeNonConfirm {
			t.Fatalf("通知应为 NON 2.05: %+v", msg)
		}
		obs := msg.OptionsByNumber(coap.OptionObserve)
		if len(obs) != 1 || obs[0][2] != 1 {
			t.Fatalf("observe 选项 seq 不符: %x", obs)
		}
		if string(msg.Payload) != "v=1" {
			t.Fatalf("载荷=%q", msg.Payload)
		}
	}
	// Token 透传校验
	if !strings.Contains(string(sent[0].raw), string([]byte{1, 2})) {
		// token 在消息头部，直接解码核对
		m0, _ := coap.Decode(sent[0].raw)
		if string(m0.Token) != string([]byte{1, 2}) && string(m0.Token) != string([]byte{3}) {
			t.Fatalf("token 未透传: %x", m0.Token)
		}
	}

	// 注销 devA 后只剩 devB
	hub.Deregister("/19/0/0", "devA")
	sent = nil
	n, _ = hub.Notify("/19/0/0", 2, []byte("v=2"), send)
	if n != 1 || len(sent) != 1 {
		t.Fatalf("注销后应只发 1: n=%d sent=%d", n, len(sent))
	}
	hub.Deregister("/19/0/0", "devB") // 幂等
	hub.Deregister("/19/0/0", "ghost")
	if hub.Count() != 1 { // 还有 /19/0/1 的 devA
		t.Fatalf("Count=%d", hub.Count())
	}
	sent = nil
	n, _ = hub.Notify("/19/0/0", 3, []byte("v=3"), send)
	if n != 0 || len(sent) != 0 {
		t.Fatalf("空订阅应零发送: n=%d sent=%d", n, len(sent))
	}
}

func TestObserverSeqIncrements(t *testing.T) {
	hub := NewObserverHub()
	hub.Register("/p", "k", []byte{1})
	var last []byte
	send := func(_ string, data []byte) error { last = data; return nil }
	for seq := 1; seq <= 3; seq++ {
		if _, err := hub.Notify("/p", seq, []byte("x"), send); err != nil {
			t.Fatal(err)
		}
		msg, _ := coap.Decode(last)
		obs := msg.OptionsByNumber(coap.OptionObserve)
		if len(obs) == 0 || obs[0][2] != byte(seq) {
			t.Fatalf("seq=%d 不符: %x", seq, obs)
		}
	}
}
