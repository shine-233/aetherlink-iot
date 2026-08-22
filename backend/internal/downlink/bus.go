// 文件用途：承载下行消息模块的 bus 逻辑。
// 核心逻辑：定义下行消息类型、发布订阅总线、处理器接口和 MQTT 发布处理流程，主要围绕 type Bus、func NewBus、func (b *Bus) PublishCommand、func (b *Bus) PublishAttributeSet 等声明展开。
// 关键注意事项：下行链路需保持消息类型、topic 构造和发布错误语义兼容。发布与关闭共享生命周期闸门：Close 先置位关闭状态并 close(abortPublish)，等在途发布全部退出后才关闭数据 channel，因此并发 Publish 不会触发 send on closed channel；发布在队列满时只做有限阻塞，超时即丢弃并计数。
// 重构建议：后续可把总线、处理器和发布器边界继续接口化，便于独立压测与替换。

package downlink

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// defaultPublishTimeout 发布在队列满时的有限阻塞上限，超时后丢弃消息，避免调用方永久挂起。
const defaultPublishTimeout = 5 * time.Second

// Bus 下行消息总线
type Bus struct {
	commandChan      chan *Message
	attributeSetChan chan *Message
	attributeGetChan chan *Message
	telemetryChan    chan *Message
	bufferSize       int

	// mu 保护发布闸门状态。closing 一旦置位就不再登记新发布，
	// Close 因此可以安全等待所有已获准的 Publish 退出后再关闭数据 channel。
	mu      sync.Mutex
	closing bool
	closed  bool

	// publishers 登记在途发布；abortPublish 关闭后所有阻塞中的发布会立刻退出。
	publishers   sync.WaitGroup
	closeOnce    sync.Once
	abortPublish chan struct{}

	// wg 等待 Start 启动的消费协程退出。
	wg sync.WaitGroup

	// timeout 是单次发布的有限阻塞上限；<=0 时回退到 defaultPublishTimeout。
	// 仅供测试注入短超时使用，正常调用方无需修改。
	timeout time.Duration

	dropped atomic.Uint64

	logger *logrus.Logger
}

// NewBus 创建消息总线
func NewBus(bufferSize int) *Bus {
	return &Bus{
		commandChan:      make(chan *Message, bufferSize),
		attributeSetChan: make(chan *Message, bufferSize),
		attributeGetChan: make(chan *Message, bufferSize),
		telemetryChan:    make(chan *Message, bufferSize),
		bufferSize:       bufferSize,
		abortPublish:     make(chan struct{}),
		logger:           logrus.StandardLogger(),
	}
}

// PublishCommand 发布命令下发消息。
// 总线关闭中或已关闭时被闸门直接拒绝并计数；队列满时最多阻塞一个发布超时周期，超时即丢弃，不会永久挂起。
func (b *Bus) PublishCommand(msg *Message) {
	b.publish(b.commandChan, msg, "command")
}

// PublishAttributeSet 发布属性设置消息。
// 关闭与满队列语义同 PublishCommand。
func (b *Bus) PublishAttributeSet(msg *Message) {
	b.publish(b.attributeSetChan, msg, "attribute_set")
}

// PublishAttributeGet 发布属性获取消息。
// 关闭与满队列语义同 PublishCommand。
func (b *Bus) PublishAttributeGet(msg *Message) {
	b.publish(b.attributeGetChan, msg, "attribute_get")
}

// PublishTelemetry 发布遥测下发消息。
// 关闭与满队列语义同 PublishCommand。
func (b *Bus) PublishTelemetry(msg *Message) {
	b.publish(b.telemetryChan, msg, "telemetry")
}

// publish 是各发布入口共用的闸门登记与背压写入逻辑。
func (b *Bus) publish(ch chan *Message, msg *Message, queueName string) {
	if !b.beginPublish() {
		b.recordDrop(queueName, "bus closing or closed")
		return
	}
	defer b.publishers.Done()

	if msg == nil {
		b.recordDrop(queueName, "nil message")
		return
	}

	b.publishWithBackpressure(ch, msg, queueName)
}

// beginPublish 在关闭状态下拒绝新的发布，否则登记一次在途发布。
func (b *Bus) beginPublish() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closing || b.closed {
		return false
	}

	b.publishers.Add(1)
	return true
}

// publishWithBackpressure 把消息写入目标队列：先尝试非阻塞发送，
// 队列满时记录 warn 并进入有限阻塞，超时或发布闸门关闭则丢弃并计数。
func (b *Bus) publishWithBackpressure(ch chan *Message, msg *Message, queueName string) {
	timeout := b.publishTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case ch <- msg:
		return
	case <-timer.C:
		b.recordDrop(queueName, "publish timeout")
		return
	case <-b.abortPublish:
		b.recordDrop(queueName, "bus closing or closed")
		return
	default:
		b.logger.WithField("module", "downlink").Warnf("%s queue full, blocking publish", queueName)
	}

	select {
	case ch <- msg:
	case <-timer.C:
		b.recordDrop(queueName, "publish timeout")
	case <-b.abortPublish:
		b.recordDrop(queueName, "bus closing or closed")
	}
}

func (b *Bus) publishTimeout() time.Duration {
	if b.timeout > 0 {
		return b.timeout
	}
	return defaultPublishTimeout
}

// recordDrop 记录一条因闸门拒绝、超时或空消息而未入队的消息。
func (b *Bus) recordDrop(queueName, reason string) {
	total := b.dropped.Add(1)
	b.logger.WithFields(logrus.Fields{
		"module": "downlink",
		"queue":  queueName,
		"reason": reason,
	}).Warnf("downlink message dropped, total=%d", total)
}

// DroppedMessages 返回累计被丢弃（未成功入队）的消息总数，供监控与测试观测背压。
func (b *Bus) DroppedMessages() uint64 {
	return b.dropped.Load()
}

// SubscribeCommand 订阅命令消息
func (b *Bus) SubscribeCommand() <-chan *Message {
	return b.commandChan
}

// SubscribeAttributeSet 订阅属性设置消息
func (b *Bus) SubscribeAttributeSet() <-chan *Message {
	return b.attributeSetChan
}

// SubscribeAttributeGet 订阅属性获取消息
func (b *Bus) SubscribeAttributeGet() <-chan *Message {
	return b.attributeGetChan
}

// SubscribeTelemetry 订阅遥测下发消息
func (b *Bus) SubscribeTelemetry() <-chan *Message {
	return b.telemetryChan
}

// Close 关闭总线。
// 顺序：先置位关闭状态并 close(abortPublish) 打开发布闸门 → 等待在途发布全部退出
// → 此时已无生产者，再关闭数据 channel（不会触发 send on closed channel）
// → 消费协程通过 comma-ok 感知 channel 关闭后退出，wg.Wait 收尾。
// 可安全重复调用。注意：Close 会等待消费协程结束，若消费处理依赖外部 ctx 取消，
// 调用方应先取消 Start 传入的 ctx（应用编排层 Stop 已按此顺序执行）。
func (b *Bus) Close() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closing = true
		b.mu.Unlock()

		close(b.abortPublish)
		b.publishers.Wait()

		close(b.commandChan)
		close(b.attributeSetChan)
		close(b.attributeGetChan)
		close(b.telemetryChan)

		b.wg.Wait()

		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
	})
}

// Start 启动总线（与 Handler 配合使用）
func (b *Bus) Start(ctx context.Context, handler *Handler) {
	// 启动命令处理协程
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-b.commandChan:
				if !ok {
					// 数据 channel 已被 Close 关闭，退出循环，避免收到 nil 后空转死循环。
					return
				}
				if msg != nil {
					handler.HandleCommand(ctx, msg)
				}
			}
		}
	}()

	// 启动属性设置处理协程
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-b.attributeSetChan:
				if !ok {
					return
				}
				if msg != nil {
					handler.HandleAttributeSet(ctx, msg)
				}
			}
		}
	}()

	// 启动属性获取处理协程
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-b.attributeGetChan:
				if !ok {
					return
				}
				if msg != nil {
					handler.HandleAttributeGet(ctx, msg)
				}
			}
		}
	}()

	// 启动遥测下发处理协程
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-b.telemetryChan:
				if !ok {
					return
				}
				if msg != nil {
					handler.HandleTelemetry(ctx, msg)
				}
			}
		}
	}()
}
