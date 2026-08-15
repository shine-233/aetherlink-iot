// 文件用途：维护 RDI 物模型字段和平台模型之间的转换。
// 核心逻辑：把 RDI 设备属性、遥测、命令和事件定义映射到平台统一物模型结构。
// 关键注意事项：模型映射影响前端配置和设备解析，未知字段和旧版本别名需保持兼容。
// 重构建议：抽出映射表和版本适配层，补齐字段缺失、坏 JSON、兼容别名和回归测试。
package service

import (
	"aetherlink-iot/backend/internal/model"
)

func RDIThingModelDefinition() model.RDIThingModel {
	return model.RDIThingModel{
		Telemetry: []model.RDIThingModelItem{
			{Identifier: "temperature_1", Name: "传感器 1 温度", Kind: "telemetry", DataType: "Number", Unit: "C", Range: "-40~125", Required: true, Description: "NTC T1 环境温度"},
			{Identifier: "temperature_2", Name: "传感器 2 温度", Kind: "telemetry", DataType: "Number", Unit: "C", Range: "-40~125", Required: true, Description: "NTC T2 吸气管温度"},
			{Identifier: "switch_1", Name: "开关 1 状态", Kind: "telemetry", DataType: "Bool", Required: true, Description: "N/C 节点 1 上电状态"},
			{Identifier: "switch_2", Name: "开关 2 状态", Kind: "telemetry", DataType: "Bool", Required: true, Description: "N/C 节点 2 上电状态"},
			{Identifier: "dry_contact_output", Name: "干接点输出", Kind: "telemetry", DataType: "Bool", Required: true, Description: "true 表示高电平/闭合，false 表示低电平/断开"},
			{Identifier: "electricity_consumption", Name: "用电量", Kind: "telemetry", DataType: "Number", Unit: "kWh", Range: "0~999999", Required: true, Description: "日、周、月累计用电量"},
			{Identifier: "led_status", Name: "LED1 状态", Kind: "telemetry", DataType: "Enum", Enum: []string{"off", "solid", "slow_blink", "fast_blink", "error"}, Required: false, Description: "LED1 指示模式：off/solid/slow_blink/fast_blink/error"},
		},
		Properties: []model.RDIThingModelItem{
			{Identifier: "pid_number", Name: "PID 编号", Kind: "property", DataType: "String", ReadWrite: "r", Range: "12 alphanumeric characters", Required: true},
			{Identifier: "firmware_version", Name: "固件版本", Kind: "property", DataType: "String", ReadWrite: "r", Required: true},
			{Identifier: "wifi_rssi", Name: "WiFi 信号强度 RSSI", Kind: "property", DataType: "Number", Unit: "dBm", ReadWrite: "r", Required: false},
			{Identifier: "ethernet_connected", Name: "以太网连接状态", Kind: "property", DataType: "Bool", ReadWrite: "r", Required: false},
			{Identifier: "connection_type", Name: "联网方式", Kind: "property", DataType: "Enum", ReadWrite: "r", Enum: []string{"wifi", "ethernet"}, Required: true},
			{Identifier: "data_collection_interval", Name: "数据采集间隔", Kind: "property", DataType: "Number", Unit: "s", Range: "45~60", ReadWrite: "rw", Default: 60, Required: true},
			{Identifier: "alarm_sensor_1_enabled", Name: "传感器 1 报警启用", Kind: "property", DataType: "Bool", ReadWrite: "rw", Default: true, Required: true},
			{Identifier: "alarm_sensor_2_enabled", Name: "传感器 2 报警启用", Kind: "property", DataType: "Bool", ReadWrite: "rw", Default: true, Required: true},
			{Identifier: "sensor_1_upper", Name: "传感器 1 上限", Kind: "property", DataType: "Number", Unit: "C", Range: "-40~125", ReadWrite: "rw", Default: 80, Required: true},
			{Identifier: "sensor_1_lower", Name: "传感器 1 下限", Kind: "property", DataType: "Number", Unit: "C", Range: "-40~125", ReadWrite: "rw", Default: -10, Required: true},
			{Identifier: "sensor_2_upper", Name: "传感器 2 上限", Kind: "property", DataType: "Number", Unit: "C", Range: "-40~125", ReadWrite: "rw", Default: 80, Required: true},
			{Identifier: "sensor_2_lower", Name: "传感器 2 下限", Kind: "property", DataType: "Number", Unit: "C", Range: "-40~125", ReadWrite: "rw", Default: -10, Required: true},
			{Identifier: "sensor_1_duration", Name: "传感器 1 报警持续时间", Kind: "property", DataType: "Number", Unit: "s", Range: "0~86400", ReadWrite: "rw", Default: 30, Required: true},
			{Identifier: "sensor_2_duration", Name: "传感器 2 报警持续时间", Kind: "property", DataType: "Number", Unit: "s", Range: "0~86400", ReadWrite: "rw", Default: 30, Required: true},
			{Identifier: "switch_1_alarm_mode", Name: "开关 1 报警模式", Kind: "property", DataType: "Enum", ReadWrite: "rw", Enum: []string{"powered_on", "powered_off", "disabled"}, Required: true},
			{Identifier: "switch_2_alarm_mode", Name: "开关 2 报警模式", Kind: "property", DataType: "Enum", ReadWrite: "rw", Enum: []string{"powered_on", "powered_off", "disabled"}, Required: true},
			{Identifier: "switch_1_alarm_duration", Name: "开关 1 报警持续时间", Kind: "property", DataType: "Number", Unit: "s", Range: "0~86400", ReadWrite: "rw", Default: 30, Required: true},
			{Identifier: "switch_2_alarm_duration", Name: "开关 2 报警持续时间", Kind: "property", DataType: "Number", Unit: "s", Range: "0~86400", ReadWrite: "rw", Default: 30, Required: true},
			{Identifier: "dry_contact_alarm_level", Name: "干接点报警电平", Kind: "property", DataType: "Enum", ReadWrite: "rw", Enum: []string{"high", "low"}, Required: true},
			{Identifier: "dry_contact_normal_level", Name: "干接点正常电平", Kind: "property", DataType: "Enum", ReadWrite: "rw", Enum: []string{"high", "low"}, Required: true},
			{Identifier: "dry_contact_alarm_delay", Name: "干接点报警延迟", Kind: "property", DataType: "Number", Unit: "s", Range: "0~86400", ReadWrite: "rw", Default: 0, Required: true, Description: "0 到 24 小时，和控制器手册及参考界面保持一致"},
			{Identifier: "dry_contact_normal_delay", Name: "干接点恢复延迟", Kind: "property", DataType: "Number", Unit: "s", Range: "0~86400", ReadWrite: "rw", Default: 0, Required: true, Description: "0 到 24 小时，和控制器手册及参考界面保持一致"},
			{Identifier: "notification_enabled", Name: "消息推送启用", Kind: "property", DataType: "Bool", ReadWrite: "rw", Default: false, Required: true},
			{Identifier: "notification_temperature_alarm", Name: "温度报警通知", Kind: "property", DataType: "Bool", ReadWrite: "rw", Default: true, Required: true},
			{Identifier: "notification_switch_alarm", Name: "开关报警通知", Kind: "property", DataType: "Bool", ReadWrite: "rw", Default: true, Required: true},
			{Identifier: "notification_warranty_alarm", Name: "质保报警通知", Kind: "property", DataType: "Bool", ReadWrite: "rw", Default: true, Required: false},
			{Identifier: "sensor_alarm_emails", Name: "传感器报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false},
			{Identifier: "switch_alarm_emails", Name: "开关报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false},
			{Identifier: "warranty_alarm_emails", Name: "质保报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false},
			{Identifier: "sensor_1_alarm_emails", Name: "传感器 1 报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false, Description: "设置后覆盖 T1 的 sensor_alarm_emails"},
			{Identifier: "sensor_2_alarm_emails", Name: "传感器 2 报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false, Description: "设置后覆盖 T2 的 sensor_alarm_emails"},
			{Identifier: "switch_1_alarm_emails", Name: "开关 1 报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false, Description: "设置后覆盖 NC_INPUT_1 的 switch_alarm_emails"},
			{Identifier: "switch_2_alarm_emails", Name: "开关 2 报警邮箱", Kind: "property", DataType: "String", ReadWrite: "rw", Required: false, Description: "设置后覆盖 NC_INPUT_2 的 switch_alarm_emails"},
		},
		Events: []model.RDIThingModelItem{
			{Identifier: "temperature_alarm", Name: "温度报警", Kind: "event", DataType: "alarm", Required: true, Description: "包含 sensor_id、temperature、threshold、direction"},
			{Identifier: "switch_alarm", Name: "开关报警", Kind: "event", DataType: "alarm", Required: true, Description: "包含 switch_id、state、alarm_mode"},
			{Identifier: "warranty_alarm", Name: "质保报警", Kind: "event", DataType: "alarm", Required: false, Description: "包含 component_id、warranty_status、description"},
			{Identifier: "switch_change", Name: "开关状态变化", Kind: "event", DataType: "info", Required: true, Description: "包含 switch_id、new_state"},
			{Identifier: "device_online", Name: "设备上线", Kind: "event", DataType: "info", Required: true, Description: "包含 ip、connection_type"},
			{Identifier: "device_offline", Name: "设备离线", Kind: "event", DataType: "info", Required: true, Description: "包含 last_seen"},
			{Identifier: "sw3_short_press", Name: "SW3 短按", Kind: "event", DataType: "info", Required: true, Description: "解绑平台账号"},
			{Identifier: "sw3_long_press", Name: "SW3 长按", Kind: "event", DataType: "info", Required: true, Description: "长按到达阈值后恢复出厂设置"},
			{Identifier: "sw2_long_press", Name: "SW2 长按", Kind: "event", DataType: "info", Required: false, Description: "长按 3 秒及以上进入 WiFi 配网模式，参数：duration"},
			{Identifier: "ota_progress", Name: "OTA 进度", Kind: "event", DataType: "info", Required: true, Description: "包含 status、progress、version"},
		},
		Services: []model.RDIServiceModelItem{
			{Identifier: "set_dry_contact", Name: "设置干接点输出", CallType: "async", Inputs: []string{"level", "delay_seconds"}, Outputs: []string{"result"}},
			{Identifier: "set_alarm_config", Name: "设置报警配置", CallType: "async", Inputs: []string{
				"data_collection_interval",
				"alarm_sensor_1_enabled",
				"alarm_sensor_2_enabled",
				"sensor_1_upper",
				"sensor_1_lower",
				"sensor_2_upper",
				"sensor_2_lower",
				"sensor_1_duration",
				"sensor_2_duration",
				"switch_1_alarm_mode",
				"switch_2_alarm_mode",
				"switch_1_alarm_duration",
				"switch_2_alarm_duration",
				"dry_contact_alarm_level",
				"dry_contact_normal_level",
				"dry_contact_alarm_delay",
				"dry_contact_normal_delay",
				"notification_enabled",
				"notification_temperature_alarm",
				"notification_switch_alarm",
				"notification_warranty_alarm",
				"sensor_alarm_emails",
				"switch_alarm_emails",
				"warranty_alarm_emails",
				"sensor_1_alarm_emails",
				"sensor_2_alarm_emails",
				"switch_1_alarm_emails",
				"switch_2_alarm_emails",
			}, Outputs: []string{"result"}},
			{Identifier: "set_field_setting", Name: "设置现场参数", CallType: "async", Inputs: []string{"n00-n07", "sw1-sw4"}, Outputs: []string{"result"}},
			{Identifier: "test_dry_contact", Name: "测试干接点输出", CallType: "async", Inputs: []string{"level", "duration_seconds"}, Outputs: []string{"result"}},
			{Identifier: "ota_upgrade", Name: "OTA 升级", CallType: "async", Inputs: []string{"firmware_url", "version", "size", "md5"}, Outputs: []string{"result"}},
			{Identifier: "unbind_device", Name: "解绑设备", CallType: "async", Inputs: []string{}, Outputs: []string{"result"}},
			{Identifier: "factory_reset", Name: "恢复出厂设置", CallType: "async", Inputs: []string{}, Outputs: []string{"result"}},
		},
	}
}

func (*RDI) ThingModel() model.RDIThingModel {
	return RDIThingModelDefinition()
}
