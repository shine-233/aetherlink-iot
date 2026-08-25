// 文件用途：维护 service 包聚合入口和通用数据转换 helper。
// 核心逻辑：集中暴露各服务实例，并提供字符串指针、JSON 判断和结构体转 map 等共享工具。
// 关键注意事项：入口结构变更会影响 handler 注入，通用 helper 变更会波及设备、用户和自动化服务。
// 重构建议：将通用 helper 下沉到独立工具包，补齐 nil、标签过滤、JSON 校验和调用方迁移测试。
package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ServiceGroup 聚合所有业务服务，通过嵌入各服务结构体对外提供统一入口
type ServiceGroup struct {
	User
	Role
	Dict
	OTA
	ProtocolPlugin
	Device
	DeviceDebug
	DeviceModel
	DeviceTemplate
	DeviceGroup
	UiElements
	TelemetryData
	EventData
	AttributeData
	CommandData
	OperationLogs
	Logo
	DataPolicy
	Board
	DeviceConfig
	DataScript
	Casbin
	NotificationGroup
	NotificationHisory
	NotificationServicesConfig
	Alarm
	Scene
	SceneAutomation
	SceneAutomationLog
	Automate
	AutomateTask
	SysFunction
	ServicePlugin
	ServiceAccess
	ExpectedData
	OpenAPIKey
	MessagePush
	SystemMonitor
	DeviceAuth
	DeviceTopicMapping
	FleetSavedFilter
	DashboardMenu
	DeviceTwin
	DeviceShadow
	RDI
	PayloadSchema
}

// GroupApp 是全局业务服务入口，供 API 层和中间件层调用
var GroupApp = new(ServiceGroup)

// safeDeref 安全地解引用字符串指针，如果是nil则返回空字符串
func SafeDeref(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// stringPtr 返回一个字符串的指针
func StringPtr(s string) *string {
	return &s
}

// IsJSON 校验一个字符串是否为有效的 JSON 格式
func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

// contains checks if a slice contains a specific string.
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// StructToMapAndVerifyJson 将结构体转换为 map，并校验指定 json tag 字段的格式
// 对于 nil 值的字段不会被转换
// 例如：StructToMapAndVerifyJson(req, "additional_info")
func StructToMapAndVerifyJson(obj interface{}, jsonTagsToCheck ...string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input is not a struct")
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := typ.Field(i)

		jsonTag := typeField.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		jsonKey := strings.Split(jsonTag, ",")[0]

		// Adjust the condition to check if the field is a pointer or not before calling IsNil
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			strVal, ok := field.Interface().(*string)
			if ok && strVal != nil && contains(jsonTagsToCheck, jsonKey) {
				if !IsJSON(*strVal) {
					return nil, fmt.Errorf("%s is not valid JSON", jsonKey)
				}
			}
		}

		if field.IsValid() && (field.Kind() != reflect.Ptr || !field.IsNil()) {
			result[jsonKey] = field.Interface()
		}
	}
	return result, nil
}

// StructToMap 将结构体转换为 map，对于 nil 值的字段不会被转换
func StructToMap(obj interface{}, _ ...string) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := typ.Field(i)

		jsonTag := typeField.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		// Get the first part of the json tag, ignore omitempty etc.
		jsonKey := strings.Split(jsonTag, ",")[0]

		if field.Kind() == reflect.Ptr || field.Kind() == reflect.Slice || field.Kind() == reflect.Map || field.Kind() == reflect.Interface || field.Kind() == reflect.Chan || field.Kind() == reflect.Func {
			if !field.IsNil() {
				result[jsonKey] = field.Interface()
			}
		} else {
			result[jsonKey] = field.Interface()
		}
	}
	return result
}
