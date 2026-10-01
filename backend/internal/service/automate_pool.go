// 文件用途：自动化触发的按设备分片有界工作池。
// 核心逻辑：hash(deviceID) 选定分片，每个分片一个 worker 串行消费一个有界队列：
//   - 同一设备的触发永远落在同一分片，按入队顺序串行执行（保序、且同设备的告警缓存读改写不交错）；
//   - 不同设备分散到不同分片并发执行（替代旧的进程级互斥锁全局串行）；
//   - 队列满立即丢弃并计数，绝不阻塞上行链路；
//   - Stop 拒绝新任务、排空已入队任务，并受 ctx 截止时间约束。
//
// 关键注意事项：worker 内执行的任务不得再同步等待本池（会自锁同一分片）。
package service

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

var (
	// ErrAutomationQueueFull 目标分片队列已满，本次触发被丢弃。
	ErrAutomationQueueFull = errors.New("automation queue full: trigger dropped")
	// ErrAutomationPoolStopped 工作池已停止，不再接受触发。
	ErrAutomationPoolStopped = errors.New("automation pool stopped: trigger rejected")
)

const (
	defaultAutomationPoolWorkers   = 16
	defaultAutomationPoolQueueSize = 256
)

// AutomationPoolConfig 工作池参数。Workers 即分片数（每分片一个 worker）。
type AutomationPoolConfig struct {
	Workers   int
	QueueSize int // 每个分片的队列容量
}

func (c AutomationPoolConfig) normalized() AutomationPoolConfig {
	if c.Workers <= 0 {
		c.Workers = defaultAutomationPoolWorkers
	}
	if c.QueueSize <= 0 {
		c.QueueSize = defaultAutomationPoolQueueSize
	}
	return c
}

// AutomationPoolStats 工作池计数快照。
type AutomationPoolStats struct {
	Submitted uint64 // 成功入队
	Completed uint64 // 已执行完毕（含 panic 恢复）
	Dropped   uint64 // 队列满丢弃
	Rejected  uint64 // 停止后拒绝
	Panics    uint64 // 任务 panic 次数
}

type shardedPool struct {
	mu      sync.RWMutex // 保护 stopped 与分片通道关闭，避免向已关闭通道发送
	stopped bool
	shards  []chan func()
	wg      sync.WaitGroup

	submitted atomic.Uint64
	completed atomic.Uint64
	dropped   atomic.Uint64
	rejected  atomic.Uint64
	panics    atomic.Uint64
}

func newShardedPool(cfg AutomationPoolConfig) *shardedPool {
	cfg = cfg.normalized()
	p := &shardedPool{shards: make([]chan func(), cfg.Workers)}
	for i := range p.shards {
		ch := make(chan func(), cfg.QueueSize)
		p.shards[i] = ch
		p.wg.Add(1)
		go p.worker(ch)
	}
	return p
}

func (p *shardedPool) worker(ch <-chan func()) {
	defer p.wg.Done()
	for task := range ch {
		p.runTask(task)
	}
}

func (p *shardedPool) runTask(task func()) {
	defer func() {
		if r := recover(); r != nil {
			p.panics.Add(1)
			logAutomationPanic("automation pool task", r)
		}
		p.completed.Add(1)
	}()
	task()
}

// shardFor 以 FNV-1a 把键映射到分片；同一键恒定落在同一分片。
func (p *shardedPool) shardFor(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(len(p.shards)))
}

// submit 非阻塞入队。队列满返回 ErrAutomationQueueFull，停止后返回 ErrAutomationPoolStopped。
func (p *shardedPool) submit(key string, task func()) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stopped {
		p.rejected.Add(1)
		return ErrAutomationPoolStopped
	}
	select {
	case p.shards[p.shardFor(key)] <- task:
		p.submitted.Add(1)
		return nil
	default:
		if n := p.dropped.Add(1); n == 1 || n%1000 == 0 {
			logrus.WithFields(logrus.Fields{"key": key, "dropped": n}).
				Warn("automation queue full, dropping trigger")
		}
		return ErrAutomationQueueFull
	}
}

// stop 拒绝新任务并排空已入队任务；ctx 到期先返回 ctx.Err()（worker 仍会在后台跑完）。
func (p *shardedPool) stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.stopped {
		p.stopped = true
		for _, ch := range p.shards {
			close(ch)
		}
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *shardedPool) isStopped() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stopped
}

func (p *shardedPool) stats() AutomationPoolStats {
	return AutomationPoolStats{
		Submitted: p.submitted.Load(),
		Completed: p.completed.Load(),
		Dropped:   p.dropped.Load(),
		Rejected:  p.rejected.Load(),
		Panics:    p.panics.Load(),
	}
}
