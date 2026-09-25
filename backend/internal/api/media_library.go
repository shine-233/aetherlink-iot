// 文件用途：媒体库（media_files，ROADMAP TB-41）HTTP 处理器。
// 核心逻辑：入参绑定校验、鉴权边界注入（claims）、Service 分发与统一 API 响应包装；
//
//	列表/详情/删除三入口，删除被引用拒绝时引用方列表随错误 Data 返回。
//
// 关键注意事项：请求上下文一律透传 c.Request.Context()，不引入 context.Background()
//
//	（internal/api 的 context.Background() 有数量预算守卫）。
//	id 一律取路径参数（c.Param("id")），不信任请求体标识；租户作用域由 claims 注入 service 层。
//
// 重构建议：后续若加批量删除或引用明细页，新增独立 handler，不在删除语义里加覆盖开关。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// MediaLibraryApi 媒体库控制器。
type MediaLibraryApi struct{}

// ListMediaFiles 分页查询本租户媒体登记。
// @Summary List media files by page
// @Tags MediaLibrary
// @Produce json
// @Param request query model.GetMediaFileListReq true "Pagination and search"
// @Success 200 {object} model.GetMediaFileListRsp "Media file list"
// @Router /api/v1/media/files [get]
func (*MediaLibraryApi) ListMediaFiles(c *gin.Context) {
	var req model.GetMediaFileListReq
	if !BindAndValidate(c, &req) {
		return
	}
	claimsValue, _ := c.Get("claims")
	claims, _ := claimsValue.(*utils.UserClaims)
	data, err := service.GroupApp.MediaLibrary.ListMediaFiles(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetMediaFileDetail 查询单个媒体详情（含实时引用统计）。
// @Summary Get a media file
// @Tags MediaLibrary
// @Produce json
// @Param id path string true "Media file id"
// @Success 200 {object} model.MediaFileDetailRsp "Media file detail"
// @Router /api/v1/media/files/{id} [get]
func (*MediaLibraryApi) GetMediaFileDetail(c *gin.Context) {
	claimsValue, _ := c.Get("claims")
	claims, _ := claimsValue.(*utils.UserClaims)
	data, err := service.GroupApp.MediaLibrary.GetMediaFileDetail(c.Request.Context(), c.Param("id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// DeleteMediaFile 删除媒体：引用计数>0 时拒绝并返回引用方，否则删文件+删登记行。
// @Summary Delete a media file
// @Tags MediaLibrary
// @Produce json
// @Param id path string true "Media file id"
// @Success 200 {object} model.MediaFileDeleteRsp "Delete result"
// @Router /api/v1/media/files/{id} [delete]
func (*MediaLibraryApi) DeleteMediaFile(c *gin.Context) {
	claimsValue, _ := c.Get("claims")
	claims, _ := claimsValue.(*utils.UserClaims)
	data, err := service.GroupApp.MediaLibrary.DeleteMediaFile(c.Request.Context(), c.Param("id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
