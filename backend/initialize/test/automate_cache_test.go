//go:build integration

// 文件用途：验证自动化缓存初始化后的写入、查询和删除行为。
// 核心逻辑：加载本地配置并连接 Redis，围绕场景自动化、设备和设备配置维度检查缓存结果。
// 维护提示：本文件带 integration 标签，运行前需确认 conf-localdev.yml 和 Redis 可用。
// 关键注意事项：集成测试依赖 conf-localdev.yml 与 Redis，默认 go test 只能验证包边界不能证明缓存闭环。
// 重构建议：可抽出 Redis client 与配置加载 seam，用 fake 存储覆盖缓存索引规则的单元测试。

package test

import (
	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"testing"
)

func init() {
	initialize.ViperInit("../../configs/conf-localdev.yml")
	initialize.RedisInit()
	cache = initialize.NewAutomateCache()
}

var (
	sceneAutomateId = "sceneAutomateId_test"
	cache           *initialize.AutomateCache
)

func StringPoints(s string) *string {
	return &s
}

func requireAutomateCache(t *testing.T) {
	t.Helper()
	if cache == nil || global.REDIS == nil {
		t.Skip("redis is not configured for automate cache integration test")
	}
}

func getConditions(sceneAutomateId string) []model.DeviceTriggerCondition {
	var conditions []model.DeviceTriggerCondition

	condition := model.DeviceTriggerCondition{
		SceneAutomationID:    sceneAutomateId,
		GroupID:              "groupId",
		TriggerConditionType: "10",
		TriggerValue:         "30",
		TriggerSource:        StringPoints("condition_deviceIds01"),
		TriggerParamType:     StringPoints("TEL"),
		TriggerParam:         StringPoints("temperature"),
		TriggerOperator:      StringPoints(">"),
	}

	conditions = append(conditions, condition)

	condition1 := model.DeviceTriggerCondition{
		SceneAutomationID:    sceneAutomateId,
		GroupID:              "groupId",
		TriggerConditionType: "10",
		TriggerValue:         "30",
		TriggerSource:        StringPoints("condition_deviceIds02"),
		TriggerParamType:     StringPoints("TEL"),
		TriggerParam:         StringPoints("temperature"),
		TriggerOperator:      StringPoints(">"),
	}
	conditions = append(conditions, condition1)
	return conditions
}

func getActions(sceneAutomateId string) []model.ActionInfo {
	var actions []model.ActionInfo
	action1 := model.ActionInfo{
		SceneAutomationID: sceneAutomateId,
		ActionType:        "10",
		ActionTarget:      StringPoints("action_deviceIds01"),
		ActionParamType:   StringPoints("CMD"),
		ActionParam:       StringPoints("test_cmd"),
		ActionValue:       StringPoints("test_val"),
	}
	actions = append(actions, action1)
	action2 := model.ActionInfo{
		SceneAutomationID: sceneAutomateId,
		ActionType:        "11",
		ActionTarget:      StringPoints("action_deviceIds02"),
		ActionParamType:   StringPoints("CMD"),
		ActionParam:       StringPoints("test_cmd"),
		ActionValue:       StringPoints("test_val"),
	}
	actions = append(actions, action2)
	return actions
}

// conditions []model.DeviceTriggerCondition, actions []model.ActionInfo
func TestSetCacheBySceneAutomationId(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试创建缓存...")
	//cache := initialize.NewAutomateCache()
	conditions := getConditions(sceneAutomateId)
	err := cache.SetCacheBySceneAutomationId(sceneAutomateId, conditions, getActions(sceneAutomateId))
	if err != nil {
		t.Error("自动化缓存保存失败", err)
	}
}

func TestGetCacheByDeviceId(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试查询缓存存在的情况...")
	//cache := initialize.NewAutomateCache()
	res, resultInt, err := cache.GetCacheByDeviceId("condition_deviceIds01", "")
	if err != nil {
		t.Error("根据设备获取自动化缓存失败", err)
	}
	if resultInt != initialize.AUTOMATE_CACHE_RESULT_OK {
		t.Errorf("查询异常, 查询状态:%d, 结果: %#v", resultInt, res)
	}
	t.Logf("结果:%#v", res)
}

func TestDeleteCacheBySceneAutomationId(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试删除场景缓存...")
	//cache := initialize.NewAutomateCache()
	err := cache.DeleteCacheBySceneAutomationId(sceneAutomateId)
	if err != nil {
		t.Error("删除缓存失败", err)
	}
}

func TestGetCacheByDeviceIdNotExists(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试设备缓存中无数据...")
	//cache := initialize.NewAutomateCache()
	res, resultInt, err := cache.GetCacheByDeviceId("condition_deviceIds0004", "")
	t.Logf("查询状态:%d", resultInt)
	if err != nil {
		t.Error("根据设备获取自动化缓存失败", err)
	}
	if resultInt != initialize.AUTOMATE_CACHE_RESULT_NOT_FOUND {
		t.Errorf("查询异常, 查询状态:%d, 结果: %#v", resultInt, res)
	}
	t.Logf("结果:%#v", res)
}

func TestSetCacheByDeviceIdWithNoTask(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试缓存中保存无任务设备...")
	err := cache.SetCacheByDeviceIdWithNoTask("condition_deviceIds00005", "")
	if err != nil {
		t.Error("测试缓存中保存无任务设备失败", err)
	}
	_, resultInt, err := cache.GetCacheByDeviceId("condition_deviceIds00005", "")
	t.Logf("查询结果: resultInt:%d", resultInt)
	if err != nil {
		t.Error("根据设备获取自动化缓存失败", err)
	}
	if resultInt != initialize.AUTOMATE_CACHE_RESULT_NOT_TASK {
		t.Errorf("查询异常, 查询状态:%d", resultInt)
	}
}

func TestSetCacheByDeviceId(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试根据设备id自动化缓存信息...")
	deviceId := "condition_deviceIds_with_device"
	conditions := []model.DeviceTriggerCondition{
		{
			SceneAutomationID:    "sceneAutomateId_with_device_01",
			GroupID:              "groupId",
			TriggerConditionType: "10",
			TriggerValue:         "20-20",
			TriggerSource:        StringPoints(deviceId),
			TriggerParamType:     StringPoints("TEL"),
			TriggerParam:         StringPoints("temperature"),
			TriggerOperator:      StringPoints(">"),
		},
		{
			SceneAutomationID:    "sceneAutomateId_with_device_02",
			GroupID:              "groupId",
			TriggerConditionType: "10",
			TriggerValue:         "20-20",
			TriggerSource:        StringPoints(deviceId),
			TriggerParamType:     StringPoints("TEL"),
			TriggerParam:         StringPoints("temperature"),
			TriggerOperator:      StringPoints(">"),
		},
	}

	actions := []model.ActionInfo{
		{
			SceneAutomationID: "sceneAutomateId_with_device_01",
			ActionType:        "10",
			ActionTarget:      StringPoints("action_deviceIds01"),
			ActionParamType:   StringPoints("CMD"),
			ActionParam:       StringPoints("test_cmd"),
			ActionValue:       StringPoints("test_val"),
		},
		{
			SceneAutomationID: "sceneAutomateId_with_device_02",
			ActionType:        "10",
			ActionTarget:      StringPoints("action_deviceIds01"),
			ActionParamType:   StringPoints("CMD"),
			ActionParam:       StringPoints("test_cmd"),
			ActionValue:       StringPoints("test_val"),
		},
	}

	err := cache.SetCacheByDeviceId(deviceId, "", conditions, actions)
	if err != nil {
		t.Error("测试缓存中保存无任务设备失败", err)
	}
	res, resultInt, err := cache.GetCacheByDeviceId(deviceId, "")
	t.Logf("查询结果: resultInt:%d; 缓存信息: %#v", resultInt, res)
	if err != nil {
		t.Error("根据设备获取自动化缓存失败", err)
	}
	if resultInt != initialize.AUTOMATE_CACHE_RESULT_OK {
		t.Errorf("查询异常, 查询状态:%d", resultInt)
	}
}

func TestSetCacheByDeviceConfidId(t *testing.T) {
	requireAutomateCache(t)
	t.Log("测试根据设备id自动化缓存信息...")
	deviceId := "condition_deviceIds_with_device"
	deviceConfigId := "condition_deviceIds_with_device_config_id"
	conditions := []model.DeviceTriggerCondition{
		{
			SceneAutomationID:    "sceneAutomateId_with_device_01",
			GroupID:              "groupId",
			TriggerConditionType: "11",
			TriggerValue:         "21",
			TriggerSource:        StringPoints(deviceConfigId),
			TriggerParamType:     StringPoints("TEL"),
			TriggerParam:         StringPoints("temperature"),
			TriggerOperator:      StringPoints(">"),
		},
		{
			SceneAutomationID:    "sceneAutomateId_with_device_01",
			GroupID:              "groupId",
			TriggerConditionType: "22",
			TriggerValue:         "137|06:30:00+00:00|16:30:00+00:00",
		},
		{
			SceneAutomationID:    "sceneAutomateId_with_device_02",
			GroupID:              "groupId02",
			TriggerConditionType: "11",
			TriggerValue:         "21",
			TriggerSource:        StringPoints(deviceConfigId),
			TriggerParamType:     StringPoints("TEL"),
			TriggerParam:         StringPoints("temperature"),
			TriggerOperator:      StringPoints(">"),
		},
	}

	actions := []model.ActionInfo{
		{
			SceneAutomationID: "sceneAutomateId_with_device_01",
			ActionType:        "10",
			ActionTarget:      StringPoints("action_deviceIds01"),
			ActionParamType:   StringPoints("CMD"),
			ActionParam:       StringPoints("test_cmd"),
			ActionValue:       StringPoints("test_val"),
		},
		{
			SceneAutomationID: "sceneAutomateId_with_device_02",
			ActionType:        "10",
			ActionTarget:      StringPoints("action_deviceIds01"),
			ActionParamType:   StringPoints("CMD"),
			ActionParam:       StringPoints("test_cmd"),
			ActionValue:       StringPoints("test_val"),
		},
	}

	err := cache.SetCacheByDeviceId(deviceId, deviceConfigId, conditions, actions)
	if err != nil {
		t.Error("测试缓存中保存无任务设备失败", err)
	}
	res, resultInt, err := cache.GetCacheByDeviceId(deviceId, deviceConfigId)
	t.Logf("查询结果: resultInt:%d; 缓存信息: %#v", resultInt, res)
	if err != nil {
		t.Error("根据设备获取自动化缓存失败", err)
	}
	if resultInt != initialize.AUTOMATE_CACHE_RESULT_OK {
		t.Errorf("查询异常, 查询状态:%d", resultInt)
	}
}
