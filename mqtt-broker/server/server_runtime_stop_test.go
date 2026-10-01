package server

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// stopTestPlugin 是可控 Unload 行为的插件替身：block 不为 nil 时 Unload 阻塞到其关闭。
type stopTestPlugin struct {
	name     string
	block    chan struct{}
	panicMsg string
	unloads  *int32
}

func (p stopTestPlugin) Load(Server) error        { return nil }
func (p stopTestPlugin) HookWrapper() HookWrapper { return HookWrapper{} }
func (p stopTestPlugin) Name() string             { return p.name }
func (p stopTestPlugin) Unload() error {
	atomic.AddInt32(p.unloads, 1)
	if p.panicMsg != "" {
		panic(p.panicMsg)
	}
	if p.block != nil {
		<-p.block
	}
	return nil
}

func withPluginUnloadTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	orig := pluginUnloadTimeout
	pluginUnloadTimeout = d
	t.Cleanup(func() { pluginUnloadTimeout = orig })
}

func TestServerStopReturnsWithinDeadlineWhenClientNeverCloses(t *testing.T) {
	withPluginUnloadTimeout(t, time.Second)
	var unloads int32
	srv := defaultServer()
	srv.plugins = []Plugin{stopTestPlugin{name: "flush", unloads: &unloads}}
	// closed 永不关闭：模拟卡死在写阻塞上的客户端。
	srv.clients["stuck"] = &client{closed: make(chan struct{})}
	var onStop int32
	srv.hooks.OnStop = func(context.Context) { atomic.AddInt32(&onStop, 1) }

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := srv.Stop(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop error = %v, want deadline exceeded", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Stop took %s, want bounded by drain deadline", elapsed)
	}
	if got := atomic.LoadInt32(&unloads); got != 1 {
		t.Fatalf("plugin Unload calls = %d, want 1", got)
	}
	if atomic.LoadInt32(&onStop) != 1 {
		t.Fatal("OnStop was not called after drain timeout")
	}
	select {
	case <-srv.exitedChan:
	default:
		t.Fatal("Stop did not close exitedChan")
	}
}

func TestServerStopBoundsHungPluginUnloadAndContinues(t *testing.T) {
	withPluginUnloadTimeout(t, 50*time.Millisecond)
	var hungUnloads, nextUnloads, panicUnloads int32
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	srv := defaultServer()
	srv.plugins = []Plugin{
		stopTestPlugin{name: "hung", block: release, unloads: &hungUnloads},
		stopTestPlugin{name: "panics", panicMsg: "boom", unloads: &panicUnloads},
		stopTestPlugin{name: "next", unloads: &nextUnloads},
	}
	var onStop int32
	srv.hooks.OnStop = func(context.Context) { atomic.AddInt32(&onStop, 1) }

	start := time.Now()
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("Stop error = %v, want nil when clients drained", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Stop took %s, hung plugin Unload was not bounded", elapsed)
	}
	if atomic.LoadInt32(&hungUnloads) != 1 || atomic.LoadInt32(&panicUnloads) != 1 {
		t.Fatalf("hung/panic Unload calls = %d/%d, want 1/1", atomic.LoadInt32(&hungUnloads), atomic.LoadInt32(&panicUnloads))
	}
	if atomic.LoadInt32(&nextUnloads) != 1 {
		t.Fatal("plugin after a hung/panicking one was not unloaded")
	}
	if atomic.LoadInt32(&onStop) != 1 {
		t.Fatal("OnStop was not called")
	}
}

func TestServerStopIsIdempotent(t *testing.T) {
	var unloads int32
	srv := defaultServer()
	srv.plugins = []Plugin{stopTestPlugin{name: "once", unloads: &unloads}}
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	if got := atomic.LoadInt32(&unloads); got != 1 {
		t.Fatalf("Unload calls = %d, want 1", got)
	}
}
