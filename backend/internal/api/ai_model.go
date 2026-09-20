// 文件用途：AI 2.0（ROADMAP D7）HTTP 入口——模型中心 CRUD 与助手对话。
// 边界说明：租户边界在 service 层处理；本层只做绑定、claims 提取与错误出口。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

type AiModelApi struct{}

// CreateModel 录入模型档案。
// POST /api/v1/ai/models
func (*AiModelApi) CreateModel(c *gin.Context) {
	var req model.CreateAiModelReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiModel.CreateAiModel(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// UpdateModel 更新模型档案。
// PUT /api/v1/ai/models
func (*AiModelApi) UpdateModel(c *gin.Context) {
	var req model.UpdateAiModelReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiModel.UpdateAiModel(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// DeleteModel 删除模型档案。
// DELETE /api/v1/ai/models/:id
func (*AiModelApi) DeleteModel(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.AiModel.DeleteAiModel(id, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"id": id})
}

// ListModels 列模型档案（api_key 脱敏）。
// GET /api/v1/ai/models?purpose=&limit=
func (*AiModelApi) ListModels(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiModel.ListAiModels(c.Query("purpose"), 0, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// GetModel 单条模型档案。
// GET /api/v1/ai/models/:id
func (*AiModelApi) GetModel(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiModel.GetAiModel(id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// Chat 助手对话（模型中心优先，回退全局 ai.llm.* 配置）。
// POST /api/v1/ai/assistant/chat
func (*AiModelApi) Chat(c *gin.Context) {
	var req model.AiAssistantChatReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.AiModel.AiAssistantChat(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
