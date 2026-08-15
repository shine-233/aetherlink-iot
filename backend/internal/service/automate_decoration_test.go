// 文件用途：验证自动化场景触发、装饰和动作执行的服务行为。
// 核心逻辑：用表驱动用例覆盖遥测条件、事件参数、执行失败和装饰结果生成。
// 关键注意事项：自动化触发顺序直接影响用户场景，测试需同时保护匹配语义、限流语义和失败隔离。
// 重构建议：按条件解析、动作副作用和装饰输出拆分夹具，增加事务失败与外部执行失败边界。
package service

import (
	"testing"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"
)

func TestActionAfterAlarmIgnoresEmptyActions(t *testing.T) {
	t.Parallel()

	if err := ActionAfterAlarm(nil, "device-1", nil); err != nil {
		t.Fatalf("ActionAfterAlarm(nil) error = %v", err)
	}
}

func TestConditionAfterAlarmSkipsSingleDeviceConditionWithoutSource(t *testing.T) {
	t.Parallel()

	conditions := initialize.DTConditions{
		{
			GroupID:              "group-1",
			SceneAutomationID:    "scene-1",
			TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE,
		},
	}

	if err := ConditionAfterAlarm(true, conditions, "device-1", []string{"content"}); err != nil {
		t.Fatalf("ConditionAfterAlarm() error = %v", err)
	}
}
