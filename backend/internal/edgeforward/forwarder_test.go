// 文件用途：edgeforward 单元测试——内置最小 MQTT broker（回环）验证
// 实时转发、断连缓冲+重连重投、缓冲溢出丢最旧三场景。
package edgeforward

import (
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/uplink"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- 最小 MQTT broker（CONNECT/CONNACK、PUBLISH 捕获、PINGREQ） ----------

type pubMsg struct {
	topic   string
	payload []byte
}

type testBroker struct {
	ln       net.Listener
	received chan pubMsg
	mu       sync.Mutex
	conns    []net.Conn
	stopOnce sync.Once
}

func newTestBroker(t *testing.T) *testBroker {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	b := &testBroker{ln: ln, received: make(chan pubMsg, 256)}
	go b.acceptLoop()
	t.Cleanup(b.close)
	return b
}

func (b *testBroker) addr() string { return b.ln.Addr().String() }

func (b *testBroker) close() {
	b.stopOnce.Do(func() {
		b.mu.Lock()
		for _, c := range b.conns {
			_ = c.Close()
		}
		b.mu.Unlock()
		_ = b.ln.Close()
	})
}

func (b *testBroker) acceptLoop() {
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		b.conns = append(b.conns, conn)
		b.mu.Unlock()
		go b.handleConn(conn)
	}
}

// readVarint 解析 MQTT 剩余长度变int。
func readVarint(conn net.Conn) (int, error) {
	mul := 1
	value := 0
	for i := 0; i < 4; i++ {
		buf := make([]byte, 1)
		if _, err := conn.Read(buf); err != nil {
			return 0, err
		}
		value += int(buf[0]&0x7f) * mul
		if buf[0]&0x80 == 0 {
			return value, nil
		}
		mul *= 128
	}
	return value, nil
}

func (b *testBroker) handleConn(conn net.Conn) {
	defer conn.Close()
	header := make([]byte, 1)
	for {
		if _, err := conn.Read(header); err != nil {
			return
		}
		switch header[0] >> 4 {
		case 1: // CONNECT -> CONNACK（读掉整个剩余部分，防止流错位）
			remain, err := readVarint(conn)
			if err != nil {
				return
			}
			if remain > 0 {
				if _, err := readFull(conn, make([]byte, remain)); err != nil {
					return
				}
			}
			if _, err := conn.Write([]byte{0x20, 0x02, 0x00, 0x00}); err != nil {
				return
			}
		case 3: // PUBLISH（支持 QoS0/QoS1：QoS1 需回 PUBACK，否则客户端 5s 超时假失败）
			remain, err := readVarint(conn)
			if err != nil {
				return
			}
			body := make([]byte, remain)
			if _, err := readFull(conn, body); err != nil {
				return
			}
			qos := (header[0] >> 1) & 0x03
			topicLen := int(body[0])<<8 | int(body[1])
			topic := string(body[2 : 2+topicLen])
			payloadStart := 2 + topicLen
			if qos >= 1 {
				payloadStart += 2 // packet id
			}
			payload := body[payloadStart:]
			select {
			case b.received <- pubMsg{topic: topic, payload: payload}:
			default:
			}
			if qos == 1 {
				pid := body[2+topicLen : 2+topicLen+2]
				if _, err := conn.Write(append([]byte{0x40, 0x02}, pid...)); err != nil {
					return
				}
			}
		case 12: // PINGREQ -> PINGRESP
			if _, err := readVarint(conn); err != nil {
				return
			}
			if _, err := conn.Write([]byte{0xd0, 0x00}); err != nil {
				return
			}
		case 14: // DISCONNECT
			return
		default:
			if _, err := readVarint(conn); err != nil {
				return
			}
		}
	}
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// ---------- 测试辅助 ----------

func newTestBus(t *testing.T) *uplink.Bus {
	t.Helper()
	return uplink.NewBus(uplink.BusConfig{BufferSize: 100}, logrus.New())
}

func publishTelemetry(t *testing.T, bus *uplink.Bus, deviceID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		msg := &uplink.DeviceMessage{
			Type:      "telemetry",
			DeviceID:  deviceID,
			TenantID:  "tenant-e2e",
			Timestamp: 1788600000000 + int64(i),
			Payload:   []byte(`{"temperature":26.5}`),
		}
		require.NoError(t, bus.Publish(msg))
	}
}

func waitReceived(t *testing.T, b *testBroker, n int) []pubMsg {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	got := make([]pubMsg, 0, n)
	for time.Now().Before(deadline) && len(got) < n {
		select {
		case m := <-b.received:
			got = append(got, m)
		case <-time.After(200 * time.Millisecond):
		}
	}
	return got
}

func forwarderConfig(broker string, bufferLimit int) Config {
	return Config{
		Enabled:     true,
		Broker:      broker,
		TopicPrefix: "aetherlink/edge",
		ClientID:    "edge-test",
		BufferLimit: bufferLimit,
		QoS:         1,
	}
}

// ---------- 场景测试 ----------

// 场景 1：broker 在线时，总线消息实时转发且 topic/信封正确。
func TestEdgeForwardLive(t *testing.T) {
	broker := newTestBroker(t)
	bus := newTestBus(t)
	f := New(bus, forwarderConfig(broker.addr(), 100), logrus.New())
	f.Start()
	defer f.Stop()
	select {
	case <-f.Subscribed():
	case <-time.After(5 * time.Second):
		t.Fatal("订阅未就绪")
	}

	publishTelemetry(t, bus, "dev-live", 3)
	got := waitReceived(t, broker, 3)
	require.Len(t, got, 3, "应实时收到 3 条转发")
	for _, m := range got {
		assert.Equal(t, "aetherlink/edge/telemetry/dev-live", m.topic)
		var env map[string]interface{}
		require.NoError(t, json.Unmarshal(m.payload, &env))
		assert.Equal(t, "dev-live", env["device_id"])
		assert.Equal(t, "tenant-e2e", env["tenant_id"])
		assert.Equal(t, "telemetry", env["type"])
		assert.NotNil(t, env["payload"])
	}
}

// 场景 2：broker 初始不可达 → 消息入缓冲；broker 就绪后按序重投。
func TestEdgeForwardOfflineBufferAndRedrive(t *testing.T) {
	// 先占用一个端口再释放：得到一个"确定空闲"的地址供稍后启动 broker。
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := probe.Addr().String()
	_ = probe.Close()

	bus := newTestBus(t)
	f := New(bus, forwarderConfig(addr, 100), logrus.New())
	f.Start()
	defer f.Stop()
	select {
	case <-f.Subscribed():
	case <-time.After(5 * time.Second):
		t.Fatal("订阅未就绪")
	}

	publishTelemetry(t, bus, "dev-offline", 3)
	// 等 1s 确认无 broker 时消息进缓冲（不丢、不阻塞）。
	time.Sleep(1 * time.Second)

	broker := newTestBrokerOn(t, addr)
	got := waitReceived(t, broker, 3)
	require.Len(t, got, 3, "重连后应按序重投 3 条缓冲消息")
	assert.Equal(t, "aetherlink/edge/telemetry/dev-offline", got[0].topic)
}

// 场景 3：缓冲满时丢最旧，仅保留最新 BufferLimit 条。
func TestEdgeForwardBufferOverflowDropsOldest(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := probe.Addr().String()
	_ = probe.Close()

	bus := newTestBus(t)
	f := New(bus, forwarderConfig(addr, 2), logrus.New())
	f.Start()
	defer f.Stop()
	select {
	case <-f.Subscribed():
	case <-time.After(5 * time.Second):
		t.Fatal("订阅未就绪")
	}

	publishTelemetry(t, bus, "dev-overflow", 5)
	time.Sleep(1 * time.Second)

	broker := newTestBrokerOn(t, addr)
	got := waitReceived(t, broker, 2)
	require.Len(t, got, 2, "溢出后仅剩最新 2 条可重投")
	// 最旧 3 条被丢弃：剩 mess#3、#4（按时间戳区分）。
	for _, m := range got {
		var env map[string]interface{}
		require.NoError(t, json.Unmarshal(m.payload, &env))
		ts := int64(env["ts"].(float64))
		assert.GreaterOrEqual(t, ts, int64(1788600000002), "被保留的应是最新的消息")
	}
	assert.True(t, atomic.LoadUint64(&f.bufDropped) >= 3, "应记录丢最旧计数")
}

// newTestBrokerOn 在指定地址启动测试 broker（用于"先缓冲后开 broker"场景）。
func newTestBrokerOn(t *testing.T, addr string) *testBroker {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	b := &testBroker{ln: ln, received: make(chan pubMsg, 256)}
	go b.acceptLoop()
	t.Cleanup(b.close)
	return b
}
