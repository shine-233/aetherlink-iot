// 文件用途：SNMP/OPC UA 轮询采集器共享骨架（ROADMAP C6 收尾——库级协议栈的管理侧接入）。
// 核心链路：device_configs.protocol_type 门控发现启用设备 → 解析 protocol_config 点表 →
//
//	周期轮询（Poller 协议实现）→ 与 MQTT/CoAP 同一 UplinkMessage → uplink.Bus。
//
// 关键约定：
//  1. 身份可信：设备归属取自 DB（devices.tenant_id），不依赖客户端自报（与 protocolgw 一致）；
//  2. fail-closed：单目标解析/采集失败只计数告警，绝不阻塞其他目标与采集循环；
//  3. 发现缓存 TTL 60s：与 protocolgw DBNumberResolver 缓存语义一致，点表变更最迟 TTL 后生效；
//  4. 轮询模型为拉式（与 CoAP/LwM2M 推式互补）：网关侧仅出站 UDP/TCP 连接，无需平台入站端口。
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/adapter/mqttadapter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// Config 采集器配置（viper 键 collectors.*）。
type Config struct {
	SNMPEnabled  bool
	OpcuaEnabled bool
	Interval     time.Duration // 采集周期（下限 5s 防误配打爆目标设备）
	Timeout      time.Duration // 单目标采集预算
}

// defaultInterval/defaultTimeout 默认值：60s 周期覆盖常规监控场景；3s 单目标预算。
const (
	defaultInterval = 60 * time.Second
	defaultTimeout  = 3 * time.Second
	minInterval     = 5 * time.Second
)

// DefaultConfig 读取 viper；缺省双协议关闭（显式启用才启动，与 protocols.coap.enabled 同策略）。
func DefaultConfig() Config {
	interval := viper.GetDuration("collectors.interval")
	if interval <= 0 {
		interval = defaultInterval
	}
	if interval < minInterval {
		interval = minInterval
	}
	timeout := viper.GetDuration("collectors.timeout")
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return Config{
		SNMPEnabled:  viper.GetBool("collectors.snmp.enabled"),
		OpcuaEnabled: viper.GetBool("collectors.opcua.enabled"),
		Interval:     interval,
		Timeout:      timeout,
	}
}

// Publisher 遥测汇入接口；*uplink.Bus 天然满足（与 protocolgw.UplinkPublisher 同形）。
type Publisher interface {
	Publish(msg *mqttadapter.UplinkMessage) error
}

// deviceTarget 一次采集目标：设备身份 + 其 device_configs 协议点表原文。
type deviceTarget struct {
	DeviceID     string `gorm:"column:device_id"`
	TenantID     string `gorm:"column:tenant_id"`
	DeviceNumber string `gorm:"column:device_number"`
	ConfigJSON   string `gorm:"column:config_json"`
}

// discoverTargets 发现启用设备 + 指定协议类型的点表（devices ⨝ device_configs）。
// 协议类型取 device_configs.protocol_type（"SNMP"/"OPCUA"，与 "MQTT" 命名口径一致）。
func discoverTargets(db *gorm.DB, protocolType string) ([]deviceTarget, error) {
	var rows []deviceTarget
	err := db.Table("devices d").
		Select("d.id AS device_id, d.tenant_id AS tenant_id, d.device_number AS device_number, dc.protocol_config AS config_json").
		Joins("JOIN device_configs dc ON dc.id = d.device_config_id").
		Where("d.is_enabled = ? AND dc.protocol_type = ?", "enabled", protocolType).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("collector: 发现 %s 设备失败: %w", protocolType, err)
	}
	return rows, nil
}

// targetsCacheTTL 发现结果缓存 TTL。
const targetsCacheTTL = 60 * time.Second

// Stats 诊断计数（轮询/成功/失败/发布/丢弃）。
type Stats struct {
	Polls     uint64
	OK        uint64
	Failed    uint64
	Published uint64
	Dropped   uint64
}

// Poller 单协议采集实现（snmp/opcua 各一份）。
type Poller interface {
	// Protocol 返回 source_protocol 遥测标记（小写："snmp"/"opcua"）。
	Protocol() string
	// ConfigType 返回 device_configs.protocol_type 过滤值（大写，与 "MQTT" 命名口径一致）。
	ConfigType() string
	// Poll 采集一个目标；返回遥测键值（空 map 表示本轮无可上报数据）。
	Poll(ctx context.Context, t deviceTarget) (map[string]interface{}, error)
}

// Runner 驱动一个 Poller 按周期采集全部目标。
type Runner struct {
	db        *gorm.DB
	publisher Publisher
	poller    Poller
	interval  time.Duration
	timeout   time.Duration
	log       *logrus.Logger

	mu              sync.Mutex
	stats           Stats
	targets         []deviceTarget
	targetsExpireAt time.Time

	stop chan struct{}
	done chan struct{}
}

// NewRunner 构造采集 Runner（db/publisher/poller 必填）。
func NewRunner(db *gorm.DB, publisher Publisher, poller Poller, interval, timeout time.Duration, log *logrus.Logger) *Runner {
	if log == nil {
		log = logrus.New()
	}
	return &Runner{
		db:        db,
		publisher: publisher,
		poller:    poller,
		interval:  interval,
		timeout:   timeout,
		log:       log,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Run 阻塞运行采集循环（随服务生命周期常驻；Stop 后返回）。
func (r *Runner) Run() {
	defer close(r.done)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	// 启动先采一轮：设备上线无需等待首个完整周期。
	r.pollOnce()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.pollOnce()
		}
	}
}

// Stop 停止采集循环（进程级常驻语义：不 flush 在途目标）。
func (r *Runner) Stop() {
	close(r.stop)
	<-r.done
}

// Stats 返回诊断计数快照。
func (r *Runner) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

// currentTargets 返回发现结果（TTL 缓存内直读，过期重查；查询失败沿用旧列表）。
func (r *Runner) currentTargets() []deviceTarget {
	r.mu.Lock()
	cached := r.targets
	valid := time.Now().Before(r.targetsExpireAt)
	r.mu.Unlock()
	if valid {
		return cached
	}
	rows, err := discoverTargets(r.db, r.poller.ConfigType())
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		// 查询失败沿用旧列表（库闪断不中断采集），并告警。
		r.log.WithError(err).Warn("collector: 设备发现失败，沿用上轮目标列表")
		return r.targets
	}
	r.targets = rows
	r.targetsExpireAt = time.Now().Add(targetsCacheTTL)
	return r.targets
}

// pollOnce 一轮采集：逐目标带超时采集 → 汇总发布。
func (r *Runner) pollOnce() {
	targets := r.currentTargets()
	for _, t := range targets {
		select {
		case <-r.stop:
			return
		default:
		}
		r.pollTarget(t)
	}
}

// pollTarget 单目标采集与发布；任何失败 fail-closed 计数。
func (r *Runner) pollTarget(t deviceTarget) {
	r.mu.Lock()
	r.stats.Polls++
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	values, err := r.poller.Poll(ctx, t)
	if err != nil {
		r.mu.Lock()
		r.stats.Failed++
		r.mu.Unlock()
		r.log.WithFields(logrus.Fields{
			"device_id": t.DeviceID,
			"protocol":  r.poller.Protocol(),
			"error":     err,
		}).Warn("collector: 目标采集失败")
		return
	}
	r.mu.Lock()
	r.stats.OK++
	r.mu.Unlock()
	if len(values) == 0 {
		return
	}
	r.publish(t, values)
}

// publish 汇总键值发布为一条遥测（与 MQTT/CoAP 消费端同一 UplinkMessage 形状）。
func (r *Runner) publish(t deviceTarget, values map[string]interface{}) {
	payload, err := json.Marshal(values)
	if err != nil {
		r.mu.Lock()
		r.stats.Dropped++
		r.mu.Unlock()
		r.log.WithField("device_id", t.DeviceID).Warn("collector: 遥测序列化失败，丢弃")
		return
	}
	msg := &mqttadapter.UplinkMessage{
		Type:      "telemetry",
		DeviceID:  t.DeviceID,
		TenantID:  t.TenantID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
		Metadata: map[string]interface{}{
			"device_id":       t.DeviceID,
			"device_number":   t.DeviceNumber,
			"source_protocol": r.poller.Protocol(),
		},
	}
	if err := r.publisher.Publish(msg); err != nil {
		r.log.WithFields(logrus.Fields{
			"device_id": t.DeviceID,
			"protocol":  r.poller.Protocol(),
			"error":     err,
		}).Error("collector: 遥测发布失败")
		return
	}
	r.mu.Lock()
	r.stats.Published++
	r.mu.Unlock()
}
