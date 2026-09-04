// 文件用途：C6 收尾（WORKPLAN P1-C）——设备凭证映射 + 遥测汇入现有 uplink 管道。
// 核心链路：LwM2M/CoAP 客户端 PUT/POST 对象资源 → ObjectStore.OnChange → TelemetryBridge
//
//	（端点→设备解析 + IPSO 资源键转换）→ 与 mqttadapter 相同的 UplinkMessage → uplink.Bus。
//
// 关键约定：
//  1. 凭证映射：端点名称 == devices.device_number（与 MQTT 设备号同源）；租户取自设备
//     记录，不信任客户端上报；is_enabled != enabled 的设备拒绝上报（CoAP 无连接级认证，
//     本解析器即准入边界，弱凭证边界已知，PSK 升级为后续安全增强）。
//  2. fail-closed：端点未注册/解析失败/值转换失败一律丢弃并计数，绝不阻塞 CoAP 写路径。
//  3. 拓扑：当前 BuildRegistry 为共享单 ObjectStore（单客户端模型），端点以最近一次
//     /rd 注册为准（last-wins）；多客户端隔离留待上层按注册分发 store 时替换。
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

// storeEvent 一次资源写入事件。
type storeEvent struct {
	obj, inst, res uint16
	value          string
}

// TelemetryBridge 把 LwM2M 对象写入转换为平台遥测并发往 uplink Bus。
type TelemetryBridge struct {
	resolver  DeviceResolver
	publisher UplinkPublisher
	log       *logrus.Logger

	events chan storeEvent
	stop   chan struct{}
	done   chan struct{}

	mu       sync.Mutex
	endpoint string // 最近一次 /rd 注册的端点名（单客户端拓扑 last-wins）
	store    *lwm2m.ObjectStore

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

// Attach 绑定对象存储：资源写入经 OnChange 入队（异步移交，不阻塞写路径）。
func (b *TelemetryBridge) Attach(store *lwm2m.ObjectStore) {
	b.mu.Lock()
	b.store = store
	b.mu.Unlock()
	store.SetOnChange(func(obj, inst, res uint16, value string) {
		select {
		case b.events <- storeEvent{obj: obj, inst: inst, res: res, value: value}:
		default:
			b.mu.Lock()
			b.dropped++
			b.mu.Unlock()
		}
	})
}

// OnRegister /rd 注册成功回调：刷新当前端点（last-wins）。
func (b *TelemetryBridge) OnRegister(endpoint string) {
	b.mu.Lock()
	b.endpoint = endpoint
	b.mu.Unlock()
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

// handleEvent 单事件处理：端点→设备→值转换→UplinkMessage→Bus。任何失败 fail-closed 计数。
func (b *TelemetryBridge) handleEvent(ev storeEvent) {
	b.mu.Lock()
	endpoint := b.endpoint
	b.mu.Unlock()
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
