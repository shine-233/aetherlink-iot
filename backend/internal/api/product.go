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
	var req model.CreateProductReq
	if !BindAndValidate(c, &req) {
		return
	}
	// TB-15: Query 参数兜底（当 Body 中未指定 conflict_policy 时）
	if req.ConflictPolicy == nil || *req.ConflictPolicy == "" {
		if qPolicy := strings.TrimSpace(c.Query("conflict_policy")); qPolicy != "" {
			req.ConflictPolicy = &qPolicy
		}
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Product.CreateProduct(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleUpdateProduct 修改产品
// @Router   /api/v1/product [put]
func (*ProductApi) HandleUpdateProduct(c *gin.Context) {
	var req model.UpdateProductReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Product.UpdateProduct(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleDeleteProduct 删除产品
// @Router   /api/v1/product/:id [delete]
func (*ProductApi) HandleDeleteProduct(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.Error(errcode.New(errcode.CodeParamError))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.Product.DeleteProduct(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// HandleGetProductByID 查询产品详情
// @Router   /api/v1/product/:id [get]
func (*ProductApi) HandleGetProductByID(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.Error(errcode.New(errcode.CodeParamError))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.Product.GetProductByID(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleProductSelectListByPage 分页查询当前租户可选产品
// @Router   /api/v1/product [get]
func (*ProductApi) HandleProductSelectListByPage(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if c.Query("product_model") != "" || c.Query("product_type") != "" {
		var req model.GetProductListByPageReq
		if !BindAndValidate(c, &req) {
			return
		}
		data, err := service.GroupApp.Product.GetProductList(&req, userClaims)
		if err != nil {
			c.Error(err)
			return
		}
		c.Set("data", data)
		return
	}

	var req model.GetProductSelectListReq
	if !BindAndValidate(c, &req) {
		return
	}
	data, err := service.GroupApp.Device.GetProductSelectListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}
