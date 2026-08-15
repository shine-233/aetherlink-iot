// alarm_trigger_duration.go 负责“条件持续 N 秒才触发”这一告警触发时长约束。
//
// 主要职责：
// 1. 校验 alarm_config.trigger_duration 的取值范围，越界时统一返回参数错误。
// 2. 依据告警缓存分组记录的条件成立起始时间，判断持续时长是否已满足。
//
// 关键约束：
// - trigger_duration 为 0（或数据库中的 NULL 回落值 0）时必须保持历史行为：条件一成立即触发。
// - 条件成立起始时间由 initialize.AlarmCache 在写入分组时记录，本文件只做纯判断，不写缓存。
//
// 静态审查建议：
// - 判断函数保持无副作用，便于在没有 Redis/数据库的环境下用表驱动测试覆盖边界。
package service

import (
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/sirupsen/logrus"
)

const (
	// 触发持续时长上下界与 40.sql 中的 CHECK 约束保持一致。
	alarmTriggerDurationMinSeconds = 0
	alarmTriggerDurationMaxSeconds = 24 * 60 * 60

	alarmTriggerDurationNotHeldReason = "alarm trigger duration not satisfied"
)

// validateAlarmTriggerDuration 校验触发持续时长是否落在 [0, 86400] 秒区间。
// nil 表示调用方未提交该字段，沿用旧值或默认值 0。
func validateAlarmTriggerDuration(duration *int32) error {
	if duration == nil {
		return nil
	}
	if *duration < alarmTriggerDurationMinSeconds || *duration > alarmTriggerDurationMaxSeconds {
		return errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("trigger_duration must be between %d and %d seconds", alarmTriggerDurationMinSeconds, alarmTriggerDurationMaxSeconds))
	}
	return nil
}

// normalizeAlarmTriggerDuration 把未提交的字段折叠成默认值 0，保持“立即触发”的旧行为。
// 调用前应先经过 validateAlarmTriggerDuration，这里不再重复做范围裁剪。
func normalizeAlarmTriggerDuration(duration *int32) int32 {
	if duration == nil {
		return alarmTriggerDurationMinSeconds
	}
	return *duration
}

// alarmTriggerDurationSatisfied 判断条件是否已连续成立到达配置的持续时长。
//
// 语义约定：
//   - triggerDuration <= 0：立即触发，等价于引入本特性之前的行为。
//   - conditionTrueSince <= 0：起始时间未知（例如升级前写入的旧缓存），
//     此时无法证明条件已连续成立，按“尚未满足”处理，等下一次观测补齐起点后再触发。
//   - now 早于 conditionTrueSince（时钟回拨）时同样视为尚未满足，避免提前触发。
func alarmTriggerDurationSatisfied(triggerDuration int32, conditionTrueSince int64, now int64) bool {
	if triggerDuration <= 0 {
		return true
	}
	if conditionTrueSince <= 0 {
		return false
	}
	elapsed := now - conditionTrueSince
	if elapsed < 0 {
		return false
	}
	return elapsed >= int64(triggerDuration)
}

// alarmTriggerDurationHeld 读取告警配置的持续时长，并结合缓存记录的条件成立起点做判断。
// 读取告警配置失败时不阻断既有触发链路，退化为立即触发以保持向后兼容。
func alarmTriggerDurationHeld(alarmConfigID string, conditionTrueSince int64) bool {
	alarmConfig, err := dal.GetAlarmByID(alarmConfigID)
	if err != nil {
		logrus.WithField("alarm_config_id", alarmConfigID).
			Warn("read alarm trigger duration failed; falling back to immediate trigger")
		return true
	}
	return alarmTriggerDurationSatisfied(alarmConfig.TriggerDuration, conditionTrueSince, time.Now().UTC().Unix())
}
