// 文件用途：产品（Product）API 控制器。
// 核心逻辑：提供产品的增删改查、TB-15 冲突策略解析及下拉列表查询。
package api

import (
	"strings"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type ProductApi struct{}

// HandleCreateProduct 创建产品
// @Router   /api/v1/product [post]
func (*ProductApi) HandleCreateProduct(c *gin.Context) {
	// 迁移形态：Handle（绑定 -> 校验 -> claims -> service）。conflict_policy 的 query 兜底
	// 保留在闭包内、service 调用之前，执行顺序与迁移前一致。
	Handle(c, func(req *model.CreateProductReq, userClaims *utils.UserClaims) (interface{}, error) {
		// TB-15: Query 参数兜底（当 Body 中未指定 conflict_policy 时）
		if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
			if qPolicy := strings.TrimSpace(c.Query("conflict_policy")); qPolicy != "" {
				req.ConflictPolicy = &qPolicy
			}
		}
		return service.GroupApp.Product.CreateProduct(req, userClaims)
	})
}

// HandleUpdateProduct 修改产品
// @Router   /api/v1/product [put]
func (*ProductApi) HandleUpdateProduct(c *gin.Context) {
	Handle(c, func(req *model.UpdateProductReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.Product.UpdateProduct(req, userClaims)
	})
}

// HandleDeleteProduct 删除产品
// @Router   /api/v1/product/:id [delete]
func (*ProductApi) HandleDeleteProduct(c *gin.Context) {
	// 迁移形态：HandlePathAction（路径参数 id + claims，成功时 data 为 nil 即从包络省略）。
	// id 的 TrimSpace 与空值校验保留在闭包内，先于 service 调用。
	HandlePathAction(c, "id", func(rawID string, userClaims *utils.UserClaims) error {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return errcode.New(errcode.CodeParamError)
		}
		return service.GroupApp.Product.DeleteProduct(id, userClaims)
	})
}

// HandleGetProductByID 查询产品详情
// @Router   /api/v1/product/:id [get]
func (*ProductApi) HandleGetProductByID(c *gin.Context) {
	// 迁移形态：HandlePath（路径参数 id + claims）。id 的 TrimSpace 与空值校验保留在闭包内。
	HandlePath(c, "id", func(rawID string, userClaims *utils.UserClaims) (interface{}, error) {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, errcode.New(errcode.CodeParamError)
		}
		return service.GroupApp.Product.GetProductByID(id, userClaims)
	})
}

// HandleProductSelectListByPage 分页查询当前租户可选产品
// @Router   /api/v1/product [get]
func (*ProductApi) HandleProductSelectListByPage(c *gin.Context) {
	// 迁移形态：只复用 RequireClaims / BindAndValidate / respond 出口。
	// 本入口按 query 是否存在在两个请求结构体间二选一，且 claims 读取必须先于绑定，
	// 泛型适配器表达不了分支绑定，故保持手工编排（顺序与迁移前逐字一致）。
	userClaims, ok := RequireClaims(c)
	if !ok {
		return
	}
	if c.Query("product_model") != "" || c.Query("product_type") != "" {
		var req model.GetProductListByPageReq
		if !BindAndValidate(c, &req) {
			return
		}
		data, err := service.GroupApp.Product.GetProductList(&req, userClaims)
		respond(c, data, err)
		return
	}

	var req model.GetProductSelectListReq
	if !BindAndValidate(c, &req) {
		return
	}
	data, err := service.GroupApp.Device.GetProductSelectListByPage(&req, userClaims)
	respond(c, data, err)
}
