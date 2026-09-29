package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const poolTestTimeout = 5 * time.Second

func waitOrFail(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(poolTestTimeout):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// keysOnDistinctShards 找出落在不同分片上的两个键。
func keysOnDistinctShards(t *testing.T, p *shardedPool) (string, string) {
	t.Helper()
	first := "device-0"
	for i := 1; i < 1000; i++ {
		k := fmt.Sprintf("device-%d", i)
		if p.shardFor(k) != p.shardFor(first) {
			return first, k
		}
	}
	t.Fatal("could not find keys on distinct shards")
	return "", ""
}

// 多设备、多生产者并发投递：每个设备的执行顺序必须与其投递顺序完全一致，且无丢失。
func TestShardedPoolPreservesPerKeyOrderUnderConcurrency(t *testing.T) {
	const devices, perDevice = 40, 200
	p := newShardedPool(AutomationPoolConfig{Workers: 8, QueueSize: devices * perDevice})

	var mu sync.Mutex
	seen := make(map[string][]int, devices)
	var producers sync.WaitGroup
	for d := 0; d < devices; d++ {
		key := fmt.Sprintf("device-%d", d)
		producers.Add(1)
		go func() {
			defer producers.Done()
			for i := 0; i < perDevice; i++ {
				seq := i
				if err := p.submit(key, func() {
					mu.Lock()
					seen[key] = append(seen[key], seq)
					mu.Unlock()
				}); err != nil {
					t.Errorf("submit %s#%d: %v", key, seq, err)
					return
				}
			}
		}()
	}
	producers.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
	defer cancel()
	if err := p.stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if len(seen) != devices {
		t.Fatalf("devices seen = %d, want %d", len(seen), devices)
	}
	for key, got := range seen {
		if len(got) != perDevice {
			t.Fatalf("%s executed %d tasks, want %d", key, len(got), perDevice)
		}
		for i, seq := range got {
			if seq != i {
				t.Fatalf("%s out of order at %d: got seq %d", key, i, seq)
			}
		}
	}
	stats := p.stats()
	if stats.Submitted != devices*perDevice || stats.Completed != devices*perDevice || stats.Dropped != 0 {
		t.Fatalf("stats = %+v", stats)
	}
}

// 同一设备的任务绝不并发：分片内串行是告警缓存读改写不交错的前提。
func TestShardedPoolNeverRunsSameKeyConcurrently(t *testing.T) {
	p := newShardedPool(AutomationPoolConfig{Workers: 4, QueueSize: 1000})
	var inflight, maxInflight atomic.Int32
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = p.submit("same-device", func() {
					n := inflight.Add(1)
					for {
						m := maxInflight.Load()
						if n <= m || maxInflight.CompareAndSwap(m, n) {
							break
						}
					}
					time.Sleep(10 * time.Microsecond)
					inflight.Add(-1)
				})
			}
		}()
	}
	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
	defer cancel()
	if err := p.stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := maxInflight.Load(); got != 1 {
		t.Fatalf("max concurrent executions for one device = %d, want 1", got)
	}
	if got := p.stats().Completed; got != 500 {
		t.Fatalf("completed = %d, want 500", got)
	}
}

// 一个设备的慢任务不能挡住其它设备（旧实现的全局互斥锁正是这样串行化全部设备）。
func TestShardedPoolRunsDifferentKeysConcurrently(t *testing.T) {
	p := newShardedPool(AutomationPoolConfig{Workers: 4, QueueSize: 4})
	slowKey, fastKey := keysOnDistinctShards(t, p)
	release := make(chan struct{})
	slowStarted := make(chan struct{})
	fastDone := make(chan struct{})

	if err := p.submit(slowKey, func() { close(slowStarted); <-release }); err != nil {
		t.Fatal(err)
	}
	waitOrFail(t, slowStarted, "slow task start")
	if err := p.submit(fastKey, func() { close(fastDone) }); err != nil {
		t.Fatal(err)
	}
	waitOrFail(t, fastDone, "fast device while slow device is blocked")
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
	defer cancel()
	if err := p.stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

// 队列有界：满了立即返回 ErrAutomationQueueFull 并计数，不阻塞调用方。
func TestShardedPoolDropsWhenShardQueueFull(t *testing.T) {
	const queueSize = 3
	p := newShardedPool(AutomationPoolConfig{Workers: 1, QueueSize: queueSize})
	release := make(chan struct{})
	started := make(chan struct{})
	if err := p.submit("d", func() { close(started); <-release }); err != nil {
		t.Fatal(err)
	}
	waitOrFail(t, started, "blocking task start")

	var ran atomic.Int32
	for i := 0; i < queueSize; i++ {
		if err := p.submit("d", func() { ran.Add(1) }); err != nil {
			t.Fatalf("submit %d within capacity: %v", i, err)
		}
	}
	const extra = 5
	for i := 0; i < extra; i++ {
		begin := time.Now()
		if err := p.submit("d", func() { ran.Add(1) }); !errors.Is(err, ErrAutomationQueueFull) {
			t.Fatalf("submit beyond capacity err = %v, want ErrAutomationQueueFull", err)
		}
		if time.Since(begin) > time.Second {
			t.Fatal("submit blocked on a full queue")
		}
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
	defer cancel()
	if err := p.stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stats := p.stats()
	if stats.Dropped != extra || stats.Submitted != queueSize+1 || ran.Load() != queueSize {
		t.Fatalf("stats = %+v ran = %d", stats, ran.Load())
	}
}

// 优雅停机：已入队任务全部执行，停机后的投递被拒绝；重复 stop 安全。
func TestShardedPoolStopDrainsQueuedAndRejectsNew(t *testing.T) {
	p := newShardedPool(AutomationPoolConfig{Workers: 2, QueueSize: 100})
	release := make(chan struct{})
	var ran atomic.Int32
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("device-%d", i%7)
		if err := p.submit(key, func() { <-release; ran.Add(1) }); err != nil {
			t.Fatal(err)
		}
	}
	stopErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
		defer cancel()
		stopErr <- p.stop(ctx)
	}()
	// stop 置位后才放行，确保排空发生在"已停止"状态下。
	deadline := time.Now().Add(poolTestTimeout)
	for !p.isStopped() {
		if time.Now().After(deadline) {
			t.Fatal("pool never entered stopped state")
		}
		time.Sleep(time.Millisecond)
	}
	if err := p.submit("late", func() {}); !errors.Is(err, ErrAutomationPoolStopped) {
		t.Fatalf("submit after stop err = %v, want ErrAutomationPoolStopped", err)
	}
	close(release)
	if err := <-stopErr; err != nil {
		t.Fatalf("stop: %v", err)
	}
	if ran.Load() != 100 {
		t.Fatalf("drained %d tasks, want 100", ran.Load())
	}
	if err := p.stop(context.Background()); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if got := p.stats().Rejected; got != 1 {
		t.Fatalf("rejected = %d, want 1", got)
	}
}

func TestShardedPoolStopHonorsContextDeadline(t *testing.T) {
	p := newShardedPool(AutomationPoolConfig{Workers: 1, QueueSize: 1})
	release := make(chan struct{})
	started := make(chan struct{})
	_ = p.submit("d", func() { close(started); <-release })
	waitOrFail(t, started, "blocking task start")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop err = %v, want deadline exceeded", err)
	}
	close(release)
}

// 任务 panic 不得杀死 worker：同分片后续任务照常执行。
func TestShardedPoolRecoversTaskPanic(t *testing.T) {
	p := newShardedPool(AutomationPoolConfig{Workers: 1, QueueSize: 4})
	done := make(chan struct{})
	_ = p.submit("d", func() { panic("boom") })
	_ = p.submit("d", func() { close(done) })
	waitOrFail(t, done, "task after panic")
	ctx, cancel := context.WithTimeout(context.Background(), poolTestTimeout)
	defer cancel()
	_ = p.stop(ctx)
	if stats := p.stats(); stats.Panics != 1 || stats.Completed != 2 {
		t.Fatalf("stats = %+v", stats)
	}
}
