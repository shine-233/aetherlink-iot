// 文件用途：规则链域的 HTTP 入口（ROADMAP B2）。
// 边界说明：租户守卫与 DAG 校验在 service 层；本层只做绑定、claims 提取和错误出口。
// 迁移说明：适配器骨架收敛在 handler_adapter.go；本文件保留手写的部分只有
// 「多路径参数 + DefaultQuery 容错解析」（NodeTraces/DeadLetters/ExecutionTraces）、
// 「原始 body 读取」（Create/Update 的 512KB LimitReader）和 Replay 的
// 「路径检查先于 BindAndValidate」顺序——这些都是泛型适配器表达不了或顺序敏感的形态。
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
// 不走 Handle 泛型适配器：service 入参是原始 body 字节（带 512KB 上限），不是可绑定结构体。
func (*RuleChainApi) HandleCreateRuleChain(c *gin.Context) {
	raw, ok := readRuleChainBody(c)
	if !ok {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	chain, err := service.GroupApp.RuleChain.CreateChain(raw, claims)
	respond(c, chain, err)
}

// HandleUpdateRuleChain 更新规则链。
// PUT /api/v1/rule-chains
// 不走 Handle 泛型适配器：service 入参是原始 body 字节（带 512KB 上限），不是可绑定结构体。
func (*RuleChainApi) HandleUpdateRuleChain(c *gin.Context) {
	raw, ok := readRuleChainBody(c)
	if !ok {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	chain, err := service.GroupApp.RuleChain.UpdateChain(raw, claims)
	respond(c, chain, err)
}

// HandleDeleteRuleChain 删除规则链。
// DELETE /api/v1/rule-chains/:id
// 注意：成功包络的 data 是空对象（保持旧实现 c.Set("data", map[string]interface{}{}) 的字节输出，
// data 为非 nil 空 map 会渲染成 "data":{}），因此不能改用 HandlePathAction（其成功包络省略 data 字段）。
func (*RuleChainApi) HandleDeleteRuleChain(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "id is required")
		}
		if err := service.GroupApp.RuleChain.DeleteChain(id, claims); err != nil {
			return nil, err
		}
		return map[string]interface{}{}, nil
	})
}

// HandleGetRuleChain 规则链详情。
// GET /api/v1/rule-chains/:id
func (*RuleChainApi) HandleGetRuleChain(c *gin.Context) {
	HandlePath(c, "id", func(id string, claims *utils.UserClaims) (interface{}, error) {
		if id == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "id is required")
		}
		return service.GroupApp.RuleChain.GetChain(id, claims)
	})
}

// HandleListRuleChains 分页列表。
// GET /api/v1/rule-chains/list?keyword=&page=1&page_size=20
// HandleQuery 内部即 bindQueryAndValidate（ShouldBindQuery + ValidateStructLang）；
// listReq 只有 binding 标签没有 validate 标签，追加的 ValidateStructLang 恒通过，
// 绑定失败时的错误消息与旧实现（直接包 CodeParamError）逐字节一致。
func (*RuleChainApi) HandleListRuleChains(c *gin.Context) {
	type listReq struct {
		Keyword  string `form:"keyword"`
		Page     int    `form:"page"`
		PageSize int    `form:"page_size" binding:"omitempty,max=200"`
	}
	HandleQuery(c, func(req *listReq, claims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.RuleChain.ListChains(req.Keyword, req.Page, req.PageSize, claims)
	})
}

// HandleGetRuleChainNodeTraces 节点最近调试 trace（PHASE-D-D1）。
// GET /api/v1/rule-chains/:id/nodes/:nodeId/traces?limit=10
// 保持手写：双路径参数 + DefaultQuery 的 limit 解析容错（解析失败置 0 交由 service 钳制），
// 泛型适配器没有双路径参数形态，且必须保留「路径检查先于 claims」的旧顺序。
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
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	traces, svcErr := service.GroupApp.RuleChain.GetNodeTraces(chainID, nodeID, limit, claims)
	respond(c, traces, svcErr)
}

// HandleListRuleChainDeadLetters 查询规则链死信列表（P1.2 护城河）。
// GET /api/v1/rule-chains/:id/dead-letters or GET /api/v1/rule-chains/dead-letters
// 保持手写：param 缺省回退 query、page/page_size 走 Atoi 忽略错误的容错语义（缺失→默认、非法→0），
// 若改用 HandlePathQuery 的结构体绑定，缺失参数会绑成 0 而非 1/20，包络之外的分页行为会被改变。
func (*RuleChainApi) HandleListRuleChainDeadLetters(c *gin.Context) {
	chainID := c.Param("id")
	if chainID == "" {
		chainID = c.Query("chain_id")
	}
	execID := c.Query("exec_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := service.GroupApp.RuleChain.ListRuleChainDeadLetters(chainID, execID, page, pageSize, claims)
	respond(c, res, err)
}

// HandleGetRuleChainExecutionTraces 查询单次执行批次全节点串联 trace 链路（P1.2 护城河）。
// GET /api/v1/rule-chains/:id/executions/:execId/traces or GET /api/v1/rule-chains/executions/:execId/traces
// 保持手写：execId 存在 param 缺省回退 query 的兼容路径，双路径参数无适配器形态。
func (*RuleChainApi) HandleGetRuleChainExecutionTraces(c *gin.Context) {
	chainID := c.Param("id")
	execID := c.Param("execId")
	if execID == "" {
		execID = c.Query("exec_id")
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	traces, err := service.GroupApp.RuleChain.GetExecutionTraces(chainID, execID, claims)
	respond(c, traces, err)
}

// HandleListRuleChainReplayRecords 查询某次执行的回放输入快照记录（P1.2 护城河）。
// GET /api/v1/rule-chains/:id/executions/:execId/replay-records
func (*RuleChainApi) HandleListRuleChainReplayRecords(c *gin.Context) {
	HandlePath(c, "id", func(chainID string, userClaims *utils.UserClaims) (interface{}, error) {
		execID := c.Param("execId")
		return service.GroupApp.RuleChain.GetReplayRecords(c.Request.Context(), chainID, execID, userClaims)
	})
}

// HandleReplayRuleChainExecution 触发单次执行输入回放，严格施加副作用确认闸门（P1.2 护城河）。
// POST /api/v1/rule-chains/:id/replay
// 不用 HandlePathBody：它先绑定再取 claims，而旧实现（保持不变）是「路径检查 → BindAndValidate → claims」，
// 顺序敏感——chainID 缺失且 body 非法时必须优先报 chain id 错误，换形态会改变错误包络。
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
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := service.GroupApp.RuleChain.ReplayExecution(c.Request.Context(), chainID, req.ExecutionID, req.ConfirmSideEffects, claims)
	respond(c, res, err)
}

// HandleResolveDeviceEffectiveRuleChains 解析单设备生效规则链（TB-18，125.sql）：
// 档案绑定链优先、租户级启用链兜底。设备归属/租户隔离校验在 service 层完成。
// GET /api/v1/rule-chains/device-effective/:deviceId
func (*RuleChainApi) HandleResolveDeviceEffectiveRuleChains(c *gin.Context) {
	HandlePath(c, "deviceId", func(deviceID string, claims *utils.UserClaims) (interface{}, error) {
		if strings.TrimSpace(deviceID) == "" {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "deviceId is required")
		}
		return service.GroupApp.RuleChain.ResolveEffectiveRuleChainsForDevice(deviceID, claims)
	})
}

