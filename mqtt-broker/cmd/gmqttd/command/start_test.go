// 文件用途：维护 cmd\gmqttd\command\start_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package command

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt/config"
)

func TestNewStartCmdExposesStartCommandWithoutRunningBroker(t *testing.T) {
	cmd := NewStartCmd()

	if cmd.Use != "start" {
		t.Fatalf("command use = %q, want start", cmd.Use)
	}
	if cmd.Short != "Start gmqtt broker" {
		t.Fatalf("command short = %q", cmd.Short)
	}
	if cmd.Run == nil {
		t.Fatal("start command must install a Run handler")
	}
}

func TestGetListenersBuildsWebsocketServerWithoutOpeningTCPPort(t *testing.T) {
	listeners, websockets, err := GetListeners(config.Config{
		Listeners: []*config.ListenerConfig{
			{
				Address: "127.0.0.1:0",
				Websocket: &config.WebsocketOptions{
					Path: "/mqtt",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GetListeners returned error: %v", err)
	}
	if len(listeners) != 0 {
		t.Fatalf("websocket listener should not create TCP listeners, got %d", len(listeners))
	}
	if len(websockets) != 1 {
		t.Fatalf("websocket servers = %d, want 1", len(websockets))
	}
	if websockets[0].Server.Addr != "127.0.0.1:0" || websockets[0].Path != "/mqtt" {
		t.Fatalf("websocket server = addr %q path %q", websockets[0].Server.Addr, websockets[0].Path)
	}
}

type fakeLifecycleServer struct {
	mu           sync.Mutex
	applied      []config.Config
	stopCalls    int
	stopDeadline time.Time
	stopErr      error
}

func (f *fakeLifecycleServer) ApplyConfig(c config.Config) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, c)
}

func (f *fakeLifecycleServer) Stop(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
	f.stopDeadline, _ = ctx.Deadline()
	return f.stopErr
}

// runLoop 在后台运行信号循环，返回其结束时的错误通道。
func runLoop(t *testing.T, l signalLoop) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- l.run() }()
	return done
}

func waitLoop(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("signal loop did not exit after stop signal")
		return nil
	}
}

func TestSignalLoopFailedReloadStillHonoursStop(t *testing.T) {
	srv := &fakeLifecycleServer{}
	reloadCh := make(chan os.Signal, 1)
	stopCh := make(chan os.Signal, 1)
	loadCalls := make(chan struct{}, 4)
	released := false

	done := runLoop(t, signalLoop{
		srv:      srv,
		reloadCh: reloadCh,
		stopCh:   stopCh,
		loadConfig: func() (config.Config, error) {
			loadCalls <- struct{}{}
			return config.Config{}, errors.New("bad yaml")
		},
		shutdownTimeout: 3 * time.Second,
		releaseSignals:  func() { released = true },
	})

	// 两次失败的热加载：旧实现第一次就 return，SIGTERM 从此被吞掉。
	for i := 0; i < 2; i++ {
		reloadCh <- syscall.SIGHUP
		select {
		case <-loadCalls:
		case <-time.After(2 * time.Second):
			t.Fatalf("reload %d was not processed; loop exited after failed reload", i)
		}
	}
	before := time.Now()
	stopCh <- syscall.SIGTERM
	if err := waitLoop(t, done); err != nil {
		t.Fatalf("run returned %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.stopCalls != 1 {
		t.Fatalf("Stop calls = %d, want 1", srv.stopCalls)
	}
	if len(srv.applied) != 0 {
		t.Fatalf("failed reload applied config %d times", len(srv.applied))
	}
	if srv.stopDeadline.IsZero() {
		t.Fatal("Stop ctx has no deadline; one stuck client would block shutdown forever")
	}
	if d := srv.stopDeadline.Sub(before); d <= 0 || d > 4*time.Second {
		t.Fatalf("Stop deadline %s from signal, want ~3s", d)
	}
	if !released {
		t.Fatal("signal capture was not released before Stop")
	}
}

func TestSignalLoopSuccessfulReloadAppliesConfig(t *testing.T) {
	srv := &fakeLifecycleServer{stopErr: context.DeadlineExceeded}
	reloadCh := make(chan os.Signal, 1)
	stopCh := make(chan os.Signal, 1)
	want := config.Config{PidFile: "reloaded.pid"}

	done := runLoop(t, signalLoop{
		srv:        srv,
		reloadCh:   reloadCh,
		stopCh:     stopCh,
		loadConfig: func() (config.Config, error) { return want, nil },
	})
	reloadCh <- syscall.SIGHUP
	deadline := time.Now().Add(2 * time.Second)
	for {
		srv.mu.Lock()
		n := len(srv.applied)
		srv.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reload was not applied")
		}
		time.Sleep(5 * time.Millisecond)
	}
	stopCh <- os.Interrupt
	if err := waitLoop(t, done); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run error = %v, want Stop error propagated", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.applied[0].PidFile != want.PidFile {
		t.Fatalf("applied config = %+v", srv.applied[0])
	}
	if srv.stopDeadline.IsZero() {
		t.Fatal("default shutdown timeout not applied")
	}
}

func TestShutdownTimeoutFromEnv(t *testing.T) {
	cases := []struct {
		raw      string
		want     time.Duration
		wantWarn bool
	}{
		{"", defaultShutdownTimeout, false},
		{"45s", 45 * time.Second, false},
		{"1m30s", 90 * time.Second, false},
		{"abc", defaultShutdownTimeout, true},
		{"0s", defaultShutdownTimeout, true},
		{"-5s", defaultShutdownTimeout, true},
	}
	for _, tc := range cases {
		got, warn := shutdownTimeoutFromEnv(tc.raw)
		if got != tc.want || (warn != "") != tc.wantWarn {
			t.Errorf("shutdownTimeoutFromEnv(%q) = %s, warn=%q", tc.raw, got, warn)
		}
	}
}
