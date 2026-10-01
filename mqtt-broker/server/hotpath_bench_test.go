// 文件用途：broker 内核热路径基准（连接/发布/投递），为热路径优化提供 before/after 对比基线。
//
// 方法学：
//  1. 启动真实 gmqtt broker（内存持久化、临时 TCP 端口、匿名放行），不挂任何 hook，
//     测的是内核自身开销（decode→dispatch→deliver→queue→encode→write→stats）。
//  2. 客户端用 gmqtt 自带 packets 编解码器直连裸 TCP（不用 paho），
//     避免第三方客户端实现噪声；每 op 用协议级同步（PUBACK / 订阅端收到 Publish），
//     保证计时覆盖的是 broker 完成该项工作的往返延迟，而非客户端刷包速度。
//  3. 固定 op 数（benchtime=Nx）而非固定时长，减少 Windows 回环网络抖动带来的
//     N 漂移，使 before/after 数字可比。
//
// 关键注意事项：
//   - 外部测试包 server_test：空导入 persistence 注册 memory 工厂，避免 import cycle。
//   - Connect.Pack 不自动填 ProtocolName/ProtocolLevel，构造时必须显式设置，否则报文非法。
package server_test

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/config"
	_ "github.com/DrmagicE/gmqtt/persistence" // 注册 memory 持久化工厂（Run 依赖）
	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
	_ "github.com/DrmagicE/gmqtt/topicalias/fifo" // 注册 fifo topic-alias 工厂（Run 依赖）
)

const (
	benchTopic   = "bench/hotpath"
	benchPayload = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!!" // 64B
)

// startBenchBroker 启动一个监听临时端口的真实 broker，返回地址与停止函数。
func startBenchBroker(b *testing.B) (string, func()) {
	b.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	cfg := config.DefaultConfig()
	cfg.MQTT.AllowAnonymous = true
	srv := server.New(
		server.WithConfig(cfg),
		server.WithTCPListener(ln),
	)
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run() }()
	select {
	case err := <-runErr:
		b.Fatalf("broker Run returned early: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if derr == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return addr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}
}

// benchClient 是最小 MQTT v3.11 裸 TCP 客户端，供基准做协议级同步。
type benchClient struct {
	conn net.Conn
	rd   *packets.Reader
	wr   *packets.Writer
}

// close 使用 SO_LINGER=0 的中止式关闭：连接churn基准（尤其连接基准）在 Windows 回环上
// 若走正常 FIN 关闭，客户端侧数万个临时端口会滞留 TIME_WAIT，随后 connect 报
// WSAEADDRINUSE（"Only one usage of each socket address"）。RST 关闭不进入 TIME_WAIT。
func (c *benchClient) close() {
	if tc, ok := c.conn.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = c.conn.Close()
}

func dialBenchClient(b *testing.B, addr string, clientID string, doConnect bool) *benchClient {
	b.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	c := &benchClient{
		conn: conn,
		rd:   packets.NewReader(conn),
		wr:   packets.NewWriter(conn),
	}
	if doConnect {
		c.mustConnect(b, clientID)
	}
	return c
}

func (c *benchClient) mustConnect(b *testing.B, clientID string) {
	b.Helper()
	err := c.wr.WriteAndFlush(&packets.Connect{
		Version:       packets.Version311,
		ProtocolName:  []byte("MQTT"),
		ProtocolLevel: 4,
		CleanStart:    true,
		KeepAlive:     0,
		ClientID:      []byte(clientID),
	})
	if err != nil {
		b.Fatalf("write connect: %v", err)
	}
	for {
		p, rerr := c.rd.ReadPacket()
		if rerr != nil {
			b.Fatalf("read connack: %v", rerr)
		}
		if ack, ok := p.(*packets.Connack); ok {
			if ack.Code != codes.Success {
				b.Fatalf("connack refused: %v", ack.Code)
			}
			return
		}
	}
}

func (c *benchClient) mustSubscribe(b *testing.B, packetID uint16, topic string) {
	b.Helper()
	err := c.wr.WriteAndFlush(&packets.Subscribe{
		Version:  packets.Version311,
		PacketID: packetID,
		Topics: []packets.Topic{
			{
				SubOptions: packets.SubOptions{Qos: 0},
				Name:       topic,
			},
		},
	})
	if err != nil {
		b.Fatalf("write subscribe: %v", err)
	}
	for {
		p, rerr := c.rd.ReadPacket()
		if rerr != nil {
			b.Fatalf("read suback: %v", rerr)
		}
		if _, ok := p.(*packets.Suback); ok {
			return
		}
	}
}
func (c *benchClient) writePublishQoS0(b *testing.B, packetID uint16) {
	b.Helper()
	err := c.wr.WriteAndFlush(&packets.Publish{
		Version:   packets.Version311,
		Qos:       packets.Qos0,
		PacketID:  packetID,
		TopicName: []byte(benchTopic),
		Payload:   []byte(benchPayload),
	})
	if err != nil {
		b.Fatalf("write publish: %v", err)
	}
}

// writePublishQoS1AndWaitAck 每 op 一条 QoS1 发布并阻塞等待 PUBACK（协议级同步点）。
func (c *benchClient) writePublishQoS1AndWaitAck(b *testing.B, packetID uint16) {
	b.Helper()
	c.writePublishQoS1(b, packetID)
	for {
		p, rerr := c.rd.ReadPacket()
		if rerr != nil {
			b.Fatalf("read puback: %v", rerr)
		}
		if _, ok := p.(*packets.Puback); ok {
			return
		}
	}
}

func (c *benchClient) writePublishQoS1(b *testing.B, packetID uint16) {
	b.Helper()
	err := c.wr.WriteAndFlush(&packets.Publish{
		Version:   packets.Version311,
		Qos:       packets.Qos1,
		PacketID:  packetID,
		TopicName: []byte(benchTopic),
		Payload:   []byte(benchPayload),
	})
	if err != nil {
		b.Fatalf("write publish: %v", err)
	}
}

// waitPublish 阻塞等待订阅端收到一条 Publish（fanout 投递的同步点）。
func (c *benchClient) waitPublish(b *testing.B) {
	b.Helper()
	for {
		p, rerr := c.rd.ReadPacket()
		if rerr != nil {
			b.Fatalf("read publish: %v", rerr)
		}
		if _, ok := p.(*packets.Publish); ok {
			return
		}
	}
}

// BenchmarkBrokerConnect 每次迭代：TCP 建连 + CONNECT/CONNACK + 断开。
// 覆盖 accept 循环、连接建立、CONNECT 处理与会话注册路径。
func BenchmarkBrokerConnect(b *testing.B) {
	addr, stop := startBenchBroker(b)
	defer stop()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := dialBenchClient(b, addr, "bench-conn", false)
		c.mustConnect(b, "bench-conn")
		c.close()
	}
}

// BenchmarkBrokerPublishQoS1 每 op 一条 QoS1 发布并等待 PUBACK（单连接、无订阅者）。
// 覆盖入站热路径：read→decode→publishHandler→无订阅匹配→PUBACK encode→write→stats。
func BenchmarkBrokerPublishQoS1(b *testing.B) {
	addr, stop := startBenchBroker(b)
	defer stop()
	c := dialBenchClient(b, addr, "bench-pub-qos1", true)
	defer c.close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.writePublishQoS1AndWaitAck(b, uint16(i%65535+1))
	}
}

// BenchmarkBrokerDeliverQoS0Fanout1 每 op 一条 QoS0 发布，订阅端阻塞收到该条消息。
// 覆盖完整投递路径：decode→OnMsgArrived→订阅匹配→队列→poll→outbound encode→写出。
func BenchmarkBrokerDeliverQoS0Fanout1(b *testing.B) {
	addr, stop := startBenchBroker(b)
	defer stop()
	sub := dialBenchClient(b, addr, "bench-sub-qos0", true)
	defer sub.close()
	sub.mustSubscribe(b, 1, benchTopic)
	pub := dialBenchClient(b, addr, "bench-pub-qos0", true)
	defer pub.close()
	// 等订阅关系在 broker 内生效（SUBACK 已确认，留一拍缓冲）。
	time.Sleep(100 * time.Millisecond)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pub.writePublishQoS0(b, 0)
		sub.waitPublish(b)
	}
}

// BenchmarkBrokerPublishQoS1Parallel4 并发发布端（每 worker 独立连接）做 QoS1 往返，
// 用于观察全局锁（srv.mu / stats RWMutex）在并发下的争用放大。
// 说明：testing.PB 不暴露 worker 编号，连接在每个 worker 闭包内建立（建立成本对数千
// op 摊销可忽略），clientID 用原子计数保证唯一，避免会话接管互相踢线。
func BenchmarkBrokerPublishQoS1Parallel4(b *testing.B) {
	addr, stop := startBenchBroker(b)
	defer stop()
	var uid uint32
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := fmt.Sprintf("bench-par-%d", atomic.AddUint32(&uid, 1))
		c := dialBenchClient(b, addr, id, true)
		defer c.close()
		var seq uint16
		for pb.Next() {
			seq++
			c.writePublishQoS1AndWaitAck(b, seq%65535+1)
		}
	})
}
