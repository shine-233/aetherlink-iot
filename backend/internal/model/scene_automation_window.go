package model

import "time"

// SceneAutomationWindow 场景自动化的持久化执行窗口（P0.4）。
//
// 单独放在非 .gen.go 文件中：scene_automations.gen.go 是生成产物，
// 按项目约定不手改（同 P0.2 中 device_shadow 的做法）。
// 新增列由 DAL 以字符串列名读写，领域语义收敛在这里。
type SceneAutomationWindow struct {
	SceneAutomationID string
	StartsAt          *time.Time
	ExpiresAt         *time.Time
	Timezone          string
}

// HasBounds 报告该场景是否配置了任何窗口边界。
// 未配置任何一侧即视为无界，执行侧必须保持既有"始终可执行"的行为，
// 不能因为取不到配置就把存量场景全部拦掉。
func (w SceneAutomationWindow) HasBounds() bool {
	return w.StartsAt != nil || w.ExpiresAt != nil
}

// NormalizedTimezone 返回去空白后的时区；空值按 UTC 处理。
// 注意这里只做空白归一，**不做合法性校验**：非法时区由服务层 fail closed，
// 若在模型层悄悄兜底成 UTC，窗口边界会整体偏移，等于伪造可执行性。
func (w SceneAutomationWindow) NormalizedTimezone() string {
	if w.Timezone == "" {
		return "UTC"
	}
	return w.Timezone
}
