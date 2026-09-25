// 文件用途：告警 SLA 计时与超时升级（TB-27，126.sql）的服务层编排与判定纯函数。
// 核心逻辑：告警触发时按 alarm_config.sla_hours 从现在起算 sla_due_at（saveAlarmHistoryRecord）；
//
//	cron 周期扫描到期、未恢复且未 breach 的活动告警，标记 sla_breached 并把严重度升一档
//	（L→M→H，H 到顶保持，N 恢复行不参与），remark 追加 sla_escalation 审计并广播实时事件。
//
// 关键注意事项：升级判定为无副作用纯函数（表驱动单测锚点）；执行体逐行条件更新
//
//	（MarkAlarmHistorySlaBreached 带 sla_breached=FALSE 守卫），cron 重跑/多实例并发天然幂等。
//
// 重构建议：如后续需要"目标严重度可配置或多级升级链"，把 alarmSlaEscalationTarget 换成查表策略即可，
//
//	cron 骨架与审计格式保持不变。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/sirupsen/logrus"
)

const (
	// alarmSlaHoursMax SLA 时限上限（约 10 年）：拦住明显误填的巨型数值，
	// 下界 0/NULL 表示不启用；与 validateAlarmTriggerDuration 的上下界收口风格一致。
	alarmSlaHoursMax = 24 * 365 * 10
)

// validateAlarmSlaHours 校验 SLA 时限取值：nil 表示调用方未提交该字段（不校验），
// 合法区间 [0, alarmSlaHoursMax]，越界统一参数错误。
func validateAlarmSlaHours(hours *int32) error {
	if hours == nil {
		return nil
	}
	if *hours < 0 || *hours > alarmSlaHoursMax {
		return errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("sla_hours must be between 0 and %d hours", alarmSlaHoursMax))
	}
	return nil
}

// normalizeAlarmSlaHours 把"未启用 SLA"（nil 或 0）折叠为 NULL 语义（nil 指针），
// 正值原样保留副本避免共享调用方指针。
func normalizeAlarmSlaHours(hours *int32) *int32 {
	if hours == nil || *hours <= 0 {
		return nil
	}
	normalized := *hours
	return &normalized
}

// alarmSlaDueAt 按 SLA 时限从触发时刻起算到期时间（纯函数）。
// 未启用（nil 或 <=0）返回 nil，落库为 NULL；触发时刻一律用 UTC，与 CreateAt 同源。
func alarmSlaDueAt(slaHours *int32, triggerAt time.Time) *time.Time {
	if slaHours == nil || *slaHours <= 0 {
		return nil
	}
	due := triggerAt.Add(time.Duration(*slaHours) * time.Hour)
	return &due
}

// alarmSlaEscalationTarget 返回超时升级后的严重度：L→M→H 各升一档，H 已到顶保持；
// N（恢复）与未知值原样返回——调用方（alarmSlaEscalationDue）已把非活动态挡在扫描外，
// 这里对未知值保持透传以兼容未来新增档位时不误降级。
func alarmSlaEscalationTarget(current string) string {
	switch strings.ToUpper(strings.TrimSpace(current)) {
	case "L":
		return "M"
	case "M":
		return "H"
	default:
		return current
	}
}

// alarmSlaEscalationDue 判定一条告警是否满足超时升级条件（纯函数）：
// 已配置 SLA（slaDueAt 非 nil）、处于活动状态（H/M/L）、已到期（dueAt < now）且尚未 breach。
func alarmSlaEscalationDue(alarmStatus string, slaDueAt *time.Time, slaBreached bool, now time.Time) bool {
	if slaBreached || slaDueAt == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(alarmStatus)) {
	case "H", "M", "L":
	default:
		return false
	}
	return slaDueAt.Before(now)
}

// EscalateOverdueAlarmSlaByCron 扫描并升级超时 SLA 告警，返回本轮实际升级的行数。
// 由 initialize/croninit/cron.go 注册为周期任务；失败逐行跳过不阻断整轮，
// 单行失败会由下一轮扫描自然重试（未 breach 的行仍满足扫描条件）。
func (*Alarm) EscalateOverdueAlarmSlaByCron() (int, error) {
	now := time.Now().UTC()
	candidates, err := dal.FindOverdueSlaAlarmHistories(now, 0)
	if err != nil {
		return 0, wrapAlarmDBError(err)
	}
	escalated := 0
	for _, candidate := range candidates {
		// 扫描层已带同样的条件，这里复用纯函数判定作为统一收口点，
		// 保证"哪些行会被升级"永远由 alarmSlaEscalationDue 一处定义。
		if !alarmSlaEscalationDue(candidate.AlarmStatus, candidate.SlaDueAt, false, now) {
			continue
		}
		target := alarmSlaEscalationTarget(candidate.AlarmStatus)
		remark := dal.MergeAlarmHistorySlaEscalationRemark(
			candidate.Remark,
			now.Format(time.RFC3339),
			candidate.AlarmStatus,
			target,
		)
		if err := dal.MarkAlarmHistorySlaBreached(candidate.ID, target, remark); err != nil {
			if errors.Is(err, dal.ErrAlarmSlaAlreadyBreached) {
				continue
			}
			logrus.WithFields(logrus.Fields{
				"alarm_history_id": candidate.ID,
				"tenant_id":        candidate.TenantID,
				"from_level":       candidate.AlarmStatus,
				"to_level":         target,
			}).WithError(err).Warn("alarm SLA escalation update failed")
			continue
		}
		escalated++
		// 广播租户级实时事件：前端告警页订阅 alarm status WS（TB-30）后去抖刷新，
		// 让 SLA 升级无需用户手动刷新即可看到新的严重度与超时标记。
		PublishAlarmEvent(context.Background(), candidate.TenantID, map[string]interface{}{
			"type":            "sla_escalation",
			"alarm_id":        candidate.ID,
			"alarm_config_id": candidate.AlarmConfigID,
			"from_level":      candidate.AlarmStatus,
			"to_level":        target,
		})
	}
	return escalated, nil
}
