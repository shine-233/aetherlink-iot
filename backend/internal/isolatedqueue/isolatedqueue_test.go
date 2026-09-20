package isolatedqueue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStandardQueuePolicies(t *testing.T) {
	ctx := context.Background()

	// 1. DropNewest 满队列测试
	qNewest := NewStandardQueue(QueueConfig{
		Name:       "test-newest",
		Capacity:   2,
		DropPolicy: DropPolicyDropNewest,
	})
	defer qNewest.Close(ctx)

	if err := qNewest.Submit(ctx, &QueueMessage{ID: "m1"}); err != nil {
		t.Fatalf("submit m1 failed: %v", err)
	}
	if err := qNewest.Submit(ctx, &QueueMessage{ID: "m2"}); err != nil {
		t.Fatalf("submit m2 failed: %v", err)
	}
	// m3 应被满队列丢弃
	if err := qNewest.Submit(ctx, &QueueMessage{ID: "m3"}); err != ErrQueueFull {
		t.Fatalf("submit m3 expected ErrQueueFull, got %v", err)
	}
	stats := qNewest.GetStats()
	if stats.TotalDropped != 1 || stats.Size != 2 {
		t.Errorf("mismatch stats: %+v", stats)
	}

	// 2. DropOldest 满队列测试
	qOldest := NewStandardQueue(QueueConfig{
		Name:       "test-oldest",
		Capacity:   2,
		DropPolicy: DropPolicyDropOldest,
	})
	defer qOldest.Close(ctx)

	_ = qOldest.Submit(ctx, &QueueMessage{ID: "old1"})
	_ = qOldest.Submit(ctx, &QueueMessage{ID: "old2"})
	// 投递第 3 个，应该顶掉 old1
	if err := qOldest.Submit(ctx, &QueueMessage{ID: "new3"}); err != nil {
		t.Fatalf("submit new3 failed: %v", err)
	}
	// 读取应先拿到 old2，再拿到 new3
	first := <-qOldest.Out()
	if first.ID != "old2" {
		t.Errorf("expected first msg 'old2', got %q", first.ID)
	}
	second := <-qOldest.Out()
	if second.ID != "new3" {
		t.Errorf("expected second msg 'new3', got %q", second.ID)
	}
}

func TestSequentialQueuePerOriginatorFIFO(t *testing.T) {
	ctx := context.Background()
	q := NewSequentialQueue(QueueConfig{
		Name:           "test-seq",
		Capacity:       100,
		PartitionCount: 4,
	})
	defer q.Close(ctx)

	const count = 30
	originatorA := "device-001"

	// 串行写入同设备 30 条消息
	for i := 0; i < count; i++ {
		err := q.SubmitByOriginator(ctx, originatorA, &QueueMessage{
			ID:           fmt.Sprintf("msg-%d", i),
			OriginatorID: originatorA,
		})
		if err != nil {
			t.Fatalf("submit %d error: %v", i, err)
		}
	}

	// 验证消费出来的消息严格保持 0 到 count-1 顺序（单实体严格 FIFO）
	for i := 0; i < count; i++ {
		select {
		case msg := <-q.Out():
			expectedID := fmt.Sprintf("msg-%d", i)
			if msg.ID != expectedID {
				t.Fatalf("FIFO violation at index %d: expected %s, got %s", i, expectedID, msg.ID)
			}
			q.Ack(msg)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}

	stats := q.GetStats()
	if stats.TotalProcessed != count {
		t.Errorf("expected %d processed, got %d", count, stats.TotalProcessed)
	}
}

func TestSequentialQueueParallelDifferentOriginators(t *testing.T) {
	ctx := context.Background()
	q := NewSequentialQueue(QueueConfig{
		Name:           "test-parallel-seq",
		Capacity:       200,
		PartitionCount: 8,
	})
	defer q.Close(ctx)

	// 并发向 10 个不同设备投递
	var wg sync.WaitGroup
	for d := 0; d < 10; d++ {
		wg.Add(1)
		go func(devIdx int) {
			defer wg.Done()
			devID := fmt.Sprintf("dev-%d", devIdx)
			for m := 0; m < 5; m++ {
				_ = q.SubmitByOriginator(ctx, devID, &QueueMessage{
					ID:           fmt.Sprintf("%s-m%d", devID, m),
					OriginatorID: devID,
				})
			}
		}(d)
	}
	wg.Wait()

	// 消费 50 条消息
	consumed := 0
	timeout := time.After(3 * time.Second)
	for consumed < 50 {
		select {
		case msg := <-q.Out():
			q.Ack(msg)
			consumed++
		case <-timeout:
			t.Fatalf("timed out after consuming %d/50 messages", consumed)
		}
	}

	if consumed != 50 {
		t.Errorf("expected 50 consumed, got %d", consumed)
	}
}

func TestQueueManagerStandardQueuesAndMetrics(t *testing.T) {
	ctx := context.Background()
	mgr := NewQueueManager()
	defer mgr.Close(ctx)

	// 1. 验证默认三大队列均已初始化
	for _, qName := range []string{QueueTypeMain, QueueTypeHighPriority, QueueTypeSequentialByOriginator} {
		q, ok := mgr.GetQueue(qName)
		if !ok || q == nil {
			t.Fatalf("default queue %q not registered", qName)
		}
	}

	// 2. 向 Main 队列投递消息
	if err := mgr.Submit(ctx, QueueTypeMain, &QueueMessage{ID: "m-telemetry-1", Type: "telemetry"}); err != nil {
		t.Fatalf("Submit to Main error: %v", err)
	}

	// 3. 向 HighPriority 投递紧急告警
	if err := mgr.Submit(ctx, QueueTypeHighPriority, &QueueMessage{ID: "m-alarm-1", Type: "alarm"}); err != nil {
		t.Fatalf("Submit to HighPriority error: %v", err)
	}

	// 4. 统计与度量验证
	allStats := mgr.GetAllStats()
	if len(allStats) != 3 {
		t.Fatalf("expected 3 queue stats, got %d", len(allStats))
	}

	mainStats, err := mgr.GetQueueStats(QueueTypeMain)
	if err != nil || mainStats.TotalSubmitted != 1 || mainStats.Size != 1 {
		t.Errorf("mainStats error or mismatch: %+v, err: %v", mainStats, err)
	}

	highStats, err := mgr.GetQueueStats(QueueTypeHighPriority)
	if err != nil || highStats.TotalSubmitted != 1 || highStats.Size != 1 {
		t.Errorf("highStats error or mismatch: %+v, err: %v", highStats, err)
	}
}
