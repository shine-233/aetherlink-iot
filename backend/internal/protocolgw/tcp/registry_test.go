// 文件用途：会话注册簿单测（TP-03）——device_number 映射、同号顶替、身份精确去注册、Known 留存。
// 核心逻辑：net.Pipe 充当会话连接验证 CloseAll 真正关闭底层连接；回调触发次序与幂等性钉死。
// 关键注意事项：顶替后旧会话去注册不得误删新会话（身份指针校验）——断网缓冲正确性的前提。
// 重构建议：引入会话过期（空闲踢除已在网关层实现）时补过期语义用例。
package tcp

import (
	"net"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/protocolgw"
)

// pipeSession 用 net.Pipe 构造一条测试会话（返回客户端侧连接供断言读写）。
func pipeSession(t *testing.T, number string) (*session, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	s := &session{
		number:      number,
		remote:      "pipe",
		conn:        server,
		identity:    &protocolgw.DeviceIdentity{DeviceID: "id-" + number, TenantID: "t1", DeviceNumber: number},
		connectedAt: time.Now(),
	}
	t.Cleanup(func() { _ = client.Close() })
	return s, client
}

func TestRegistryRegisterLookupDeregister(t *testing.T) {
	r := NewRegistry(nil, nil)
	s, _ := pipeSession(t, "dev-1")
	if old := r.Register(s); old != nil {
		t.Fatalf("first register should not displace, got %v", old)
	}
	if r.Lookup("dev-1") != s {
		t.Fatal("lookup should return registered session")
	}
	if r.Count() != 1 || r.KnownCount() != 1 || !r.Known("dev-1") {
		t.Fatalf("count=%d known=%d known(dev-1)=%v", r.Count(), r.KnownCount(), r.Known("dev-1"))
	}
	if !r.Deregister(s) {
		t.Fatal("deregister by identity should succeed")
	}
	if r.Lookup("dev-1") != nil {
		t.Fatal("session should be gone after deregister")
	}
	// 去注册后 Known 仍留存（断网缓冲路由依据）。
	if !r.Known("dev-1") {
		t.Fatal("known set must retain number after offline")
	}
}

func TestRegistryDeregisterWrongIdentityIsNoop(t *testing.T) {
	r := NewRegistry(nil, nil)
	s1, _ := pipeSession(t, "dev-1")
	s2, _ := pipeSession(t, "dev-1")
	r.Register(s1)
	// s2 从未注册：按它去注册应失败且不影响 s1。
	if r.Deregister(s2) {
		t.Fatal("deregister of unregistered session should return false")
	}
	if r.Lookup("dev-1") != s1 {
		t.Fatal("original session must remain")
	}
}

func TestRegistryDisplacementKeepsNewSession(t *testing.T) {
	var registered []string
	r := NewRegistry(func(number string) { registered = append(registered, number) }, nil)
	s1, c1 := pipeSession(t, "dev-1")
	s2, _ := pipeSession(t, "dev-1")

	if old := r.Register(s1); old != nil {
		t.Fatalf("register 1 displaced %v", old)
	}
	old := r.Register(s2)
	if old != s1 {
		t.Fatalf("register 2 should displace s1, got %v", old)
	}
	if r.Lookup("dev-1") != s2 {
		t.Fatal("new session must win the mapping")
	}
	// 旧会话（已被顶替）去注册不得误删新会话。
	if r.Deregister(s1) {
		t.Fatal("displaced session deregister should be noop")
	}
	if r.Lookup("dev-1") != s2 {
		t.Fatal("new session must survive displaced deregister")
	}
	if len(registered) != 2 {
		t.Fatalf("register callbacks = %v, want 2", registered)
	}
	_ = c1.Close()
}

func TestRegistryCloseAllClosesConns(t *testing.T) {
	r := NewRegistry(nil, nil)
	s1, c1 := pipeSession(t, "dev-1")
	s2, c2 := pipeSession(t, "dev-2")
	r.Register(s1)
	r.Register(s2)
	if n := r.CloseAll(); n != 2 {
		t.Fatalf("CloseAll closed %d, want 2", n)
	}
	if r.Count() != 0 {
		t.Fatalf("count after CloseAll = %d, want 0", r.Count())
	}
	// net.Pipe 关闭后对端读写立即报错，验证连接真被关闭。
	buf := make([]byte, 1)
	if _, err := c1.Read(buf); err == nil {
		t.Fatal("client conn should be closed after CloseAll")
	}
	if _, err := c2.Read(buf); err == nil {
		t.Fatal("client conn 2 should be closed after CloseAll")
	}
}
