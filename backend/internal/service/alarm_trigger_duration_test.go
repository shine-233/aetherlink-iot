// alarm_trigger_duration_test.go 锁定“条件持续 N 秒才触发”的纯逻辑边界。
//
// Purpose: 覆盖 trigger_duration 的范围校验、默认值折叠，以及连续成立判断的关键分支。
// Core logic: 全部用表驱动断言纯函数，不依赖 PostgreSQL/Redis，因此在无数据库环境下也能执行。
// Important notes: duration 为 0 必须等价于引入该特性之前的“立即触发”，这是向后兼容的硬约束。
// Refactor suggestion: 若后续把起始时间挪到独立存储，应在这里补充对应的 seam 测试。
package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
)

func int32Ptr(value int32) *int32 {
	return &value
}

func TestValidateAlarmTriggerDurationRange(t *testing.T) {
	tests := []struct {
		name     string
		duration *int32
		wantErr  bool
	}{
		{name: "not submitted keeps old value", duration: nil},
		{name: "zero fires immediately", duration: int32Ptr(0)},
		{name: "one second", duration: int32Ptr(1)},
		{name: "upper bound 86400", duration: int32Ptr(86400)},
		{name: "negative rejected", duration: int32Ptr(-1), wantErr: true},
		{name: "above upper bound rejected", duration: int32Ptr(86401), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAlarmTriggerDuration(tt.duration)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("validateAlarmTriggerDuration(%v) = %v, want nil", tt.duration, err)
				}
				return
			}
			rdiTestRequireError(
				t,
				err,
				"validateAlarmTriggerDuration",
				errcode.CodeParamError,
				"trigger_duration must be between 0 and 86400 seconds",
			)
		})
	}
}

func TestNormalizeAlarmTriggerDurationFoldsMissingValueToZero(t *testing.T) {
	if got := normalizeAlarmTriggerDuration(nil); got != 0 {
		t.Fatalf("normalizeAlarmTriggerDuration(nil) = %d, want 0", got)
	}
	if got := normalizeAlarmTriggerDuration(int32Ptr(0)); got != 0 {
		t.Fatalf("normalizeAlarmTriggerDuration(0) = %d, want 0", got)
	}
	if got := normalizeAlarmTriggerDuration(int32Ptr(120)); got != 120 {
		t.Fatalf("normalizeAlarmTriggerDuration(120) = %d, want 120", got)
	}
}

func TestAlarmTriggerDurationSatisfiedHoldsConditionForConfiguredSeconds(t *testing.T) {
	const now int64 = 1_700_000_000

	tests := []struct {
		name               string
		triggerDuration    int32
		conditionTrueSince int64
		now                int64
		want               bool
	}{
		// duration 0/NULL 回落值必须保持历史行为：第一次观测即触发。
		{
			name:               "duration zero fires immediately",
			triggerDuration:    0,
			conditionTrueSince: now,
			now:                now,
			want:               true,
		},
		{
			name:               "duration zero fires even without a known start",
			triggerDuration:    0,
			conditionTrueSince: 0,
			now:                now,
			want:               true,
		},
		{
			name:               "negative duration is treated as immediate",
			triggerDuration:    -5,
			conditionTrueSince: 0,
			now:                now,
			want:               true,
		},
		{
			name:               "held shorter than duration does not fire",
			triggerDuration:    60,
			conditionTrueSince: now - 59,
			now:                now,
			want:               false,
		},
		{
			name:               "held exactly the duration fires",
			triggerDuration:    60,
			conditionTrueSince: now - 60,
			now:                now,
			want:               true,
		},
		{
			name:               "held longer than duration fires",
			triggerDuration:    60,
			conditionTrueSince: now - 600,
			now:                now,
			want:               true,
		},
		{
			name:               "just became true does not fire",
			triggerDuration:    60,
			conditionTrueSince: now,
			now:                now,
			want:               false,
		},
		// 升级前写入的旧缓存没有起始时间戳，无法证明连续成立，必须先不触发。
		{
			name:               "unknown start with positive duration does not fire",
			triggerDuration:    60,
			conditionTrueSince: 0,
			now:                now,
			want:               false,
		},
		{
			name:               "clock skew backwards does not fire early",
			triggerDuration:    60,
			conditionTrueSince: now + 30,
			now:                now,
			want:               false,
		},
		{
			name:               "upper bound duration not yet held",
			triggerDuration:    86400,
			conditionTrueSince: now - 86399,
			now:                now,
			want:               false,
		},
		{
			name:               "upper bound duration held",
			triggerDuration:    86400,
			conditionTrueSince: now - 86400,
			now:                now,
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alarmTriggerDurationSatisfied(tt.triggerDuration, tt.conditionTrueSince, tt.now)
			if got != tt.want {
				t.Fatalf(
					"alarmTriggerDurationSatisfied(%d, %d, %d) = %t, want %t",
					tt.triggerDuration, tt.conditionTrueSince, tt.now, got, tt.want,
				)
			}
		})
	}
}
