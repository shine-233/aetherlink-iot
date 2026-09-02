// 文件用途：资产域 HTTP 入口（ROADMAP C2）。
// 边界说明：租户作用域（self∪祖先）与成环校验在 service 层；本层只做绑定/claims/错误出口。
package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type AssetApi struct{}

// HandleAssetCreate 新建资产。
// POST /api/v1/asset
func (*AssetApi) HandleAssetCreate(c *gin.Context) {
	var req service.AssetReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Asset.Create(userClaims, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleAssetUpdate 更新资产。
// PUT /api/v1/asset
func (*AssetApi) HandleAssetUpdate(c *gin.Context) {
	var req service.AssetReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Asset.Update(userClaims, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleAssetDelete 删除资产（无子节点）。
// DELETE /api/v1/asset/:id
func (*AssetApi) HandleAssetDelete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "asset id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.Asset.Delete(userClaims, id); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// HandleAssetList 分页列出根/指定父节点下的资产。
// GET /api/v1/asset/list?parent_id=&keyword=&page=&page_size=
func (*AssetApi) HandleAssetList(c *gin.Context) {
	parentID := c.Query("parent_id")
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	list, total, err := service.GroupApp.Asset.List(userClaims, parentID, keyword, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"list": list, "total": total})
}

// HandleAssetGet 读取单个资产。
// GET /api/v1/asset/:id
func (*AssetApi) HandleAssetGet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "asset id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Asset.Get(userClaims, id)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// HandleAssetTree 返回租户作用域内资产树。
// GET /api/v1/asset/tree
func (*AssetApi) HandleAssetTree(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.Asset.Tree(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
