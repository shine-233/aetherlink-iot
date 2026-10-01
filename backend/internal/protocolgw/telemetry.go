// 文件用途：C6 收尾（WORKPLAN P1-C）——设备凭证映射 + 遥测汇入现有 uplink 管道。
// 核心链路：LwM2M/CoAP 客户端 PUT/POST 对象资源 → 端点 ObjectStore.OnChange → TelemetryBridge
//
//	（端点→设备解析 + IPSO 资源键转换）→ 与 mqttadapter 相同的 UplinkMessage → uplink.Bus。
//
// 关键约定：
//  1. 凭证映射：端点名称 == devices.device_number（与 MQTT 设备号同源）；租户取自设备
//     记录，不信任客户端上报；is_enabled != enabled 的设备拒绝上报（CoAP 无连接级认证，
//     本解析器即准入边界，弱凭证边界已知，PSK 升级为后续安全增强）。
//  2. fail-closed：端点未注册/解析失败/值转换失败一律丢弃并计数，绝不阻塞 CoAP 写路径。
//  3. 拓扑（TB-22 多客户端隔离）：per-endpoint ObjectStore（sync.Map）由 /rd 注册/去注册
//     事件挂接；UDP 源地址经 OnRegister 绑定端点，对象写入按源地址路由到对应端点 store。
//     端点归因在事件入队时固写（写侧归因），消费侧不再依赖"最近注册端点"（原 last-wins 已移除）。
package protocolgw

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/lwm2m"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// DeviceIdentity 设备身份（凭证映射解析结果）。
type DeviceIdentity struct {
	DeviceID     string
	TenantID     string
	DeviceNumber string
}

// DeviceResolver 设备凭证映射：LwM2M 端点名称 → 平台设备。
type DeviceResolver interface {
	ResolveByNumber(number string) (*DeviceIdentity, error)
}

// UplinkPublisher 遥测汇入接口；*uplink.Bus 天然满足（与 mqttadapter 共用发布面）。
type UplinkPublisher interface {
	Publish(msg *mqttadapter.UplinkMessage) error
}

// resolverCacheTTL 设备解析缓存 TTL：UDP 高频写入下防 DB 击穿；
// 命中即代表沿用最近一次解析结果（设备启停最迟 TTL 后生效，与设备缓存语义一致）。
const resolverCacheTTL = 60 * time.Second

// DBNumberResolver gorm 实现：按 device_number 查设备，带进程内 TTL 缓存。
type DBNumberResolver struct {
	db    *gorm.DB
	mu    sync.Mutex
	cache map[string]resolverCacheEntry
}

type resolverCacheEntry struct {
	identity *DeviceIdentity
	expireAt time.Time
}

// NewDBNumberResolver 构造 DB 解析器（db 由 app 装配层注入）。
func NewDBNumberResolver(db *gorm.DB) *DBNumberResolver {
	return &DBNumberResolver{db: db, cache: map[string]resolverCacheEntry{}}
}

// ResolveByNumber 按 device_number 解析设备身份；未找到/禁用返回错误（fail-closed）。
func (r *DBNumberResolver) ResolveByNumber(number string) (*DeviceIdentity, error) {
	if number == "" {
		return nil, fmt.Errorf("lwm2m: 空端点名")
	}
	r.mu.Lock()
	if e, ok := r.cache[number]; ok && time.Now().Before(e.expireAt) {
		r.mu.Unlock()
		return e.identity, nil
	}
	r.mu.Unlock()

	var dev model.Device
	err := r.db.Where("device_number = ? AND is_enabled = ?", number, "enabled").First(&dev).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("lwm2m: 端点 %q 无对应启用设备", number)
		}
		return nil, fmt.Errorf("lwm2m: 设备查询失败: %w", err)
	}
	identity := &DeviceIdentity{DeviceID: dev.ID, TenantID: dev.TenantID, DeviceNumber: dev.DeviceNumber}

	r.mu.Lock()
	r.cache[number] = resolverCacheEntry{identity: identity, expireAt: time.Now().Add(resolverCacheTTL)}
	r.mu.Unlock()
	return identity, nil
}

// ipsoKeys 常用 IPSO 对象/资源 → 平台遥测键（覆盖演示对象 3303 及常见传感器，实例 0）；
// 未命中一律回退 "lwm2m/{obj}/{inst}/{res}" 键，保证任何资源上报不丢语义。
var ipsoKeys = map[string]string{
	"3303/0/5700": "temperature",   // 温度传感器
	"3304/0/5700": "humidity",      // 湿度
	"3323/0/5700": "pressure",      // 气压
	"3325/0/5700": "illuminance",   // 照度
	"3330/0/5700": "battery_level", // 电池
}

// ipsoValue 把资源文本值转为遥测值：可解析为数字则用 float64，否则保留原文（空值丢弃）。
func ipsoValue(raw string) (interface{}, bool) {
	if raw == "" {
		return nil, false
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f, true
	}
	return raw, true
}

// telemetryKey 把 (obj, inst, res) 映射为遥测键。
func telemetryKey(obj, inst, res uint16) string {
	if k, ok := ipsoKeys[fmt.Sprintf("%d/%d/%d", obj, inst, res)]; ok {
		return k
	}
	return fmt.Sprintf("lwm2m/%d/%d/%d", obj, inst, res)
}

// eventQueueSize 事件队列长度：满时丢弃并计数（UDP 场景宁可丢点不阻塞写路径）。
const eventQueueSize = 1024

// storeEvent 一次资源写入事件（endpoint 为写侧归因：写入发生在哪个端点的 store）。
type storeEvent struct {
	endpoint string
	obj, inst, res uint16
	value    string
}

// TelemetryBridge 把 LwM2M 对象写入转换为平台遥测并发往 uplink Bus。
// TB-22：端点→ObjectStore 与 UDP 源地址→端点均为 per-endpoint 映射（sync.Map），
// 由 /rd 注册/去注册事件挂接；不再存在跨端点共享的单一 store。
type TelemetryBridge struct {
	resolver  DeviceResolver
	publisher UplinkPublisher
	log       *logrus.Logger

	events chan storeEvent
	stop   chan struct{}
	done   chan struct{}

	// 多客户端隔离映射：端点→store 由 OnRegister/OnDeregister 维护；
	// 源地址→端点随注册事件刷新（同地址换端点重注册时以最新注册为准）。
	stores sync.Map // endpoint(string) → *lwm2m.ObjectStore
	addrs  sync.Map // remote addr(string) → endpoint(string)

	mu       sync.Mutex
	// 计数（诊断面，原子由 mu 内单 worker/单写者保证读侧仅 Stats 用 mu）
	published uint64
	unknown   uint64 // 端点未注册或解析失败
	dropped   uint64 // 队列满丢弃
}

// NewTelemetryBridge 构造遥测桥；resolver/publisher 均必填。
func NewTelemetryBridge(resolver DeviceResolver, publisher UplinkPublisher, log *logrus.Logger) *TelemetryBridge {
	if log == nil {
		log = logrus.New()
	}
	return &TelemetryBridge{
		resolver:  resolver,
		publisher: publisher,
		log:       log,
		events:    make(chan storeEvent, eventQueueSize),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// OnEvent 注册簿事件分派入口（Start 装配线挂接 /rd 处理器用）。
func (b *TelemetryBridge) OnEvent(ev lwm2m.RegistryEvent) {
	switch ev.Kind {
	case lwm2m.EventRegister:
		b.OnRegister(ev.Endpoint, ev.Addr)
	case lwm2m.EventDeregister:
		b.OnDeregister(ev.Endpoint)
	}
}

// OnRegister /rd 注册事件：为端点建独立 ObjectStore（幂等，刷新注册不清空已写资源），
// 并把 UDP 源地址绑定到端点（写路径路由依据）。端点归因自此在写侧固写，无 last-wins 覆盖。
func (b *TelemetryBridge) OnRegister(endpoint, addr string) {
	if endpoint == "" {
		return
	}
	st, loaded := b.stores.LoadOrStore(endpoint, lwm2m.NewObjectStore())
	if !loaded {
		store := st.(*lwm2m.ObjectStore)
		ep := endpoint // 归因固写：回调闭包携带端点名
		store.SetOnChange(func(obj, inst, res uint16, value string) {
			select {
			case b.events <- storeEvent{endpoint: ep, obj: obj, inst: inst, res: res, value: value}:
			default:
				b.mu.Lock()
				b.dropped++
				b.mu.Unlock()
			}
		})
	}
	if addr != "" {
		b.addrs.Store(addr, endpoint)
	}
}

// OnDeregister /rd 去注册事件：移除端点 store 与所有指向该端点的源地址绑定。
func (b *TelemetryBridge) OnDeregister(endpoint string) {
	if endpoint == "" {
		return
	}
	b.stores.Delete(endpoint)
	b.addrs.Range(func(k, v interface{}) bool {
		if ep, _ := v.(string); ep == endpoint {
			b.addrs.Delete(k)
		}
		return true
	})
}

// StoreForAddr 按源地址取端点 ObjectStore（未注册源返回 nil——对象路由层据此 4.04 拒绝）。
func (b *TelemetryBridge) StoreForAddr(addr string) *lwm2m.ObjectStore {
	if addr == "" {
		return nil
	}
	ep, ok := b.addrs.Load(addr)
	if !ok {
		return nil
	}
	st, ok := b.stores.Load(ep)
	if !ok {
		return nil
	}
	store, _ := st.(*lwm2m.ObjectStore)
	return store
}

// EndpointCount 返回当前挂接的端点 store 数（测试与诊断面用）。
func (b *TelemetryBridge) EndpointCount() int {
	n := 0
	b.stores.Range(func(_, _ interface{}) bool {
		n++
		return true
	})
	return n
}

// EndpointForAddr 返回源地址当前绑定的端点名（未绑定为空串，测试与诊断面用）。
func (b *TelemetryBridge) EndpointForAddr(addr string) string {
	if addr == "" {
		return ""
	}
	ep, _ := b.addrs.Load(addr)
	s, _ := ep.(string)
	return s
}

// Run 启动单 worker 消费事件（随网关生命周期常驻）。
func (b *TelemetryBridge) Run() {
	defer close(b.done)
	for {
		select {
		case <-b.stop:
			return
		case ev := <-b.events:
			b.handleEvent(ev)
		}
	}
}

// Stop 停止 worker（不flush残留事件——进程级常驻语义下 Stop 仅测试与关停用）。
func (b *TelemetryBridge) Stop() {
	close(b.stop)
	<-b.done
}

// Stats 返回诊断计数（published/unknown/dropped）。
func (b *TelemetryBridge) Stats() (published, unknown, dropped uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.published, b.unknown, b.dropped
}

// handleEvent 单事件处理：事件端点→设备→值转换→UplinkMessage→Bus。任何失败 fail-closed 计数。
// 端点取自事件本身（写侧归因固写），消费侧不再读取任何"当前端点"状态。
func (b *TelemetryBridge) handleEvent(ev storeEvent) {
	endpoint := ev.endpoint
	if endpoint == "" {
		b.mu.Lock()
		b.unknown++
		b.mu.Unlock()
		return
	}
	identity, err := b.resolver.ResolveByNumber(endpoint)
	if err != nil || identity == nil {
		b.mu.Lock()
		b.unknown++
		b.mu.Unlock()
		b.log.WithFields(logrus.Fields{
			"endpoint": endpoint,
			"resource": fmt.Sprintf("%d/%d/%d", ev.obj, ev.inst, ev.res),
			"error":    err,
		}).Warn("lwm2m 遥测丢弃：设备凭证映射失败")
		return
	}

	value, ok := ipsoValue(ev.value)
	if !ok {
		b.mu.Lock()
		b.dropped++
		b.mu.Unlock()
		return
	}
	values := map[string]interface{}{telemetryKey(ev.obj, ev.inst, ev.res): value}
	payload, err := json.Marshal(values)
	if err != nil {
		b.mu.Lock()
		b.dropped++
		b.mu.Unlock()
		return
	}

	msg := &mqttadapter.UplinkMessage{
		Type:      "telemetry",
		DeviceID:  identity.DeviceID,
		TenantID:  identity.TenantID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
		Metadata: map[string]interface{}{
			"device_id":       identity.DeviceID,
			"endpoint":        endpoint,
			"resource":        fmt.Sprintf("%d/%d/%d", ev.obj, ev.inst, ev.res),
			"source_protocol": "coap",
		},
	}
	if err := b.publisher.Publish(msg); err != nil {
		b.log.WithFields(logrus.Fields{
			"device_id": identity.DeviceID,
			"endpoint":  endpoint,
			"error":     err,
		}).Error("lwm2m 遥测发布失败")
		return
	}
	b.mu.Lock()
	b.published++
	b.mu.Unlock()
}
