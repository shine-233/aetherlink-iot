package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

//go:embed clustered_rate_limit.lua
var clusteredRateLimitScript string

// Evaluator 定义限流评测器接口。
type Evaluator interface {
	// Allow 评测 subject 在 scope 作用域下是否允许通过 rules 规则校验。
	// 返回：allowed 是否允许、retryAfter 建议重试等待秒数、violatedRule 违规规则字符串、err 评测异常。
	Allow(ctx context.Context, scope string, subject string, rules []RateLimitRule) (allowed bool, retryAfter int64, violatedRule string, err error)
}

// ==================== Redis 集群多窗口评测器 ====================

const redisRateLimitPrefix = "aetherlink:ratelimit:"

type RedisClusterEvaluator struct {
	client      *redis.Client
	logger      *logrus.Logger
	mu          sync.Mutex
	lastWarnAt  time.Time
	failOpenCnt uint64
}

func NewRedisClusterEvaluator(client *redis.Client, logger *logrus.Logger) *RedisClusterEvaluator {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &RedisClusterEvaluator{
		client: client,
		logger: logger,
	}
}

func (e *RedisClusterEvaluator) Allow(ctx context.Context, scope string, subject string, rules []RateLimitRule) (bool, int64, string, error) {
	if len(rules) == 0 || subject == "" {
		return true, 0, "", nil
	}

	keys := make([]string, len(rules))
	argv := make([]interface{}, len(rules)*2)
	for i, r := range rules {
		// key 包含 scope, subject 以及窗口大小，保证不同窗口独立计数
		keys[i] = fmt.Sprintf("%s%s:%s:%ds", redisRateLimitPrefix, scope, subject, r.WindowSeconds)
		argv[i*2] = r.Limit
		argv[i*2+1] = r.Window.Milliseconds()
	}

	// 2s 截止时间，防止 Redis 延迟拖垮上行或请求
	evalCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := e.client.Eval(evalCtx, clusteredRateLimitScript, keys, argv...).Slice()
	if err != nil {
		e.warnFailOpen(err)
		return true, 0, "", nil // Fail-open
	}

	if len(res) < 4 {
		e.warnFailOpen(fmt.Errorf("unexpected script return length: %v", res))
		return true, 0, "", nil // Fail-open
	}

	allowedInt, _ := res[0].(int64)
	retryAfter, _ := res[1].(int64)
	violatedLimit, _ := res[2].(int64)
	violatedWindowMs, _ := res[3].(int64)

	if allowedInt == 0 {
		violatedRule := fmt.Sprintf("%d:%d", violatedLimit, (violatedWindowMs+999)/1000)
		return false, retryAfter, violatedRule, nil
	}

	return true, 0, "", nil
}

func (e *RedisClusterEvaluator) warnFailOpen(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failOpenCnt++
	now := time.Now()
	if now.Sub(e.lastWarnAt) < time.Minute {
		return
	}
	e.lastWarnAt = now
	e.logger.WithError(err).Warnf("ratelimit: Redis unavailable or timed out, failing open (total %d times)", e.failOpenCnt)
}

// ==================== 内存多窗口评测器（单机/降级回退） ====================

type memoryWindowEntry struct {
	count     int64
	windowEnd time.Time
}

type MemoryEvaluator struct {
	mu     sync.Mutex
	now    func() time.Time
	counts map[string]*memoryWindowEntry
}

func NewMemoryEvaluator() *MemoryEvaluator {
	return &MemoryEvaluator{
		now:    time.Now,
		counts: make(map[string]*memoryWindowEntry),
	}
}

func (m *MemoryEvaluator) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

func (m *MemoryEvaluator) Allow(_ context.Context, scope string, subject string, rules []RateLimitRule) (bool, int64, string, error) {
	if len(rules) == 0 || subject == "" {
		return true, 0, "", nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	var violatedRule string
	var maxRetryAfter int64 = 0

	// 阶段 1：预检所有窗口是否已达限额
	for _, r := range rules {
		bucketKey := fmt.Sprintf("%s:%s:%ds", scope, subject, r.WindowSeconds)
		entry, exists := m.counts[bucketKey]
		if !exists || now.After(entry.windowEnd) {
			continue
		}

		if entry.count >= r.Limit {
			retry := int64(entry.windowEnd.Sub(now).Seconds()) + 1
			if retry <= 0 {
				retry = 1
			}
			if retry > maxRetryAfter {
				maxRetryAfter = retry
				violatedRule = r.String()
			}
		}
	}

	// 若任一窗口超限，直接拒绝且不消耗任何配额
	if maxRetryAfter > 0 {
		return false, maxRetryAfter, violatedRule, nil
	}

	// 阶段 2：所有窗口均未超限，统一递增消费配额
	for _, r := range rules {
		bucketKey := fmt.Sprintf("%s:%s:%ds", scope, subject, r.WindowSeconds)
		entry, exists := m.counts[bucketKey]
		if !exists || now.After(entry.windowEnd) {
			m.counts[bucketKey] = &memoryWindowEntry{
				count:     1,
				windowEnd: now.Add(r.Window),
			}
		} else {
			entry.count++
		}
	}

	return true, 0, "", nil
}
