// 文件用途：资源中心（TP-5）HTTP API 处理器。
// 核心逻辑：
// 1. 行业分类目录检索与跨类型分页查询；
// 2. 统一资源包导出与签名打包；
// 3. 统一资源包验签导入、冲突预览与显式覆盖确认；
// 4. 一键安装应用资源模板。
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ResourceCenterApi struct{}

// HandleResourceCenterCatalog 获取资源中心全貌目录。
// @Router /api/v1/resource/center/catalog [get]
func (*ResourceCenterApi) HandleResourceCenterCatalog(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	catalog, err := service.GroupApp.ResourceCenter.ResourceCenterCatalog(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", catalog)
}

// HandleResourceCenterList 跨形态综合分页检索。
// @Router /api/v1/resource/center/list [get]
func (*ResourceCenterApi) HandleResourceCenterList(c *gin.Context) {
	var req model.ResourceCenterListReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rsp, err := service.GroupApp.ResourceCenter.ResourceCenterList(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rsp)
}

// HandleExportResourceBundle 统一打包导出。
// @Router /api/v1/resource/center/bundle [get]
func (*ResourceCenterApi) HandleExportResourceBundle(c *gin.Context) {
	typeKey := c.Query("type_key")
	resourceType := c.Query("resource_type")
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	bundle, err := service.GroupApp.ResourceCenter.ExportResourceBundle(typeKey, resourceType, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		c.Error(err)
		return
	}

	slug := typeKey
	if slug == "" {
		slug = "all"
	}
	fileName := fmt.Sprintf("resource_bundle_%s_%d.json", slug, time.Now().Unix())

	c.Set("data", map[string]interface{}{
		"file_name":      fileName,
		"content_base64": base64.StdEncoding.EncodeToString(data),
		"count":          bundle.Count,
		"bundle":         bundle,
	})
}

// HandleImportResourceBundle 统一资源包导入/冲突预览。
// @Router /api/v1/resource/center/bundle/import [post]
func (*ResourceCenterApi) HandleImportResourceBundle(c *gin.Context) {
	var req model.ImportMarketBundleReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rsp, err := service.GroupApp.ResourceCenter.ImportResourceBundle(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rsp)
}

// HandleApplyResource 一键应用资源到当前租户。
// @Router /api/v1/resource/center/apply [post]
func (*ResourceCenterApi) HandleApplyResource(c *gin.Context) {
	var req model.ResourceCenterApplyReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	rsp, err := service.GroupApp.ResourceCenter.ApplyResource(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", rsp)
}
