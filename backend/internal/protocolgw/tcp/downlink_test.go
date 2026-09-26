// 文件用途：下行命令断网缓冲单测（TP-03）——环形缓冲丢最旧、在线直投、离线缓冲、注册冲刷保序。
// 核心逻辑：net.Pipe 充当会话连接，读写双 goroutine 验证 DeliverCommand 与 flushOnRegister 的
// 完整语义（含写失败转缓冲、续传中断剩余命令按原序塞回队首）。
// 关键注意事项：net.Pipe 是同步管道——注册冲刷的写会阻塞到读侧消费，用例必须先起读 goroutine；
// 读 goroutine 不得调用 t.Fatalf（Fatal 只允许测试主 goroutine），错误经 channel 回传断言。
// 重构建议：引入持久化 spool 后，此处补"进程重启不丢缓冲"的行为锚点（当前明确为内存态）。
package tcp

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/protocolgw"

	"github.com/sirupsen/logrus"
)

// newTestGateway 构造未监听的网关（单测直连 DeliverCommand/registry 层）。
func newTestGateway(cfg Config, deps Dependencies) *Gateway {
	cfg = cfg.normalize()
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	g := &Gateway{
		cfg:      cfg,
		deps:     deps,
		log:      log,
		ingestor: NewIngestor(deps.Publisher, log),
		bufs:     newDownlinkBufferer(cfg.CommandBufferLimit),
	}
	g.registry = NewRegistry(g.flushOnRegister, nil)
	return g
}

func testDeps(pub *fakePublisher) Dependencies {
	return Dependencies{
		Resolver: &fakeResolver{byNumber: map[string]*protocolgw.DeviceIdentity{
			"dev-1": {DeviceID: "uuid-1", TenantID: "tenant-1", DeviceNumber: "dev-1"},
			"dev-2": {DeviceID: "uuid-2", TenantID: "tenant-1", DeviceNumber: "dev-2"},
		}},
		Publisher: pub,
	}
}

// readFrameErr 从客户端连接读一帧下行数据并解码 payload（带超时；goroutine 安全版）。
func readFrameErr(conn net.Conn, mode FrameMode) ([]byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if mode == FrameModeLine {
		buf := make([]byte, 0, 64)
		one := make([]byte, 1)
		for {
			if _, err := io.ReadFull(conn, one); err != nil {
				return nil, err
			}
			if one[0] == '\n' {
				break
			}
			buf = append(buf, one[0])
		}
		return buf, nil
	}
	head := make([]byte, lengthPrefixHeaderSize)
	if _, err := io.ReadFull(conn, head); err != nil {
		return nil, err
	}
	body := make([]byte, binary.BigEndian.Uint32(head))
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	return body, nil
}

// readFrame 主 goroutine 版：失败直接 Fatal。
func readFrame(t *testing.T, conn net.Conn, mode FrameMode) []byte {
	t.Helper()
	payload, err := readFrameErr(conn, mode)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return payload
}

func TestCommandRingDropOldestAndRequeue(t *testing.T) {
	b := newDownlinkBufferer(2)
	b.enqueue("dev", []byte("a"))
	b.enqueue("dev", []byte("b"))
	b.enqueue("dev", []byte("c")) // 满队：丢 "a"
	got := b.drain("dev")
	if len(got) != 2 || string(got[0]) != "b" || string(got[1]) != "c" {
		t.Fatalf("drain = %v, want [b c]", got)
	}
	if pending, dropped := b.totals(); pending != 0 || dropped != 1 {
		t.Fatalf("totals = (%d, %d), want (0, 1)", pending, dropped)
	}
	// requeueFront：塞回 3 条（超限 2）应丢尾部（最新）保队首（最旧）。
	b.requeueFront("dev", [][]byte{[]byte("x"), []byte("y"), []byte("z")})
	got = b.drain("dev")
	if len(got) != 2 || string(got[0]) != "x" || string(got[1]) != "y" {
		t.Fatalf("requeued drain = %v, want [x y]", got)
	}
	if _, dropped := b.totals(); dropped != 2 {
		t.Fatalf("dropped = %d, want 2", dropped)
	}
}

func TestDeliverCommandOnlineWritesFrame(t *testing.T) {
	g := newTestGateway(Config{Enabled: true}, testDeps(&fakePublisher{}))
	s, client := pipeSession(t, "dev-1")
	g.registry.Register(s)

	// net.Pipe 写阻塞到读侧消费：先起读 goroutine 再投递。
	frames := make(chan []byte, 1)
	readErr := make(chan error, 1)
	go func() {
		payload, err := readFrameErr(client, g.cfg.Mode)
		if err != nil {
			readErr <- err
			return
		}
		frames <- payload
	}()

	if err := g.DeliverCommand("dev-1", []byte(`{"led":"on"}`)); err != nil {
		t.Fatalf("DeliverCommand: %v", err)
	}
	select {
	case got := <-frames:
		if string(got) != `{"led":"on"}` {
			t.Fatalf("frame payload = %q", got)
		}
	case err := <-readErr:
		t.Fatalf("read frame: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("frame read timeout")
	}
	st := g.Stats()
	if st.CmdDelivered != 1 || st.CmdBuffered != 0 || st.Sessions != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestDeliverCommandOfflineBuffersUnknownRejected(t *testing.T) {
	g := newTestGateway(Config{Enabled: true}, testDeps(&fakePublisher{}))
	s, _ := pipeSession(t, "dev-1")
	g.registry.Register(s)
	g.registry.Deregister(s) // 离线但 Known

	if err := g.DeliverCommand("dev-1", []byte("cmd-1")); err != nil {
		t.Fatalf("offline DeliverCommand: %v", err)
	}
	if pending, _ := g.bufs.totals(); pending != 1 {
		t.Fatalf("pending = %d, want 1", pending)
	}
	// 从未注册过的设备号必须显式报错（调用方回退 MQTT）。
	if err := g.DeliverCommand("stranger", []byte("x")); !errors.Is(err, ErrUnknownDevice) {
		t.Fatalf("unknown device err = %v, want ErrUnknownDevice", err)
	}
	if !g.HandlesDevice("dev-1") || g.HandlesDevice("stranger") {
		t.Fatal("HandlesDevice must track known numbers only")
	}
	st := g.Stats()
	if st.CmdBuffered != 1 || st.CmdDelivered != 0 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestDeliverCommandWriteFailureFallsBackToBuffer(t *testing.T) {
	g := newTestGateway(Config{Enabled: true}, testDeps(&fakePublisher{}))
	s, client := pipeSession(t, "dev-1")
	g.registry.Register(s)
	_ = client.Close() // 模拟连接死亡（对端断开，写侧立即报错）
	_ = s.conn.Close()

	if err := g.DeliverCommand("dev-1", []byte("cmd-x")); err != nil {
		t.Fatalf("write-failure DeliverCommand should buffer silently, got %v", err)
	}
	st := g.Stats()
	if st.CmdDelivered != 0 || st.CmdBuffered != 1 {
		t.Fatalf("stats = %+v", st)
	}
	if g.registry.Lookup("dev-1") != nil {
		t.Fatal("dead session must be deregistered after write failure")
	}
	if !g.HandlesDevice("dev-1") {
		t.Fatal("device must stay known after write failure")
	}
}

func TestFlushOnRegisterDeliversFIFO(t *testing.T) {
	g := newTestGateway(Config{Enabled: true}, testDeps(&fakePublisher{}))
	s0, _ := pipeSession(t, "dev-1")
	g.registry.Register(s0)
	g.registry.Deregister(s0)
	for _, c := range []string{"cmd-1", "cmd-2", "cmd-3"} {
		if err := g.DeliverCommand("dev-1", []byte(c)); err != nil {
			t.Fatalf("buffer %s: %v", c, err)
		}
	}

	s, client := pipeSession(t, "dev-1")
	// 读 goroutine 先行：net.Pipe 写阻塞到读侧消费；错误经 channel 回传主 goroutine 断言。
	frames := make(chan []byte, 3)
	readErr := make(chan error, 3)
	go func() {
		for i := 0; i < 3; i++ {
			payload, err := readFrameErr(client, g.cfg.Mode)
			if err != nil {
				readErr <- err
				return
			}
			frames <- payload
		}
	}()
	g.registry.Register(s) // 注册回调触发冲刷

	want := []string{"cmd-1", "cmd-2", "cmd-3"}
	for i, w := range want {
		select {
		case got := <-frames:
			if string(got) != w {
				t.Fatalf("flushed[%d] = %q, want %q（必须按 FIFO）", i, got, w)
			}
		case err := <-readErr:
			t.Fatalf("flushed frame %d read error: %v", i, err)
		case <-time.After(3 * time.Second):
			t.Fatalf("flushed frame %d timeout", i)
		}
	}
	st := g.Stats()
	if st.CmdFlushed != 3 || st.CmdPending != 0 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestFlushOnRegisterWriteFailureKeepsRemainingInOrder(t *testing.T) {
	g := newTestGateway(Config{Enabled: true}, testDeps(&fakePublisher{}))
	s0, _ := pipeSession(t, "dev-1")
	g.registry.Register(s0)
	g.registry.Deregister(s0)
	for _, c := range []string{"cmd-1", "cmd-2"} {
		_ = g.DeliverCommand("dev-1", []byte(c))
	}

	// 注册一条"写必失败"的会话（连接已关）：冲刷应中断且剩余命令按原序塞回。
	s, _ := pipeSession(t, "dev-1")
	_ = s.conn.Close()
	g.registry.Register(s)

	// 先断言计数（drain 是取走语义，先取会清空 pending）。
	st := g.Stats()
	if st.CmdFlushed != 0 || st.CmdPending != 2 {
		t.Fatalf("stats = %+v, want flushed=0 pending=2", st)
	}
	got := g.bufs.drain("dev-1")
	if len(got) != 2 || string(got[0]) != "cmd-1" || string(got[1]) != "cmd-2" {
		t.Fatalf("remaining after failed flush = %v, want [cmd-1 cmd-2]", got)
	}
}
