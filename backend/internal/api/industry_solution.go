// File purpose: TB-19 解决方案模板引擎 HTTP 层——创建/列表/详情/删除/一键安装。
// Core logic: 绑定校验 + claims 提取 + 委派 service；租户上下文只来自 claims。
package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type IndustrySolutionApi struct{}

// CreateIndustrySolution 创建行业方案。
// @Summary  创建行业方案模板
// @Tags     IndustrySolution
// @Router   /api/v1/solutions [post]
func (*IndustrySolutionApi) CreateIndustrySolution(c *gin.Context) {
	var req model.CreateIndustrySolutionReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.IndustrySolution.CreateIndustrySolution(c.Request.Context(), &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// ListIndustrySolutions 方案分页列表。
// @Summary  行业方案列表
// @Tags     IndustrySolution
// @Router   /api/v1/solutions [get]
func (*IndustrySolutionApi) ListIndustrySolutions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.IndustrySolution.ListIndustrySolutions(c.Request.Context(), page, pageSize, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// GetIndustrySolution 方案详情（含安装流水）。
// @Summary  行业方案详情
// @Tags     IndustrySolution
// @Router   /api/v1/solutions/{id} [get]
func (*IndustrySolutionApi) GetIndustrySolution(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.IndustrySolution.GetIndustrySolution(c.Request.Context(), id, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}

// DeleteIndustrySolution 删除方案。
// @Summary  删除行业方案
// @Tags     IndustrySolution
// @Router   /api/v1/solutions/{id} [delete]
func (*IndustrySolutionApi) DeleteIndustrySolution(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.IndustrySolution.DeleteIndustrySolution(c.Request.Context(), id, claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"deleted": true})
}

// InstallIndustrySolution 一键安装方案（逐项应用并留流水）。
// @Summary  一键安装行业方案
// @Tags     IndustrySolution
// @Router   /api/v1/solutions/{id}/install [post]
func (*IndustrySolutionApi) InstallIndustrySolution(c *gin.Context) {
	id := c.Param("id")
	var req model.InstallIndustrySolutionReq
	// body 可选：空 body = 全部缺省语义。
	_ = c.ShouldBindJSON(&req)
	claims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.IndustrySolution.InstallIndustrySolution(c.Request.Context(), id, &req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
