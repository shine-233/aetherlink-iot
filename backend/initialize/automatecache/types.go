// 文件用途：自动化触发缓存的数据结构与返回状态常量。
// 关键注意事项：JSON 字段名是 Redis 中既有缓存值的线上格式，改名会让存量缓存全部失效或误读。

package automatecache

import (
	"encoding/json"

	"aetherlink-iot/backend/internal/model"
)

// 缓存读取的三种结果。数值与历史常量 AUTOMATE_CACHE_RESULT_* 保持一致。
const (
	ResultNotFound = iota + 1 // 缓存中无数据
	ResultNoTask              // 缓存有数据 无任务
	ResultOK                  // 缓存有数据 有任务
)

// ContentNoTask 设备无任务时写入的占位字符串。
const ContentNoTask = "NOTASK"

type AutomateDeviceInfo struct {
	SceneAutomationId string   `json:"scene_automation_id"`
	GroupIds          []string `json:"group_id"`
}

type AutomateActionInfo struct {
	GroupIds []string           `json:"group_id"`
	Actions  []model.ActionInfo `json:"actions"`
}

type AutomateExecteSceneInfo struct {
	SceneAutomationId string             `json:"scene_automation_id"`
	GroupsCondition   DTConditions       `json:"groups_condition"`
	Actions           []model.ActionInfo `json:"actions"`
}

type AutomateExecteParams struct {
	DeviceId                string                    `json:"device_id"`
	DeviceConfigId          string                    `json:"device_config_id"`
	AutomateExecteSceeInfos []AutomateExecteSceneInfo `json:"automate_execte_scene_infos"`
}

type AutomateDeviceInfos []AutomateDeviceInfo

// UnmarshalBinary 适配 go-redis Scan，将设备缓存列表从 JSON 反序列化回来。
func (a *AutomateDeviceInfos) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, a)
}

type DTConditions []model.DeviceTriggerCondition

// UnmarshalBinary 适配 go-redis Scan，将条件组列表从 JSON 反序列化回来。
func (a *DTConditions) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, a)
}

// UnmarshalBinary 适配 go-redis Scan，将动作信息从 JSON 反序列化回来。
func (a *AutomateActionInfo) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, a)
}
