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
	// 迁移形态：Handle（绑定 -> 校验 -> claims -> service）。conflict_policy 的 query 兜底
	// 保留在闭包内、service 调用之前，执行顺序与迁移前一致。
	Handle(c, func(req *service.AssetReq, userClaims *utils.UserClaims) (interface{}, error) {
		if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
			if q := c.Query("conflict_policy"); q != "" {
				req.ConflictPolicy = &q
			}
		}
		return service.GroupApp.Asset.Create(userClaims, req)
	})
}

// HandleAssetUpdate 更新资产。
// PUT /api/v1/asset
func (*AssetApi) HandleAssetUpdate(c *gin.Context) {
	Handle(c, func(req *service.AssetReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Asset.Update(userClaims, req)
	})
}

// HandleAssetDelete 删除资产（无子节点）。
// DELETE /api/v1/asset/:id
func (*AssetApi) HandleAssetDelete(c *gin.Context) {
	// 迁移形态：HandlePath（路径参数 id + claims）。成功包络保留旧的空对象 data {}，
	// 故不能改用 HandlePathAction（其成功时 data 为 nil 并从包络中省略该字段）。
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "asset id is required")
		}
		if err := service.GroupApp.Asset.Delete(userClaims, id); err != nil {
			return nil, err
		}
		return map[string]interface{}{}, nil
	})
}

// HandleAssetList 分页列出根/指定父节点下的资产。
// GET /api/v1/asset/list?parent_id=&keyword=&page=&page_size=
func (*AssetApi) HandleAssetList(c *gin.Context) {
	// 迁移形态：HandleNoBody（不绑定请求体，分页参数仍在闭包内按 query/DefaultQuery 读取）。
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		parentID := c.Query("parent_id")
		keyword := c.Query("keyword")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
		list, total, err := service.GroupApp.Asset.List(userClaims, parentID, keyword, page, pageSize)
		if err != nil {
			return nil, err
		}
		return gin.H{"list": list, "total": total}, nil
	})
}

// HandleAssetGet 读取单个资产。
// GET /api/v1/asset/:id
func (*AssetApi) HandleAssetGet(c *gin.Context) {
	// 迁移形态：HandlePath（路径参数 id + claims）。id 空值校验保留在闭包内、先于 service 调用。
	HandlePath(c, "id", func(id string, userClaims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "asset id is required")
		}
		return service.GroupApp.Asset.Get(userClaims, id)
	})
}

// HandleAssetTree 返回租户作用域内资产树。
// GET /api/v1/asset/tree
func (*AssetApi) HandleAssetTree(c *gin.Context) {
	HandleNoBody(c, func(userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Asset.Tree(userClaims)
	})
}
