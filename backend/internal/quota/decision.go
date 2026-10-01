// 文件用途：TB-17 日配额执法判定纯函数——给定限额、已用量与当前时刻，产出放行/拒绝决策。
// 核心逻辑：DecideDailyQuota 是无 IO 的纯函数（Go 单测钉死全部边界）；限额 <=0 视为未配置
// 执法阈值（放行 + unlimited）；used > limit 拒绝，Retry-After 为距次日 UTC 零点的秒数。
// 关键注意事项：判定与累计解耦——调用方（meter/service）先原子累加拿到新计数，再交本函数判定；
// 被拒请求同样计入当日用量（计费口径：尝试调用即消耗），此语义在此处固化。
// 重构建议：若后续引入时区感知的日窗口（按租户时区切日），只需扩展本文件的日期函数，
// 判定主体无需改动。
package quota

import (
	"math"
	"time"

	"aetherlink-iot/backend/internal/model"
)

// usageDateLayout 计量日的标准格式（UTC 日期，YYYY-MM-DD）。
const usageDateLayout = "2006-01-02"

// Decision 日配额执法判定结果。
type Decision struct {
	Enforced   bool    // 是否参与执法（限额 >0 才执法）
	Allowed    bool    // 是否放行本次调用
	Used       int64   // 判定时点该租户当日已用调用数（含本次）
	Limit      int64   // 套餐日调用限额；<=0 表示未配置
	Remaining  int64   // 剩余额度；未执法时为 -1（不限量哨兵值）
	UsagePct   float64 // 已用百分比（一位小数）；未执法时为 0
	RetryAfter int64   // 被拒时建议等待秒数（距次日 UTC 零点；放行为 0）
	Status     string  // model.QuotaStatus*：normal | warning | exceeded | unlimited
}

// UsageDate 返回时刻 t 对应的计量日（UTC 日期字符串）。全链路统一用本函数取日期，
// 保证 Redis 键、落库 usage_date 与执法判定三方口径一致。
func UsageDate(t time.Time) string {
	return t.UTC().Format(usageDateLayout)
}

// SecondsToNextUTCMidnight 返回距下一个 UTC 零点的秒数（向上取整，最小 1）。
// 用作日配额被拒时的 Retry-After：配额随次日零点的新计数键自然重置。
func SecondsToNextUTCMidnight(t time.Time) int64 {
	u := t.UTC()
	next := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	secs := int64(math.Ceil(next.Sub(u).Seconds()))
	if secs < 1 {
		secs = 1
	}
	return secs
}

// DecideDailyQuota 执法判定纯函数。
//
// 语义：
//   - limit <= 0：未配置执法阈值 → 放行、不执法（fail-open 与套餐容错一致：无套餐不封人）；
//   - used <= limit：放行；used == limit 时已用尽（remaining=0、状态 exceeded），
//     下一次调用才会被拒；
//   - used > limit：拒绝，Retry-After = 距次日 UTC 零点秒数。
func DecideDailyQuota(limit, used int64, now time.Time) Decision {
	decision := Decision{
		Used:      used,
		Limit:     limit,
		Remaining: -1,
	}

	if limit <= 0 {
		decision.Status = model.QuotaStatusUnlimited
		decision.Allowed = true
		return decision
	}

	decision.Enforced = true
	decision.Remaining = limit - used
	if decision.Remaining < 0 {
		decision.Remaining = 0
	}
	decision.UsagePct = usagePct(used, limit)

	if used > limit {
		decision.Allowed = false
		decision.Status = model.QuotaStatusExceeded
		decision.RetryAfter = SecondsToNextUTCMidnight(now)
		return decision
	}

	decision.Allowed = true
	switch {
	case used >= limit:
		decision.Status = model.QuotaStatusExceeded
	case decision.UsagePct >= 80.0:
		decision.Status = model.QuotaStatusWarning
	default:
		decision.Status = model.QuotaStatusNormal
	}
	return decision
}

// usagePct 已用百分比，保留一位小数（与 billing 的 calcPct 口径一致）。
func usagePct(used, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	pct := (float64(used) / float64(limit)) * 100.0
	return math.Round(pct*10) / 10
}
