// 文件用途：TelemetryBridge（端点凭证映射 + 遥测汇入 uplink）与 DBNumberResolver 测试（P1-C）。
package protocolgw

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// --- 假实现 ---

type fakeResolver struct {
	mu   sync.Mutex
	byEp map[string]*DeviceIdentity
	err  error
}

func (f *fakeResolver) ResolveByNumber(number string) (*DeviceIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if id, ok := f.byEp[number]; ok {
		return id, nil
	}
	return nil, errFakeNotFound
}

var errFakeNotFound = &fakeError{"no device"}

type fakeError struct{ s string }

func (e *fakeError) Error() string { return e.s }

type fakePublisher struct {
	mu   sync.Mutex
	msgs []*mqttadapter.UplinkMessage
	err  error
}

func (f *fakePublisher) Publish(msg *mqttadapter.UplinkMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.msgs = append(f.msgs, msg)
	return nil
}

func (f *fakePublisher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.msgs)
}

// --- 键/值转换 ---

func TestTelemetryKeyKnownIPSO(t *testing.T) {
	if got := telemetryKey(3303, 0, 5700); got != "temperature" {
		t.Fatalf("telemetryKey(3303,0,5700) = %q, 期望 temperature", got)
	}
	if got := telemetryKey(3304, 0, 5700); got != "humidity" {
		t.Fatalf("telemetryKey(3304,0,5700) = %q, 期望 humidity", got)
	}
}

func TestTelemetryKeyFallbackForUnknownResource(t *testing.T) {
	if got := telemetryKey(9999, 2, 1); got != "lwm2m/9999/2/1" {
		t.Fatalf("未知资源键 = %q, 期望 lwm2m/9999/2/1", got)
	}
}

func TestIPSOValueNumericAndString(t *testing.T) {
	if v, ok := ipsoValue("23.5"); !ok || v != 23.5 {
		t.Fatalf("数值转换不符: %v ok=%v", v, ok)
	}
	if v, ok := ipsoValue("open"); !ok || v != "open" {
		t.Fatalf("字符串保留不符: %v ok=%v", v, ok)
	}
	if _, ok := ipsoValue(""); ok {
		t.Fatal("空值必须丢弃")
	}
}

// --- Bridge.handleEvent 同步路径 ---

func newTestBridge(res DeviceResolver, pub UplinkPublisher) *TelemetryBridge {
	return NewTelemetryBridge(res, pub, nil)
}

func TestBridgeHandleEventPublishesTelemetry(t *testing.T) {
	res := &fakeResolver{byEp: map[string]*DeviceIdentity{
		"urn:dev-001": {DeviceID: "dev-uuid-1", TenantID: "tenant-A", DeviceNumber: "urn:dev-001"},
	}}
	pub := &fakePublisher{}
	b := newTestBridge(res, pub)

	b.handleEvent(storeEvent{endpoint: "urn:dev-001", obj: 3303, inst: 0, res: 5700, value: "23.5"})

	if pub.count() != 1 {
		t.Fatalf("发布条数 = %d, 期望 1", pub.count())
	}
	msg := pub.msgs[0]
	if msg.Type != "telemetry" || msg.DeviceID != "dev-uuid-1" || msg.TenantID != "tenant-A" {
		t.Fatalf("消息头不符: %+v", msg)
	}
	var values map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &values); err != nil {
		t.Fatalf("payload 必须为 JSON: %v", err)
	}
	if v, ok := values["temperature"].(float64); !ok || v != 23.5 {
		t.Fatalf("values[temperature] = %v, 期望 23.5", values["temperature"])
	}
	if msg.Metadata["source_protocol"] != "coap" {
		t.Fatalf("source_protocol = %v, 期望 coap", msg.Metadata["source_protocol"])
	}
	if published, _, _ := b.Stats(); published != 1 {
		t.Fatalf("published = %d, 期望 1", published)
	}
}

func TestBridgeHandleEventFailClosedWithoutEndpoint(t *testing.T) {
	pub := &fakePublisher{}
	b := newTestBridge(&fakeResolver{byEp: map[string]*DeviceIdentity{"x": {DeviceID: "d"}}}, pub)
	// 端点未知的写入事件（写侧归因缺失）直接丢弃。
	b.handleEvent(storeEvent{obj: 3303, inst: 0, res: 5700, value: "1"})
	if pub.count() != 0 {
		t.Fatal("无端点映射时不得发布")
	}
	if _, unknown, _ := b.Stats(); unknown != 1 {
		t.Fatalf("unknown = %d, 期望 1", unknown)
	}
}

func TestBridgeHandleEventFailClosedOnResolverError(t *testing.T) {
	res := &fakeResolver{err: &fakeError{"db down"}}
	pub := &fakePublisher{}
	b := newTestBridge(res, pub)
	b.handleEvent(storeEvent{endpoint: "urn:any", obj: 3303, inst: 0, res: 5700, value: "1"})
	if pub.count() != 0 {
		t.Fatal("解析失败时不得发布")
	}
	if _, unknown, _ := b.Stats(); unknown != 1 {
		t.Fatalf("unknown = %d, 期望 1", unknown)
	}
}

func TestBridgeHandleEventDropsEmptyValue(t *testing.T) {
	res := &fakeResolver{byEp: map[string]*DeviceIdentity{"ep": {DeviceID: "d", TenantID: "t"}}}
	pub := &fakePublisher{}
	b := newTestBridge(res, pub)
	b.handleEvent(storeEvent{endpoint: "ep", obj: 3303, inst: 0, res: 5700, value: ""})
	if pub.count() != 0 {
		t.Fatal("空值不得发布")
	}
	if _, _, dropped := b.Stats(); dropped != 1 {
		t.Fatalf("dropped = %d, 期望 1", dropped)
	}
}

// --- Bridge 端到端（注册 → 端点 store → Set → worker → publish）---

func TestBridgeAttachAndRunEndToEnd(t *testing.T) {
	res := &fakeResolver{byEp: map[string]*DeviceIdentity{
		"urn:dev-e2e": {DeviceID: "dev-e2e", TenantID: "tenant-E2E", DeviceNumber: "urn:dev-e2e"},
	}}
	pub := &fakePublisher{}
	b := newTestBridge(res, pub)
	addr := "127.0.0.1:45100"
	b.OnRegister("urn:dev-e2e", addr)
	store := b.StoreForAddr(addr)
	if store == nil {
		t.Fatal("注册后源地址必须路由到端点 store")
	}
	go b.Run()
	defer b.Stop()

	if err := store.Set(3303, 0, 5700, "26.8"); err != nil {
		t.Fatalf("set: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if published, _, _ := b.Stats(); published == 1 && pub.count() == 1 {
			msg := pub.msgs[0]
			if !strings.Contains(string(msg.Payload), "26.8") {
				t.Fatalf("payload 缺少上报值: %s", msg.Payload)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("3s 内未完成端到端发布")
}

// --- TB-22 多端点隔离 ---

func TestBridgePerEndpointIsolation(t *testing.T) {
	res := &fakeResolver{byEp: map[string]*DeviceIdentity{
		"urn:ep-a": {DeviceID: "dev-a", TenantID: "tenant-A", DeviceNumber: "urn:ep-a"},
		"urn:ep-b": {DeviceID: "dev-b", TenantID: "tenant-B", DeviceNumber: "urn:ep-b"},
	}}
	pub := &fakePublisher{}
	b := newTestBridge(res, pub)
	addrA, addrB := "10.1.0.1:5000", "10.1.0.2:5001"
	b.OnRegister("urn:ep-a", addrA)
	b.OnRegister("urn:ep-b", addrB)

	storeA := b.StoreForAddr(addrA)
	storeB := b.StoreForAddr(addrB)
	if storeA == nil || storeB == nil {
		t.Fatal("两端点 store 必须就绪")
	}
	if storeA == storeB {
		t.Fatal("端点间不得共享 store")
	}
	if b.EndpointCount() != 2 {
		t.Fatalf("端点数=%d, 期望 2", b.EndpointCount())
	}
	if ep := b.EndpointForAddr(addrB); ep != "urn:ep-b" {
		t.Fatalf("addrB 归属端点=%q", ep)
	}

	go b.Run()
	defer b.Stop()
	// 各端点写入自己的资源值。
	if err := storeA.Set(3303, 0, 5700, "21.0"); err != nil {
		t.Fatal(err)
	}
	if err := storeB.Set(3304, 0, 5700, "55.0"); err != nil {
		t.Fatal(err)
	}

	// 两端点各自发布到自己的设备身份，互不串扰。
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if pub.count() == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pub.count() != 2 {
		t.Fatalf("发布条数=%d, 期望 2", pub.count())
	}
	seen := map[string]map[string]interface{}{}
	for _, msg := range pub.msgs {
		if msg.DeviceID == "dev-a" && msg.TenantID != "tenant-A" {
			t.Fatalf("端点 A 遥测租户错乱: %+v", msg)
		}
		if msg.DeviceID == "dev-b" && msg.TenantID != "tenant-B" {
			t.Fatalf("端点 B 遥测租户错乱: %+v", msg)
		}
		if msg.Metadata["endpoint"] == "" {
			t.Fatalf("metadata 缺端点: %+v", msg.Metadata)
		}
		var values map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &values); err != nil {
			t.Fatalf("payload 非法: %v", err)
		}
		seen[msg.DeviceID] = values
	}
	// 关键断言：A 的设备只收到 A 的值，B 同理（资源值不跨端点泄漏）。
	if _, ok := seen["dev-a"]["temperature"]; !ok {
		t.Fatalf("dev-a 缺 temperature: %v", seen["dev-a"])
	}
	if _, ok := seen["dev-a"]["humidity"]; ok {
		t.Fatalf("dev-a 不得出现 B 端点的 humidity: %v", seen["dev-a"])
	}
	if _, ok := seen["dev-b"]["humidity"]; !ok {
		t.Fatalf("dev-b 缺 humidity: %v", seen["dev-b"])
	}
	if _, ok := seen["dev-b"]["temperature"]; ok {
		t.Fatalf("dev-b 不得出现 A 端点的 temperature: %v", seen["dev-b"])
	}
	// store 层面同样互不可见。
	if _, ok := storeA.Get(3304, 0, 5700); ok {
		t.Fatal("storeA 不得含 B 端点写入")
	}
	if _, ok := storeB.Get(3303, 0, 5700); ok {
		t.Fatal("storeB 不得含 A 端点写入")
	}
}

func TestBridgeDeregisterRemovesStoreAndAddrBinding(t *testing.T) {
	pub := &fakePublisher{}
	b := newTestBridge(&fakeResolver{}, pub)
	addr := "10.2.0.1:6000"
	b.OnRegister("urn:ep-x", addr)
	if b.EndpointCount() != 1 {
		t.Fatalf("注册后端点数=%d", b.EndpointCount())
	}
	b.OnDeregister("urn:ep-x")
	if b.EndpointCount() != 0 {
		t.Fatalf("去注册后端点数=%d", b.EndpointCount())
	}
	if st := b.StoreForAddr(addr); st != nil {
		t.Fatal("去注册后源地址不得再路由到 store")
	}
	// 去注册重复调用与未知端点必须幂等。
	b.OnDeregister("urn:ep-x")
	b.OnDeregister("")
	if b.EndpointCount() != 0 {
		t.Fatal("幂等去注册后端点数应仍为 0")
	}
}

func TestBridgeReregisterRefreshKeepsStoreValues(t *testing.T) {
	pub := &fakePublisher{}
	b := newTestBridge(&fakeResolver{}, pub)
	addr := "10.3.0.1:7000"
	b.OnRegister("urn:ep-r", addr)
	store := b.StoreForAddr(addr)
	if err := store.Set(3303, 0, 5700, "19.5"); err != nil {
		t.Fatal(err)
	}
	// 刷新注册（同端点）必须复用既有 store（不清空已写资源），不得覆盖归因。
	b.OnRegister("urn:ep-r", addr)
	if b.EndpointCount() != 1 {
		t.Fatalf("刷新注册后端点数=%d", b.EndpointCount())
	}
	if v, ok := b.StoreForAddr(addr).Get(3303, 0, 5700); !ok || v != "19.5" {
		t.Fatalf("刷新注册丢失已写资源: %q ok=%v", v, ok)
	}
}

func TestBridgeSameAddrRebindSwitchesEndpoint(t *testing.T) {
	pub := &fakePublisher{}
	b := newTestBridge(&fakeResolver{}, pub)
	addr := "10.4.0.1:8000"
	b.OnRegister("urn:old-ep", addr)
	b.OnRegister("urn:new-ep", addr) // 同源地址换端点重注册 → 以最新注册为准
	if ep := b.EndpointForAddr(addr); ep != "urn:new-ep" {
		t.Fatalf("源地址应改绑最新端点, got %q", ep)
	}
}

// --- DBNumberResolver（sqlite 真库）---

func newResolverDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:resolver_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Device{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDBNumberResolverResolvesEnabledDevice(t *testing.T) {
	db := newResolverDB(t)
	if err := db.Create(&model.Device{ID: "uuid-1", DeviceNumber: "urn:dev-9", TenantID: "tenant-9", IsEnabled: "enabled"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := NewDBNumberResolver(db)
	id, err := r.ResolveByNumber("urn:dev-9")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if id.DeviceID != "uuid-1" || id.TenantID != "tenant-9" || id.DeviceNumber != "urn:dev-9" {
		t.Fatalf("identity 不符: %+v", id)
	}
}

func TestDBNumberResolverRejectsDisabledAndMissing(t *testing.T) {
	db := newResolverDB(t)
	if err := db.Create(&model.Device{ID: "uuid-2", DeviceNumber: "urn:dev-off", TenantID: "t", IsEnabled: "disabled"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := NewDBNumberResolver(db)
	if _, err := r.ResolveByNumber("urn:dev-off"); err == nil {
		t.Fatal("禁用设备必须拒绝")
	}
	if _, err := r.ResolveByNumber("urn:ghost"); err == nil {
		t.Fatal("未知端点必须拒绝")
	}
	if _, err := r.ResolveByNumber(""); err == nil {
		t.Fatal("空端点必须拒绝")
	}
}

func TestDBNumberResolverCacheHitSurvivesRowRemoval(t *testing.T) {
	db := newResolverDB(t)
	if err := db.Create(&model.Device{ID: "uuid-3", DeviceNumber: "urn:dev-c", TenantID: "t", IsEnabled: "enabled"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := NewDBNumberResolver(db)
	if _, err := r.ResolveByNumber("urn:dev-c"); err != nil {
		t.Fatalf("首次解析: %v", err)
	}
	// 删库行：TTL 缓存内仍应命中（防 UDP 高频击穿的既定语义）。
	if err := db.Where("device_number = ?", "urn:dev-c").Delete(&model.Device{}).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	id, err := r.ResolveByNumber("urn:dev-c")
	if err != nil || id == nil {
		t.Fatalf("缓存命中应成功: id=%v err=%v", id, err)
	}
}
