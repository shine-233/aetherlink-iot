// 文件用途：承载设备编解码脚本处理模块的 constants 逻辑。
// 核心逻辑：围绕脚本缓存、Lua 沙箱执行、输入输出模型和处理器接口实现上下行数据转换，主要围绕 type DataType、var dataTypeToScriptType、func GetScriptType、func GetCacheKey 等声明展开。
// 关键注意事项：脚本处理涉及超时、沙箱和错误码，修改需保持上下行方向及失败语义清晰。
// 重构建议：后续可进一步拆分执行器、缓存和领域模型，降低处理器聚合复杂度。

package processor

import "time"

// DataType 数据类型
type DataType string

const (
	// 上行数据类型（设备 -> 平台）
	DataTypeTelemetry DataType = "telemetry" // 遥测数据上报
	DataTypeAttribute DataType = "attribute" // 属性数据上报
	DataTypeEvent     DataType = "event"     // 事件数据上报

	// 下行数据类型（平台 -> 设备）
	DataTypeTelemetryControl DataType = "telemetry_control" // 遥测数据下发
	DataTypeAttributeSet     DataType = "attribute_set"     // 属性设置
	DataTypeCommand          DataType = "command"           // 命令下发
)

// ScriptType 脚本类型（数据库中的脚本类型标识）
const (
	ScriptTypeTelemetryUplink   = "A" // 遥测上报预处理
	ScriptTypeTelemetryDownlink = "B" // 遥测下发预处理
	ScriptTypeAttributeUplink   = "C" // 属性上报预处理
	ScriptTypeAttributeDownlink = "D" // 属性下发预处理
	ScriptTypeCommand           = "E" // 命令
	ScriptTypeEvent             = "F" // 事件
)

// DataType 到 ScriptType 的映射关系
var dataTypeToScriptType = map[DataType]string{
	DataTypeTelemetry:        ScriptTypeTelemetryUplink,
	DataTypeAttribute:        ScriptTypeAttributeUplink,
	DataTypeEvent:            ScriptTypeEvent,
	DataTypeTelemetryControl: ScriptTypeTelemetryDownlink,
	DataTypeAttributeSet:     ScriptTypeAttributeDownlink,
	DataTypeCommand:          ScriptTypeCommand,
}

// GetScriptType 根据 DataType 获取对应的 ScriptType
func GetScriptType(dataType DataType) (string, bool) {
	scriptType, ok := dataTypeToScriptType[dataType]
	return scriptType, ok
}

// 执行引擎配置常量
const (
	ScriptTimeout        = 3 * time.Second  // 脚本执行超时时间：3秒
	ScriptMaxMemory      = 50 * 1024 * 1024 // 脚本最大内存：50MB
	ScriptMaxOpsCount    = 100000           // 脚本最大操作数：10万次
	CacheKeyPrefix       = ""               // 缓存 key 前缀（可根据需要添加）
	EnableFlagEnabled    = "Y"              // 脚本启用标识
	ScriptNotFoundMarker = "__NOT_FOUND__"  // 脚本不存在的标记（用于缓存）
)

// GetCacheKey 生成脚本缓存 key
// 格式: {deviceConfigID}_{scriptType}_script
func GetCacheKey(deviceConfigID, scriptType string) string {
	return CacheKeyPrefix + deviceConfigID + "_" + scriptType + "_script"
}
