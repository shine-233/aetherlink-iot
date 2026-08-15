// open_api_keys.go 提供 OpenAPI 密钥管理相关的 HTTP 入口。
// 核心链路：
// 1. 绑定创建、列表、更新、删除请求，把查询条件和表单参数整理成 model DTO。
// 2. 从 gin 上下文提取 claims，将租户边界、操作者身份和权限判定统一下沉到 OpenAPIKey service。
// 3. 把结果返回给管理页，用于密钥列表展示、启停、备注维护和删除等运维动作。
// 静态审查建议：
// 1. OpenAPI 密钥属于高敏资产，后续若增加明文展示、导出或复制接口，必须先补审计与脱敏策略说明。
// 2. 当前四个 handler 都保持薄控制器形态，后续不要把密钥生成、签名策略或审计日志逻辑回灌到 API 层。
// 3. 删除与更新依赖 DTO 主键或路径参数，若管理页契约调整，要同步复核路由注释、前端弹窗和 service 入参。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type OpenAPIKeyApi struct{}

// CreateOpenAPIKey 创建 OpenAPI 密钥。
// API 层只做请求体绑定和 claims 注入，不负责密钥生成规则、权限范围拼装或持久化细节。
// @Router /api/v1/open/keys [post]
func (*OpenAPIKeyApi) CreateOpenAPIKey(c *gin.Context) {
	var createReq model.CreateOpenAPIKeyReq
	if !BindAndValidate(c, &createReq) {
		return
	}

	// claims 用于限定租户、操作者和可授权范围，避免前端自行声明越权字段。
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.OpenAPIKey.CreateOpenAPIKey(&createReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// GetOpenAPIKeyList 获取 OpenAPI 密钥列表。
// 常用于管理页回显密钥名称、状态、备注和创建时间等元数据，列表过滤与脱敏边界由 service 决定。
// @Router /api/v1/open/keys [get]
func (*OpenAPIKeyApi) GetOpenAPIKeyList(c *gin.Context) {
	var listReq model.OpenAPIKeyListReq
	if !BindAndValidate(c, &listReq) {
		return
	}

	// 查询也必须带 claims，防止跨租户读取其他空间的密钥元数据。
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	list, err := service.GroupApp.OpenAPIKey.GetOpenAPIKeyList(&listReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", list)
}

// UpdateOpenAPIKey 更新 OpenAPI 密钥。
// 常见场景包括启停、备注或授权范围调整；真正的字段级校验和安全约束依赖 service 落实。
// @Router /api/v1/open/keys [put]
func (*OpenAPIKeyApi) UpdateOpenAPIKey(c *gin.Context) {
	var updateReq model.UpdateOpenAPIKeyReq
	if !BindAndValidate(c, &updateReq) {
		return
	}

	// 更新属于高影响操作，claims 是 service 判断是否允许修改该密钥的基础上下文。
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.OpenAPIKey.UpdateOpenAPIKey(&updateReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// DeleteOpenAPIKey 删除 OpenAPI 密钥。
// API 层仅解析路径 ID 并透传 claims，删除后的失效、副作用和审计留痕都应在 service 收口。
// @Router /api/v1/open/keys/{id} [delete]
func (*OpenAPIKeyApi) DeleteOpenAPIKey(c *gin.Context) {
	id := c.Param("id")

	// 删除接口不从请求体取主键，路由参数与前端行操作契约必须长期保持一致。
	var userClaims = c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.OpenAPIKey.DeleteOpenAPIKey(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}
