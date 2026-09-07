// 文件用途：AI 2.0（ROADMAP D7）服务层——模型中心 CRUD、共享 LLM 调用与助手对话。
// 核心逻辑：
//   - 模型中心：租户内录入/维护 OpenAI 兼容模型档案；出参一律脱敏 api_key。
//   - 共享调用：aiChatCompletion(ctx, 端点, 密钥, 模型名, messages, ...) 统一 chat/completions 调用，
//     供助手（HTTP 入口）与规则链 ai.inference 节点复用。
//   - 助手：model_id 命中模型中心 → 用档案；省略 → 回退全局 ai.llm.* 配置（viper）；
//     两者皆无 → 显式"未配置"错误，不伪装成功（与 C4 AI 集成口径一致）。
// 关键注意事项：api_key 仅落库不出参；HTTP 超时与 C4 保持同级（30s）。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
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

// aiModelMasked 出参脱敏转换。
func aiModelMasked(m *model.AiModel) *model.AiModelResp {
	masked := "****"
	if len(m.APIKey) > aiModelAPIKeyMaskHead {
		masked = m.APIKey[:aiModelAPIKeyMaskHead] + "****"
	}
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
func (AiModelService) CreateAiModel(req *model.CreateAiModelReq, claims *utils.UserClaims) (*model.AiModelResp, error) {
	now := time.Now()
	m := &model.AiModel{
		ID:        uuid.New().String(),
		TenantID:  claims.TenantID,
		Name:      req.Name,
		Provider:  "openai",
		BaseURL:   strings.TrimRight(req.BaseURL, "/"),
		Model:     req.Model,
		APIKey:    req.APIKey,
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
	if err := dal.CreateAiModel(m); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return aiModelMasked(m), nil
}

// UpdateAiModel 更新模型档案（api_key 留空不改）。
func (AiModelService) UpdateAiModel(req *model.UpdateAiModelReq, claims *utils.UserClaims) (*model.AiModelResp, error) {
	m, err := dal.GetAiModelInTenant(req.ID, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "ai model not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if req.Name != "" {
		m.Name = req.Name
	}
	if req.BaseURL != "" {
		m.BaseURL = strings.TrimRight(req.BaseURL, "/")
	}
	if req.Model != "" {
		m.Model = req.Model
	}
	if req.APIKey != "" {
		m.APIKey = req.APIKey
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
	return aiModelMasked(m), nil
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
	return aiModelMasked(m), nil
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
		out = append(out, aiModelMasked(m))
	}
	return out, nil
}

// AiAssistantChat 助手对话：模型中心优先，回退全局 ai.llm.* 配置。
func (AiModelService) AiAssistantChat(req *model.AiAssistantChatReq, claims *utils.UserClaims) (*model.AiAssistantChatResp, error) {
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
		reply, cerr := aiChatCompletion(context.Background(), m.BaseURL, m.APIKey, m.Model, msgs, req.MaxTokens, req.Temperature)
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
	reply, cerr := aiChatCompletion(context.Background(), baseURL, apiKey, modelName, msgs, req.MaxTokens, req.Temperature)
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

// aiLLMChatMessage chat/completions 消息体。
type aiLLMChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// aiChatCompletion 统一 OpenAI 兼容 chat/completions 调用（助手与 ai.inference 节点共用）。
func aiChatCompletion(ctx context.Context, baseURL, apiKey, modelName string, msgs []aiLLMChatMessage, maxTokens int, temperature *float64) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "AI api_key is empty; configure model center entry or ai.llm.api_key")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	body := map[string]any{
		"model":    modelName,
		"messages": msgs,
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	if temperature != nil {
		body["temperature"] = *temperature
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "marshal llm request: "+err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "build llm request: "+err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: aiModelHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "llm request failed: "+err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, aiModelMaxReplyChars))
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "read llm response: "+err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return "", errcode.NewWithMessage(errcode.CodeParamError,
			fmt.Sprintf("llm http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody))))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "parse llm response: "+err.Error())
	}
	if len(parsed.Choices) == 0 {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "llm response has no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
