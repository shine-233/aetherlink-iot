// 文件用途：定义 device topic mappings 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateDeviceTopicMappingReq struct {
	DeviceConfigID string  `json:"device_config_id" validate:"required,uuid"`      // 设备配置ID
	Name           string  `json:"name" validate:"required,max=500"`               // 规则名称
	Direction      string  `json:"direction" validate:"required,oneof=up down"`    // 方向：up-上行，down-下行
	SourceTopic    string  `json:"source_topic" validate:"required,max=500"`       // 源主题
	TargetTopic    string  `json:"target_topic" validate:"required,max=500"`       // 目标主题
	Priority       *int32  `json:"priority" validate:"omitempty,gte=0,lte=100000"` // 优先级，数值越小优先级越高
	Enabled        *bool   `json:"enabled" validate:"omitempty"`                   // 是否启用
	Description    *string `json:"description" validate:"omitempty,max=2000"`      // 描述信息
	DataIdentifier *string `json:"data_identifier" validate:"omitempty,max=500"`   // 数据标识符
}

type ListDeviceTopicMappingReq struct {
	DeviceConfigID string  `form:"device_config_id" validate:"required,uuid"`
	Page           int     `form:"page" validate:"omitempty,gte=1"`
	PageSize       int     `form:"page_size" validate:"omitempty,gte=1,lte=1000"`
	Direction      *string `form:"direction" validate:"omitempty,oneof=up down"`
	SourceTopic    *string `form:"source_topic" validate:"omitempty,max=500"` // 模糊匹配
	TargetTopic    *string `form:"target_topic" validate:"omitempty,max=500"` // 精确匹配
	Enabled        *bool   `form:"enabled" validate:"omitempty"`
	Description    *string `form:"description" validate:"omitempty,max=2000"`    // 模糊匹配
	DataIdentifier *string `form:"data_identifier" validate:"omitempty,max=500"` // 精确匹配
}

type UpdateDeviceTopicMappingReq struct {
	DeviceConfigID *string `json:"device_config_id" validate:"omitempty,uuid"`
	Name           *string `json:"name" validate:"omitempty,max=500"`
	Direction      *string `json:"direction" validate:"omitempty,oneof=up down"`
	SourceTopic    *string `json:"source_topic" validate:"omitempty,max=500"`
	TargetTopic    *string `json:"target_topic" validate:"omitempty,max=500"`
	Priority       *int32  `json:"priority" validate:"omitempty,gte=0,lte=100000"`
	Enabled        *bool   `json:"enabled" validate:"omitempty"`
	Description    *string `json:"description" validate:"omitempty,max=2000"`
	DataIdentifier *string `json:"data_identifier" validate:"omitempty,max=500"`
}

type DryRunDeviceTopicMappingReq struct {
	DeviceConfigID string  `json:"device_config_id" validate:"required,uuid"`
	Direction      string  `json:"direction" validate:"required,oneof=up down"`
	SourceTopic    string  `json:"source_topic" validate:"required,max=500"`
	TargetTopic    string  `json:"target_topic" validate:"required,max=500"`
	TestTopic      string  `json:"test_topic" validate:"omitempty,max=500"`
	SampleTopic    string  `json:"sample_topic" validate:"omitempty,max=500"`
	DataIdentifier *string `json:"data_identifier" validate:"omitempty,max=500"`
}

type DeviceTopicMappingDryRunDiagnostic struct {
	Severity string `json:"severity"`
	Scope    string `json:"scope"`
	Message  string `json:"message"`
}

type DryRunDeviceTopicMappingResp struct {
	Matched        bool                                 `json:"matched"`
	Direction      string                               `json:"direction"`
	SourceTopic    string                               `json:"source_topic"`
	TargetTopic    string                               `json:"target_topic"`
	TestTopic      string                               `json:"test_topic"`
	SampleTopic    string                               `json:"sample_topic"`
	ResolvedTopic  string                               `json:"resolved_topic"`
	DataIdentifier *string                              `json:"data_identifier,omitempty"`
	Diagnostics    []DeviceTopicMappingDryRunDiagnostic `json:"diagnostics"`
	NextSteps      []string                             `json:"next_steps"`
}
