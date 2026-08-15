// 文件用途：承载下行消息模块的 bus 逻辑。
// 核心逻辑：定义下行消息类型、发布订阅总线、处理器接口和 MQTT 发布处理流程，主要围绕 type Bus、func NewBus、func (b *Bus) PublishCommand、func (b *Bus) PublishAttributeSet 等声明展开。
// 关键注意事项：下行链路需保持消息类型、topic 构造和发布错误语义兼容。
// 重构建议：后续可把总线、处理器和发布器边界继续接口化，便于独立压测与替换。

package downlink

import (
	"context"
	"sync"
)

// Bus 下行消息总线
type Bus struct {
	commandChan      chan *Message
	attributeSetChan chan *Message
	attributeGetChan chan *Message
	telemetryChan    chan *Message
	bufferSize       int
	wg               sync.WaitGroup
}

// NewBus 创建消息总线
func NewBus(bufferSize int) *Bus {
	return &Bus{
		commandChan:      make(chan *Message, bufferSize),
		attributeSetChan: make(chan *Message, bufferSize),
		attributeGetChan: make(chan *Message, bufferSize),
		telemetryChan:    make(chan *Message, bufferSize),
		bufferSize:       bufferSize,
	}
}

// PublishCommand 发布命令下发消息
func (b *Bus) PublishCommand(msg *Message) {
	b.commandChan <- msg
}

// PublishAttributeSet 发布属性设置消息
func (b *Bus) PublishAttributeSet(msg *Message) {
	b.attributeSetChan <- msg
}

// PublishAttributeGet 发布属性获取消息
func (b *Bus) PublishAttributeGet(msg *Message) {
	b.attributeGetChan <- msg
}

// PublishTelemetry 发布遥测下发消息
func (b *Bus) PublishTelemetry(msg *Message) {
	b.telemetryChan <- msg
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

// Close 关闭总线
func (b *Bus) Close() {
	close(b.commandChan)
	close(b.attributeSetChan)
	close(b.attributeGetChan)
	close(b.telemetryChan)
	b.wg.Wait()
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
			case msg := <-b.commandChan:
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
			case msg := <-b.attributeSetChan:
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
			case msg := <-b.attributeGetChan:
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
			case msg := <-b.telemetryChan:
				if msg != nil {
					handler.HandleTelemetry(ctx, msg)
				}
			}
		}
	}()
}
