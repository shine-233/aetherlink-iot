// 文件用途：Casbin 集群策略同步 watcher（ROADMAP C7+）——基于 Redis Pub/Sub 的最小
// persist.Watcher 实现，多实例部署时任一实例的策略变更即时通知其余实例重载。
// 核心逻辑：发布侧 Enforcer 变更后经 watcher.Update() 发布自身实例 ID；订阅侧收到非自身
// ID 的消息即触发回调（Enforcer.SetWatcher 默认回调 = LoadPolicy）。
// 关键注意事项：
//  - 自通知跳过：本实例变更已同步生效，重复 LoadPolicy 徒增开销（官方 redis-watcher 同语义）；
//  - 全量重载语义：不解析增量类型（WatcherEx），每次通知全量 LoadPolicy——策略规模
//    （数百行 p + 数百行 g）下开销可忽略，换取实现最小化与强一致；
//  - 订阅断线由 go-redis 自动重连并重放订阅；断线窗口内的变更在下一次任意实例变更时收敛；
//  - Redis 不可用时 Update() 返回错误：变更已落库，仅同步延迟，重启/恢复后自动收敛。
package casbinwatcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultChannel 默认 Pub/Sub 频道。
const DefaultChannel = "aetherlink:casbin:policy-updates"

// Options watcher 构造选项。
type Options struct {
	Channel string
}

// Option watcher 构造函数式选项。
type Option func(*Options)

// WithChannel 自定义 Pub/Sub 频道（多套环境共用一个 Redis 时隔离用）。
func WithChannel(ch string) Option {
	return func(o *Options) { o.Channel = ch }
}

// Watcher Redis Pub/Sub 实现的 casbin persist.Watcher。
type Watcher struct {
	client  *redis.Client
	channel string
	localID string

	ctx    context.Context
	cancel context.CancelFunc
	pubsub *redis.PubSub

	mu       sync.Mutex
	callback func(string)
	closed   bool
}

// New 构造并启动订阅循环（client 必填；订阅确认在返回前完成）。
func New(client *redis.Client, opts ...Option) (*Watcher, error) {
	if client == nil {
		return nil, fmt.Errorf("casbinwatcher: redis client 必填")
	}
	o := Options{Channel: DefaultChannel}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Channel == "" {
		o.Channel = DefaultChannel
	}

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("casbinwatcher: 生成实例 ID 失败: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		client:  client,
		channel: o.Channel,
		localID: hex.EncodeToString(id),
		ctx:     ctx,
		cancel:  cancel,
	}

	pubsub := client.Subscribe(ctx, w.channel)
	// 订阅确认：3s 内未收到确认视为 Redis 不可达，fail-fast 暴露给装配方。
	if err := pubsub.Subscribe(ctx, w.channel); err != nil {
		cancel()
		_ = pubsub.Close()
		return nil, fmt.Errorf("casbinwatcher: 订阅 %s 失败: %w", w.channel, err)
	}
	recvCtx, recvCancel := context.WithTimeout(ctx, 3*time.Second)
	_, err := pubsub.Receive(recvCtx)
	recvCancel()
	if err != nil {
		cancel()
		_ = pubsub.Close()
		return nil, fmt.Errorf("casbinwatcher: 订阅确认超时: %w", err)
	}
	w.pubsub = pubsub

	go w.listen()
	return w, nil
}

// listen 订阅循环：非自身消息分发回调；通道关闭即退出。
func (w *Watcher) listen() {
	msgCh := w.pubsub.Channel()
	for {
		select {
		case <-w.ctx.Done():
			return
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			if msg.Payload == w.localID {
				continue // 自通知跳过
			}
			w.mu.Lock()
			cb := w.callback
			w.mu.Unlock()
			if cb != nil {
				cb(msg.Payload)
			}
		}
	}
}

// Channel 返回订阅频道（观测/日志用）。
func (w *Watcher) Channel() string { return w.channel }

// SetUpdateCallback 设置跨实例变更回调（casbin SetWatcher 后覆盖默认回调）。
func (w *Watcher) SetUpdateCallback(fn func(string)) error {
	w.mu.Lock()
	w.callback = fn
	w.mu.Unlock()
	return nil
}

// Update 向集群广播本实例策略已变更（enforcer 变更方法自动调用）。
func (w *Watcher) Update() error {
	w.mu.Lock()
	closed := w.closed
	w.mu.Unlock()
	if closed {
		return nil // 已关闭的 watcher 静默跳过（关停路径不再阻塞业务变更）
	}
	return w.client.Publish(context.Background(), w.channel, w.localID).Err()
}

// Close 释放订阅（幂等）。
func (w *Watcher) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.mu.Unlock()
	w.cancel()
	_ = w.pubsub.Close()
}
