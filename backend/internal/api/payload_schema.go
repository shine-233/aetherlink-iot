// 文件用途：提供 payload schema registry 的静态校验 HTTP Handler。
// 核心链路：Handler 绑定请求、注入 claims，再把校验决策下沉给 PayloadSchema service。
// 关键注意事项：该接口是无副作用的静态校验(dry-run 风格),不落库、不连 broker、不下发消息。
//
//	broker 侧对上行 payload 的真实拦截属于外部 MQTT 契约的破坏性变更,需运行时验证,不在本接口范围内。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type PayloadSchemaApi struct{}

// ValidatePayload 针对提交的字段约束和样本 payload 做静态校验,不保存、不连接 broker、不下发消息。
// @Summary Validate a sample payload against declared schema fields
// @Tags PayloadSchema
// @Accept json
// @Produce json
// @Param request body model.ValidatePayloadReq true "Payload schema validate request"
// @Success 200 {object} model.ValidatePayloadResult "Static validation result"
// @Router /api/v1/payload-schema/validate [post]
func (*PayloadSchemaApi) ValidatePayload(c *gin.Context) {
	var req model.ValidatePayloadReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.PayloadSchema.ValidatePayload(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// SavePayloadSchema 创建或更新一个持久化 payload schema（租户隔离）。
// @Summary Create or update a persisted payload schema
// @Tags PayloadSchema
// @Accept json
// @Produce json
// @Param request body model.SavePayloadSchemaReq true "Payload schema save request"
// @Success 200 {object} model.PayloadSchemaRsp "Saved payload schema"
// @Router /api/v1/payload-schema [post]
func (*PayloadSchemaApi) SavePayloadSchema(c *gin.Context) {
	var req model.SavePayloadSchemaReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.PayloadSchema.SaveSchema(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdatePayloadSchema 按路径 id 更新一个持久化 payload schema。
// @Summary Update a persisted payload schema
// @Tags PayloadSchema
// @Accept json
// @Produce json
// @Param schema_id path string true "Payload schema id"
// @Param request body model.SavePayloadSchemaReq true "Payload schema save request"
// @Success 200 {object} model.PayloadSchemaRsp "Saved payload schema"
// @Router /api/v1/payload-schema/{schema_id} [put]
func (*PayloadSchemaApi) UpdatePayloadSchema(c *gin.Context) {
	var req model.SavePayloadSchemaReq
	if !BindAndValidate(c, &req) {
		return
	}
	req.ID = c.Param("schema_id")

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.PayloadSchema.SaveSchema(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// ListPayloadSchemas 返回当前租户的 payload schema 列表。
// @Summary List persisted payload schemas
// @Tags PayloadSchema
// @Produce json
// @Success 200 {object} model.PayloadSchemaListRsp "Payload schema list"
// @Router /api/v1/payload-schema [get]
func (*PayloadSchemaApi) ListPayloadSchemas(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.PayloadSchema.ListSchemas(userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// DeletePayloadSchema 按路径 id 删除一个 payload schema。
// @Summary Delete a persisted payload schema
// @Tags PayloadSchema
// @Produce json
// @Param schema_id path string true "Payload schema id"
// @Success 200 {object} nil "Deleted"
// @Router /api/v1/payload-schema/{schema_id} [delete]
func (*PayloadSchemaApi) DeletePayloadSchema(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.PayloadSchema.DeleteSchema(c.Param("schema_id"), userClaims); err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}
