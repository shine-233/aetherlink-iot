// bus.go 负责定义上行模块内部的消息总线。
// 主要职责：
// 1. 为遥测、属性、事件、状态、响应五类消息提供独立缓冲队列。
// 2. 兼容 adapter 层与 uplink 层消息结构，在发布入口完成轻量转换。
// 3. 为各类 Uplink Flow 提供统一订阅入口，并暴露队列监控信息。
// 关键注意事项：
// 1. Publish 会按消息类型路由到不同 channel，部分分支在缓冲区打满后会退化为阻塞发送，调用方需要接受背压。
// 2. Publish 使用生命周期闸门登记活跃发布；Close 只会在闸门关闭且活跃发布全部退出后关闭 channel。
// 3. 响应消息与普通上行消息的满队列策略不同：响应链路会直接丢弃并返回 ErrChannelFull，而不是阻塞等待。
// 静态审查建议：
// 1. 后续可把“消息兼容转换”“类型路由”“队列写入策略”拆开，减少 Publish 的职责密度。
// 2. 新的队列类型必须复用 beginPublish/abortPublish 协议，不得绕过闸门直接写入 channel。

package uplink

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// MessageType 定义总线支持的消息类型常量。
const (
	MessageTypeTelemetry = "telemetry"
	MessageTypeAttribute = "attribute"
	MessageTypeEvent     = "event"
	MessageTypeStatus    = "status"

	// 下行指令/属性设置回执会复用总线，但进入单独的响应消费链路。
	MessageTypeCommandResponse             = "command_response"
	MessageTypeAttributeSetResponse        = "attribute_set_response"
	MessageTypeGatewayCommandResponse      = "gateway_command_response"
	MessageTypeGatewayAttributeSetResponse = "gateway_attribute_set_response"
)

// Bus 是 Adapter 与各类 Uplink Flow 之间的内存消息总线。
// 它不负责业务处理，只负责分类缓存、路由分发和关闭控制。
type Bus struct {
	// 各队列按消息类型拆分，避免不同消费链路互相阻塞。
	telemetryChan chan *DeviceMessage
	attributeChan chan *DeviceMessage
	eventChan     chan *DeviceMessage
	statusChan    chan *DeviceMessage

	// 响应链路单独排队，避免与遥测等高频数据争抢处理配额。
	responseChan chan *DeviceMessage

	// bufferSize 仅用于建队列和监控展示，不会动态扩缩容。
	bufferSize int

	// mu 保护发布闸门。closing 一旦置位就不再允许 WaitGroup.Add，
	// 因此关闭收尾可以安全等待所有已获准的 Publish 退出。
	mu      sync.Mutex
	closing bool
	closed  bool

	publishers   sync.WaitGroup
	closeOnce    sync.Once
	abortOnce    sync.Once
	abortPublish chan struct{}
	closeDone    chan struct{}

	observerMu     sync.RWMutex
	observerNextID uint64
	observers      map[uint64]*acceptedMessageObserver

	// logger 用于记录满队列、未知类型和关闭态丢弃等运行信号。
	logger *logrus.Logger
}

// BusConfig 定义总线初始化参数。
type BusConfig struct {
	BufferSize int // BufferSize 是每类 channel 的容量，<=0 时回退到默认值 10000。
}

// NewBus 创建并初始化总线。
// 参数说明：
// 1. config.BufferSize 控制每类消息队列容量。
// 2. logger 允许注入上层统一日志器；为空时回退到标准 logger。
func NewBus(config BusConfig, logger *logrus.Logger) *Bus {
	if config.BufferSize <= 0 {
		config.BufferSize = 10000 // 默认缓冲区大小
	}

	if logger == nil {
		logger = logrus.StandardLogger()
	}

	return &Bus{
		telemetryChan: make(chan *DeviceMessage, config.BufferSize),
		attributeChan: make(chan *DeviceMessage, config.BufferSize),
		eventChan:     make(chan *DeviceMessage, config.BufferSize),
		statusChan:    make(chan *DeviceMessage, config.BufferSize),

		// ✨ 新增：响应 channel
		responseChan: make(chan *DeviceMessage, config.BufferSize),

		bufferSize:   config.BufferSize,
		abortPublish: make(chan struct{}),
		closeDone:    make(chan struct{}),
		observers:    make(map[uint64]*acceptedMessageObserver),
		logger:       logger,
	}
}

// AcceptedMessageSubscription is a bounded, read-only observation stream of
// messages that the bus has already admitted. It is fan-out only: consuming it
// never steals work from the telemetry, attribute, event, status or response
// flows. Slow observers drop their own copies instead of backpressuring device
// ingestion.
type AcceptedMessageSubscription struct {
	Messages  <-chan *DeviceMessage
	closeOnce sync.Once
	close     func()
	observer  *acceptedMessageObserver
}

type acceptedMessageObserver struct {
	messages chan *DeviceMessage
	dropped  atomic.Uint64
}

func (subscription *AcceptedMessageSubscription) Close() {
	if subscription == nil {
		return
	}
	subscription.closeOnce.Do(func() {
		if subscription.close != nil {
			subscription.close()
		}
	})
}

func (subscription *AcceptedMessageSubscription) DroppedMessages() uint64 {
	if subscription == nil || subscription.observer == nil {
		return 0
	}
	return subscription.observer.dropped.Load()
}

// SubscribeAcceptedMessages registers a bounded fan-out observer. The stream
// contains defensive message copies and is closed by either subscription.Close
// or Bus.Close. A full observer buffer drops only that observer's next copy.
func (b *Bus) SubscribeAcceptedMessages(buffer int) (*AcceptedMessageSubscription, error) {
	if buffer <= 0 {
		buffer = 256
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closing || b.closed {
		return nil, ErrBusClosed
	}
	b.observerMu.Lock()
	b.observerNextID++
	id := b.observerNextID
	observer := &acceptedMessageObserver{messages: make(chan *DeviceMessage, buffer)}
	b.observers[id] = observer
	b.observerMu.Unlock()
	return &AcceptedMessageSubscription{
		Messages: observer.messages,
		observer: observer,
		close: func() {
			b.observerMu.Lock()
			if current, ok := b.observers[id]; ok {
				delete(b.observers, id)
				close(current.messages)
			}
			b.observerMu.Unlock()
		},
	}, nil
}

// MessageLike 是发布入口接受的宽类型占位，主要用于避免与 adapter 包产生循环导入。
type MessageLike interface{}

// Publish 将上行消息发布到对应分类队列。
// 关键逻辑：
// 1. 若输入不是 *DeviceMessage，会先通过 JSON 编解码做兼容转换。
// 2. 遥测/属性/事件/状态分支在满队列时会阻塞写入，以背压方式保护消息不丢失。
// 3. 响应分支改为显式调用 PublishResponse，由响应链路决定是否允许丢弃。
// 使用注意：
// 1. JSON 转换依赖字段结构兼容，后续若 adapter 与 uplink 结构漂移，这里会先于业务处理暴露问题。
// 2. Publish 先完成兼容转换，再通过生命周期闸门登记；Close 会拒绝新发布，并在关闭 channel 前等待已登记发布退出。
func (b *Bus) Publish(msgInterface MessageLike) error {
	return b.PublishContext(context.Background(), msgInterface)
}

// PublishContext publishes one message while allowing the caller or bus
// shutdown to release a blocked send. A message is accepted only when this
// method returns nil.
func (b *Bus) PublishContext(ctx context.Context, msgInterface MessageLike) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// 将 interface{} 转换为 DeviceMessage
	// 这里依赖运行时的结构体字段兼容性
	var msg *DeviceMessage

	// 通过 JSON 序列化/反序列化实现类型转换
	// adapter.UplinkMessage 和 uplink.DeviceMessage 结构完全一致
	switch v := msgInterface.(type) {
	case *DeviceMessage:
		msg = v
	default:
		// Reject calls that begin after shutdown before invoking arbitrary
		// MessageLike marshaling. The authoritative admission check still runs
		// below so a concurrent Close cannot create a send-after-close window.
		if err := b.checkOpen(); err != nil {
			b.logger.Warn("Bus is closing or closed, message dropped")
			return err
		}

		// 使用 JSON 转换（adapter.UplinkMessage -> uplink.DeviceMessage）
		jsonData, err := json.Marshal(msgInterface)
		if err != nil {
			b.logger.WithError(err).Error("Failed to marshal message")
			return err
		}

		msg = &DeviceMessage{}
		if err := json.Unmarshal(jsonData, msg); err != nil {
			b.logger.WithError(err).Error("Failed to unmarshal message")
			return err
		}
	}

	if msg == nil {
		return ErrInvalidPayload
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Conversion deliberately happens before admission. Closing the bus must not
	// wait for arbitrary MessageLike JSON marshaling; a late conversion is simply
	// rejected here without ever touching a channel.
	if err := b.beginPublish(); err != nil {
		b.logger.Warn("Bus is closing or closed, message dropped")
		return err
	}
	defer b.publishers.Done()

	// Clone for observers before the accepted message becomes visible to its
	// flow consumer, so observer capture cannot race a downstream mutation.
	observerMessage := b.prepareAcceptedMessageObservation(msg)

	// 根据消息类型路由到不同的 channel，同时兼容网关透传类型。
	var publishErr error
	switch msg.Type {
	case MessageTypeTelemetry, "gateway_telemetry":
		publishErr = b.publishWithBackpressure(ctx, b.telemetryChan, msg, "【设备遥测】Telemetry")

	case MessageTypeAttribute, "gateway_attribute":
		publishErr = b.publishWithBackpressure(ctx, b.attributeChan, msg, "【设备属性】Attribute")

	case MessageTypeEvent, "gateway_event":
		publishErr = b.publishWithBackpressure(ctx, b.eventChan, msg, "【设备事件】Event")

	case MessageTypeStatus:
		publishErr = b.publishWithBackpressure(ctx, b.statusChan, msg, "【设备上下线】Status")
		if publishErr == nil {
			b.logger.Debug("【设备上下线】Status message sent to statusChan")
		}

	// 响应消息走独立策略：队列满时直接返回错误，由调用方决定是否重试。
	case MessageTypeCommandResponse,
		MessageTypeAttributeSetResponse,
		MessageTypeGatewayCommandResponse,
		MessageTypeGatewayAttributeSetResponse:
		publishErr = b.publishResponse(ctx, msg)

	default:
		b.logger.Errorf("Unknown message type: %s", msg.Type)
		return ErrUnknownMessageType
	}
	if publishErr == nil {
		b.notifyAcceptedMessage(observerMessage)
	}
	return publishErr
}

// PublishResponse 发布响应消息到独立回执队列。
// 与普通 Publish 不同，这里不会阻塞等待空位，而是把满队列作为显式错误返回。
func (b *Bus) PublishResponse(msg *DeviceMessage) error {
	return b.PublishResponseContext(context.Background(), msg)
}

// PublishResponseContext preserves the response queue's non-blocking policy
// while sharing the same lifecycle gate as every other message type.
func (b *Bus) PublishResponseContext(ctx context.Context, msg *DeviceMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if msg == nil {
		return ErrInvalidPayload
	}
	if err := b.beginPublish(); err != nil {
		b.logger.Warn("Bus is closing or closed, response dropped")
		return err
	}
	defer b.publishers.Done()

	observerMessage := b.prepareAcceptedMessageObservation(msg)
	if err := b.publishResponse(ctx, msg); err != nil {
		return err
	}
	b.notifyAcceptedMessage(observerMessage)
	return nil
}

func (b *Bus) prepareAcceptedMessageObservation(message *DeviceMessage) *DeviceMessage {
	b.observerMu.RLock()
	hasObservers := len(b.observers) > 0
	b.observerMu.RUnlock()
	if !hasObservers || message == nil {
		return nil
	}
	return cloneDeviceMessage(message)
}

func (b *Bus) notifyAcceptedMessage(message *DeviceMessage) {
	if message == nil {
		return
	}
	b.observerMu.RLock()
	defer b.observerMu.RUnlock()
	for _, observer := range b.observers {
		select {
		case observer.messages <- message:
		default:
			observer.dropped.Add(1)
		}
	}
}

func cloneDeviceMessage(message *DeviceMessage) *DeviceMessage {
	cloned := *message
	cloned.Payload = append([]byte(nil), message.Payload...)
	if message.Metadata != nil {
		cloned.Metadata = make(map[string]interface{}, len(message.Metadata))
		for key, value := range message.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return &cloned
}

func (b *Bus) publishWithBackpressure(
	ctx context.Context,
	ch chan *DeviceMessage,
	msg *DeviceMessage,
	queueName string,
) error {
	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.abortPublish:
		return ErrBusClosed
	default:
		b.logger.Warnf("%s channel full, blocking publish", queueName)
	}

	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.abortPublish:
		return ErrBusClosed
	}
}

func (b *Bus) publishResponse(ctx context.Context, msg *DeviceMessage) error {
	select {
	case b.responseChan <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.abortPublish:
		return ErrBusClosed
	default:
		b.logger.WithFields(logrus.Fields{
			"device_id": msg.DeviceID,
			"type":      msg.Type,
		}).Warn("Response channel is full, message dropped")
		return ErrChannelFull
	}
}

// PublishStatusOffline 实现 StatusPublisher 接口，供心跳超时等链路复用。
// source 会写入 Metadata，供 status 处理器区分“设备主动上报”与“系统推断离线”。
func (b *Bus) PublishStatusOffline(deviceID, source string) error {
	return b.PublishStatusOfflineContext(context.Background(), deviceID, source)
}

// PublishStatusOfflineContext is the context-aware status publisher seam used
// by lifecycle-managed producers.
func (b *Bus) PublishStatusOfflineContext(ctx context.Context, deviceID, source string) error {
	msg := &DeviceMessage{
		Type:     MessageTypeStatus,
		DeviceID: deviceID,
		Payload:  []byte("0"), // 0 表示离线
		Metadata: map[string]interface{}{
			"source": source, // 离线来源存储在 Metadata 中
		},
	}

	return b.PublishContext(ctx, msg)
}

// SubscribeTelemetry 返回遥测消费通道。
func (b *Bus) SubscribeTelemetry() <-chan *DeviceMessage {
	return b.telemetryChan
}

// SubscribeAttribute 返回属性消费通道。
func (b *Bus) SubscribeAttribute() <-chan *DeviceMessage {
	return b.attributeChan
}

// SubscribeEvent 返回事件消费通道。
func (b *Bus) SubscribeEvent() <-chan *DeviceMessage {
	return b.eventChan
}

// SubscribeStatus 返回设备上下线状态消费通道。
func (b *Bus) SubscribeStatus() <-chan *DeviceMessage {
	return b.statusChan
}

// SubscribeResponse 返回指令/属性设置回执消费通道。
func (b *Bus) SubscribeResponse() <-chan *DeviceMessage {
	return b.responseChan
}

func (b *Bus) beginPublish() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closing || b.closed {
		return ErrBusClosed
	}

	b.publishers.Add(1)
	return nil
}

// checkOpen rejects calls that begin after shutdown without admitting them or
// invoking a potentially expensive MessageLike conversion. beginPublish still
// performs the authoritative check after conversion to close the race window.
func (b *Bus) checkOpen() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closing || b.closed {
		return ErrBusClosed
	}
	return nil
}

// Close 关闭总线及其所有内部队列。
// 该无参入口保留给现有 defer/t.Cleanup 调用。它会立即中止尚未成功
// 入队的阻塞发布，因此自身不会因满队列永久等待。需要优雅等待时使用 CloseContext。
func (b *Bus) Close() {
	b.startClose()
	b.abortPendingPublishes()
	<-b.closeDone
}

// CloseContext 先关闭发布闸门，再等待所有已获准的 Publish 退出。
// 正常期限内，已获准发布仍可被活跃消费者接收；期限到达后，尚未
// 成功入队的发布会被中止并返回 ErrBusClosed。CloseContext 会立即返回
// ctx.Err()，后台收尾继续等待已获准发布退出，绝不强关仍可能被写入的 channel。
func (b *Bus) CloseContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	b.startClose()

	select {
	case <-b.closeDone:
		return nil
	default:
	}

	select {
	case <-b.closeDone:
		return nil
	case <-ctx.Done():
		b.abortPendingPublishes()
		return ctx.Err()
	}
}

func (b *Bus) startClose() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closing = true
		b.mu.Unlock()

		go b.finishClose()
	})
}

func (b *Bus) abortPendingPublishes() {
	b.abortOnce.Do(func() { close(b.abortPublish) })
}

func (b *Bus) finishClose() {
	b.publishers.Wait()
	b.closeAcceptedMessageObservers()

	close(b.telemetryChan)
	close(b.attributeChan)
	close(b.eventChan)
	close(b.statusChan)
	close(b.responseChan)

	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	close(b.closeDone)
	b.logger.Info("Bus closed")
}

func (b *Bus) closeAcceptedMessageObservers() {
	b.observerMu.Lock()
	for id, observer := range b.observers {
		delete(b.observers, id)
		close(observer.messages)
	}
	b.observerMu.Unlock()
}

// GetChannelStats 返回当前队列长度和容量快照，供监控或诊断背压使用。
func (b *Bus) GetChannelStats() map[string]interface{} {
	return map[string]interface{}{
		"telemetry_len": len(b.telemetryChan),
		"telemetry_cap": cap(b.telemetryChan),
		"attribute_len": len(b.attributeChan),
		"attribute_cap": cap(b.attributeChan),
		"event_len":     len(b.eventChan),
		"event_cap":     cap(b.eventChan),
		"status_len":    len(b.statusChan),
		"status_cap":    cap(b.statusChan),

		// ✨ 新增：响应队列统计
		"response_queue": len(b.responseChan),

		"buffer_size": b.bufferSize,
	}
}

// 错误定义
