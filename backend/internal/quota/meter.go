// 文件用途：TB-17 API 日调用计量器——Redis 实时计数（多实例共享）+ 定期落库 api_usage_daily。
// 核心逻辑：Incr 先做当日 DB 种子对齐（进程/Redis 重启后不丢当日累计），再走原子 Lua 脚本
// INCR；后台协程按周期把脏计数快照落库（GREATEST 幂等，只增不减）。
// 关键注意事项：计量链路 fail-open——Redis 故障回退进程内计数、DB 落库失败保留脏标记下轮重试，
// 任何失败都只告警不抛给请求路径；被拒请求同样计数（计费口径，见 decision.go 注释）。
// 重构建议：若接入消息队列，可将落库改为事件驱动；当前周期快照已满足按日执法精度。
package quota

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

//go:embed api_daily_incr.lua
var apiDailyIncrScript string

const (
	// redisAPIUsageKeyPrefix 当日计数键前缀；键含日期，次日自动启用新键，旧键随 TTL 过期。
	redisAPIUsageKeyPrefix = "aetherlink:quota:api_daily:"
	// defaultFlushInterval 计量落库周期：执法用 Redis 实时值，本表只做持久化快照，30s 足够。
	defaultFlushInterval = 30 * time.Second
	// redisCallTimeout Redis 调用短超时：与限流中间件同口径，抖动至多拖慢 2s。
	redisCallTimeout = 2 * time.Second
	// meterKeyTTLBuffer 计数键在次日零点后多保留的时长：给在途落库留读取窗口。
	meterKeyTTLBuffer = 5 * time.Minute
)

// meterEntry 进程内的租户当日计量镜像：seeded 表示已完成当日 DB 对齐，dirty 表示尚未落库。
type meterEntry struct {
	date   string
	count  int64
	seeded bool
	dirty  bool
}

// DailyMeter API 日调用计量器。Redis 可用时为多实例共享的权威计数；否则退化为进程内计数。
type DailyMeter struct {
	mu     sync.Mutex
	now    func() time.Time
	counts map[string]*meterEntry // key: tenantID

	redis *redis.Client

	// 可注入的持久化函数（默认 dal 实现；单测替换，避免依赖真实 DB）。
	seedLoader  func(tenantID, date string) (int64, error)
	persistFunc func(tenantID, date string, calls int64) error

	flushInterval time.Duration
	stopCh        chan struct{}
	stopOnce      sync.Once

	// 告警节流：fail-open 期间每分钟至多一条告警，防日志风暴。
	warnMu      sync.Mutex
	lastWarnAt  time.Time
	failOpenCnt uint64
}

// NewDailyMeter 构建计量器；rdb 为 nil 时使用纯进程内计数（单机/降级）。
func NewDailyMeter(rdb *redis.Client, flushInterval time.Duration) *DailyMeter {
	if flushInterval <= 0 {
		flushInterval = defaultFlushInterval
	}
	return &DailyMeter{
		now:           time.Now,
		counts:        make(map[string]*meterEntry),
		redis:         rdb,
		seedLoader:    dal.GetAPIUsage,
		persistFunc:   dal.UpsertAPIUsage,
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
	}
}

// SetClock 注入时钟（单测用）：翻转日期边界用例不依赖真实时间。
func (m *DailyMeter) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

// SetPersistence 注入种子读取与落库函数（单测用）。
func (m *DailyMeter) SetPersistence(seed func(tenantID, date string) (int64, error), persist func(tenantID, date string, calls int64) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seed != nil {
		m.seedLoader = seed
	}
	if persist != nil {
		m.persistFunc = persist
	}
}

// StartFlusher 启动后台落库协程；重复调用是幂等的。
func (m *DailyMeter) StartFlusher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				// 退出前兜底落库一次，压低进程重启窗口内的计量丢失。
				m.FlushOnce(context.WithoutCancel(ctx))
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.FlushOnce(context.Background())
			}
		}
	}()
}

// StopFlusher 停止后台落库协程（进程优雅退出与单测收尾用）。
func (m *DailyMeter) StopFlusher() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// Incr 同日累加一次 API 调用并返回累加后的当日计数（含本次）。
// 错误仅表示"计数不可用"——调用方必须 fail-open，不得以本错误拒绝请求。
func (m *DailyMeter) Incr(ctx context.Context, tenantID string) (int64, error) {
	if tenantID == "" {
		return 0, fmt.Errorf("quota: empty tenant for api usage metering")
	}
	now := m.now()
	today := UsageDate(now)

	seed, err := m.ensureSeed(tenantID, today)
	if err != nil {
		return 0, err
	}

	if m.redis != nil {
		if count, rerr := m.redisIncr(ctx, tenantID, today, seed, now); rerr == nil {
			m.updateMirror(tenantID, today, count)
			return count, nil
		} else {
			m.warnFailOpen(fmt.Errorf("redis incr unavailable, fallback to in-memory metering: %w", rerr))
		}
	}
	count := m.memoryIncr(tenantID, today, seed)
	m.updateMirror(tenantID, today, count)
	return count, nil
}

// TodayUsage 读取租户当日已用调用数：Redis 在用时读权威计数，否则读进程镜像；
// 两者皆无（冷启动）回落 DB 落库值。供配额查询端点展示，同样 fail-open。
func (m *DailyMeter) TodayUsage(ctx context.Context, tenantID string) (int64, error) {
	if tenantID == "" {
		return 0, fmt.Errorf("quota: empty tenant for api usage query")
	}
	today := UsageDate(m.now())

	if m.redis != nil {
		readCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
		defer cancel()
		val, err := m.redis.Get(readCtx, m.redisKey(today, tenantID)).Int64()
		if err == nil {
			return val, nil
		}
		if err != redis.Nil {
			m.warnFailOpen(fmt.Errorf("redis get today usage failed, fallback: %w", err))
		}
	}

	m.mu.Lock()
	entry, ok := m.counts[tenantID]
	m.mu.Unlock()
	if ok && entry.date == today && entry.seeded {
		return entry.count, nil
	}

	usage, err := m.seedLoader(tenantID, today)
	if err != nil {
		return 0, err
	}
	return usage, nil
}

// FlushOnce 把脏计数快照落库（幂等，GREATEST 只增不减）；错误仅告警，脏标记保留待下轮。
func (m *DailyMeter) FlushOnce(ctx context.Context) {
	type pending struct {
		tenantID string
		date     string
		count    int64
	}
	m.mu.Lock()
	pendings := make([]pending, 0, len(m.counts))
	for tenantID, entry := range m.counts {
		if entry.dirty {
			pendings = append(pendings, pending{tenantID: tenantID, date: entry.date, count: entry.count})
		}
	}
	m.mu.Unlock()
	if len(pendings) == 0 {
		return
	}

	for _, p := range pendings {
		value := p.count
		if m.redis != nil {
			// Redis 是权威计数：落库前读取权威值，避免本进程镜像落后于其他实例。
			readCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
			val, err := m.redis.Get(readCtx, m.redisKey(p.date, p.tenantID)).Int64()
			cancel()
			if err == nil {
				value = val
			} else if err != redis.Nil {
				m.warnFailOpen(fmt.Errorf("flush read canonical count failed: %w", err))
				continue // 读不到权威值就不落库，保留脏标记
			}
		}
		if err := m.persistFunc(p.tenantID, p.date, value); err != nil {
			m.warnFailOpen(fmt.Errorf("persist api usage failed: %w", err))
			continue
		}
		m.mu.Lock()
		if entry, ok := m.counts[p.tenantID]; ok && entry.date == p.date {
			entry.dirty = false
			if entry.count < value {
				entry.count = value
			}
		}
		m.mu.Unlock()
	}
}

// ensureSeed 保证当日镜像已与 DB 落库值对齐（每租户每天至多一次 DB 读）。
// 进程或 Redis 重启后，当日累计从 DB 快照续起，执法不会从 0 重新放行一整天。
func (m *DailyMeter) ensureSeed(tenantID, today string) (int64, error) {
	m.mu.Lock()
	entry, ok := m.counts[tenantID]
	seeded := ok && entry.date == today && entry.seeded
	m.mu.Unlock()
	if seeded {
		return 0, nil
	}
	usage, err := m.seedLoader(tenantID, today)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	entry, ok = m.counts[tenantID]
	if !ok || entry.date != today {
		entry = &meterEntry{date: today, count: usage}
		m.counts[tenantID] = entry
	}
	entry.seeded = true
	if entry.count < usage {
		entry.count = usage
	}
	m.mu.Unlock()
	return usage, nil
}

// redisIncr 执行原子累加脚本：键缺失先写 DB 种子，再 INCR，并维护至次日零点的 TTL。
func (m *DailyMeter) redisIncr(ctx context.Context, tenantID, today string, seed int64, now time.Time) (int64, error) {
	callCtx, cancel := context.WithTimeout(ctx, redisCallTimeout)
	defer cancel()
	ttlMs := (SecondsToNextUTCMidnight(now) + int64(meterKeyTTLBuffer.Seconds())) * 1000
	return m.redis.Eval(callCtx, apiDailyIncrScript,
		[]string{m.redisKey(today, tenantID)},
		seed, ttlMs).Int64()
}

// memoryIncr 进程内累加（Redis 缺失/故障时的降级路径）：当日新建镜像时以种子值续起。
func (m *DailyMeter) memoryIncr(tenantID, today string, seed int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.counts[tenantID]
	if !ok || entry.date != today {
		entry = &meterEntry{date: today, count: seed}
		m.counts[tenantID] = entry
	}
	entry.seeded = true
	entry.count++
	entry.dirty = true
	return entry.count
}

// updateMirror 用权威计数刷新进程镜像并标记待落库。
func (m *DailyMeter) updateMirror(tenantID, today string, count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.counts[tenantID]
	if !ok || entry.date != today {
		entry = &meterEntry{date: today}
		m.counts[tenantID] = entry
	}
	entry.seeded = true
	entry.count = count
	entry.dirty = true
}

// redisKey 当日计数键：日期内嵌，跨日自然切换新键。
func (*DailyMeter) redisKey(date, tenantID string) string {
	return fmt.Sprintf("%s%s:%s", redisAPIUsageKeyPrefix, date, tenantID)
}

// warnFailOpen 记录 fail-open（每分钟至多一条）。
func (m *DailyMeter) warnFailOpen(err error) {
	m.warnMu.Lock()
	defer m.warnMu.Unlock()
	m.failOpenCnt++
	now := time.Now()
	if now.Sub(m.lastWarnAt) < time.Minute {
		return
	}
	m.lastWarnAt = now
	logrus.WithError(err).Warnf("quota: api 用量计量降级 fail-open（累计 %d 次）", m.failOpenCnt)
}
