// 文件用途：计算字段 HTTP Handler，承接管理端的 CRUD、启停开关与分页查询。
// 核心链路：Handler 绑定请求、注入 claims，再把业务下沉给 CalculatedFieldService；
// 更新/开关接口的 id 一律取自路径参数（rule.ID = c.Param("id")）。
// 关键注意事项：本 Handler 不做权限判断与租户解析，统一由 claims 与 service 层负责。
// 重构建议：若后续暴露批量启停或表达式 dry-run，优先新增独立端点而不是复用 update 语义。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// CalculatedFieldApi 计算字段控制器。
type CalculatedFieldApi struct{}

// HandleGetCalculatedFieldList 分页查询当前租户的计算字段。
// @Summary List calculated fields by page
// @Tags CalculatedField
// @Produce json
// @Param request query model.CalculatedFieldListReq true "Pagination and filters"
// @Success 200 {object} model.CalculatedFieldListRsp "Calculated field list"
// @Router /api/v1/calculated_fields [get]
func (*CalculatedFieldApi) HandleGetCalculatedFieldList(c *gin.Context) {
	var req model.CalculatedFieldListReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CalculatedField.GetCalculatedFieldList(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleCreateCalculatedField 创建计算字段。
// @Summary Create a calculated field
// @Tags CalculatedField
// @Accept json
// @Produce json
// @Param request body model.CalculatedFieldCreateReq true "Calculated field create request"
// @Success 200 {object} model.CalculatedField "Created calculated field"
// @Router /api/v1/calculated_fields [post]
func (*CalculatedFieldApi) HandleCreateCalculatedField(c *gin.Context) {
	var req model.CalculatedFieldCreateReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CalculatedField.CreateCalculatedField(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleUpdateCalculatedField 按路径 id 更新计算字段基础信息。
// @Summary Update a calculated field
// @Tags CalculatedField
// @Accept json
// @Produce json
// @Param id path string true "Calculated field id"
// @Param request body model.CalculatedFieldUpdateReq true "Calculated field update request"
// @Success 200 {object} model.CalculatedField "Updated calculated field"
// @Router /api/v1/calculated_fields/{id} [put]
func (*CalculatedFieldApi) HandleUpdateCalculatedField(c *gin.Context) {
	var req model.CalculatedFieldUpdateReq
	req.ID = c.Param("id")
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CalculatedField.UpdateCalculatedField(req.ID, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleToggleCalculatedField 启用/停用计算字段；body 可省略时按当前值取反。
// @Summary Toggle a calculated field enabled state
// @Tags CalculatedField
// @Accept json
// @Produce json
// @Param id path string true "Calculated field id"
// @Param request body model.CalculatedFieldToggleReq false "Target state; omit to flip"
// @Success 200 {object} model.CalculatedField "Toggled calculated field"
// @Router /api/v1/calculated_fields/{id}/toggle [put]
func (*CalculatedFieldApi) HandleToggleCalculatedField(c *gin.Context) {
	var req model.CalculatedFieldToggleReq
	// toggle 允许空 body（取反语义），绑定失败不视为参数错误。
	_ = bindRequest(c, &req)
	id := c.Param("id")

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CalculatedField.ToggleCalculatedField(id, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleDeleteCalculatedField 删除计算字段。
// @Summary Delete a calculated field
// @Tags CalculatedField
// @Produce json
// @Param id path string true "Calculated field id"
// @Success 200 {object} nil "Deleted"
// @Router /api/v1/calculated_fields/{id} [delete]
func (*CalculatedFieldApi) HandleDeleteCalculatedField(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.CalculatedField.DeleteCalculatedField(c.Param("id"), userClaims); err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// HandleGetCalculatedField 按 id 查询单条计算字段。
// @Summary Get a calculated field by id
// @Tags CalculatedField
// @Produce json
// @Param id path string true "Calculated field id"
// @Success 200 {object} model.CalculatedField "Calculated field detail"
// @Router /api/v1/calculated_fields/{id} [get]
func (*CalculatedFieldApi) HandleGetCalculatedField(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CalculatedField.GetCalculatedField(c.Param("id"), userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}
