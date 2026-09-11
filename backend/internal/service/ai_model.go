// 文件用途：AI 2.0（ROADMAP D7）服务层——模型中心 CRUD、共享 LLM 调用与助手对话。
// 核心逻辑：
//   - 模型中心：租户内录入/维护 OpenAI 兼容模型档案；出参一律脱敏 api_key。
//   - 共享调用：aiChatCompletion(ctx, 端点, 密钥, 模型名, messages, ...) 统一 chat/completions 调用，
//     供助手（HTTP 入口）与规则链 ai.inference 节点复用。
//   - 助手：model_id 命中模型中心 → 用档案；省略 → 回退全局 ai.llm.* 配置（viper）；
//     两者皆无 → 显式"未配置"错误，不伪装成功（与 C4 AI 集成口径一致）。
//
// 关键注意事项：api_key 仅落库不出参；HTTP 超时与 C4 保持同级（30s）。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/secrets"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// AiModelService AI 2.0 模型中心业务服务。
type AiModelService struct{}

const (
	aiModelHTTPTimeout    = 30 * time.Second
	aiModelMaxMessages    = 40
	aiModelMaxReplyChars  = 65536
	aiModelAPIKeyMaskHead = 4
)

// sealModelAPIKey 用租户绑定的 AAD 加密明文密钥并写回档案。
// 主密钥不可用时 fail closed：绝不把明文落库（P0.7 门禁）。
func sealModelAPIKey(m *model.AiModel, plain string) error {
	sealed, err := secrets.Seal(plain, m.TenantID)
	if err != nil {
		return err
	}
	m.APIKey = sealed
	return nil
}

// openModelAPIKey 取回明文密钥，并报告是否需要重新封装。
// 迁移窗口内遗留明文行仍可读，但必须标记为需要重新封装。
func openModelAPIKey(m *model.AiModel) (string, bool, error) {
	if m == nil {
		return "", false, errors.New("ai model is nil")
	}
	if !secrets.IsEnvelope(m.APIKey) {
		return m.APIKey, true, nil
	}
	plain, err := secrets.Open(m.APIKey, m.TenantID)
	if err != nil {
		return "", false, err
	}
	return plain, secrets.NeedsReseal(m.APIKey), nil
}

// aiModelMasked 出参脱敏转换；plain 为已解密明文。
// 解密失败时传空串，只出全掩码，不回显任何密文或明文片段。
func aiModelMasked(m *model.AiModel, plain string) *model.AiModelResp {
	masked := secrets.Mask(plain, aiModelAPIKeyMaskHead)
	return &model.AiModelResp{
		ID:           m.ID,
		Name:         m.Name,
		Provider:     m.Provider,
		BaseURL:      m.BaseURL,
		Model:        m.Model,
		APIKeyMasked: masked,
		Purpose:      m.Purpose,
		Enabled:      m.Enabled,
		CreatedAt:    m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    m.UpdatedAt.Format(time.RFC3339),
	}
}

// CreateAiModel 录入模型档案。
func (AiModelService) CreateAiModel(ctx context.Context, req *model.CreateAiModelReq, claims *utils.UserClaims) (*model.AiModelResp, error) {
	now := time.Now()
	baseURL := normalizeAILLMBaseURL(req.BaseURL)
	if _, err := validateAILLMBaseURL(ctx, baseURL); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, errAILLMInvalidConfiguration.Error())
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "AI api_key is required")
	}
	m := &model.AiModel{
		ID:        uuid.New().String(),
		TenantID:  claims.TenantID,
		Name:      strings.TrimSpace(req.Name),
		Provider:  "openai",
		BaseURL:   baseURL,
		Model:     strings.TrimSpace(req.Model),
		Purpose:   req.Purpose,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if m.BaseURL == "" {
		m.BaseURL = "https://api.openai.com/v1"
	}
	if m.Purpose == "" {
		m.Purpose = model.AiModelPurposeChat
	}
	// 明文密钥只在此处短暂存在，落库前必须封装成功，否则 fail closed。
	if err := sealModelAPIKey(m, apiKey); err != nil {
		return nil, errcode.WithData(errcode.CodeSystemError,
			map[string]interface{}{"error": "ai credential encryption unavailable: " + err.Error()})
	}
	if err := dal.CreateAiModel(m); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return aiModelMasked(m, apiKey), nil
}

// UpdateAiModel 更新模型档案（api_key 留空不改）。
func (AiModelService) UpdateAiModel(ctx context.Context, req *model.UpdateAiModelReq, claims *utils.UserClaims) (*model.AiModelResp, error) {
	m, err := dal.GetAiModelInTenant(req.ID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ai model not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if req.Name != "" {
		m.Name = strings.TrimSpace(req.Name)
	}
	if req.BaseURL != "" {
		baseURL := normalizeAILLMBaseURL(req.BaseURL)
		if _, err := validateAILLMBaseURL(ctx, baseURL); err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, errAILLMInvalidConfiguration.Error())
		}
		m.BaseURL = baseURL
	}
	if req.Model != "" {
		m.Model = strings.TrimSpace(req.Model)
	}
	// 出参掩码需要明文：本次更新提供则直接用，否则从信封解出。
	maskSource := ""
	if req.APIKey != "" {
		apiKey := strings.TrimSpace(req.APIKey)
		if apiKey == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "AI api_key must not be blank")
		}
		if err := sealModelAPIKey(m, apiKey); err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError,
				map[string]interface{}{"error": "ai credential encryption unavailable: " + err.Error()})
		}
		maskSource = apiKey
	} else {
		plain, _, err := openModelAPIKey(m)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeSystemError,
				map[string]interface{}{"error": "ai credential decryption failed: " + err.Error()})
		}
		maskSource = plain
	}
	if req.Purpose != "" {
		m.Purpose = req.Purpose
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}
	m.UpdatedAt = time.Now()
	if err := dal.UpdateAiModel(m); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return aiModelMasked(m, maskSource), nil
}

// DeleteAiModel 删除模型档案。
func (AiModelService) DeleteAiModel(id string, claims *utils.UserClaims) error {
	rows, err := dal.DeleteAiModelInTenant(id, claims.TenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if rows == 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "ai model not found")
	}
	return nil
}

// GetAiModel 单条档案（脱敏）。
func (AiModelService) GetAiModel(id string, claims *utils.UserClaims) (*model.AiModelResp, error) {
	m, err := dal.GetAiModelInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ai model not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	plain, _, err := openModelAPIKey(m)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeSystemError,
			map[string]interface{}{"error": "ai credential decryption failed: " + err.Error()})
	}
	return aiModelMasked(m, plain), nil
}

// ListAiModels 列档案（脱敏）。
func (AiModelService) ListAiModels(purpose string, limit int, claims *utils.UserClaims) ([]*model.AiModelResp, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	list, err := dal.ListAiModels(claims.TenantID, purpose, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	out := make([]*model.AiModelResp, 0, len(list))
	for _, m := range list {
		// 单行解密失败不回显密文：仅该行退化为全掩码，其余行照常返回。
		plain, _, openErr := openModelAPIKey(m)
		if openErr != nil {
			plain = ""
		}
		out = append(out, aiModelMasked(m, plain))
	}
	return out, nil
}

func normalizeAILLMBaseURL(rawURL string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if baseURL == "" {
		return "https://api.openai.com/v1"
	}
	return baseURL
}

// AiAssistantChat 助手对话：模型中心优先，回退全局 ai.llm.* 配置。
func (AiModelService) AiAssistantChat(ctx context.Context, req *model.AiAssistantChatReq, claims *utils.UserClaims) (*model.AiAssistantChatResp, error) {
	if len(req.Messages) > aiModelMaxMessages {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "too many messages")
	}
	msgs := make([]aiLLMChatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, aiLLMChatMessage{Role: m.Role, Content: m.Content})
	}

	// 取数路径 1：模型中心档案。
	if req.ModelID != "" {
		m, err := dal.GetAiModelInTenant(req.ModelID, claims.TenantID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "ai model not found")
			}
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		if !m.Enabled {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ai model is disabled")
		}
		apiKey, needsReseal, derr := openModelAPIKey(m)
		if derr != nil {
			return nil, errcode.WithData(errcode.CodeSystemError,
				map[string]interface{}{"error": "ai credential decryption failed: " + derr.Error()})
		}
		if apiKey == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ai model api_key is not configured")
		}
		// 轮换/迁移窗口：顺手把旧密文或遗留明文重写到当前主密钥。
		if needsReseal {
			if serr := sealModelAPIKey(m, apiKey); serr == nil {
				_ = dal.UpdateAiModel(m)
			}
		}
		reply, cerr := aiChatCompletion(ctx, m.BaseURL, apiKey, m.Model, msgs, req.MaxTokens, req.Temperature)
		if cerr != nil {
			return nil, cerr
		}
		return &model.AiAssistantChatResp{Reply: reply, Model: m.Model, SourceKind: "model_center"}, nil
	}

	// 取数路径 2：全局 ai.llm.* 配置回退（与 C4 同源）。
	baseURL, apiKey, modelName := aiLLMConfigFallback()
	if apiKey == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			"AI integration is not configured; provide model_id or set ai.llm.api_key (and optionally ai.llm.base_url / ai.llm.model)")
	}
	reply, cerr := aiChatCompletion(ctx, baseURL, apiKey, modelName, msgs, req.MaxTokens, req.Temperature)
	if cerr != nil {
		return nil, cerr
	}
	return &model.AiAssistantChatResp{Reply: reply, Model: modelName, SourceKind: "global_config"}, nil
}

// aiLLMConfigFallback 全局 LLM 配置回退（复用 C4 的 viper 键位）。
func aiLLMConfigFallback() (baseURL, apiKey, modelName string) {
	if v := strings.TrimSpace(viper.GetString("ai.llm.base_url")); v != "" {
		baseURL = strings.TrimRight(v, "/")
	} else {
		baseURL = "https://api.openai.com/v1"
	}
	if v := strings.TrimSpace(viper.GetString("ai.llm.api_key")); v != "" {
		apiKey = v
	}
	if v := strings.TrimSpace(viper.GetString("ai.llm.model")); v != "" {
		modelName = v
	} else {
		modelName = "gpt-4o-mini"
	}
	return
}
