// 文件用途：告警 SLA 超时升级（TB-27，126.sql）的持久化访问：到期候选扫描、breach 标记落库、
//
//	remark 审计合并与告警配置 SLA 时限的单列更新。
//
// 核心逻辑：FindOverdueSlaAlarmHistories 扫描 sla_due_at 已过、未恢复且未 breach 的活动告警；
//
//	MarkAlarmHistorySlaBreached 以 id + 未 breach 条件更新（严重度一档+sla_breached+remark），
//	保证 cron 重跑或多实例并发时同一告警只会升级一次。
//
// 关键注意事项：到期扫描是系统级 cron 任务，按设计跨租户（函数头带 tenant-scope 标记）；
//
//	remark 为 text 列存 JSON（43.sql 已加宽），审计合并沿用 mergeAlarmHistoryRemark 既有模式。
//
// 重构建议：details 结构化 JSONB 列迁移落地后，sla_escalation 审计可随迁独立列并删除合并逻辑。
package dal

import (
	"errors"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// alarmSlaEscalationAuditKey remark JSON 中的 sla_escalation 审计键（expandMapRemarkFields 同名展开）。
const alarmSlaEscalationAuditKey = "sla_escalation"

// alarmSlaEscalationBatchSize 单轮升级扫描的行数上限：SLA 到期通常是长周期事件（小时级），
// 正常一轮远小于该值；上限仅用于故障恢复积压时分批消化，避免一次性锁过多行。
const alarmSlaEscalationBatchSize = 500

// ErrAlarmSlaAlreadyBreached 表示目标告警在更新前已被升级（并发 cron 或重跑命中同一行），
// 调用方应视为 benign skip 而不是失败。
var ErrAlarmSlaAlreadyBreached = errors.New("alarm sla already breached")

// AlarmSlaEscalationCandidate 到期待升级告警的最小投影：升级动作只需要主键、租户（实时事件路由）、
// 关联配置（事件负载）、当前严重度、到期时间与 remark 原文。
type AlarmSlaEscalationCandidate struct {
	ID            string
	TenantID      string
	AlarmConfigID string
	AlarmStatus   string
	SlaDueAt      *time.Time
	Remark        *string
}

// FindOverdueSlaAlarmHistories 返回 SLA 已到期、未恢复（H/M/L）且未 breach 的活动告警候选。
// limit<=0 时使用 alarmSlaEscalationBatchSize。
// tenant-scope: system-cron（后台超时升级任务按设计跨租户全量扫描；行级更新仍以主键收口，
// 面向用户的列表/详情读路径不经过本查询，继续由 service 层完成租户过滤）。
func FindOverdueSlaAlarmHistories(now time.Time, limit int) ([]AlarmSlaEscalationCandidate, error) {
	if limit <= 0 {
		limit = alarmSlaEscalationBatchSize
	}
	candidates := make([]AlarmSlaEscalationCandidate, 0)
	err := global.DB.Table("alarm_history AS ah").
		Select("ah.id, ah.tenant_id, ah.alarm_config_id, ah.alarm_status, ah.sla_due_at, ah.remark").
		Where("ah.sla_due_at IS NOT NULL AND ah.sla_due_at < ?", now).
		Where("ah.alarm_status IN ?", []string{"H", "M", "L"}).
		Where("ah.sla_breached = ?", false).
		Order("ah.sla_due_at ASC").
		Limit(limit).
		Scan(&candidates).Error
	return candidates, err
}

// MarkAlarmHistorySlaBreached 标记一条告警 SLA 超时：严重度升到 targetLevel、sla_breached=TRUE、
// remark 换成已合并 sla_escalation 审计的 JSON。更新条件保留 alarm_status IN (H,M,L) 与
// sla_breached=FALSE，行已被并发升级或已恢复时返回 ErrAlarmSlaAlreadyBreached。
func MarkAlarmHistorySlaBreached(id, targetLevel, remark string) error {
	result := global.DB.Table("alarm_history").
		Where("id = ? AND alarm_status IN ? AND sla_breached = ?", id, []string{"H", "M", "L"}, false).
		Updates(map[string]interface{}{
			"alarm_status": targetLevel,
			"sla_breached": true,
			"remark":       remark,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAlarmSlaAlreadyBreached
	}
	return nil
}

// MergeAlarmHistorySlaEscalationRemark 在原 remark JSON 基础上追加 sla_escalation 审计对象，
// 记录升级时间与升级前后的严重度；原 remark 非法 JSON 时按 mergeAlarmHistoryRemark 的
// previous_remark 兜底保留，不丢历史审计。
func MergeAlarmHistorySlaEscalationRemark(raw *string, escalatedAt, fromLevel, toLevel string) string {
	return mergeAlarmHistoryRemark(raw, map[string]interface{}{
		alarmSlaEscalationAuditKey: map[string]interface{}{
			"escalated":    true,
			"escalated_at": escalatedAt,
			"from_level":   fromLevel,
			"to_level":     toLevel,
		},
	})
}

// UpdateAlarmConfigSlaHours 按主键单列写告警配置的 SLA 时限（TB-27）。
// nil 也写入（NULL=关闭 SLA）：结构体 Updates 会跳过零值/nil 指针，关闭语义必须走单列更新，
// 与 UpdateAlarmConfigTriggerDuration、UpdateDeviceConfigDefaultRuleChainID 同一口径。
// tenant-scope: caller-enforced（主键更新路径，租户归属由 service 层 ensureAlarmConfigWriteAccess 前置校验）。
func UpdateAlarmConfigSlaHours(id string, slaHours *int32) error {
	result := global.DB.Model(&model.AlarmConfig{}).
		Where("id = ?", id).
		Update("sla_hours", slaHours)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
