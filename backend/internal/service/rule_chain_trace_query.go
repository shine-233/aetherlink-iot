package service

import (
	"context"
	"strconv"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// PHASE-D-D1 BEGIN 节点 trace 查询（画布"最近调试记录"面板数据源）

// ruleChainTraceDefaultLimit / ruleChainTraceMaxLimit 单次拉取条数边界。
const (
	ruleChainTraceDefaultLimit = 10
	ruleChainTraceMaxLimit     = 50
)

// GetNodeTraces 返回指定链节点最近 N 条执行 trace。
// 租户守卫：链不存在或不属该租户一律按 not found 处理，杜绝跨租户探测。
func (*RuleChain) GetNodeTraces(chainID, nodeID string, limit int, claims *utils.UserClaims) ([]model.RuleChainNodeTrace, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	chain, err := dal.GetRuleChainByID(chainID, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if chain == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	}
	if strings.TrimSpace(nodeID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "nodeId is required")
	}
	if limit <= 0 || limit > ruleChainTraceMaxLimit {
		limit = ruleChainTraceDefaultLimit
	}
	if global.DB == nil {
		return nil, errRuleChainDBNotInitialized
	}
	// 容量提示用固定常量：请求侧 limit 已做业务钳制，但分配尺寸不信任外部值（CodeQL allocation-size）。
	traces := make([]model.RuleChainNodeTrace, 0, ruleChainTraceDefaultLimit)
	if err := global.DB.WithContext(context.Background()).
		Table(model.TableNameRuleChainNodeTrace).
		Where("chain_id = ? AND node_id = ? AND tenant_id = ?", chainID, nodeID, tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&traces).Error; err != nil {
		logrus.Error("查询节点 trace 失败:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return traces, nil
}

// GetExecutionTraces 返回指定单次执行批次（execID）全部节点的 trace，按时间正序排列（串联出一条完整流转链路）。
func (*RuleChain) GetExecutionTraces(chainID, execID string, claims *utils.UserClaims) ([]model.RuleChainNodeTrace, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(execID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "execId is required")
	}
	if strings.TrimSpace(chainID) != "" {
		chain, err := dal.GetRuleChainByID(chainID, tenantID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
		}
		if chain == nil {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
		}
	}
	if global.DB == nil {
		return nil, errRuleChainDBNotInitialized
	}
	traces := make([]model.RuleChainNodeTrace, 0)
	query := global.DB.WithContext(context.Background()).
		Table(model.TableNameRuleChainNodeTrace).
		Where("tenant_id = ? AND exec_id = ?", tenantID, strings.TrimSpace(execID))
	if strings.TrimSpace(chainID) != "" {
		query = query.Where("chain_id = ?", strings.TrimSpace(chainID))
	}
	if err := query.Order("created_at ASC").Find(&traces).Error; err != nil {
		logrus.Error("查询执行批次 trace 失败:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return traces, nil
}

// normalizeTraceLimitString 查询参数解析容错。
func normalizeTraceLimitString(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return ruleChainTraceDefaultLimit
	}
	return limit
}

// PHASE-D-D1 END
