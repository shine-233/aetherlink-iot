// 文件用途：casbin Redis watcher 单测（miniredis）——跨实例通知送达、自通知跳过、
// 关闭后静默、client 缺失拒绝。真实多实例同步在隔离栈启动时验证（conf 门控）。
package casbinwatcher

import (
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/alicebob/miniredis/v2"
)

func newTestWatcher(t *testing.T, server *miniredis.Miniredis, opts ...Option) *Watcher {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	w, err := New(client, opts...)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(w.Close)
	return w
}

// waitSignal 轮询等待信号置位（上限 2s）。
func waitSignal(t *testing.T, hit func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hit() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等待信号超时")
}

func TestWatcherCrossInstanceNotification(t *testing.T) {
	server := miniredis.RunT(t)
	w1 := newTestWatcher(t, server)
	w2 := newTestWatcher(t, server)

	var mu sync.Mutex
	hits1, hits2 := 0, 0
	if err := w1.SetUpdateCallback(func(string) { mu.Lock(); hits1++; mu.Unlock() }); err != nil {
		t.Fatalf("SetUpdateCallback: %v", err)
	}
	if err := w2.SetUpdateCallback(func(string) { mu.Lock(); hits2++; mu.Unlock() }); err != nil {
		t.Fatalf("SetUpdateCallback: %v", err)
	}

	// w1 变更 → w2 必须收到；w1 自通知跳过。
	if err := w1.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	waitSignal(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return hits2 == 1 && hits1 == 0
	})

	// w2 变更 → w1 收到。
	if err := w2.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	waitSignal(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return hits1 == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if hits2 != 1 {
		t.Fatalf("w2 应只收到 1 次通知，实际 %d", hits2)
	}
}

func TestWatcherCloseSilencesUpdate(t *testing.T) {
	server := miniredis.RunT(t)
	w := newTestWatcher(t, server)
	w.Close()
	// 已关闭再 Update：静默成功不报错（关停路径不阻塞业务变更）。
	if err := w.Update(); err != nil {
		t.Fatalf("关闭后 Update 应静默，实际 %v", err)
	}
	w.Close() // 幂等
}

func TestWatcherNilClientRejected(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("nil client 应拒绝")
	}
}

func TestWatcherDefaultChannel(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	w, err := New(client)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(w.Close)
	if w.channel != DefaultChannel {
		t.Fatalf("channel=%q want %q", w.channel, DefaultChannel)
	}
}
