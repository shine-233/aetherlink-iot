// 文件用途：payload-schema 强制的“生产决策函数 + 网关接线”经真实 socket 的统一端到端证据。
//
// 背景：既有的 server 包 e2e（payload_schema_broker_e2e_test.go）证明了 broker 的
// Drop 语义能经真实 MQTT 会话生效，但它用的是一份手写内联规则（decideTempSchema），
// 并非生产决策函数。本用例弥补这一缺口：
//   - 通过 SetPayloadSchemaResolver 注入一份真实 PayloadSchemaEnforcement（temp 必填数字 0..100）；
//   - broker 的 OnMsgArrived 钩子直接调用生产网关 enforcePayloadSchemaOnUplink（其内部调用
//     生产纯决策 DecidePayloadSchemaEnforcement），reject 时 req.Drop() 真正丢弃；
//   - 用真实 paho 客户端 publish 一条越界 payload、一条合法 payload，断言订阅端只收到合法那条，
//     且生产网关命中并丢弃了非法那条。
//
// 与 server 包 e2e 的区别：这里驱动的是生产 enforcePayloadSchemaOnUplink + DecidePayloadSchemaEnforcement
// 本体（同包可见），而非再实现一份规则；因此“生产决策逻辑经真实 socket 拦截”得到单一断言的证明。
// 仍不覆盖的部分：插件的 OnMsgArrivedWrapper → routeMQTTDeviceMessage 设备路由需要 DB/registry，
// 属运行时集成，不在此纯 broker+socket 用例范围内（此点在既有测试头中已有说明）。
package aetherlink

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/config"
	_ "github.com/DrmagicE/gmqtt/persistence" // 注册 memory 持久化工厂（server.Run 依赖）
	"github.com/DrmagicE/gmqtt/server"
	_ "github.com/DrmagicE/gmqtt/topicalias/fifo" // 注册 fifo topic-alias 工厂（server.Run 依赖）
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// This test injects the resolver through the package test seam. It proves the
// decision gate and broker drop semantics over a real socket, but it does not
// prove that AetherLinkPlugin.Load wires a production registry resolver.
func TestPayloadSchemaDecisionGateOverRealSocket(t *testing.T) {
	// 注入真实强制配置：temp 必填、数字、范围 [0,100]。用完清空，避免污染其它用例。
	SetPayloadSchemaResolver(func(deviceID, deviceConfigID string) (PayloadSchemaEnforcement, bool) {
		return PayloadSchemaEnforcement{
			Fields: []PayloadSchemaFieldConstraint{
				{Name: "temp", Type: PayloadSchemaFieldTypeNumber, Required: true, Min: f64(0), Max: f64(100)},
			},
		}, true
	})
	t.Cleanup(func() { SetPayloadSchemaResolver(nil) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	brokerAddr := ln.Addr().String()

	var enforced, dropped int
	var mu sync.Mutex
	hook := server.Hooks{
		OnMsgArrived: func(ctx context.Context, client server.Client, req *server.MsgArrivedRequest) error {
			mu.Lock()
			enforced++
			mu.Unlock()
			// 驱动生产网关本体（内部调用生产纯决策），reject → 真正 Drop。
			if enforcePayloadSchemaOnUplink("dev-e2e", "cfg-e2e", req.Message.Payload) {
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
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	const topic = "device/attributes"
	received := make(chan string, 8)

	subOpts := mqtt.NewClientOptions().
		AddBroker("tcp://" + brokerAddr).
		SetClientID("prod-e2e-sub").
		SetCleanSession(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(200 * time.Millisecond).
		SetConnectTimeout(2 * time.Second)
	sub := mqtt.NewClient(subOpts)
	if tok := sub.Connect(); !tok.WaitTimeout(15*time.Second) || tok.Error() != nil {
		t.Fatalf("subscriber connect: %v", tok.Error())
	}
	t.Cleanup(func() { sub.Disconnect(200) })
	stok := sub.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
		received <- string(m.Payload())
	})
	if !stok.WaitTimeout(5*time.Second) || stok.Error() != nil {
		t.Fatalf("subscribe: %v", stok.Error())
	}

	pubOpts := mqtt.NewClientOptions().
		AddBroker("tcp://" + brokerAddr).
		SetClientID("prod-e2e-pub").
		SetCleanSession(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(200 * time.Millisecond).
		SetConnectTimeout(2 * time.Second)
	pub := mqtt.NewClient(pubOpts)
	if tok := pub.Connect(); !tok.WaitTimeout(15*time.Second) || tok.Error() != nil {
		t.Fatalf("publisher connect: %v", tok.Error())
	}
	t.Cleanup(func() { pub.Disconnect(200) })

	// 越界（temp=150，生产决策 reject → Drop，订阅端收不到）。
	if tok := pub.Publish(topic, 0, false, []byte(`{"temp":150}`)); !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("publish invalid: %v", tok.Error())
	}
	// 合法（temp=24.5，生产决策 accept → 正常投递）。
	if tok := pub.Publish(topic, 0, false, []byte(`{"temp":24.5}`)); !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("publish valid: %v", tok.Error())
	}

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
		t.Errorf("decision gate ran %d times, want >=2 (both publishes)", enforced)
	}
	if dropped != 1 {
		t.Errorf("decision gate dropped %d messages, want exactly 1 (the out-of-range one)", dropped)
	}
}
