// 文件用途：场景/Flow 的重复触发幂等（ROADMAP P0.4）。
// 核心逻辑：以 (flow, device, 秒级时刻) 为键吸收定时器亚秒抖动，把"一次触发被放大成多次执行"
// 挡在动作执行之前。
//
// 关键注意事项：
//  1. **与既有频率限制不是一回事**。`allowSceneExecutionRate` 限的是"单位时间内最多几次"，
//     在限额内它照放行——定时器抖动产生的两次触发只要没超限就会被执行两次。
//     幂等限的是"同一次触发只能执行一次"，两者互补，不可互相替代。
//  2. **登记时机必须在条件判定之后**。若在条件匹配之前就登记幂等键，
//     那么"条件其实没命中"的场景也会把键吃掉，同一秒内真正满足条件的上报反而被误杀。
//     因此本检查排在 conditionsMatchScene 之后、allowSceneExecutionRate 之前。
//  3. **存储异常一律 fail open 并记 warn**。与执行窗口读取失败保持同一取向：
//     幂等是"收敛重复"，不是"放行条件"，读不到就沿用既有行为，
//     不因为一个旁路设施故障而让存量场景整体停摆。
//  4. 本实现仅**进程内**有效，重启即失（与移动端命令幂等存储同源取舍）。
//     它要吸收的是毫秒～秒级的抖动，进程内足够；跨实例的强幂等需换成 Redis 实现同一接口。
package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// errSceneTriggerStoreUnavailable 幂等存储不可用（未装配）。
// 调用方按 fail open 处理：宁可放行重复执行，也不让场景整体停摆。
var errSceneTriggerStoreUnavailable = errors.New("scene trigger idempotency store is unavailable")

// sceneTriggerTTL 幂等键保留时长。
// 键已截断到秒，故只需覆盖"同一秒内的重复到达 + 少量时钟回拨"，
// 取 5s 是为了给跨进程/网络排队留余量，同时不至于让 map 长时间堆积。
const sceneTriggerTTL = 5 * time.Second

// sceneTriggerStore 触发幂等键存储。
type sceneTriggerStore interface {
	// Claim 尝试认领键。返回 true 表示首次出现（调用方应继续执行），
	// false 表示该触发已在 TTL 内被认领过（调用方应跳过）。
	Claim(ctx context.Context, key string, now time.Time) (bool, error)
}

// InMemorySceneTriggerStore 进程内触发幂等存储。
// 局限（明确声明，不假装是分布式）：仅当前进程有效，重启即失；多副本部署时
// 每个副本各认领各的，跨副本的重复触发不会被本层吸收。
type InMemorySceneTriggerStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	seen map[string]time.Time
}

// NewInMemorySceneTriggerStore 创建进程内触发幂等存储。ttl<=0 时使用 sceneTriggerTTL。
func NewInMemorySceneTriggerStore(ttl time.Duration) *InMemorySceneTriggerStore {
	if ttl <= 0 {
		ttl = sceneTriggerTTL
	}
	return &InMemorySceneTriggerStore{ttl: ttl, seen: make(map[string]time.Time)}
}

// Claim 认领幂等键；键已存在且未过期时返回 false。
func (s *InMemorySceneTriggerStore) Claim(_ context.Context, key string, now time.Time) (bool, error) {
	if s == nil {
		return false, errSceneTriggerStoreUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen == nil {
		s.seen = make(map[string]time.Time)
	}
	// 先清理再判定：只靠 Claim 时惰性清理也能工作，但写入路径顺带 sweep
	// 可以让"触发一次后再也不触发"的键不会永久占着内存。
	s.sweepLocked(now)
	if _, ok := s.seen[key]; ok {
		return false, nil
	}
	s.seen[key] = now
	return true, nil
}

// sweepLocked 清理过期条目；调用方必须已持有锁。
func (s *InMemorySceneTriggerStore) sweepLocked(now time.Time) {
	for key, at := range s.seen {
		if now.Sub(at) > s.ttl {
			delete(s.seen, key)
		}
	}
}

// sceneTriggerDedupe 可注入的幂等存储，便于测试替换为确定实现。
// 默认使用进程内存储；置为 nil 表示关闭幂等（将走 fail open 分支）。
var sceneTriggerDedupe sceneTriggerStore = NewInMemorySceneTriggerStore(sceneTriggerTTL)

// sceneTriggerIsNew 判定本次触发是否为同一秒内的首次到达。
// 返回 true 表示应继续执行；false 表示重复触发，应跳过。
func (a *Automate) sceneTriggerIsNew(candidate sceneExecutionCandidate) bool {
	if sceneTriggerDedupe == nil {
		// 未装配幂等存储：如实放行，不做"假装去重"。
		return true
	}
	now := time.Now()
	key := FlowTriggerKey(FlowTriggerIdentity{
		FlowID:   candidate.sceneAutomationID,
		DeviceID: candidate.deviceID,
		TriggerAt: now,
	})
	first, err := sceneTriggerDedupe.Claim(context.Background(), key, now)
	if err != nil {
		// fail open：幂等是收敛手段而非放行条件，存储故障不应让场景整体停摆。
		logrus.Warnf("scene trigger idempotency check failed for scene=%s device=%s; allowing execution: %v",
			candidate.sceneAutomationID, candidate.deviceID, err)
		return true
	}
	if !first {
		logrus.Tracef("skip duplicate scene trigger for sceneAutomationID=%s deviceID=%s (same second)",
			candidate.sceneAutomationID, candidate.deviceID)
	}
	return first
}
