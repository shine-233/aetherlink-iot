// 文件用途：规则链告警动作（ROADMAP B2 扩展点补全）——写入一条告警历史。
// tenant-scope: reviewed-2026-09-02 write-only; tenant_id is caller-supplied (rule-chain context tenant).
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CreateAlarmHistoryRow 落库一条告警历史（供规则链 action.alarm 等引擎动作使用）。
func CreateAlarmHistoryRow(h *model.AlarmHistory) error {
	return global.DB.Create(h).Error
}
