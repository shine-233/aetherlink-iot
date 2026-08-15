// 文件用途：payload-schema broker 强制的真实 socket 端到端运行时证据。
//
// 与 plugin/aetherlink 里的门控/纯决策单测不同，这里启动一个真实的 gmqtt broker
// （内存持久化、临时 TCP 端口），挂一个 OnMsgArrived 钩子，对上行 payload 做
// schema 强制：违反约束的消息用 req.Drop() 真正丢弃，不再投递给订阅者；满足约束的
// 消息正常路由。随后用真实 paho 客户端 publish 一条合法、一条非法 payload，断言订阅端
// 只收到合法那条。证明“broker 侧 OnMsgArrived 拦截”确实能经真实 MQTT 会话生效，
// 而不仅是纯函数存在——这是 businessClosureReady 需要的运行时证据之一。
//
// 关键注意事项：
//  1. 用外部测试包 server_test，以便空导入 persistence（注册 memory 工厂）而不触发
//     import cycle（persistence 依赖 server）。所有用到的标识符都是导出的。
//  2. 内联最小 schema 决策（temp 必须是 0..100 的数字）代表强制口径，语义与
//     plugin/aetherlink.DecidePayloadSchemaEnforcement 对齐；此处只验证
//     “reject→Drop→不投递”的会话级行为，不 import 上层 plugin 以免引入 DB/Redis 依赖。
package server_test

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/config"
	_ "github.com/DrmagicE/gmqtt/persistence" // 注册 memory 持久化工厂（Run 依赖）
	"github.com/DrmagicE/gmqtt/server"
	_ "github.com/DrmagicE/gmqtt/topicalias/fifo" // 注册 fifo topic-alias 工厂（Run 依赖）
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// decideTempSchema 是最小的 payload 强制口径：payload 必须是 JSON 对象，且含数字字段
// temp 落在 [0,100]。返回 true 表示应拒收。
func decideTempSchema(payload []byte) bool {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return true // 非 JSON → 拒收
	}
	v, ok := obj["temp"]
	if !ok {
		return true // 缺必填字段 → 拒收
	}
	num, ok := v.(float64)
	if !ok {
		return true // 类型不符 → 拒收
	}
	return num < 0 || num > 100 // 越界 → 拒收
}

func TestPayloadSchemaBrokerEnforcementRealSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	brokerAddr := ln.Addr().String()

	// OnMsgArrived 钩子：对上行 payload 做 schema 强制，违反则 Drop（真正不投递）。
	var enforced, dropped int
	var mu sync.Mutex
	hook := server.Hooks{
		OnMsgArrived: func(ctx context.Context, client server.Client, req *server.MsgArrivedRequest) error {
			mu.Lock()
			enforced++
			mu.Unlock()
			if decideTempSchema(req.Message.Payload) {
				req.Drop()
				mu.Lock()
				dropped++
				mu.Unlock()
			}
			return nil
		},
	}

	cfg := config.DefaultConfig()
	cfg.MQTT.AllowAnonymous = true
	srv := server.New(
		server.WithConfig(cfg),
		server.WithTCPListener(ln),
		server.WithHook(hook),
	)

	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run() }()
	select {
	case err := <-runErr:
		t.Fatalf("broker Run returned early: %v", err)
	case <-time.After(300 * time.Millisecond):
		// broker is serving
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	// 等 broker 起来。
	waitForBroker(t, brokerAddr)

	const topic = "device/attributes"
	received := make(chan string, 8)

	// 订阅端：先连接完成，再订阅（避免在 OnConnect 回调里 Wait 造成死锁）。
	subOpts := mqtt.NewClientOptions().
		AddBroker("tcp://" + brokerAddr).
		SetClientID("e2e-subscriber").
		SetCleanSession(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(200 * time.Millisecond).
		SetConnectTimeout(2 * time.Second)
	sub := mqtt.NewClient(subOpts)
	connectMQTTWithRetry(t, sub, "subscriber")
	t.Cleanup(func() { sub.Disconnect(200) })
	stok := sub.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
		received <- string(m.Payload())
	})
	if !stok.WaitTimeout(5 * time.Second) {
		t.Fatal("subscribe timed out")
	}
	if stok.Error() != nil {
		t.Fatalf("subscribe: %v", stok.Error())
	}

	// 发布端。
	pubOpts := mqtt.NewClientOptions().
		AddBroker("tcp://" + brokerAddr).
		SetClientID("e2e-publisher").
		SetCleanSession(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(200 * time.Millisecond).
		SetConnectTimeout(2 * time.Second)
	pub := mqtt.NewClient(pubOpts)
	connectMQTTWithRetry(t, pub, "publisher")
	t.Cleanup(func() { pub.Disconnect(200) })

	// 先发一条非法（temp 越界，应被 broker Drop，订阅端收不到），
	// 再发一条合法（应正常投递）。
	if tok := pub.Publish(topic, 0, false, []byte(`{"temp":150}`)); !tok.WaitTimeout(5 * time.Second) {
		t.Fatal("publish invalid timed out")
	} else if tok.Error() != nil {
		t.Fatalf("publish invalid: %v", tok.Error())
	}
	if tok := pub.Publish(topic, 0, false, []byte(`{"temp":24.5}`)); !tok.WaitTimeout(5 * time.Second) {
		t.Fatal("publish valid timed out")
	} else if tok.Error() != nil {
		t.Fatalf("publish valid: %v", tok.Error())
	}

	// 只应收到合法那条。
	var got []string
collect:
	for {
		select {
		case msg := <-received:
			got = append(got, msg)
		case <-time.After(2 * time.Second):
			break collect
		}
	}

	if len(got) != 1 {
		t.Fatalf("subscriber received %d messages, want exactly 1 (the valid one); got=%v", len(got), got)
	}
	if got[0] != `{"temp":24.5}` {
		t.Errorf("delivered payload = %q, want the valid {\"temp\":24.5}", got[0])
	}

	mu.Lock()
	defer mu.Unlock()
	if enforced < 2 {
		t.Errorf("OnMsgArrived enforced %d times, want >=2 (both publishes)", enforced)
	}
	if dropped != 1 {
		t.Errorf("broker dropped %d messages, want exactly 1 (the invalid one)", dropped)
	}
}

func waitForBroker(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("broker did not start listening on %s", addr)
}

// connectMQTTWithRetry 依赖 paho 自带的连接重试（SetConnectRetry），
// 即使 broker 的 accept 循环稍晚于 TCP 监听就绪，也会重试直到 CONNACK。
func connectMQTTWithRetry(t *testing.T, c mqtt.Client, role string) {
	t.Helper()
	tok := c.Connect()
	if !tok.WaitTimeout(15 * time.Second) {
		t.Fatalf("%s connect timed out", role)
	}
	if tok.Error() != nil {
		t.Fatalf("%s connect: %v", role, tok.Error())
	}
}
