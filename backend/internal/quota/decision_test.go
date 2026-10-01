// 文件用途：TB-17 日配额执法判定纯函数（DecideDailyQuota）的单元测试。
// 核心逻辑：表驱动钉死执法边界——未配置阈值放行、80%/100% 状态带、超限拒绝与
// Retry-After（距次日 UTC 零点秒数、向上取整）、日期口径（UsageDate）。
// 关键注意事项：全部用固定时刻断言，不依赖真实时钟；这是执法语义的唯一权威锚点，
// 改判定语义必须先改这里再改实现。
// 重构建议：引入时区感知日窗口时，在本文件补跨时区用例。
package quota

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/stretchr/testify/assert"
)

// fixedNow 固定时刻：2026-09-25 10:30:15 UTC（周五上午），距次日零点 13h29m45s = 48585s。
var fixedNow = time.Date(2026, 9, 25, 10, 30, 15, 0, time.UTC)

func TestDecideDailyQuotaUnlimitedWhenLimitNotConfigured(t *testing.T) {
	for _, limit := range []int64{0, -1, -100} {
		decision := DecideDailyQuota(limit, 999999, fixedNow)
		assert.False(t, decision.Enforced, "limit %d must not enforce", limit)
		assert.True(t, decision.Allowed, "limit %d must allow", limit)
		assert.Equal(t, model.QuotaStatusUnlimited, decision.Status)
		assert.Equal(t, int64(-1), decision.Remaining, "unlimited sentinel remaining")
		assert.Zero(t, decision.RetryAfter)
		assert.Zero(t, decision.UsagePct)
	}
}

func TestDecideDailyQuotaAllowsBelowThreshold(t *testing.T) {
	// free 套餐 5000/日，用 1234 次：normal 放行，剩余 3766。
	decision := DecideDailyQuota(5000, 1234, fixedNow)
	assert.True(t, decision.Enforced)
	assert.True(t, decision.Allowed)
	assert.Equal(t, model.QuotaStatusNormal, decision.Status)
	assert.Equal(t, int64(3766), decision.Remaining)
	assert.Zero(t, decision.RetryAfter)
	assert.Equal(t, 24.7, decision.UsagePct)
}

func TestDecideDailyQuotaWarnsAtEightyPercent(t *testing.T) {
	// 80.0% 恰好进 warning；79.9% 仍是 normal（边界含 80 不含 100）。
	warn := DecideDailyQuota(100, 80, fixedNow)
	assert.True(t, warn.Allowed)
	assert.Equal(t, model.QuotaStatusWarning, warn.Status)
	assert.Equal(t, int64(20), warn.Remaining)
	assert.Equal(t, 80.0, warn.UsagePct)

	normal := DecideDailyQuota(1000, 799, fixedNow)
	assert.Equal(t, model.QuotaStatusNormal, normal.Status)
}

func TestDecideDailyQuotaAllowsLastCallAndMarksExceeded(t *testing.T) {
	// used == limit：最后一次放行，但状态已是 exceeded、剩余 0；下一次调用才被拒。
	decision := DecideDailyQuota(5000, 5000, fixedNow)
	assert.True(t, decision.Enforced)
	assert.True(t, decision.Allowed)
	assert.Equal(t, model.QuotaStatusExceeded, decision.Status)
	assert.Zero(t, decision.Remaining)
	assert.Equal(t, 100.0, decision.UsagePct)
	assert.Zero(t, decision.RetryAfter)
}

func TestDecideDailyQuotaBlocksOverLimitWithRetryAfter(t *testing.T) {
	decision := DecideDailyQuota(5000, 5001, fixedNow)
	assert.True(t, decision.Enforced)
	assert.False(t, decision.Allowed)
	assert.Equal(t, model.QuotaStatusExceeded, decision.Status)
	assert.Zero(t, decision.Remaining, "over limit must report zero remaining")
	assert.Equal(t, int64(48585), decision.RetryAfter, "retry-after = seconds to next UTC midnight")
}

func TestDecideDailyQuotaRemainingNeverNegative(t *testing.T) {
	decision := DecideDailyQuota(10, 15, fixedNow)
	assert.False(t, decision.Allowed)
	assert.Zero(t, decision.Remaining)
}

func TestSecondsToNextUTCMidnight(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want int64
	}{
		{
			name: "mid day",
			now:  time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
			want: 12 * 3600,
		},
		{
			name: "exact midnight rolls a full day",
			now:  time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
			want: 86400,
		},
		{
			name: "one second before midnight",
			now:  time.Date(2026, 9, 25, 23, 59, 59, 0, time.UTC),
			want: 1,
		},
		{
			name: "fractional seconds round up",
			now:  time.Date(2026, 9, 25, 23, 59, 59, int(500*time.Millisecond), time.UTC),
			want: 1,
		},
		{
			name: "non UTC input normalized to UTC",
			// 东八区 2026-09-26 04:00 = UTC 2026-09-25 20:00 → 距 UTC 零点 4h。
			now:  time.Date(2026, 9, 26, 4, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)),
			want: 4 * 3600,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SecondsToNextUTCMidnight(tc.now))
		})
	}
}

func TestUsageDateUsesUTC(t *testing.T) {
	// 东八区 2026-09-26 04:00 对应 UTC 2026-09-25 —— 计量日以 UTC 切分。
	cnTime := time.Date(2026, 9, 26, 4, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	assert.Equal(t, "2026-09-25", UsageDate(cnTime))
	assert.Equal(t, "2026-09-25", UsageDate(fixedNow))
}
