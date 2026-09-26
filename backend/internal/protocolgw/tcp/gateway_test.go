// 文件用途：TCP 网关端到端单测（TP-03）——真实 net.Listen 上的注册→遥测→下行→断网缓冲续传全链路。
// 核心逻辑：127.0.0.1:0 随机端口起真实监听；验证 length-prefix 与 line 两种帧模式、
// 未知设备 fail-closed 断连、禁用返回 nil、Stop 踢会话与拒投、端口占用报错。
// 关键注意事项：断网缓冲用例依赖"对端关闭后服务端读循环完成去注册"——用 Stats 轮询等待而非 sleep。
// 重构建议：引入 TLS 后在此补 TLS 握手用例（residual 项落地时同步）。
package tcp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// startTestGateway 在 127.0.0.1:0 起真实网关（注册 dev-1/dev-2 映射），返回实例与地址。
func startTestGateway(t *testing.T, mutate func(*Config)) (*Gateway, *fakePublisher, string) {
	t.Helper()
	pub := &fakePublisher{}
	cfg := Config{Enabled: true, Addr: "127.0.0.1:0"}
	if mutate != nil {
		mutate(&cfg)
	}
	gw, err := Start(cfg, testDeps(pub), logrus.New())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(gw.Stop)
	return gw, pub, gw.ln.Addr().String()
}

// waitFor 轮询条件直到成立或超时（替代 sleep 等待异步 goroutine 收尾）。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// dialRegister 建连并发送注册帧。
func dialRegister(t *testing.T, addr, number string, mode FrameMode) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	frame, err := EncodeFrame(mode, []byte(number), DefaultMaxFrameSize)
	if err != nil {
		t.Fatalf("encode registration: %v", err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("send registration: %v", err)
	}
	return conn
}

// lpEncode 测试内 length-prefix 编码（与 EncodeFrame 独立实现，互为校验）。
func lpEncode(payload string) []byte {
	b := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(b, uint32(len(payload)))
	copy(b[4:], payload)
	return b
}

func TestGatewayDisabledReturnsNil(t *testing.T) {
	gw, err := Start(Config{Enabled: false, Addr: "127.0.0.1:0"}, testDeps(&fakePublisher{}), nil)
	if gw != nil || err != nil {
		t.Fatalf("disabled gateway = (%v, %v), want (nil, nil)", gw, err)
	}
}

func TestGatewayStartRequiresDependencies(t *testing.T) {
	if _, err := Start(Config{Enabled: true, Addr: "127.0.0.1:0"}, Dependencies{Publisher: &fakePublisher{}}, nil); err == nil {
		t.Fatal("missing resolver should fail")
	}
	if _, err := Start(Config{Enabled: true, Addr: "127.0.0.1:0"}, Dependencies{Resolver: &fakeResolver{}}, nil); err == nil {
		t.Fatal("missing publisher should fail")
	}
}

func TestGatewayStartPortConflict(t *testing.T) {
	_, _, addr := startTestGateway(t, nil)
	if _, err := Start(Config{Enabled: true, Addr: addr}, testDeps(&fakePublisher{}), nil); err == nil {
		t.Fatal("second listen on same addr should fail")
	}
}

func TestGatewayEndToEndTelemetryAndDownlink(t *testing.T) {
	gw, pub, addr := startTestGateway(t, nil)
	conn := dialRegister(t, addr, "dev-1", FrameModeLengthPrefix)

	// 遥测上行：注册帧后紧跟遥测帧（粘包一次写入）。
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(joinFrames(lpEncode(`{"temperature":25.5}`), lpEncode(`{"humidity":60}`))); err != nil {
		t.Fatalf("send telemetry: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return len(pub.msgs) == 2 }, "telemetry not published")
	for i, want := range []string{`{"temperature":25.5}`, `{"humidity":60}`} {
		msg := pub.msgs[i]
		if msg.Type != "telemetry" || msg.DeviceID != "uuid-1" || msg.TenantID != "tenant-1" {
			t.Fatalf("msg[%d] header = %+v", i, msg)
		}
		if string(msg.Payload) != want || msg.Metadata["source_protocol"] != SourceProtocolTCP {
			t.Fatalf("msg[%d] = payload %s metadata %v", i, msg.Payload, msg.Metadata)
		}
	}

	// 下行直投：平台命令 → 设备侧收帧。
	if err := gw.DeliverCommand("dev-1", []byte(`{"led":"on"}`)); err != nil {
		t.Fatalf("DeliverCommand: %v", err)
	}
	if got := readFrame(t, conn, FrameModeLengthPrefix); string(got) != `{"led":"on"}` {
		t.Fatalf("downlink payload = %q", got)
	}
	st := gw.Stats()
	if st.Published != 2 || st.CmdDelivered != 1 || st.Registrations != 1 || st.Sessions != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestGatewayLineModeEndToEnd(t *testing.T) {
	gw, pub, addr := startTestGateway(t, func(c *Config) { c.Mode = FrameModeLine })
	conn := dialRegister(t, addr, "dev-2", FrameModeLine)

	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte("{\"a\":1}\n")); err != nil {
		t.Fatalf("send telemetry: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool { return len(pub.msgs) == 1 }, "line-mode telemetry not published")
	if string(pub.msgs[0].Payload) != `{"a":1}` || pub.msgs[0].DeviceID != "uuid-2" {
		t.Fatalf("msg = %+v", pub.msgs[0])
	}

	if err := gw.DeliverCommand("dev-2", []byte("reboot")); err != nil {
		t.Fatalf("DeliverCommand: %v", err)
	}
	if got := readFrame(t, conn, FrameModeLine); string(got) != "reboot" {
		t.Fatalf("downlink payload = %q", got)
	}
}

func TestGatewayRejectsUnknownAndMalformedRegistration(t *testing.T) {
	_, _, addr := startTestGateway(t, nil)

	// 未知设备：连接必须被关闭（fail-closed）。
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_, _ = conn.Write(lpEncode("stranger"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("unknown device connection should be closed")
	}

	// 非法注册帧（含换行的设备号）：同样断连。
	conn2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn2.Close()
	_, _ = conn2.Write(lpEncode("de v")) // 空白字符非法
	_ = conn2.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn2.Read(buf); err == nil {
		t.Fatal("malformed registration should close connection")
	}
}

func TestGatewayOfflineBufferRedriveOnReconnect(t *testing.T) {
	gw, _, addr := startTestGateway(t, nil)

	// 第一次连接：注册建立 Known，然后断开（会话移除，Known 留存）。
	conn := dialRegister(t, addr, "dev-1", FrameModeLengthPrefix)
	waitFor(t, 3*time.Second, func() bool { return gw.Stats().Sessions == 1 }, "session not registered")
	_ = conn.Close()
	waitFor(t, 3*time.Second, func() bool { return gw.Stats().Sessions == 0 }, "session not deregistered after close")

	// 离线期下发：必须入缓冲且不报错。
	if err := gw.DeliverCommand("dev-1", []byte("buffered-cmd")); err != nil {
		t.Fatalf("offline DeliverCommand: %v", err)
	}
	if st := gw.Stats(); st.CmdBuffered != 1 || st.CmdPending != 1 {
		t.Fatalf("stats = %+v", st)
	}

	// 重连注册：缓冲命令按 FIFO 自动续传到设备。
	conn2 := dialRegister(t, addr, "dev-1", FrameModeLengthPrefix)
	payload, err := readFrameErr(conn2, FrameModeLengthPrefix)
	if err != nil {
		t.Fatalf("read flushed command: %v", err)
	}
	if string(payload) != "buffered-cmd" {
		t.Fatalf("flushed payload = %q", payload)
	}
	// 续传计数在写帧完成后才递增（写帧→cmdFlushed.Add 之间存在调度窗口），
	// 设备侧读到帧不等于计数器已可见；按本文件头部约定用 Stats 轮询等待收敛，
	// 避免全量跑时的偶发失败（表现为 CmdFlushed:0 但设备已收到缓冲命令）。
	waitFor(t, 3*time.Second, func() bool {
		st := gw.Stats()
		return st.CmdFlushed == 1 && st.CmdPending == 0 && st.CmdDelivered == 0
	}, "flushed stats not settled after redrive")
}

func TestGatewayStopClosesSessionsAndRejectsCommands(t *testing.T) {
	pub := &fakePublisher{}
	gw, err := Start(Config{Enabled: true, Addr: "127.0.0.1:0"}, testDeps(pub), logrus.New())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	conn, err := net.Dial("tcp", gw.ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_, _ = conn.Write(lpEncode("dev-1"))
	waitFor(t, 3*time.Second, func() bool { return gw.Stats().Sessions == 1 }, "session not registered")

	gw.Stop()
	// Stop 后：会话被踢、监听关闭、下行拒投。
	waitFor(t, 3*time.Second, func() bool { return gw.Stats().Sessions == 0 }, "sessions not closed on stop")
	if err := gw.DeliverCommand("dev-1", []byte("x")); err == nil {
		t.Fatal("DeliverCommand after Stop should fail")
	}
	if _, err := net.Dial("tcp", gw.ln.Addr().String()); err == nil {
		t.Fatal("listener should be closed after Stop")
	}
	_ = conn.Close()
}

func TestGatewayDefaultConfigFromViper(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Enabled {
		t.Fatal("tcp gateway must default to disabled")
	}
	if cfg.Mode != FrameModeLengthPrefix || cfg.MaxFrameSize != DefaultMaxFrameSize {
		t.Fatalf("default config = %+v", cfg)
	}
	if cfg.Addr == "" || cfg.CommandBufferLimit != DefaultCommandBufferLimit {
		t.Fatalf("default config = %+v", cfg)
	}
	if !ValidFrameMode(cfg.Mode) {
		t.Fatal("default frame mode must be valid")
	}
}
