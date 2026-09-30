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
	Handle(c, func(req *model.CreateIndustrySolutionReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.IndustrySolution.CreateIndustrySolution(c.Request.Context(), req, claims)
	})
}

// ListIndustrySolutions 方案分页列表。
// @Summary  行业方案列表
// @Tags     IndustrySolution
// @Router   /api/v1/solutions [get]
func (*IndustrySolutionApi) ListIndustrySolutions(c *gin.Context) {
	HandleNoBody(c, func(claims *utils.UserClaims) (interface{}, error) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		return service.GroupApp.IndustrySolution.ListIndustrySolutions(c.Request.Context(), page, pageSize, claims)
	})
}

// GetIndustrySolution 方案详情（含安装流水）。
// @Summary  行业方案详情
// @Tags     IndustrySolution
// @Router   /api/v1/solutions/{id} [get]
func (*IndustrySolutionApi) GetIndustrySolution(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.IndustrySolution.GetIndustrySolution(c.Request.Context(), id, claims)
	})
}

// DeleteIndustrySolution 删除方案。
// @Summary  删除行业方案
// @Tags     IndustrySolution
// @Router   /api/v1/solutions/{id} [delete]
func (*IndustrySolutionApi) DeleteIndustrySolution(c *gin.Context) {
	// 迁移形态：HandlePath（路径参数 id + claims）。成功包络保留旧的 data 对象 {"deleted": true}，
	// 因此不能改用 HandlePathAction（其成功时 data 为 nil 并从包络中省略该字段）。
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if err := service.GroupApp.IndustrySolution.DeleteIndustrySolution(c.Request.Context(), id, claims); err != nil {
			return nil, err
		}
		return gin.H{"deleted": true}, nil
	})
}

// InstallIndustrySolution 一键安装方案（逐项应用并留流水）。
// @Summary  一键安装行业方案
// @Tags     IndustrySolution
// @Router   /api/v1/solutions/{id}/install [post]
func (*IndustrySolutionApi) InstallIndustrySolution(c *gin.Context) {
	// 迁移形态：只复用 RequireClaims / respond 出口。body 可选（空 body = 全部缺省语义），
	// 绑定失败不拒绝，因此不能改用 HandlePathBody（其绑定失败即返回参数错误）。
	id := c.Param("id")
	var req model.InstallIndustrySolutionReq
	_ = c.ShouldBindJSON(&req)
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	resp, err := service.GroupApp.IndustrySolution.InstallIndustrySolution(c.Request.Context(), id, &req, claims)
	respond(c, resp, err)
}
