// 文件用途：LwM2M 注册层单测——参数解析校验、注册/更新语义、注销、TTL 过期清理、
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

func TestRegisterUpdateDeleteAndExpiry(t *testing.T) {
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
	// 过期清理：时间推进超过 600s
	r.now = func() time.Time { return base.Add(601 * time.Second) }
	if removed := r.PruneExpired(); removed != 1 {
		t.Fatalf("应清理 1 个过期客户端, got %d", removed)
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
