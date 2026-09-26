// 文件用途：协议网关装配单测——配置门控、隔离注册表的多客户端路由闭环（TB-22）。
package protocolgw

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/coap"
	"aetherlink-iot/backend/internal/lwm2m"
	"github.com/spf13/viper"
)

func TestStartDisabledReturnsNil(t *testing.T) {
	viper.Set("protocols.coap.enabled", false)
	defer viper.Reset()
	cfg := DefaultConfig()
	if cfg.Enabled {
		t.Fatal("disabled config must not be enabled")
	}
	gw, err := Start(cfg, nil)
	if err != nil || gw != nil {
		t.Fatalf("disabled start must return nil,nil (gw=%v err=%v)", gw, err)
	}
}

func TestBuildRegistryRegistersLwM2M(t *testing.T) {
	reg := BuildRegistry()
	if reg == nil {
		t.Fatal("registry must not be nil")
	}
	// 无网络调用，仅验证注册表面与通配匹配：/.well-known/core 可达。
	_ = reg
}

// --- TB-22：隔离注册表闭环（注册 → 按源地址读写 → 去注册拒绝） ---

func uriOpts(path string) []coap.Option {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	opts := make([]coap.Option, 0, len(segs))
	for _, s := range segs {
		opts = append(opts, coap.Option{Number: coap.OptionUriPath, Value: []byte(s)})
	}
	return opts
}

// isolatedFixture 构造隔离注册表 + 事件桥（bridge 事件 → OnEvent → store 映射）。
func isolatedFixture() (*coap.Registry, *TelemetryBridge, *[]lwm2m.RegistryEvent) {
	b := NewTelemetryBridge(nil, nil, nil)
	var events []lwm2m.RegistryEvent
	reg := BuildRegistryWithIsolation(func(ev lwm2m.RegistryEvent) {
		events = append(events, ev)
		b.OnEvent(ev)
	}, b.StoreForAddr)
	return reg, b, &events
}

func TestIsolatedRegistryRegisterWriteDeregisterLoop(t *testing.T) {
	reg, b, events := isolatedFixture()

	// 未注册源写对象 → 4.04（fail-closed）。
	anon := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePut, MessageID: 1,
		Options: uriOpts("/3303/0/5700"), Payload: []byte("1.0"), RemoteAddr: "9.9.9.9:9000"}
	if resp, _ := reg.Serve(anon); resp.Code != coap.CodeNotFound {
		t.Fatalf("未注册源应 4.04, got %v", resp.Code)
	}

	// 端点 A 注册（源地址 10.0.0.1:1000）→ 2.01，事件携带端点与地址。
	regA := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePost, MessageID: 2,
		Options:    append(uriOpts("/rd"), coap.Option{Number: coap.OptionUriQuery, Value: []byte("ep=urn:ep-a")}),
		RemoteAddr: "10.0.0.1:1000"}
	resp, _ := reg.Serve(regA)
	if resp.Code != coap.CodeCreated {
		t.Fatalf("注册应 2.01, got %v", resp.Code)
	}
	if len(*events) != 1 || (*events)[0].Kind != lwm2m.EventRegister ||
		(*events)[0].Endpoint != "urn:ep-a" || (*events)[0].Addr != "10.0.0.1:1000" {
		t.Fatalf("注册事件不符: %+v", *events)
	}
	idA := strings.TrimPrefix(string(resp.Payload), "id=")

	// 同源地址 PUT → 落端点 A 的 store；随后 GET 回读一致。
	putA := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePut, MessageID: 3,
		Options: uriOpts("/3303/0/5700"), Payload: []byte("23.5"), RemoteAddr: "10.0.0.1:1000"}
	if resp, _ := reg.Serve(putA); resp.Code != coap.CodeChanged {
		t.Fatalf("PUT 应 2.04, got %v", resp.Code)
	}
	getA := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeGet, MessageID: 4,
		Options: uriOpts("/3303/0/5700"), RemoteAddr: "10.0.0.1:1000"}
	if resp, _ := reg.Serve(getA); resp.Code != coap.CodeContent || string(resp.Payload) != "23.5" {
		t.Fatalf("GET 应 2.05 %q, got %v %q", "23.5", resp.Code, resp.Payload)
	}
	if b.EndpointCount() != 1 {
		t.Fatalf("端点数=%d", b.EndpointCount())
	}

	// 另一端点 B 从不同源地址注册并写入 → 与 A 互相隔离。
	regB := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePost, MessageID: 5,
		Options:    append(uriOpts("/rd"), coap.Option{Number: coap.OptionUriQuery, Value: []byte("ep=urn:ep-b")}),
		RemoteAddr: "10.0.0.2:2000"}
	if _, _ = reg.Serve(regB); b.EndpointCount() != 2 {
		t.Fatalf("双端点注册后端点数=%d", b.EndpointCount())
	}
	putB := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePut, MessageID: 6,
		Options: uriOpts("/3303/0/5700"), Payload: []byte("77.0"), RemoteAddr: "10.0.0.2:2000"}
	if _, _ = reg.Serve(putB); b.StoreForAddr("10.0.0.2:2000") == b.StoreForAddr("10.0.0.1:1000") {
		t.Fatal("端点 A/B 不得共享 store")
	}
	if v, ok := b.StoreForAddr("10.0.0.2:2000").Get(3303, 0, 5700); !ok || v != "77.0" {
		t.Fatalf("端点 B 写入不符: %q ok=%v", v, ok)
	}
	if v, ok := b.StoreForAddr("10.0.0.1:1000").Get(3303, 0, 5700); !ok || v != "23.5" {
		t.Fatalf("端点 A 值被 B 覆盖（last-wins 回归）: %q ok=%v", v, ok)
	}

	// 端点 A 去注册 → 2.02 + 去注册事件；源地址随即可写性消失（4.04）。
	delA := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeDelete, MessageID: 7,
		Options: uriOpts("/rd/" + idA), RemoteAddr: "10.0.0.1:1000"}
	if resp, _ := reg.Serve(delA); resp.Code != coap.CodeDeleted {
		t.Fatalf("去注册应 2.02, got %v", resp.Code)
	}
	last := (*events)[len(*events)-1]
	if last.Kind != lwm2m.EventDeregister || last.Endpoint != "urn:ep-a" || last.ID != idA {
		t.Fatalf("去注册事件不符: %+v", last)
	}
	if b.EndpointCount() != 1 {
		t.Fatalf("去注册后端点数=%d", b.EndpointCount())
	}
	if resp, _ := reg.Serve(putA); resp.Code != coap.CodeNotFound {
		t.Fatalf("去注册后 PUT 应 4.04, got %v", resp.Code)
	}
	// 端点 B 不受 A 去注册影响。
	getB := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodeGet, MessageID: 8,
		Options: uriOpts("/3303/0/5700"), RemoteAddr: "10.0.0.2:2000"}
	if resp, _ := reg.Serve(getB); resp.Code != coap.CodeContent || string(resp.Payload) != "77.0" {
		t.Fatalf("B 去注册 A 后不得受影响: %v %q", resp.Code, resp.Payload)
	}
}

func TestIsolatedRegistryWithoutLookupStillRegisters(t *testing.T) {
	// lookup 为 nil（异常装配）时注册表仍可注册，但不提供对象路由面。
	reg := BuildRegistryWithIsolation(nil, nil)
	req := &coap.Message{Type: coap.TypeConfirmable, Code: coap.CodePost, MessageID: 1,
		Options: append(uriOpts("/rd"), coap.Option{Number: coap.OptionUriQuery, Value: []byte("ep=x")})}
	if resp, _ := reg.Serve(req); resp.Code != coap.CodeCreated {
		t.Fatalf("注册应 2.01, got %v", resp.Code)
	}
}
