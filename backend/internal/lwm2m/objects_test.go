// 文件用途：LwM2M 对象实例模型单测——路径解析、Set/Get/Delete/上限、经 coap.Registry
// 端到端读写 /19/0/0（binaryAppDataContainer）与非法路径/方法。
package lwm2m

import (
	"testing"

	"aetherlink-iot/backend/internal/coap"
)

func TestParseResourcePath(t *testing.T) {
	if o, i, r, ok := parseResourcePath("/19/0/1"); !ok || o != 19 || i != 0 || r != 1 {
		t.Fatalf("解析失败: %d %d %d %v", o, i, r, ok)
	}
	if _, _, _, ok := parseResourcePath("/19/0"); ok {
		t.Fatal("两段路径应非法")
	}
	if _, _, _, ok := parseResourcePath("/19/abc/1"); ok {
		t.Fatal("非数字段应非法")
	}
	if _, _, _, ok := parseResourcePath("/x/0/1"); ok {
		t.Fatal("非数字对象应非法")
	}
}

func TestObjectStoreRoundTripAndLimits(t *testing.T) {
	s := NewObjectStore()
	if err := s.Set(19, 0, 0, "payload-bytes"); err != nil {
		t.Fatal(err)
	}
	if v, ok := s.Get(19, 0, 0); !ok || v != "payload-bytes" {
		t.Fatalf("读回=%q ok=%v", v, ok)
	}
	if s.SnapshotCount() != 1 {
		t.Fatalf("条目数=%d", s.SnapshotCount())
	}
	if _, ok := s.Get(19, 0, 99); ok {
		t.Fatal("不存在资源应 miss")
	}
	if err := s.Set(19, 0, 0, string(make([]byte, maxResourceValueSize+1))); err == nil {
		t.Fatal("超限值应拒绝")
	}
	if !s.Delete(19, 0, 0) {
		t.Fatal("删除应成功")
	}
	if s.SnapshotCount() != 0 {
		t.Fatalf("删除后条目=%d", s.SnapshotCount())
	}
	if s.Delete(19, 0, 0) {
		t.Fatal("二次删除应 false")
	}
}

func TestObjectHandlerEndToEndViaCoAP(t *testing.T) {
	store := NewObjectStore()
	cr := coap.NewRegistry()
	BindObjects(cr, 19, store)

	// PUT /19/0/0
	put := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePut, MessageID: 1,
		Options: uriPathOptions("/19/0/0"), Payload: []byte("v1")}
	if resp, _ := cr.Serve(put); resp.Code != coap.CodeChanged {
		t.Fatalf("PUT 应 2.04, got %v", resp.Code)
	}
	// GET 回读
	get := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeGet, MessageID: 2,
		Options: uriPathOptions("/19/0/0")}
	resp, _ := cr.Serve(get)
	if resp.Code != coap.CodeContent || string(resp.Payload) != "v1" {
		t.Fatalf("GET=%v payload=%q", resp.Code, resp.Payload)
	}
	// GET 不存在资源 → 4.04
	getMissing, _ := cr.Serve(&coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeGet, MessageID: 3,
		Options: uriPathOptions("/19/0/9")})
	if getMissing.Code != coap.CodeNotFound {
		t.Fatalf("缺资源应 4.04, got %v", getMissing.Code)
	}
	// DELETE → 2.02
	del, _ := cr.Serve(&coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeDelete, MessageID: 4,
		Options: uriPathOptions("/19/0/0")})
	if del.Code != coap.CodeDeleted {
		t.Fatalf("DELETE 应 2.02, got %v", del.Code)
	}
	// 非法路径 → 4.00
	bad, _ := cr.Serve(&coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeGet, MessageID: 5,
		Options: uriPathOptions("/19/0")})
	if bad.Code != coap.CodeBadRequest {
		t.Fatalf("非法路径应 4.00, got %v", bad.Code)
	}
}
