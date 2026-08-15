// 文件用途：定义 device debug 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

// DeviceDebugConfig mirrors the gmqtt-side debug config stored in Redis.
type DeviceDebugConfig struct {
	Enabled         bool  `json:"enabled"`
	ExpireAt        int64 `json:"expire_at"`
	MaxItems        int   `json:"max_items"`
	PayloadMaxBytes int   `json:"payload_max_bytes"`
}

// SetDeviceDebugReq enables/disables debug and updates config.
// If both Duration and ExpireAt are omitted, Duration defaults to 30 minutes.
// If Enabled is explicitly false, config will be removed (debug off).
type SetDeviceDebugReq struct {
	Enabled         *bool  `json:"enabled" validate:"omitempty"`
	Duration        *int64 `json:"duration" validate:"omitempty,gte=0,lte=604800"` // seconds, up to 7 days
	ExpireAt        *int64 `json:"expire_at" validate:"omitempty,gte=0"`
	MaxItems        *int   `json:"max_items" validate:"omitempty,gte=1,lte=5000"`
	PayloadMaxBytes *int   `json:"payload_max_bytes" validate:"omitempty,gte=0,lte=65536"`
}

type GetDeviceDebugLogsReq struct {
	Offset int64 `json:"offset" form:"offset" validate:"omitempty,gte=0"`
	Limit  int64 `json:"limit" form:"limit" validate:"omitempty,gte=1,lte=500"`
}

// DeviceDebugLogEntry is stored as JSON strings in Redis list.
type DeviceDebugLogEntry struct {
	Ts       string `json:"ts"`
	DeviceID string `json:"device_id"`

	Protocol  string `json:"protocol,omitempty"`
	Direction string `json:"direction"`

	// Current fields (protocol-agnostic)
	Action  string                 `json:"action,omitempty"`
	Outcome string                 `json:"outcome,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`

	Error   string `json:"error,omitempty"`
	Payload string `json:"payload,omitempty"`

	// Previous log fields kept readable for records written before action/outcome/meta existed.
	Event  string                 `json:"event,omitempty"`
	Result string                 `json:"result,omitempty"`
	Extra  map[string]interface{} `json:"extra,omitempty"`
}
