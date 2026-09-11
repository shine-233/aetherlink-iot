package model

import "time"

// SceneAutomationTimer 持久化的场景定时触发（P0.4）。
// 放在非 .gen.go 文件中：按项目约定不手改生成产物，新表由 DAL 以
// 字符串列名读写（同 P0.2 device_shadow、P0.4 执行窗口的做法）。
type SceneAutomationTimer struct {
	ID                  string
	TenantID            string
	SceneAutomationID   string
	CronExpr            string
	Timezone            string
	Enabled             bool
	NextRunAt           time.Time
	LastRunAt           *time.Time
	LeaseOwner          *string
	LeaseUntil          *time.Time
	ConsecutiveFailures int
	LastError           *string
}

// SceneAutomationTimerMaxConsecutiveFailures 连续失败上限。
// 达到后定时器停摆并保留 last_error，等人工介入：
// 无限重试一个必然失败的触发，只会把下游打爆并淹没真正的告警。
const SceneAutomationTimerMaxConsecutiveFailures = 5

// IsStalled 报告定时器是否因连续失败而停摆。
func (t SceneAutomationTimer) IsStalled() bool {
	return t.ConsecutiveFailures >= SceneAutomationTimerMaxConsecutiveFailures
}

// LeaseActive 报告租约是否仍然有效。
// 已过期视为无主：领取方多半已崩溃，任务必须能被重新领走，否则就是丢任务。
func (t SceneAutomationTimer) LeaseActive(now time.Time) bool {
	return t.LeaseUntil != nil && t.LeaseUntil.After(now)
}
