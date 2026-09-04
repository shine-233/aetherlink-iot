// 文件用途：LwM2M/CoAP Observe 推送中枢（RFC 7641 服务器侧，配合 coap 包）。
// 核心逻辑：按资源路径维护观察者（远端地址+token+Notify seq），资源变化时由上层调用
//   Notify 构造带 Observe 选项递增序号与载荷的 NON 消息，经注入的 send 回调逐观察者发送。
// 关键注意事项：
//   - send 由调用方注入（真实场景为 UDP PacketConn.WriteTo），本包保持可测的纯逻辑；
//   - 同一 (path, 观察者 key) 只保留一份；Deregister 幂等；
//   - seq 单调递增（2^24 内回绕由外层策略处理）。
package lwm2m

import (
	"sync"

	"aetherlink-iot/backend/internal/coap"
)

// Observer 一个观察者。
type Observer struct {
	Key   string // 观察者唯一键（如注册客户端 endpoint）
	Token []byte
}

// ObserverHub 路径级观察者表。
type ObserverHub struct {
	mu   sync.Mutex
	subs map[string]map[string]Observer // path → key → Observer
}

// NewObserverHub 新建观察者中枢。
func NewObserverHub() *ObserverHub {
	return &ObserverHub{subs: map[string]map[string]Observer{}}
}

// Register 注册观察者；重复 key 覆盖并重置其 token。
func (h *ObserverHub) Register(path, key string, token []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[path] == nil {
		h.subs[path] = map[string]Observer{}
	}
	h.subs[path][key] = Observer{Key: key, Token: append([]byte{}, token...)}
}

// Deregister 注销观察者（幂等）。
func (h *ObserverHub) Deregister(path, key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.subs[path]; ok {
		delete(m, key)
		if len(m) == 0 {
			delete(h.subs, path)
		}
	}
}

// Count 观察者总数（统计用）。
func (h *ObserverHub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, m := range h.subs {
		n += len(m)
	}
	return n
}

// Notify 向 path 的所有观察者发送递增 observe 通知。send(addr,data) 由上层注入；
// 没有观察者时返回 0 且不调用 send。
func (h *ObserverHub) Notify(path string, seq int, payload []byte,
	send func(key string, data []byte) error) (int, error) {
	h.mu.Lock()
	obs := make([]Observer, 0, 4)
	if m, ok := h.subs[path]; ok {
		for _, o := range m {
			obs = append(obs, o)
		}
	}
	h.mu.Unlock()
	if len(obs) == 0 {
		return 0, nil
	}
	// Observe 选项 = seq 的 24bit 大端值（RFC 7641）。
	observeBytes := []byte{byte(seq >> 16), byte(seq >> 8), byte(seq)}
	msg := &coap.Message{
		Type:    coap.TypeNonConfirm,
		Code:    coap.CodeContent,
		Options: []coap.Option{{Number: coap.OptionObserve, Value: observeBytes}},
		Payload: payload,
	}
	if _, err := msg.Encode(); err != nil {
		return 0, err
	}
	sent := 0
	for _, o := range obs {
		m := *msg
		m.Token = o.Token
		encoded, eerr := m.Encode()
		if eerr != nil {
			continue
		}
		if serr := send(o.Key, encoded); serr == nil {
			sent++
		}
	}
	return sent, nil
}
