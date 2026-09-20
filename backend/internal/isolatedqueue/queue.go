package isolatedqueue

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// IsolatedQueue 隔离队列通用接口。
type IsolatedQueue interface {
	Name() string
	Submit(ctx context.Context, msg *QueueMessage) error
	SubmitByOriginator(ctx context.Context, originatorID string, msg *QueueMessage) error
	Out() <-chan *QueueMessage
	Ack(msg *QueueMessage)
	GetStats() QueueStats
	Close(ctx context.Context) error
}

// ==================== 标准缓冲隔离队列（Main / HighPriority） ====================

type StandardQueue struct {
	config QueueConfig
	ch     chan *QueueMessage

	totalSubmitted atomic.Int64
	totalProcessed atomic.Int64
	totalDropped   atomic.Int64

	lastActivityMu sync.RWMutex
	lastActivityAt time.Time

	mu     sync.Mutex
	closed bool
}

func NewStandardQueue(config QueueConfig) *StandardQueue {
	if config.Capacity <= 0 {
		config.Capacity = 1000
	}
	if config.DropPolicy == "" {
		config.DropPolicy = DropPolicyBackpressure
	}
	if config.SubmitStrategy == "" {
		config.SubmitStrategy = SubmitStrategyBurst
	}

	return &StandardQueue{
		config:         config,
		ch:             make(chan *QueueMessage, config.Capacity),
		lastActivityAt: time.Now(),
	}
}

func (q *StandardQueue) Name() string {
	return q.config.Name
}

func (q *StandardQueue) Out() <-chan *QueueMessage {
	return q.ch
}

func (q *StandardQueue) Ack(msg *QueueMessage) {
	q.totalProcessed.Add(1)
	q.touchActivity()
}

func (q *StandardQueue) touchActivity() {
	q.lastActivityMu.Lock()
	q.lastActivityAt = time.Now()
	q.lastActivityMu.Unlock()
}

func (q *StandardQueue) Submit(ctx context.Context, msg *QueueMessage) error {
	if msg == nil {
		return ErrInvalidMsg
	}
	return q.SubmitByOriginator(ctx, msg.OriginatorID, msg)
}

func (q *StandardQueue) SubmitByOriginator(ctx context.Context, originatorID string, msg *QueueMessage) error {
	if msg == nil {
		return ErrInvalidMsg
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	q.mu.Unlock()

	msg.QueueName = q.config.Name
	if msg.OriginatorID == "" {
		msg.OriginatorID = originatorID
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	q.totalSubmitted.Add(1)
	q.touchActivity()

	switch q.config.DropPolicy {
	case DropPolicyDropNewest:
		select {
		case q.ch <- msg:
			return nil
		default:
			q.totalDropped.Add(1)
			return ErrQueueFull
		}

	case DropPolicyDropOldest:
		select {
		case q.ch <- msg:
			return nil
		default:
			// 弹出最旧元素为新元素腾出空间
			select {
			case <-q.ch:
				q.totalDropped.Add(1)
			default:
			}
			select {
			case q.ch <- msg:
				return nil
			default:
				q.totalDropped.Add(1)
				return ErrQueueFull
			}
		}

	default: // DropPolicyBackpressure
		select {
		case q.ch <- msg:
			return nil
		case <-ctx.Done():
			q.totalDropped.Add(1)
			return ctx.Err()
		}
	}
}

func (q *StandardQueue) GetStats() QueueStats {
	size := len(q.ch)
	cap := q.config.Capacity

	q.lastActivityMu.RLock()
	lastAct := q.lastActivityAt
	q.lastActivityMu.RUnlock()

	health := HealthStatusHealthy
	ratio := float64(size) / float64(cap)
	if ratio >= 0.95 {
		health = HealthStatusCriticalFull
	} else if ratio >= 0.70 {
		health = HealthStatusWarning
	}

	return QueueStats{
		Name:           q.config.Name,
		Strategy:       q.config.SubmitStrategy,
		Capacity:       cap,
		Size:           size,
		TotalSubmitted: q.totalSubmitted.Load(),
		TotalProcessed: q.totalProcessed.Load(),
		TotalDropped:   q.totalDropped.Load(),
		HealthStatus:   health,
		LastActivityAt: lastAct,
	}
}

func (q *StandardQueue) Close(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.ch)
	return nil
}

// ==================== 按源实体保序隔离队列（SequentialByOriginator） ====================

type SequentialQueue struct {
	config     QueueConfig
	partitions []chan *QueueMessage
	outChan    chan *QueueMessage

	totalSubmitted atomic.Int64
	totalProcessed atomic.Int64
	totalDropped   atomic.Int64

	lastActivityMu sync.RWMutex
	lastActivityAt time.Time

	mu     sync.Mutex
	closed bool
	stopCh chan struct{}
}

func NewSequentialQueue(config QueueConfig) *SequentialQueue {
	if config.Capacity <= 0 {
		config.Capacity = 2000
	}
	if config.PartitionCount <= 0 {
		config.PartitionCount = 16 // 默认 16 个分区哈希桶
	}
	config.SubmitStrategy = SubmitStrategySequentialByOriginator
	if config.DropPolicy == "" {
		config.DropPolicy = DropPolicyBackpressure
	}

	partCap := config.Capacity / config.PartitionCount
	if partCap < 10 {
		partCap = 10
	}

	partitions := make([]chan *QueueMessage, config.PartitionCount)
	for i := 0; i < config.PartitionCount; i++ {
		partitions[i] = make(chan *QueueMessage, partCap)
	}

	q := &SequentialQueue{
		config:         config,
		partitions:     partitions,
		outChan:        make(chan *QueueMessage, config.Capacity),
		stopCh:         make(chan struct{}),
		lastActivityAt: time.Now(),
	}

	// 启动各分区保序消费转发器：每个分区单协程消费，保证同分区的消息按 FIFO 顺序进入 outChan
	for i := 0; i < config.PartitionCount; i++ {
		go q.runPartitionWorker(partitions[i])
	}

	return q
}

func (q *SequentialQueue) runPartitionWorker(partChan <-chan *QueueMessage) {
	for {
		select {
		case <-q.stopCh:
			return
		case msg, ok := <-partChan:
			if !ok {
				return
			}
			// 顺序送入输出通道
			select {
			case <-q.stopCh:
				return
			case q.outChan <- msg:
			}
		}
	}
}

func (q *SequentialQueue) Name() string {
	return q.config.Name
}

func (q *SequentialQueue) Out() <-chan *QueueMessage {
	return q.outChan
}

func (q *SequentialQueue) Ack(msg *QueueMessage) {
	q.totalProcessed.Add(1)
	q.touchActivity()
}

func (q *SequentialQueue) touchActivity() {
	q.lastActivityMu.Lock()
	q.lastActivityAt = time.Now()
	q.lastActivityMu.Unlock()
}

func (q *SequentialQueue) hashOriginator(originatorID string) int {
	if originatorID == "" {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(originatorID))
	return int(h.Sum32() % uint32(q.config.PartitionCount))
}

func (q *SequentialQueue) Submit(ctx context.Context, msg *QueueMessage) error {
	if msg == nil {
		return ErrInvalidMsg
	}
	return q.SubmitByOriginator(ctx, msg.OriginatorID, msg)
}

func (q *SequentialQueue) SubmitByOriginator(ctx context.Context, originatorID string, msg *QueueMessage) error {
	if msg == nil {
		return ErrInvalidMsg
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	q.mu.Unlock()

	msg.QueueName = q.config.Name
	if msg.OriginatorID == "" {
		msg.OriginatorID = originatorID
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	partIndex := q.hashOriginator(msg.OriginatorID)
	partChan := q.partitions[partIndex]

	q.totalSubmitted.Add(1)
	q.touchActivity()

	switch q.config.DropPolicy {
	case DropPolicyDropNewest:
		select {
		case partChan <- msg:
			return nil
		default:
			q.totalDropped.Add(1)
			return ErrQueueFull
		}

	case DropPolicyDropOldest:
		select {
		case partChan <- msg:
			return nil
		default:
			select {
			case <-partChan:
				q.totalDropped.Add(1)
			default:
			}
			select {
			case partChan <- msg:
				return nil
			default:
				q.totalDropped.Add(1)
				return ErrQueueFull
			}
		}

	default: // DropPolicyBackpressure
		select {
		case partChan <- msg:
			return nil
		case <-ctx.Done():
			q.totalDropped.Add(1)
			return ctx.Err()
		}
	}
}

func (q *SequentialQueue) GetStats() QueueStats {
	totalSize := 0
	for _, p := range q.partitions {
		totalSize += len(p)
	}

	q.lastActivityMu.RLock()
	lastAct := q.lastActivityAt
	q.lastActivityMu.RUnlock()

	health := HealthStatusHealthy
	ratio := float64(totalSize) / float64(q.config.Capacity)
	if ratio >= 0.95 {
		health = HealthStatusCriticalFull
	} else if ratio >= 0.70 {
		health = HealthStatusWarning
	}

	return QueueStats{
		Name:           q.config.Name,
		Strategy:       q.config.SubmitStrategy,
		Capacity:       q.config.Capacity,
		Size:           totalSize,
		TotalSubmitted: q.totalSubmitted.Load(),
		TotalProcessed: q.totalProcessed.Load(),
		TotalDropped:   q.totalDropped.Load(),
		HealthStatus:   health,
		LastActivityAt: lastAct,
	}
}

func (q *SequentialQueue) Close(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.stopCh)
	for _, p := range q.partitions {
		close(p)
	}
	close(q.outChan)
	return nil
}
