// 文件用途：实体版本控制 HTTP Handler（ROADMAP C7），承接快照创建、版本历史、详情与恢复。
// 核心链路：Handler 绑定请求、注入 claims，再把业务下沉给 EntityVersionService；
// 详情与恢复的 id 一律取自路径参数（c.Param("id")），避免信任请求体中的标识。
// 关键注意事项：本 Handler 不做权限判断与租户解析，统一由 claims 与 service 层负责；
// 恢复支持 dry_run，只回显将写入的字段而不落库，供前端二次确认。
// 重构建议：若后续支持批量快照或版本对比，新增独立端点而不是复用 create 语义。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// EntityVersionApi 实体版本控制器。
type EntityVersionApi struct{}

// HandleGetEntityVersionList 分页查询某实体的版本历史。
// @Summary List entity versions by page
// @Tags EntityVersion
// @Produce json
// @Param request query model.EntityVersionListReq true "Pagination and entity locator"
// @Success 200 {object} model.EntityVersionListRsp "Entity version list"
// @Router /api/v1/entity_versions [get]
func (*EntityVersionApi) HandleGetEntityVersionList(c *gin.Context) {
	var req model.EntityVersionListReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.EntityVersion.ListEntityVersions(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleCreateEntityVersion 读取实体当前状态并创建一条快照版本。
// @Summary Create an entity snapshot version
// @Tags EntityVersion
// @Accept json
// @Produce json
// @Param request body model.EntityVersionCreateReq true "Entity version create request"
// @Success 200 {object} model.EntityVersion "Created entity version"
// @Router /api/v1/entity_versions [post]
func (*EntityVersionApi) HandleCreateEntityVersion(c *gin.Context) {
	var req model.EntityVersionCreateReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.EntityVersion.CreateEntityVersion(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleGetEntityVersion 按路径 id 查询单个版本详情。
// @Summary Get an entity version
// @Tags EntityVersion
// @Produce json
// @Param id path string true "Entity version id"
// @Success 200 {object} model.EntityVersion "Entity version detail"
// @Router /api/v1/entity_versions/{id} [get]
func (*EntityVersionApi) HandleGetEntityVersion(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.EntityVersion.GetEntityVersion(c.Param("id"), userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleRestoreEntityVersion 按路径 id 将快照回写到实体；dry_run 为真时只回显字段。
// @Summary Restore an entity from a version snapshot
// @Tags EntityVersion
// @Accept json
// @Produce json
// @Param id path string true "Entity version id"
// @Param request body model.EntityVersionRestoreReq false "Restore options"
// @Success 200 {object} map[string]interface{} "Restored fields"
// @Router /api/v1/entity_versions/{id}/restore [post]
func (*EntityVersionApi) HandleRestoreEntityVersion(c *gin.Context) {
	var req model.EntityVersionRestoreReq
	// 恢复接口允许无请求体（默认真实恢复），因此绑定失败不直接返回。
	_ = c.ShouldBindJSON(&req)

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	fields, dryRun, err := service.GroupApp.EntityVersion.RestoreEntityVersion(c.Param("id"), &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", gin.H{
		"dry_run": dryRun,
		"fields":  fields,
	})
}
