// 文件用途：提供统一集成实体（Integration，TB-45）HTTP 处理器。
// 核心逻辑：入参绑定校验、鉴权边界注入（claims）、Service 分发与统一 API 响应包装。
// 关键注意事项：租户边界一律取 claims，不读请求体；未登录（claims 缺失）由 service 层拒绝。
// 重构建议：后续 connector 类型增多时，可为校验类端点（如连接测试）扩展本文件而非散落路由层。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type IntegrationApi struct{}

// CreateIntegration 创建集成实例
// @Router   /api/v1/integrations [post]
func (*IntegrationApi) CreateIntegration(c *gin.Context) {
	var req model.CreateIntegrationReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Integration.CreateIntegration(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// UpdateIntegration 更新集成实例
// @Router   /api/v1/integrations [put]
func (*IntegrationApi) UpdateIntegration(c *gin.Context) {
	var req model.UpdateIntegrationReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Integration.UpdateIntegration(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetIntegrationByID 查询单个集成实例
// @Router   /api/v1/integrations/:id [get]
func (*IntegrationApi) GetIntegrationByID(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Integration.GetIntegrationByID(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteIntegration 删除集成实例
// @Router   /api/v1/integrations/:id [delete]
func (*IntegrationApi) DeleteIntegration(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.Integration.DeleteIntegration(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{"id": id})
}

// ListIntegrations 分页查询列表
// @Router   /api/v1/integrations [get]
func (*IntegrationApi) ListIntegrations(c *gin.Context) {
	var req model.GetIntegrationListReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Integration.ListIntegrations(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
