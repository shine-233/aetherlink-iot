package grpcgateway

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// PHASE-D-D9 BEGIN bufconn 全链路测试（注册→心跳→Attach 下行→上行→鉴权负路径）

// memStore 内存注册表桩。
type memStore struct {
	mu     sync.Mutex
	byName map[string]*model.PluginRegistry
}

func newMemStore() *memStore { return &memStore{byName: map[string]*model.PluginRegistry{}} }

func (m *memStore) seed(plugin *model.PluginRegistry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byName[plugin.Name] = plugin
}

func (m *memStore) GetByName(_ context.Context, name string) (*model.PluginRegistry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.byName[name]
	if row == nil {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (m *memStore) GetByID(_ context.Context, id string) (*model.PluginRegistry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.byName {
		if row.ID == id {
			cp := *row
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *memStore) UpdateStatusAndHeartbeat(_ context.Context, id, status string, now time.Time, version *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.byName {
		if row.ID == id {
			row.Status = status
			row.LastHeartbeat = &now
			if version != nil && *version != "" {
				row.Version = *version
			}
			return nil
		}
	}
	return nil
}

type memSink struct {
	mu     sync.Mutex
	frames []*UplinkFrame
}

func (s *memSink) PublishUplink(frame *UplinkFrame, _ *model.PluginRegistry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames = append(s.frames, frame)
	return nil
}

func setupGateway(t *testing.T) (*Gateway, *GatewayClient, *memSink, *memStore) {
	t.Helper()
	store := newMemStore()
	sink := &memSink{}
	gw := New(store, sink, 8)
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(ServerOptions()...)
	RegisterPluginService(server, gw)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		append(ClientOptions(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
		)...)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return gw, NewGatewayClient(conn), sink, store
}

const testToken = "plg_test_token"

func seedPlugin(store *memStore) *model.PluginRegistry {
	plugin := &model.PluginRegistry{
		ID:        "plg-1",
		Name:      "modbus-tcp",
		Status:    model.PluginStatusDisabled,
		TokenHash: HashToken(testToken),
	}
	store.seed(plugin)
	return plugin
}

func TestGatewayBufconnEndToEndD9(t *testing.T) {
	gw, client, sink, store := setupGateway(t)
	ctx := context.Background()
	plugin := seedPlugin(store)

	// 禁用态注册被拒
	if _, err := client.Register(ctx, &RegisterRequest{Name: "modbus-tcp", Token: testToken, Version: "1.0.0"}); err == nil {
		t.Fatalf("禁用态注册应被拒")
	}
	// 启用（等价管理面 SetEnabled）
	plugin.Status = model.PluginStatusOffline
	store.seed(plugin)

	// 注册成功
	reg, err := client.Register(ctx, &RegisterRequest{Name: "modbus-tcp", Token: testToken, Version: "1.0.0"})
	if err != nil || reg.PluginID != "plg-1" || reg.Status != model.PluginStatusOnline {
		t.Fatalf("注册失败: %+v err=%v", reg, err)
	}
	// 心跳
	hb, err := client.Heartbeat(ctx, &HeartbeatRequest{PluginID: "plg-1", Token: testToken, Version: "1.0.1"})
	if err != nil || !hb.Accepted {
		t.Fatalf("心跳失败: %+v err=%v", hb, err)
	}
	// 下行流
	attachCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	recv, err := client.Attach(attachCtx, &AttachRequest{PluginID: "plg-1", Token: testToken})
	if err != nil {
		t.Fatalf("Attach 失败: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !gw.SessionOnline("plg-1") {
		if time.Now().After(deadline) {
			t.Fatalf("会话未上线")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cmd := &DownlinkCommand{CommandID: "cmd-1", DeviceNumber: "gw-1", Identify: "set_speed", Params: map[string]any{"speed": 60}, IssuedAt: 1}
	if err := gw.EnqueueDownlink("plg-1", cmd); err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	got, err := recv()
	if err != nil || got.CommandID != "cmd-1" || got.Params["speed"] != float64(60) {
		t.Fatalf("下行命令不符: %+v err=%v", got, err)
	}
	// 上行
	ack, err := client.Uplink(ctx, &UplinkFrame{PluginID: "plg-1", Token: testToken, DeviceNumber: "gw-1", DataType: "telemetry", Payload: map[string]any{"temperature": 26.5}})
	if err != nil || !ack.Accepted {
		t.Fatalf("上行失败: %+v err=%v", ack, err)
	}
	sink.mu.Lock()
	frameCount := len(sink.frames)
	frame := (*UplinkFrame)(nil)
	if frameCount > 0 {
		frame = sink.frames[0]
	}
	sink.mu.Unlock()
	if frameCount != 1 || frame.DeviceNumber != "gw-1" || frame.Payload["temperature"] != 26.5 {
		t.Fatalf("上行帧不符: n=%d frame=%+v", frameCount, frame)
	}
	// 凭证不匹配
	if _, err := client.Uplink(ctx, &UplinkFrame{PluginID: "plg-1", Token: "wrong"}); err == nil {
		t.Fatalf("错误凭证应被拒")
	}
	// 未知插件
	if _, err := client.Uplink(ctx, &UplinkFrame{PluginID: "plg-404", Token: testToken}); err == nil {
		t.Fatalf("未知插件应被拒")
	}
	// 离线入队被拒（会话仍在，但用未连接的第二个插件名模拟）
	if err := gw.EnqueueDownlink("plg-none", cmd); err == nil {
		t.Fatalf("离线插件入队应被拒")
	}
}

// PHASE-D-D9 END
