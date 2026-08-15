// 文件用途：定义 payload schema registry 的校验请求与结果 DTO。
// 核心逻辑：把"设备上行 payload 应满足的字段约束"表达为一组 PayloadSchemaField,
//
//	并让校验引擎针对样本 payload 产出结构化诊断,供前端逐条展示。
//
// 关键注意事项：该校验是静态推演(stateless dry-run 风格),不落库、不连 broker、不改协议契约。
//
//	broker 侧真实拦截属于外部 MQTT 契约的破坏性变更,需运行时验证,不在本 DTO 覆盖范围内。
//
// 重构建议：若后续引入持久化 registry 表,应新增 gen 模型与迁移,并保持本校验引擎为纯函数复用。
package model

// PayloadSchemaFieldType 枚举支持的字段类型。
const (
	PayloadSchemaFieldTypeNumber  = "number"
	PayloadSchemaFieldTypeString  = "string"
	PayloadSchemaFieldTypeBoolean = "boolean"
	PayloadSchemaFieldTypeObject  = "object"
	PayloadSchemaFieldTypeArray   = "array"
)

// PayloadSchemaField 描述 payload 中单个字段的约束。
type PayloadSchemaField struct {
	Name     string   `json:"name" validate:"required,max=128"`
	Type     string   `json:"type" validate:"required,oneof=number string boolean object array"`
	Required bool     `json:"required"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	Pattern  *string  `json:"pattern,omitempty" validate:"omitempty,max=500"`
}

// ValidatePayloadReq 提交一份 schema 与一份样本 payload 做静态校验。
// SamplerPayload 为设备上行 payload 的 JSON 字符串;引擎只做结构与约束匹配,不执行任何副作用。
type ValidatePayloadReq struct {
	SchemaName    string               `json:"schema_name" validate:"omitempty,max=128"`
	Strict        bool                 `json:"strict"`
	Fields        []PayloadSchemaField `json:"fields" validate:"required,dive"`
	SamplePayload string               `json:"sample_payload" validate:"required"`
}

// PayloadSchemaValidationDiagnostic 是单条校验诊断。
type PayloadSchemaValidationDiagnostic struct {
	Severity string `json:"severity"` // error | warning | success
	Scope    string `json:"scope"`    // field name 或 payload
	Message  string `json:"message"`
}

// ValidatePayloadResult 汇总一次静态校验的结论。
type ValidatePayloadResult struct {
	Supported     bool                                `json:"supported"`
	Valid         bool                                `json:"valid"`
	Summary       string                              `json:"summary"`
	FieldCount    int                                 `json:"field_count"`
	CheckedFields int                                 `json:"checked_fields"`
	Errors        []string                            `json:"errors"`
	Warnings      []string                            `json:"warnings"`
	UnknownKeys   []string                            `json:"unknown_keys"`
	Diagnostics   []PayloadSchemaValidationDiagnostic `json:"diagnostics"`
	NextSteps     []string                            `json:"next_steps"`
	IsSimulation  bool                                `json:"is_simulation"`
}
