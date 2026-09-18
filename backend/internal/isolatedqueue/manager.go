package isolatedqueue

import (
	"context"
	"fmt"
	"sync"
)

var (
	DefaultManager *QueueManager
	mgrOnce        sync.Once
)

// GetDefaultManager 获取或初始化全局单例队列管理器。
func GetDefaultManager() *QueueManager {
	mgrOnce.Do(func() {
		DefaultManager = NewQueueManager()
	})
	return DefaultManager
}

// QueueManager 协调所有隔离队列。
type QueueManager struct {
	mu     sync.RWMutex
	queues map[string]IsolatedQueue
	order  []string
}

// NewQueueManager 创建并初始化默认三队列（Main, HighPriority, SequentialByOriginator）。
func NewQueueManager() *QueueManager {
	m := &QueueManager{
		queues: make(map[string]IsolatedQueue),
		order:  make([]string, 0),
	}

	// 1. Main Queue (标准遥测与属性队列，Burst 策略)
	m.Register(NewStandardQueue(QueueConfig{
		Name:           QueueTypeMain,
		Capacity:       10000,
		SubmitStrategy: SubmitStrategyBurst,
		DropPolicy:     DropPolicyBackpressure,
	}))

	// 2. HighPriority Queue (告警/上下线/关键回执队列，专用高优通道，Backpressure 策略)
	m.Register(NewStandardQueue(QueueConfig{
		Name:           QueueTypeHighPriority,
		Capacity:       5000,
		SubmitStrategy: SubmitStrategyBurst,
		DropPolicy:     DropPolicyBackpressure,
	}))

	// 3. SequentialByOriginator Queue (保序队列，按设备哈希多分区并发但单设备 FIFO)
	m.Register(NewSequentialQueue(QueueConfig{
		Name:           QueueTypeSequentialByOriginator,
		Capacity:       5000,
		SubmitStrategy: SubmitStrategySequentialByOriginator,
		DropPolicy:     DropPolicyBackpressure,
		PartitionCount: 16,
	}))

	return m
}

// Register 登记自定义隔离队列。
func (m *QueueManager) Register(q IsolatedQueue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := q.Name()
	if _, exists := m.queues[name]; !exists {
		m.order = append(m.order, name)
	}
	m.queues[name] = q
}

// GetQueue 获取指定队列。
func (m *QueueManager) GetQueue(name string) (IsolatedQueue, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q, ok := m.queues[name]
	return q, ok
}

// Submit 向指定队列投递消息。
func (m *QueueManager) Submit(ctx context.Context, queueName string, msg *QueueMessage) error {
	q, ok := m.GetQueue(queueName)
	if !ok {
		return fmt.Errorf("%w: %s", ErrQueueNotFound, queueName)
	}
	return q.Submit(ctx, msg)
}

// SubmitByOriginator 向指定队列按源实体投递消息。
func (m *QueueManager) SubmitByOriginator(ctx context.Context, queueName string, originatorID string, msg *QueueMessage) error {
	q, ok := m.GetQueue(queueName)
	if !ok {
		return fmt.Errorf("%w: %s", ErrQueueNotFound, queueName)
	}
	return q.SubmitByOriginator(ctx, originatorID, msg)
}

// GetAllStats 返回所有已注册队列的监控指标。
func (m *QueueManager) GetAllStats() []QueueStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make([]QueueStats, 0, len(m.order))
	for _, name := range m.order {
		if q, ok := m.queues[name]; ok {
			stats = append(stats, q.GetStats())
		}
	}
	return stats
}

// GetQueueStats 返回单个队列的指标快照。
func (m *QueueManager) GetQueueStats(queueName string) (QueueStats, error) {
	q, ok := m.GetQueue(queueName)
	if !ok {
		return QueueStats{}, fmt.Errorf("%w: %s", ErrQueueNotFound, queueName)
	}
	return q.GetStats(), nil
}

// Close 优雅关闭所有队列。
func (m *QueueManager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for _, q := range m.queues {
		if err := q.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
