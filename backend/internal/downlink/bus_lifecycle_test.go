// 文件用途：覆盖下行消息总线并发生命周期行为的 Go 测试。
// 核心逻辑：验证发布与关闭的竞争安全、满队列有限阻塞丢弃和关闭后闸门拒绝，主要围绕 func TestPublishCloseRaceDoesNotPanic、func TestPublishDropsWhenConsumerStuck 等声明展开。
// 关键注意事项：并发场景必须能在 go test -race 下稳定通过，且不允许出现永久阻塞。
// 重构建议：后续可补充消费协程在 ctx 取消与 channel 关闭双路径下的退出时序断言。

package downlink

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPublishCloseRaceDoesNotPanic(t *testing.T) {
	bus := NewBus(4)

	// 消费端不取消 ctx，验证仅靠 Close 也能让 Start 的消费协程退出。
	handler := NewHandler(&mockPublisher{}, &mockProcessor{}, newHandlerTestLogger())
	bus.Start(context.Background(), handler)

	var producers sync.WaitGroup
	for i := 0; i < 16; i++ {
		producers.Add(1)
		go func(n int) {
			defer producers.Done()
			for j := 0; j < 200; j++ {
				msg := &Message{DeviceID: fmt.Sprintf("dev-%d-%d", n, j)}
				switch j % 4 {
				case 0:
					msg.Type = MessageTypeCommand
					bus.PublishCommand(msg)
				case 1:
					msg.Type = MessageTypeAttributeSet
					bus.PublishAttributeSet(msg)
				case 2:
					msg.Type = MessageTypeAttributeGet
					bus.PublishAttributeGet(msg)
				default:
					msg.Type = MessageTypeTelemetry
					bus.PublishTelemetry(msg)
				}
			}
		}(i)
	}

	// 与生产者并发触发两次 Close，验证幂等且不会 panic。
	var closers sync.WaitGroup
	closers.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer closers.Done()
			bus.Close()
		}()
	}

	closers.Wait()
	producers.Wait()
	bus.Close()

	if bus.DroppedMessages() == 0 {
		t.Fatal("expected some publishes to be rejected or dropped during close race")
	}
}

func TestPublishDropsWhenConsumerStuck(t *testing.T) {
	bus := NewBus(1)
	bus.timeout = 80 * time.Millisecond

	filler := &Message{DeviceID: "dev-1", Type: MessageTypeCommand, Data: []byte(`{"cmd":"reset"}`)}
	bus.PublishCommand(filler) // 占满容量为 1 的队列，之后无人消费

	stuck := &Message{DeviceID: "dev-2", Type: MessageTypeCommand, Data: []byte(`{"cmd":"ping"}`)}

	done := make(chan struct{})
	started := time.Now()
	go func() {
		defer close(done)
		bus.PublishCommand(stuck)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("publish did not return after timeout, blocking forever")
	}

	elapsed := time.Since(started)
	if elapsed < 70*time.Millisecond {
		t.Fatalf("publish returned before timeout elapsed: %v", elapsed)
	}
	if got := bus.DroppedMessages(); got != 1 {
		t.Fatalf("expected exactly 1 dropped message, got %d", got)
	}
}

func TestPublishAfterCloseRejectedByGate(t *testing.T) {
	bus := NewBus(1)
	bus.Close()
	bus.Close() // 重复 Close 必须幂等

	before := bus.DroppedMessages()
	msg := &Message{DeviceID: "dev-1", Type: MessageTypeTelemetry, Data: []byte(`{"interval":30}`)}

	done := make(chan struct{})
	go func() {
		defer close(done)
		bus.PublishCommand(msg)
		bus.PublishAttributeSet(msg)
		bus.PublishAttributeGet(msg)
		bus.PublishTelemetry(msg)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("publish after close did not return, gate failed to reject")
	}

	if got := bus.DroppedMessages() - before; got != 4 {
		t.Fatalf("expected 4 gate-rejected drops, got %d", got)
	}

	// 已有消息的读取语义保持：关闭后应读到 nil 且 ok=false。
	if msg, ok := <-bus.SubscribeCommand(); ok || msg != nil {
		t.Fatalf("command channel should be closed, got msg=%#v ok=%v", msg, ok)
	}
}

func TestClosePreservesBufferedMessagesThenClosesChannels(t *testing.T) {
	bus := NewBus(4)

	first := &Message{DeviceID: "dev-1", Type: MessageTypeCommand, Data: []byte(`{"cmd":"a"}`)}
	second := &Message{DeviceID: "dev-2", Type: MessageTypeCommand, Data: []byte(`{"cmd":"b"}`)}
	bus.PublishCommand(first)
	bus.PublishCommand(second)
	bus.Close()

	if got := <-bus.SubscribeCommand(); got != first {
		t.Fatalf("first buffered message mismatch: got %#v want %#v", got, first)
	}
	if got := <-bus.SubscribeCommand(); got != second {
		t.Fatalf("second buffered message mismatch: got %#v want %#v", got, second)
	}
	if got, ok := <-bus.SubscribeCommand(); ok || got != nil {
		t.Fatalf("channel should report closed, got msg=%#v ok=%v", got, ok)
	}
}
