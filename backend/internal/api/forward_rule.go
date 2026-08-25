// 文件用途：数据转发规则 API Handler——CRUD、启停与分页查询。
// 核心逻辑：绑定参数 → 读取 claims 租户作用域 → 委托 service → 统一响应中间件返回。
// 关键注意事项：出参 mqtt_password 恒为掩码（service 层装配）；写操作建议仅租户管理员使用。

package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ForwardRuleApi struct{}

// HandleCreate 新建转发规则。
func (*ForwardRuleApi) HandleCreate(c *gin.Context) {
	var rule model.ForwardRule
	if !BindAndValidate(c, &rule) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	out, err := service.GroupApp.ForwardRuleService.CreateForwardRule(&rule, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", out)
}

// HandleUpdate 更新转发规则。
func (*ForwardRuleApi) HandleUpdate(c *gin.Context) {
	var rule model.ForwardRule
	if !BindAndValidate(c, &rule) {
		return
	}
	rule.ID = c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	out, err := service.GroupApp.ForwardRuleService.UpdateForwardRule(&rule, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", out)
}

// HandleDelete 删除转发规则。
func (*ForwardRuleApi) HandleDelete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.ForwardRuleService.DeleteForwardRule(id, claims.TenantID); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleToggle 启停切换。
func (*ForwardRuleApi) HandleToggle(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !BindAndValidate(c, &req) {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.ForwardRuleService.ToggleForwardRule(id, claims.TenantID, req.Enabled); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleGetByID 详情。
func (*ForwardRuleApi) HandleGetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	out, err := service.GroupApp.ForwardRuleService.GetForwardRuleByID(id, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", out)
}

// HandleListByPage 分页查询。
func (*ForwardRuleApi) HandleListByPage(c *gin.Context) {
	var req model.GetForwardRuleListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	out, err := service.GroupApp.ForwardRuleService.GetForwardRuleListByPage(&req, claims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", out)
}
