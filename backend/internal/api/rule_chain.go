// 文件用途：规则链域的 HTTP 入口（ROADMAP B2）。
// 边界说明：租户守卫与 DAG 校验在 service 层；本层只做绑定、claims 提取和错误出口。
package api

import (
	"io"
	"strconv"
	"strings"

	model "aetherlink-iot/backend/internal/model"
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

// HandleGetRuleChainNodeTraces 节点最近调试 trace（PHASE-D-D1）。
// GET /api/v1/rule-chains/:id/nodes/:nodeId/traces?limit=10
func (*RuleChainApi) HandleGetRuleChainNodeTraces(c *gin.Context) {
	chainID := c.Param("id")
	nodeID := c.Param("nodeId")
	if chainID == "" || nodeID == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "id and nodeId are required"))
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil {
		limit = 0
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	traces, svcErr := service.GroupApp.RuleChain.GetNodeTraces(chainID, nodeID, limit, userClaims)
	if svcErr != nil {
		c.Error(svcErr)
		return
	}
	c.Set("data", traces)
}

// HandleListRuleChainDeadLetters 查询规则链死信列表（P1.2 护城河）。
// GET /api/v1/rule-chains/:id/dead-letters or GET /api/v1/rule-chains/dead-letters
func (*RuleChainApi) HandleListRuleChainDeadLetters(c *gin.Context) {
	chainID := c.Param("id")
	if chainID == "" {
		chainID = c.Query("chain_id")
	}
	execID := c.Query("exec_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	res, err := service.GroupApp.RuleChain.ListRuleChainDeadLetters(chainID, execID, page, pageSize, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", res)
}

// HandleGetRuleChainExecutionTraces 查询单次执行批次全节点串联 trace 链路（P1.2 护城河）。
// GET /api/v1/rule-chains/:id/executions/:execId/traces or GET /api/v1/rule-chains/executions/:execId/traces
func (*RuleChainApi) HandleGetRuleChainExecutionTraces(c *gin.Context) {
	chainID := c.Param("id")
	execID := c.Param("execId")
	if execID == "" {
		execID = c.Query("exec_id")
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	traces, err := service.GroupApp.RuleChain.GetExecutionTraces(chainID, execID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", traces)
}

// HandleListRuleChainReplayRecords 查询某次执行的回放输入快照记录（P1.2 护城河）。
// GET /api/v1/rule-chains/:id/executions/:execId/replay-records
func (*RuleChainApi) HandleListRuleChainReplayRecords(c *gin.Context) {
	chainID := c.Param("id")
	execID := c.Param("execId")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	records, err := service.GroupApp.RuleChain.GetReplayRecords(c.Request.Context(), chainID, execID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", records)
}

// HandleReplayRuleChainExecution 触发单次执行输入回放，严格施加副作用确认闸门（P1.2 护城河）。
// POST /api/v1/rule-chains/:id/replay
func (*RuleChainApi) HandleReplayRuleChainExecution(c *gin.Context) {
	chainID := c.Param("id")
	if strings.TrimSpace(chainID) == "" {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "chain id is required"))
		return
	}
	var req model.RuleChainReplayReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	res, err := service.GroupApp.RuleChain.ReplayExecution(c.Request.Context(), chainID, req.ExecutionID, req.ConfirmSideEffects, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", res)
}

