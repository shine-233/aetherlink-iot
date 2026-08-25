// 文件用途：按 device_id 失效 broker 侧 voucher→device 认证缓存，消除凭证轮换/删除后的认证残窗。
// 核心逻辑：写缓存时同步维护每设备反向索引（SADD）；订阅 backend 发布的失效命令，
//
//	命中后删除该设备索引下的全部缓存键并清空索引。
//
// 关键注意事项：VoucherCacheInvalidationChannel 与
//
//	backend/internal/service/device_voucher_cache_invalidation.go 的同名常量保持一致，
//	任一侧变更必须双端同步并更新两侧契约测试。失效是幂等尽力而为——即使消息丢失，
//	残留映射最多存活 defaultCacheTTL（≤1h）自然过期，因此不设 ACK/重投递协议。
package aetherlink

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/redis.v5"
)

const (
	// VoucherCacheInvalidationChannel 是 backend→broker 的凭证缓存失效命令通道。
	VoucherCacheInvalidationChannel = "aetherlink:device-voucher:cache-invalidate"

	voucherCacheIndexPrefix = "aetherlink:voucher-cache-idx:v1:"
)

func voucherCacheIndexKey(deviceID string) string {
	return voucherCacheIndexPrefix + deviceID
}

// indexVoucherCacheKeyForDevice 在写入 voucher→deviceID 映射的同时登记反向索引，
// 使后续可按 device_id 精确失效。索引 TTL 与缓存本体一致，自然过期不留垃圾。
func indexVoucherCacheKeyForDevice(deviceID, cacheKey string, expiration time.Duration) error {
	if redisCache == nil || strings.TrimSpace(deviceID) == "" || strings.TrimSpace(cacheKey) == "" {
		return nil
	}
	indexKey := voucherCacheIndexKey(deviceID)
	if err := redisCache.SAdd(indexKey, cacheKey).Err(); err != nil {
		return fmt.Errorf("register voucher cache index member: %w", err)
	}
	if err := redisCache.Expire(indexKey, expiration).Err(); err != nil {
		return fmt.Errorf("refresh voucher cache index ttl: %w", err)
	}
	return nil
}

type voucherCacheInvalidationMessage struct {
	Version  int    `json:"version"`
	DeviceID string `json:"device_id"`
}

func parseVoucherCacheInvalidationMessage(payload string) (voucherCacheInvalidationMessage, bool) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return voucherCacheInvalidationMessage{}, false
	}
	var event voucherCacheInvalidationMessage
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return voucherCacheInvalidationMessage{}, false
	}
	event.DeviceID = strings.TrimSpace(event.DeviceID)
	if event.Version != 1 || event.DeviceID == "" {
		return voucherCacheInvalidationMessage{}, false
	}
	return event, true
}

// evictVoucherCacheForDevice 删除该设备登记的全部 voucher 缓存映射与索引本身，
// 返回实际清除的映射数量。幂等：索引不存在时返回 0。
func evictVoucherCacheForDevice(deviceID string) (int, error) {
	if redisCache == nil {
		return 0, fmt.Errorf("redis is not initialized for voucher cache invalidation")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, nil
	}
	indexKey := voucherCacheIndexKey(deviceID)
	members, err := redisCache.SMembers(indexKey).Result()
	if err != nil {
		return 0, fmt.Errorf("read voucher cache index: %w", err)
	}
	evicted := 0
	for _, member := range members {
		member = strings.TrimSpace(member)
		if member == "" {
			continue
		}
		if err := redisCache.Del(member).Err(); err != nil {
			return evicted, fmt.Errorf("evict voucher cache key: %w", err)
		}
		evicted++
	}
	if len(members) > 0 {
		if err := redisCache.Del(indexKey).Err(); err != nil {
			return evicted, fmt.Errorf("clear voucher cache index: %w", err)
		}
	}
	return evicted, nil
}

// ---- Pub/Sub monitor ----
// 复用会话撤销的订阅骨架；无 ACK 协议（失效幂等且 TTL 兜底）。

type voucherCacheInvalidationSubscription interface {
	Messages() <-chan string
	Close() error
}

type voucherCacheInvalidationSubscribe func() (voucherCacheInvalidationSubscription, error)

type voucherCacheInvalidationMonitor struct {
	mu           sync.Mutex
	subscribe    voucherCacheInvalidationSubscribe
	subscription voucherCacheInvalidationSubscription
	stop         chan struct{}
	done         chan struct{}
	started      bool
}

func newVoucherCacheInvalidationMonitor(subscribe voucherCacheInvalidationSubscribe) *voucherCacheInvalidationMonitor {
	return &voucherCacheInvalidationMonitor{subscribe: subscribe}
}

func (m *voucherCacheInvalidationMonitor) Start() error {
	if m == nil {
		return fmt.Errorf("voucher cache invalidation monitor is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	if m.subscribe == nil {
		return fmt.Errorf("voucher cache invalidation monitor is not configured")
	}
	subscription, err := m.subscribe()
	if err != nil {
		return err
	}
	if subscription == nil {
		return fmt.Errorf("voucher cache invalidation subscription is nil")
	}
	m.subscription = subscription
	m.stop = make(chan struct{})
	m.done = make(chan struct{})
	m.started = true
	go m.run(subscription.Messages(), m.stop, m.done)
	return nil
}

func (m *voucherCacheInvalidationMonitor) run(messages <-chan string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case payload, ok := <-messages:
			if !ok {
				return
			}
			event, valid := parseVoucherCacheInvalidationMessage(payload)
			if !valid {
				continue
			}
			evicted, err := evictVoucherCacheForDevice(event.DeviceID)
			if err != nil && Log != nil {
				Log.Warn("voucher cache eviction failed; residual mappings expire with cache TTL",
					zap.String("device_id", event.DeviceID),
					zap.Int("evicted", evicted),
					zap.Error(err),
				)
				continue
			}
			if Log != nil && evicted > 0 {
				Log.Info("voucher cache invalidated for device",
					zap.String("device_id", event.DeviceID),
					zap.Int("evicted", evicted),
				)
			}
		}
	}
}

func (m *voucherCacheInvalidationMonitor) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	subscription := m.subscription
	stop := m.stop
	done := m.done
	m.subscription = nil
	m.stop = nil
	m.done = nil
	m.started = false
	close(stop)
	m.mu.Unlock()

	err := subscription.Close()
	<-done
	return err
}

type redisVoucherCacheInvalidationSubscription struct {
	pubsub    *redis.PubSub
	messages  chan string
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func subscribeRedisVoucherCacheInvalidations() (voucherCacheInvalidationSubscription, error) {
	if redisCache == nil {
		return nil, fmt.Errorf("redis is not initialized for voucher cache invalidation")
	}
	pubsub, err := redisCache.Subscribe(VoucherCacheInvalidationChannel)
	if err != nil {
		return nil, fmt.Errorf("subscribe voucher cache invalidation channel: %w", err)
	}
	confirmation, err := pubsub.ReceiveTimeout(3 * time.Second)
	if err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("confirm voucher cache invalidation subscription: %w", err)
	}
	subscribed, ok := confirmation.(*redis.Subscription)
	if !ok || subscribed.Kind != "subscribe" || subscribed.Channel != VoucherCacheInvalidationChannel {
		_ = pubsub.Close()
		return nil, fmt.Errorf("unexpected voucher cache invalidation subscription confirmation: %T", confirmation)
	}

	subscription := &redisVoucherCacheInvalidationSubscription{
		pubsub:   pubsub,
		messages: make(chan string),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go subscription.forward()
	return subscription, nil
}

func (s *redisVoucherCacheInvalidationSubscription) Messages() <-chan string {
	if s == nil {
		return nil
	}
	return s.messages
}

func (s *redisVoucherCacheInvalidationSubscription) forward() {
	defer close(s.done)
	defer close(s.messages)
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		message, err := s.pubsub.ReceiveMessage()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
			}
			if Log != nil {
				Log.Warn("voucher cache invalidation subscription receive failed", zap.Error(err))
			}
			retry := time.NewTimer(time.Second)
			select {
			case <-s.stop:
				if !retry.Stop() {
					select {
					case <-retry.C:
					default:
					}
				}
				return
			case <-retry.C:
				continue
			}
		}
		if message == nil {
			continue
		}
		select {
		case s.messages <- message.Payload:
		case <-s.stop:
			return
		}
	}
}

func (s *redisVoucherCacheInvalidationSubscription) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		close(s.stop)
		s.closeErr = s.pubsub.Close()
		<-s.done
	})
	return s.closeErr
}
