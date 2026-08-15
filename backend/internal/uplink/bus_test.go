// 文件用途：覆盖上行消息模块 bus 行为的 Go 测试。
// 核心逻辑：验证总线分发、状态/响应解析和错误边界等上行处理契约，主要围绕 func newTestBus、func TestBusPublishesMessagesToTypeSpecificChannels、func TestBusPublishesResponseMessagesToResponseChannel、func TestBusConvertsMessageLikePayloadAndPublishesStatusOffline 等声明展开。
// 关键注意事项：测试需关注异步通道容量、关闭状态和消息类型路由，避免竞态断言。
// 重构建议：后续可补充网关子设备、自动化触发和存储失败的场景化用例。

package uplink

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type busTestLogHook struct {
	needle string
	fired  chan struct{}
	once   sync.Once
}

func (h *busTestLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *busTestLogHook) Fire(entry *logrus.Entry) error {
	if strings.Contains(entry.Message, h.needle) {
		h.once.Do(func() { close(h.fired) })
	}
	return nil
}

type blockingJSONMessage struct {
	started chan struct{}
	release chan struct{}
}

func (m *blockingJSONMessage) MarshalJSON() ([]byte, error) {
	close(m.started)
	<-m.release
	return []byte(`{"Type":"telemetry","DeviceID":"dev-blocked"}`), nil
}

type marshalProbeMessage struct {
	called *bool
}

func (m *marshalProbeMessage) MarshalJSON() ([]byte, error) {
	*m.called = true
	return []byte(`{"Type":"telemetry","DeviceID":"dev-probe"}`), nil
}

func newTestBus(bufferSize int) *Bus {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	return NewBus(BusConfig{BufferSize: bufferSize}, logger)
}

func TestBusAcceptedMessageObserverDoesNotStealOrShareMutablePayload(t *testing.T) {
	bus := newTestBus(2)
	t.Cleanup(bus.Close)
	subscription, err := bus.SubscribeAcceptedMessages(2)
	if err != nil {
		t.Fatalf("subscribe accepted messages: %v", err)
	}
	t.Cleanup(subscription.Close)

	original := &DeviceMessage{
		Type:     MessageTypeTelemetry,
		DeviceID: "device-1",
		TenantID: "tenant-1",
		Payload:  []byte(`{"temperature":20}`),
		Metadata: map[string]interface{}{"topic": "devices/telemetry"},
	}
	if err := bus.Publish(original); err != nil {
		t.Fatalf("publish: %v", err)
	}
	flowMessage := <-bus.SubscribeTelemetry()
	observedMessage := <-subscription.Messages
	if flowMessage != original {
		t.Fatal("flow consumer should still receive the original accepted message")
	}
	if observedMessage == original {
		t.Fatal("observer must receive a defensive message copy")
	}
	observedMessage.Payload[0] = 'X'
	if string(original.Payload) != `{"temperature":20}` {
		t.Fatalf("observer mutated flow payload: %q", original.Payload)
	}
}

func TestBusPublishesMessagesToTypeSpecificChannels(t *testing.T) {
	bus := newTestBus(2)
	t.Cleanup(bus.Close)

	tests := []struct {
		name      string
		msg       *DeviceMessage
		subscribe func() <-chan *DeviceMessage
	}{
		{
			name:      "telemetry",
			msg:       &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"},
			subscribe: bus.SubscribeTelemetry,
		},
		{
			name:      "gateway telemetry",
			msg:       &DeviceMessage{Type: "gateway_telemetry", DeviceID: "gateway-1"},
			subscribe: bus.SubscribeTelemetry,
		},
		{
			name:      "attribute",
			msg:       &DeviceMessage{Type: MessageTypeAttribute, DeviceID: "dev-1"},
			subscribe: bus.SubscribeAttribute,
		},
		{
			name:      "event",
			msg:       &DeviceMessage{Type: MessageTypeEvent, DeviceID: "dev-1"},
			subscribe: bus.SubscribeEvent,
		},
		{
			name:      "status",
			msg:       &DeviceMessage{Type: MessageTypeStatus, DeviceID: "dev-1"},
			subscribe: bus.SubscribeStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := bus.Publish(tt.msg); err != nil {
				t.Fatalf("Publish() error = %v", err)
			}
			if got := <-tt.subscribe(); got != tt.msg {
				t.Fatalf("routed message mismatch: got %#v want %#v", got, tt.msg)
			}
		})
	}
}

func TestBusPublishesResponseMessagesToResponseChannel(t *testing.T) {
	bus := newTestBus(2)
	t.Cleanup(bus.Close)

	msg := &DeviceMessage{Type: MessageTypeGatewayAttributeSetResponse, DeviceID: "gateway-1"}
	if err := bus.Publish(msg); err != nil {
		t.Fatalf("Publish() response error = %v", err)
	}
	if got := <-bus.SubscribeResponse(); got != msg {
		t.Fatalf("response route mismatch: got %#v want %#v", got, msg)
	}
}

func TestBusConvertsMessageLikePayloadAndPublishesStatusOffline(t *testing.T) {
	bus := newTestBus(2)
	t.Cleanup(bus.Close)

	messageLike := struct {
		Type     string
		DeviceID string
		Payload  []byte
		Metadata map[string]interface{}
	}{
		Type:     MessageTypeAttribute,
		DeviceID: "dev-1",
		Payload:  []byte(`{"mode":"auto"}`),
		Metadata: map[string]interface{}{"source": "adapter"},
	}

	if err := bus.Publish(messageLike); err != nil {
		t.Fatalf("Publish() message-like error = %v", err)
	}
	if got := <-bus.SubscribeAttribute(); got.DeviceID != "dev-1" || string(got.Payload) != `{"mode":"auto"}` {
		t.Fatalf("message-like conversion mismatch: %#v", got)
	}

	if err := bus.PublishStatusOffline("dev-2", "heartbeat-timeout"); err != nil {
		t.Fatalf("PublishStatusOffline() error = %v", err)
	}
	status := <-bus.SubscribeStatus()
	if status.DeviceID != "dev-2" || string(status.Payload) != "0" || status.Metadata["source"] != "heartbeat-timeout" {
		t.Fatalf("offline status message mismatch: %#v", status)
	}
}

func TestBusErrorsForUnknownTypeFullResponseChannelAndClosedBus(t *testing.T) {
	bus := newTestBus(1)

	if err := bus.Publish(&DeviceMessage{Type: "mystery", DeviceID: "dev-1"}); !errors.Is(err, ErrUnknownMessageType) {
		t.Fatalf("unknown type error mismatch: %v", err)
	}

	if err := bus.Publish(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-1"}); err != nil {
		t.Fatalf("first response publish should fit buffer: %v", err)
	}
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-2"}); !errors.Is(err, ErrChannelFull) {
		t.Fatalf("full response channel error mismatch: %v", err)
	}

	bus.Close()
	bus.Close()
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("closed bus error mismatch: %v", err)
	}
	marshalCalled := false
	if err := bus.Publish(&marshalProbeMessage{called: &marshalCalled}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("message-like Publish() after Close error = %v, want ErrBusClosed", err)
	}
	if marshalCalled {
		t.Fatal("Publish() converted a new message after the bus was closed")
	}
}

func TestBusCloseUnblocksPublisherWaitingOnFullQueue(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	fullQueue := make(chan struct{})
	logger.AddHook(&busTestLogHook{
		needle: "Telemetry channel full, blocking publish",
		fired:  fullQueue,
	})
	bus := NewBus(BusConfig{BufferSize: 1}, logger)
	t.Cleanup(bus.Close)

	first := &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}
	if err := bus.Publish(first); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}

	publishDone := make(chan error, 1)
	go func() {
		publishDone <- bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-2"})
	}()

	select {
	case <-fullQueue:
	case <-time.After(time.Second):
		t.Fatal("second publisher did not block on the full telemetry queue")
	}

	closeDone := make(chan struct{})
	go func() {
		bus.Close()
		close(closeDone)
	}()

	select {
	case err := <-publishDone:
		if !errors.Is(err, ErrBusClosed) {
			t.Fatalf("blocked Publish() error = %v, want ErrBusClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked publisher was not released by Close")
	}

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Close did not finish after the admitted publisher exited")
	}

	telemetry := bus.SubscribeTelemetry()
	if got, ok := <-telemetry; !ok || got != first {
		t.Fatalf("buffered message mismatch after close: got %#v, open=%v", got, ok)
	}
	if _, ok := <-telemetry; ok {
		t.Fatal("telemetry channel remained open after Close")
	}
}

func TestBusConcurrentAndRepeatedCloseIsIdempotent(t *testing.T) {
	bus := newTestBus(1)
	t.Cleanup(bus.Close)

	const closerCount = 16
	var closers sync.WaitGroup
	closers.Add(closerCount)
	for i := 0; i < closerCount; i++ {
		go func() {
			defer closers.Done()
			bus.Close()
		}()
	}

	allClosed := make(chan struct{})
	go func() {
		closers.Wait()
		close(allClosed)
	}()
	select {
	case <-allClosed:
	case <-time.After(time.Second):
		t.Fatal("concurrent Close calls did not converge")
	}

	bus.Close()
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeEvent, DeviceID: "dev-1"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("Publish() after Close error = %v, want ErrBusClosed", err)
	}
	if err := bus.PublishResponse(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-1"}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("PublishResponse() after Close error = %v, want ErrBusClosed", err)
	}
}

func TestBusCloseDoesNotWaitForUnadmittedMessageConversion(t *testing.T) {
	bus := newTestBus(1)
	started := make(chan struct{})
	release := make(chan struct{})
	publishDone := make(chan error, 1)
	var releaseOnce sync.Once
	releasePublisher := func() {
		releaseOnce.Do(func() { close(release) })
	}
	t.Cleanup(func() {
		releasePublisher()
		bus.Close()
	})

	go func() {
		publishDone <- bus.Publish(&blockingJSONMessage{started: started, release: release})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("publisher did not enter message conversion")
	}

	closed := make(chan struct{})
	go func() {
		bus.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close waited for a publisher that had not entered the lifecycle gate")
	}

	releasePublisher()
	select {
	case err := <-publishDone:
		if !errors.Is(err, ErrBusClosed) {
			t.Fatalf("late-converted Publish() error = %v, want ErrBusClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("late-converted publisher did not exit after release")
	}
}

func TestBusCloseContextLetsAdmittedPublisherFinishBeforeClosingChannels(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	fullQueue := make(chan struct{})
	logger.AddHook(&busTestLogHook{
		needle: "Telemetry channel full, blocking publish",
		fired:  fullQueue,
	})
	bus := NewBus(BusConfig{BufferSize: 1}, logger)
	t.Cleanup(bus.Close)

	first := &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}
	second := &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-2"}
	if err := bus.Publish(first); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}

	publishDone := make(chan error, 1)
	go func() {
		publishDone <- bus.PublishContext(context.Background(), second)
	}()
	select {
	case <-fullQueue:
	case <-time.After(time.Second):
		t.Fatal("second publisher did not block on the full telemetry queue")
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- bus.CloseContext(closeCtx)
	}()

	select {
	case err := <-closeDone:
		t.Fatalf("CloseContext returned before the admitted publish completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	telemetry := bus.SubscribeTelemetry()
	if got := <-telemetry; got != first {
		t.Fatalf("first buffered message = %#v, want %#v", got, first)
	}
	if err := <-publishDone; err != nil {
		t.Fatalf("admitted PublishContext() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("CloseContext() error = %v", err)
	}
	if got, ok := <-telemetry; !ok || got != second {
		t.Fatalf("second buffered message = (%#v, %v), want accepted message", got, ok)
	}
	if _, ok := <-telemetry; ok {
		t.Fatal("telemetry channel remained open after graceful close")
	}
}

func TestBusCloseContextTimeoutAbortsBlockedUnacceptedPublisher(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	fullQueue := make(chan struct{})
	logger.AddHook(&busTestLogHook{
		needle: "Telemetry channel full, blocking publish",
		fired:  fullQueue,
	})
	bus := NewBus(BusConfig{BufferSize: 1}, logger)
	t.Cleanup(bus.Close)

	first := &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}
	if err := bus.Publish(first); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}

	publishDone := make(chan error, 1)
	go func() {
		publishDone <- bus.PublishContext(
			context.Background(),
			&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-2"},
		)
	}()
	select {
	case <-fullQueue:
	case <-time.After(time.Second):
		t.Fatal("second publisher did not block on the full telemetry queue")
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := bus.CloseContext(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CloseContext() error = %v, want context deadline exceeded", err)
	}
	select {
	case err := <-publishDone:
		if !errors.Is(err, ErrBusClosed) {
			t.Fatalf("aborted PublishContext() error = %v, want ErrBusClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked publisher did not exit after close timeout abort")
	}
	bus.Close()

	telemetry := bus.SubscribeTelemetry()
	if got, ok := <-telemetry; !ok || got != first {
		t.Fatalf("accepted message after forced close = (%#v, %v), want first message", got, ok)
	}
	if _, ok := <-telemetry; ok {
		t.Fatal("telemetry channel remained open after forced close")
	}
}

func TestBusPublishContextHonorsAlreadyCanceledContext(t *testing.T) {
	bus := newTestBus(1)
	t.Cleanup(bus.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := bus.PublishContext(
		ctx,
		&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-canceled"},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("PublishContext() error = %v, want context canceled", err)
	}
	if got := len(bus.SubscribeTelemetry()); got != 0 {
		t.Fatalf("telemetry queue length = %d, want 0", got)
	}
}

func TestBusStatsReflectQueueLengthsAndCapacities(t *testing.T) {
	bus := newTestBus(3)
	t.Cleanup(bus.Close)

	if err := bus.Publish(&DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-1"}); err != nil {
		t.Fatalf("Publish telemetry: %v", err)
	}
	if err := bus.Publish(&DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "dev-1"}); err != nil {
		t.Fatalf("Publish response: %v", err)
	}

	stats := bus.GetChannelStats()
	if stats["telemetry_len"] != 1 || stats["telemetry_cap"] != 3 || stats["response_queue"] != 1 || stats["buffer_size"] != 3 {
		t.Fatalf("unexpected channel stats: %#v", stats)
	}
}

func TestBusBeginPublishAndCloseInterleaveWithoutLateAdmission(t *testing.T) {
	bus := newTestBus(1)
	if err := bus.beginPublish(); err != nil {
		t.Fatalf("seed beginPublish() error = %v", err)
	}

	var releaseSeedOnce sync.Once
	releaseSeed := func() {
		releaseSeedOnce.Do(func() { bus.publishers.Done() })
	}
	releaseRacers := make(chan struct{})
	var releaseRacersOnce sync.Once
	releaseConcurrentPublishers := func() {
		releaseRacersOnce.Do(func() { close(releaseRacers) })
	}
	t.Cleanup(func() {
		releaseConcurrentPublishers()
		releaseSeed()
		bus.Close()
	})

	const publisherCount = 128
	start := make(chan struct{})
	results := make(chan error, publisherCount)
	var publishers sync.WaitGroup
	publishers.Add(publisherCount)
	for i := 0; i < publisherCount; i++ {
		go func() {
			defer publishers.Done()
			<-start
			err := bus.beginPublish()
			results <- err
			if err == nil {
				<-releaseRacers
				bus.publishers.Done()
			}
		}()
	}

	closeStarted := make(chan struct{})
	closeDone := make(chan struct{})
	go func() {
		<-start
		bus.startClose()
		close(closeStarted)
		bus.Close()
		close(closeDone)
	}()

	close(start)
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("Close did not raise the lifecycle gate")
	}

	for i := 0; i < publisherCount; i++ {
		select {
		case err := <-results:
			if err != nil && !errors.Is(err, ErrBusClosed) {
				t.Fatalf("concurrent beginPublish() error = %v, want nil or ErrBusClosed", err)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent beginPublish calls did not all resolve admission")
		}
	}

	if err := bus.beginPublish(); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("beginPublish() after close gate error = %v, want ErrBusClosed", err)
	}
	select {
	case <-closeDone:
		t.Fatal("Close returned while an admitted publisher was still active")
	default:
	}

	releaseConcurrentPublishers()
	releaseSeed()
	publishersDone := make(chan struct{})
	go func() {
		publishers.Wait()
		close(publishersDone)
	}()
	select {
	case <-publishersDone:
	case <-time.After(time.Second):
		t.Fatal("admitted publishers did not leave the lifecycle gate")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Close did not finish after all admitted publishers exited")
	}
}

func TestBusCloseContextDeadlineLeavesBackgroundFinalizerToCloseChannels(t *testing.T) {
	bus := newTestBus(1)
	if err := bus.beginPublish(); err != nil {
		t.Fatalf("beginPublish() error = %v", err)
	}

	var releaseOnce sync.Once
	releasePublisher := func() {
		releaseOnce.Do(func() { bus.publishers.Done() })
	}
	t.Cleanup(func() {
		releasePublisher()
		bus.Close()
	})

	expired, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	if err := bus.CloseContext(expired); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CloseContext() error = %v, want context deadline exceeded", err)
	}

	select {
	case <-bus.closeDone:
		t.Fatal("CloseContext deadline forcibly finalized an active publisher")
	default:
	}
	select {
	case _, ok := <-bus.SubscribeResponse():
		if !ok {
			t.Fatal("response channel closed before the admitted publisher exited")
		}
	default:
	}

	// No second close call is made here: the original background finalizer must
	// finish once the already-admitted publisher leaves the lifecycle gate.
	releasePublisher()
	select {
	case <-bus.closeDone:
	case <-time.After(time.Second):
		t.Fatal("background finalizer did not complete after the publisher exited")
	}

	for name, ch := range map[string]<-chan *DeviceMessage{
		"telemetry": bus.SubscribeTelemetry(),
		"attribute": bus.SubscribeAttribute(),
		"event":     bus.SubscribeEvent(),
		"status":    bus.SubscribeStatus(),
		"response":  bus.SubscribeResponse(),
	} {
		if _, ok := <-ch; ok {
			t.Fatalf("%s channel remained open after background finalization", name)
		}
	}
}

func TestBusBlockedPublishReturnsCallerCancellationWithoutClosingBus(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&strings.Builder{})
	fullQueue := make(chan struct{})
	logger.AddHook(&busTestLogHook{
		needle: "Telemetry channel full, blocking publish",
		fired:  fullQueue,
	})
	bus := NewBus(BusConfig{BufferSize: 1}, logger)
	t.Cleanup(bus.Close)

	first := &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-buffered"}
	if err := bus.Publish(first); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- bus.PublishContext(ctx, &DeviceMessage{
			Type:     MessageTypeTelemetry,
			DeviceID: "dev-canceled",
		})
	}()

	select {
	case <-fullQueue:
	case <-time.After(time.Second):
		t.Fatal("publisher did not block on the full telemetry queue")
	}
	cancel()
	select {
	case err := <-publishDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked PublishContext() error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("caller cancellation did not release the blocked publisher")
	}

	if err := bus.checkOpen(); err != nil {
		t.Fatalf("caller cancellation changed bus lifecycle state: %v", err)
	}
	if got := <-bus.SubscribeTelemetry(); got != first {
		t.Fatalf("buffered telemetry = %#v, want %#v", got, first)
	}
	third := &DeviceMessage{Type: MessageTypeTelemetry, DeviceID: "dev-after-cancel"}
	if err := bus.Publish(third); err != nil {
		t.Fatalf("Publish() after caller cancellation error = %v", err)
	}
	if got := <-bus.SubscribeTelemetry(); got != third {
		t.Fatalf("telemetry after cancellation = %#v, want %#v", got, third)
	}
}

func TestBusConcurrentResponsePublishAndMixedClosePreserveAcceptedMessages(t *testing.T) {
	const (
		publisherCount = 128
		closerPairs    = 8
	)
	bus := newTestBus(publisherCount + 1)
	t.Cleanup(bus.Close)

	initial := &DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "response-initial"}
	if err := bus.PublishResponse(initial); err != nil {
		t.Fatalf("initial PublishResponse() error = %v", err)
	}

	type responsePublishResult struct {
		msg *DeviceMessage
		err error
	}
	start := make(chan struct{})
	publishResults := make(chan responsePublishResult, publisherCount)
	for i := 0; i < publisherCount; i++ {
		msg := &DeviceMessage{Type: MessageTypeCommandResponse, DeviceID: "response-concurrent"}
		go func() {
			<-start
			publishResults <- responsePublishResult{msg: msg, err: bus.PublishResponse(msg)}
		}()
	}

	closeResults := make(chan error, closerPairs*2)
	for i := 0; i < closerPairs; i++ {
		go func() {
			<-start
			bus.Close()
			closeResults <- nil
		}()
		go func() {
			<-start
			closeResults <- bus.CloseContext(context.Background())
		}()
	}
	close(start)

	accepted := map[*DeviceMessage]struct{}{initial: {}}
	for i := 0; i < publisherCount; i++ {
		select {
		case result := <-publishResults:
			switch {
			case result.err == nil:
				accepted[result.msg] = struct{}{}
			case errors.Is(result.err, ErrBusClosed):
			default:
				t.Fatalf("concurrent PublishResponse() error = %v, want nil or ErrBusClosed", result.err)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent response publishers did not finish")
		}
	}
	for i := 0; i < closerPairs*2; i++ {
		select {
		case err := <-closeResults:
			if err != nil {
				t.Fatalf("mixed Close/CloseContext error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("mixed Close and CloseContext calls did not converge")
		}
	}

	for msg := range bus.SubscribeResponse() {
		if _, ok := accepted[msg]; !ok {
			t.Fatalf("response queue contained an unaccepted message: %#v", msg)
		}
		delete(accepted, msg)
	}
	if len(accepted) != 0 {
		t.Fatalf("response queue lost %d accepted messages during close", len(accepted))
	}
	if err := bus.PublishResponse(&DeviceMessage{Type: MessageTypeCommandResponse}); !errors.Is(err, ErrBusClosed) {
		t.Fatalf("PublishResponse() after mixed close error = %v, want ErrBusClosed", err)
	}
}
