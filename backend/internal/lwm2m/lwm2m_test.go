// 文件用途：LwM2M 注册层单测——参数解析校验、注册/更新语义、注销、
// 以及经 coap.Registry 端到端 POST/DELETE /rd。
package lwm2m

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/coap"
)

func uriPathOptions(path string) []coap.Option {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	out := make([]coap.Option, 0, len(segs))
	for _, s := range segs {
		out = append(out, coap.Option{Number: coap.OptionUriPath, Value: []byte(s)})
	}
	return out
}

func registerReq(path string, query []string) *coap.Message {
	opts := uriPathOptions(path)
	for _, q := range query {
		opts = append(opts, coap.Option{Number: coap.OptionUriQuery, Value: []byte(q)})
	}
	return &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePost, MessageID: 1, Options: opts}
}

func TestParseRegisterParams(t *testing.T) {
	req := registerReq("/rd", []string{"ep=devA", "lt=300", "b=U"})
	ep, lt, b, err := parseRegisterParams(req)
	if err != nil || ep != "devA" || lt != 300*time.Second || b != "U" {
		t.Fatalf("解析错误: ep=%q lt=%v b=%q err=%v", ep, lt, b, err)
	}
	if _, _, _, err := parseRegisterParams(registerReq("/rd", []string{"b=U"})); err == nil {
		t.Fatal("缺 ep 应报错")
	}
	if _, _, _, err := parseRegisterParams(registerReq("/rd", []string{"ep=x", "lt=abc"})); err == nil {
		t.Fatal("非法 lt 应报错")
	}
}

func TestRegisterUpdateAndDelete(t *testing.T) {
	base := time.Now()
	r := NewRegistry()
	r.now = func() time.Time { return base }

	id, created := r.Register("devA", 300*time.Second, "U", "127.0.0.1:5683")
	if !created || id == "" {
		t.Fatalf("首次注册应新建: id=%q created=%v", id, created)
	}
	if r.Count() != 1 {
		t.Fatalf("注册数=%d", r.Count())
	}
	// 同 endpoint 再注册 = 更新而非新建
	id2, created2 := r.Register("devA", 600*time.Second, "U", "127.0.0.1:5683")
	if created2 || id2 != id {
		t.Fatalf("重复注册应更新: id2=%q created2=%v", id2, created2)
	}
	if snap := r.Snapshot(); snap[0].Lifetime != 600*time.Second {
		t.Fatalf("lifetime 未更新: %v", snap[0].Lifetime)
	}
	// DELETE 不存在返回 false
	if r.Delete("nope") {
		t.Fatal("删除不存在应 false")
	}
}

func TestHandleRegisterEndToEndViaCoAP(t *testing.T) {
	reg := NewRegistry()
	reg.now = func() time.Time { return time.Now() }
	cr := coap.NewRegistry()
	cr.Register("/rd*", reg.HandleRegister())

	// POST /rd → 2.01 Created
	req := registerReq("/rd", []string{"ep=devB", "lt=60", "b=U"})
	resp, err := cr.Serve(req)
	if err != nil || resp == nil {
		t.Fatalf("serve: %v", err)
	}
	if resp.Code != coap.CodeCreated {
		t.Fatalf("注册应 2.01, got %v", resp.Code)
	}
	if !strings.Contains(string(resp.Payload), "id=") {
		t.Fatalf("响应应含分配的 id: %q", resp.Payload)
	}
	if reg.Count() != 1 {
		t.Fatalf("注册簿应含 1 客户端: %d", reg.Count())
	}

	// 重复注册 → 2.04 Changed
	resp2, _ := cr.Serve(registerReq("/rd", []string{"ep=devB", "lt=60", "b=U"}))
	if resp2.Code != coap.CodeChanged {
		t.Fatalf("重复注册应 2.04, got %v", resp2.Code)
	}

	// 缺 ep → 4.00
	resp3, _ := cr.Serve(registerReq("/rd", []string{"b=U"}))
	if resp3.Code != coap.CodeBadRequest {
		t.Fatalf("缺 ep 应 4.00, got %v", resp3.Code)
	}

	// DELETE /rd/{id} → 2.02 Deleted
	id := strings.TrimPrefix(string(resp.Payload), "id=")
	del := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeDelete, MessageID: 5, Options: uriPathOptions("/rd/" + id)}
	respDel, _ := cr.Serve(del)
	if respDel.Code != coap.CodeDeleted {
		t.Fatalf("注销应 2.02, got %v", respDel.Code)
	}
	if reg.Count() != 0 {
		t.Fatalf("注销后注册簿应空: %d", reg.Count())
	}
}

// --- TB-22：注册/去注册事件回调（多客户端隔离接线） ---

func TestHandleRegisterWithEventsRegisterAndDeregister(t *testing.T) {
	reg := NewRegistry()
	cr := coap.NewRegistry()
	var events []RegistryEvent
	cr.Register("/rd*", reg.HandleRegisterWithEvents(func(ev RegistryEvent) {
		events = append(events, ev)
	}))

	// POST /rd 携带源地址 → 注册事件含端点、注册 ID 与 UDP 源地址。
	req := registerReq("/rd", []string{"ep=devEV", "lt=60"})
	req.RemoteAddr = "10.0.0.9:5683"
	resp, err := cr.Serve(req)
	if err != nil || resp.Code != coap.CodeCreated {
		t.Fatalf("注册应 2.01, got %v err=%v", resp.Code, err)
	}
	if len(events) != 1 {
		t.Fatalf("注册事件数=%d", len(events))
	}
	ev := events[0]
	id := strings.TrimPrefix(string(resp.Payload), "id=")
	if ev.Kind != EventRegister || ev.Endpoint != "devEV" || ev.ID != id || ev.Addr != "10.0.0.9:5683" {
		t.Fatalf("注册事件不符: %+v (期望 id=%s)", ev, id)
	}

	// 刷新注册（同端点）→ 再次注册事件。
	if _, _ = cr.Serve(registerReq("/rd", []string{"ep=devEV", "lt=60"})); len(events) != 2 {
		t.Fatalf("刷新注册应再次通知, 事件数=%d", len(events))
	}

	// DELETE /rd/{id} → 去注册事件在注销前取端点。
	del := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeDelete, MessageID: 6,
		Options: uriPathOptions("/rd/" + id), RemoteAddr: "10.0.0.9:5683"}
	if respDel, _ := cr.Serve(del); respDel.Code != coap.CodeDeleted {
		t.Fatalf("注销应 2.02, got %v", respDel.Code)
	}
	if len(events) != 3 {
		t.Fatalf("去注册事件缺失, 事件数=%d", len(events))
	}
	last := events[2]
	if last.Kind != EventDeregister || last.Endpoint != "devEV" || last.ID != id {
		t.Fatalf("去注册事件不符: %+v", last)
	}

	// 注销不存在的 ID → 4.04，无事件。
	miss := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeDelete, MessageID: 7,
		Options: uriPathOptions("/rd/999"), RemoteAddr: "10.0.0.9:5683"}
	if respMiss, _ := cr.Serve(miss); respMiss.Code != coap.CodeNotFound {
		t.Fatalf("注销不存在应 4.04, got %v", respMiss.Code)
	}
	if len(events) != 3 {
		t.Fatalf("4.04 不得产生事件, 事件数=%d", len(events))
	}
}

func TestHandleRegisterRecordsRemoteAddr(t *testing.T) {
	reg := NewRegistry()
	cr := coap.NewRegistry()
	cr.Register("/rd*", reg.HandleRegister())
	req := registerReq("/rd", []string{"ep=devAddr"})
	req.RemoteAddr = "192.168.1.5:40000"
	if _, _ = cr.Serve(req); reg.Count() != 1 {
		t.Fatal("注册应成功")
	}
	snap := reg.Snapshot()
	if snap[0].Address != "192.168.1.5:40000" {
		t.Fatalf("注册簿应记录 UDP 源地址, got %q", snap[0].Address)
	}
}

func TestEndpointByIDLookup(t *testing.T) {
	reg := NewRegistry()
	id, _ := reg.Register("dev-lookup", time.Minute, "U", "1.2.3.4:9")
	if got := reg.EndpointByID(id); got != "dev-lookup" {
		t.Fatalf("EndpointByID=%q", got)
	}
	if got := reg.EndpointByID("ghost"); got != "" {
		t.Fatalf("未知 ID 应空串, got %q", got)
	}
}
