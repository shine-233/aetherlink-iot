// 文件用途：维护设备心跳过期扫描和离线状态发布流程。
// 核心逻辑：从 Redis/配置读取心跳记录，识别过期设备，并通过状态发布器通知离线。
// 关键注意事项：误判离线会影响设备状态和告警，扫描需处理并发心跳、Redis 错误和重复发布。
// 生命周期：Start 幂等（重复启动不再叠加第二个过期事件消费协程），Stop 等待监听协程退出并关闭订阅。
// 重构建议：抽出时钟、Redis 和发布器接口，补齐过期边界、发布失败和幂等测试。
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// heartbeatMonitorStopTimeout 等待监听协程退出的停机预算，超时只告警不阻断停机序列。
const heartbeatMonitorStopTimeout = 5 * time.Second

// heartbeatExpirySubscriber 抽象过期事件订阅面，供生命周期测试注入假实现。
type heartbeatExpirySubscriber interface {
	Channel(...redis.ChannelOption) <-chan *redis.Message
	Close() error
}

// HeartbeatMonitor 心跳监控服务
type HeartbeatMonitor struct {
	redis           *redis.Client
	statusPublisher StatusPublisher // ✨ 依赖本地定义的接口（避免循环依赖）
	logger          *logrus.Logger
	ctx             context.Context
	cancel          context.CancelFunc

	// runMu 串行化 Start/Stop 并保护运行状态；stopCh 非 nil 表示监听协程在运行，
	// done 在协程完全退出后关闭。重复 Start 直接幂等返回，
	// 避免叠加第二个订阅消费协程导致离线事件被重复处理。
	runMu    sync.Mutex
	stopCh   chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	// subscribeExpiry 默认基于 redis.PSubscribe；测试可注入假实现验证启停语义。
	subscribeExpiry func(pattern string) (heartbeatExpirySubscriber, error)
}

// NewHeartbeatMonitor 创建心跳监控服务实例
func NewHeartbeatMonitor(redis *redis.Client, publisher StatusPublisher, logger *logrus.Logger) *HeartbeatMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	monitor := &HeartbeatMonitor{
		redis:           redis,
		statusPublisher: publisher,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
	}
	monitor.subscribeExpiry = func(pattern string) (heartbeatExpirySubscriber, error) {
		return monitor.redis.PSubscribe(monitor.ctx, pattern), nil
	}
	return monitor
}

// Start 启动心跳监控服务。幂等：已在运行时直接返回，不叠加第二个订阅消费协程。
func (m *HeartbeatMonitor) Start() error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	if m.stopCh != nil {
		m.logger.Warn("HeartbeatMonitor already running, skipping duplicate start")
		return nil
	}

	subscribeExpiry := m.subscribeExpiry

	// 检查是否启用订阅（单实例模式：只有一个实例订阅过期事件）
	// 如果未配置，默认为 true（启用订阅）
	subscribeEnabled := true
	if viper.IsSet("heartbeat.subscribe.enabled") {
		subscribeEnabled = viper.GetBool("heartbeat.subscribe.enabled")
	}
	if !subscribeEnabled {
		m.logger.Info("HeartbeatMonitor: Redis expiry event subscription is disabled, skipping subscription")
		return nil
	}

	// 配置 Redis 过期通知
	if err := m.configureRedis(); err != nil {
		return fmt.Errorf("failed to configure redis: %w", err)
	}

	// 获取 Redis 数据库编号
	dbNum := viper.GetInt("db.redis.db1")
	if dbNum == 0 {
		dbNum = 10 // 默认使用 db10
	}

	// 订阅过期事件
	pattern := fmt.Sprintf("__keyevent@%d__:expired", dbNum)
	pubsub, err := subscribeExpiry(pattern)
	if err != nil {
		return fmt.Errorf("subscribe heartbeat expiry events: %w", err)
	}

	stopCh := make(chan struct{})
	done := make(chan struct{})
	m.stopCh = stopCh
	m.stopOnce = sync.Once{}
	m.done = done

	m.logger.WithField("pattern", pattern).Info("HeartbeatMonitor started, subscribing to Redis expiry events")

	// 启动监听协程：退出时负责关闭订阅并 close(done)，让 Stop 可确定性等待。
	go func() {
		defer close(done)
		ch := pubsub.Channel()
		for {
			select {
			case <-stopCh:
				m.logger.Info("HeartbeatMonitor stopped")
				pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					// 订阅通道被外部关闭时同样退出并释放资源，避免空转。
					m.logger.Info("HeartbeatMonitor subscription channel closed")
					pubsub.Close()
					return
				}
				if msg != nil {
					m.handleExpiredKey(msg)
				}
			}
		}
	}()

	return nil
}

// Stop 停止心跳监控服务：关闭 stopCh 触发监听协程退出、取消订阅 ctx，
// 并有界等待协程真正退出后清空运行状态；未启动或已停止时为幂等空操作。
func (m *HeartbeatMonitor) Stop() error {
	m.runMu.Lock()
	stopCh := m.stopCh
	done := m.done
	m.runMu.Unlock()

	if stopCh == nil || done == nil {
		// 保持旧行为兼容：即使从未 Start，也取消构造期创建的订阅 ctx。
		m.cancel()
		return nil
	}

	m.stopOnce.Do(func() { close(stopCh) })
	m.cancel()

	timer := time.NewTimer(heartbeatMonitorStopTimeout)
	defer timer.Stop()
	select {
	case <-done:
		m.logger.Info("HeartbeatMonitor listener exited cleanly")
	case <-timer.C:
		m.logger.Warn("HeartbeatMonitor stop timed out waiting for listener exit")
	}

	m.runMu.Lock()
	m.stopCh = nil
	m.done = nil
	m.runMu.Unlock()
	return nil
}

// configureRedis 配置 Redis 启用过期事件通知
func (m *HeartbeatMonitor) configureRedis() error {
	if m.redis == nil {
		// 无真实客户端时（如生命周期测试的注入场景）跳过服务端配置，
		// 订阅行为由 subscribeExpiry 注入面提供。
		return nil
	}
	// 设置 Redis 配置: notify-keyspace-events Ex
	// E - keyevent 事件, x - 过期事件
	err := m.redis.ConfigSet(m.ctx, "notify-keyspace-events", "Ex").Err()
	if err != nil {
		m.logger.WithError(err).Warn("Failed to set Redis notify-keyspace-events, may already be configured")
		// 不返回错误,可能已经配置过
	}
	return nil
}

// handleExpiredKey 处理过期的 Redis key
func (m *HeartbeatMonitor) handleExpiredKey(msg *redis.Message) {
	// 解析 key: device:{deviceId}:{type}
	if !strings.HasPrefix(msg.Payload, "device:") {
		return
	}

	parts := strings.Split(msg.Payload, ":")
	if len(parts) != 3 {
		return
	}

	keyType := parts[2]
	if keyType != "heartbeat" && keyType != "timeout" {
		return
	}

	deviceID := parts[1]

	m.logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"key_type":  keyType,
		"key":       msg.Payload,
	}).Info("Device heartbeat/timeout expired, marking as offline")

	// 确定离线来源
	source := "heartbeat_expired"
	if keyType == "timeout" {
		source = "timeout_expired"
	}

	// ✨ 通过 StatusPublisher 接口发送离线状态到 Flow Bus → StatusUplink
	// 协议无关设计：无论 MQTT/Kafka 都通过统一的接口处理
	if m.statusPublisher != nil {
		if err := m.statusPublisher.PublishStatusOffline(deviceID, source); err != nil {
			m.logger.WithError(err).WithFields(logrus.Fields{
				"device_id": deviceID,
				"source":    source,
			}).Error("Failed to publish device offline event")
		}
	} else {
		m.logger.Warn("StatusPublisher not available, cannot send offline event")
	}
}
