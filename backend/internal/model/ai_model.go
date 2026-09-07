// 文件用途：AI 2.0（ROADMAP D7）模型中心数据模型与 HTTP 请求/响应体。
// 核心逻辑：租户级 AI 模型档案（OpenAI 兼容端点 + 模型名 + 密钥 + 用途），供助手与规则链 AI 节点取用。
// 关键注意事项：api_key 落库存原文（本地栈）；任何出参一律脱敏（仅前 4 位 + 掩码），详见 service 层。
package model

import "time"

const TableNameAiModel = "ai_models"

// 模型用途。
const (
	AiModelPurposeChat     = "chat"
	AiModelPurposeAnalysis = "analysis"
	AiModelPurposeIntent   = "intent"
)

// AiModel 租户级 AI 模型档案（模型中心）。
type AiModel struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID  string    `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	Name      string    `gorm:"column:name;not null" json:"name"`
	Provider  string    `gorm:"column:provider;not null;default:openai" json:"provider"` // openai 兼容
	BaseURL   string    `gorm:"column:base_url;not null" json:"base_url"`
	Model     string    `gorm:"column:model;not null" json:"model"`
	APIKey    string    `gorm:"column:api_key;type:text;not null" json:"-"`
	Purpose   string    `gorm:"column:purpose;not null;default:chat" json:"purpose"`
	Enabled   bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (*AiModel) TableName() string { return TableNameAiModel }

// AiModelResp 模型出参（api_key 脱敏）。
type AiModelResp struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	APIKeyMasked string `json:"api_key_masked"`
	Purpose      string `json:"purpose"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ---- HTTP 请求结构体 ----

// CreateAiModelReq 录入模型档案。
type CreateAiModelReq struct {
	Name    string `json:"name" validate:"required,max=128"`
	BaseURL string `json:"base_url" validate:"omitempty,url,max=512"`
	Model   string `json:"model" validate:"required,max=128"`
	APIKey  string `json:"api_key" validate:"required,max=512"`
	Purpose string `json:"purpose" validate:"omitempty,oneof=chat analysis intent"`
}

// UpdateAiModelReq 更新模型档案（api_key 留空表示不修改）。
type UpdateAiModelReq struct {
	ID      string `json:"id" validate:"required"`
	Name    string `json:"name" validate:"omitempty,max=128"`
	BaseURL string `json:"base_url" validate:"omitempty,url,max=512"`
	Model   string `json:"model" validate:"omitempty,max=128"`
	APIKey  string `json:"api_key" validate:"omitempty,max=512"`
	Purpose string `json:"purpose" validate:"omitempty,oneof=chat analysis intent"`
	Enabled *bool  `json:"enabled"`
}

// AiChatMessage 助手对话消息。
type AiChatMessage struct {
	Role    string `json:"role" validate:"required,oneof=system user assistant"`
	Content string `json:"content" validate:"required,max=32768"`
}

// AiAssistantChatReq 助手对话请求；model_id 省略时回退全局 ai.llm.* 配置。
type AiAssistantChatReq struct {
	ModelID     string          `json:"model_id" validate:"omitempty,max=36"`
	Messages    []AiChatMessage `json:"messages" validate:"required,min=1,dive"`
	MaxTokens   int             `json:"max_tokens" validate:"omitempty,min=1,max=32768"`
	Temperature *float64        `json:"temperature" validate:"omitempty,gte=0,lte=2"`
}

// AiAssistantChatResp 助手对话响应。
type AiAssistantChatResp struct {
	Reply      string `json:"reply"`
	Model      string `json:"model"`
	SourceKind string `json:"source_kind"` // model_center | global_config
}
