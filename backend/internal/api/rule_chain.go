// 文件用途：规则链域的 HTTP 入口（ROADMAP B2）。
// 边界说明：租户守卫与 DAG 校验在 service 层；本层只做绑定、claims 提取和错误出口。
package api

import (
	"io"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type RuleChainApi struct{}

const ruleChainBodyLimit = 512 * 1024

func readRuleChainBody(c *gin.Context) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, ruleChainBodyLimit))
	if err != nil || len(raw) == 0 {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "request body is required"))
		return nil, false
	}
	return raw, true
}

// HandleCreateRuleChain 新建规则链。
// POST /api/v1/rule-chains
func (*RuleChainApi) HandleCreateRuleChain(c *gin.Context) {
	raw, ok := readRuleChainBody(c)
	if !ok {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	chain, err := service.GroupApp.RuleChain.CreateChain(raw, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", chain)
}

// HandleUpdateRuleChain 更新规则链。
// PUT /api/v1/rule-chains
func (*RuleChainApi) HandleUpdateRuleChain(c *gin.Context) {
	raw, ok := readRuleChainBody(c)
	if !ok {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	chain, err := service.GroupApp.RuleChain.UpdateChain(raw, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", chain)
}

// HandleDeleteRuleChain 删除规则链。
// DELETE /api/v1/rule-chains/:id
func (*RuleChainApi) HandleDeleteRuleChain(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.RuleChain.DeleteChain(id, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", map[string]interface{}{})
}

// HandleGetRuleChain 规则链详情。
// GET /api/v1/rule-chains/:id
func (*RuleChainApi) HandleGetRuleChain(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id is required"))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	chain, err := service.GroupApp.RuleChain.GetChain(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", chain)
}

// HandleListRuleChains 分页列表。
// GET /api/v1/rule-chains/list?keyword=&page=1&page_size=20
func (*RuleChainApi) HandleListRuleChains(c *gin.Context) {
	type listReq struct {
		Keyword  string `form:"keyword"`
		Page     int    `form:"page"`
		PageSize int    `form:"page_size" binding:"omitempty,max=200"`
	}
	var req listReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, err.Error()))
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	resp, err := service.GroupApp.RuleChain.ListChains(req.Keyword, req.Page, req.PageSize, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", resp)
}
