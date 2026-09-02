// 文件用途：规则链服务层（ROADMAP B2）——CRUD、图校验与上行执行入口。
// 核心逻辑：CRUD 带租户守卫与 DAG 校验；执行入口按租户拉启用链（60s 缓存），
//   写操作失效缓存；OnTelemetry/OnDeviceOnline 供上行钩子以 goroutine 调用。
// 关键注意事项：空租户 fail-closed；执行错误只记录不阻断上行主流程；
//   单设备单次执行的节点扇出由 maxNodes 上限约束，防止放大。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"github.com/go-basic/uuid"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

const ruleChainCacheTTL = 60 * time.Second

// RuleChain 规则链服务入口。
type RuleChain struct{}

type createRuleChainReq struct {
	Name        string          `json:"name" validate:"required,max=128"`
	Description *string         `json:"description"`
	TenantID    string          `json:"tenant_id"` // 仅 SYS_ADMIN 可指定
	Graph       json.RawMessage `json:"graph" validate:"required"`
}

type updateRuleChainReq struct {
	ID          string          `json:"id" validate:"required"`
	Name        string          `json:"name" validate:"required,max=128"`
	Description *string         `json:"description"`
	Enabled     bool            `json:"enabled"`
	Graph       json.RawMessage `json:"graph" validate:"required"`
}

func normalizeRuleChainTenant(reqTenantID string, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.New(errcode.CodeNoPermission)
	}
	if claims.Authority == constant.SYS_ADMIN {
		if tenantID := strings.TrimSpace(reqTenantID); tenantID != "" {
			return tenantID, nil
		}
	}
	tenantID := strings.TrimSpace(claims.TenantID)
	if tenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant id in claims")
	}
	return tenantID, nil
}

// CreateChain 新建规则链。
func (*RuleChain) CreateChain(raw []byte, claims *utils.UserClaims) (*model.RuleChain, error) {
	var req createRuleChainReq
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "body is not valid json")
	}
	tenantID, err := normalizeRuleChainTenant(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	if _, err := ParseRuleChainGraph(string(req.Graph)); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid graph: "+err.Error())
	}
	now := time.Now().UTC()
	chain := &model.RuleChain{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Enabled:     true,
		Graph:       req.Graph,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := dal.CreateRuleChain(chain); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	invalidateRuleChainCache(tenantID)
	return chain, nil
}

// UpdateChain 更新规则链（名称/描述/启用/图）。
func (*RuleChain) UpdateChain(raw []byte, claims *utils.UserClaims) (*model.RuleChain, error) {
	var req updateRuleChainReq
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "body is not valid json")
	}
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	existing, err := dal.GetRuleChainByID(req.ID, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if existing == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	}
	if _, err := ParseRuleChainGraph(string(req.Graph)); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid graph: "+err.Error())
	}
	updated := &model.RuleChain{
		ID:          existing.ID,
		TenantID:    tenantID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Enabled:     req.Enabled,
		Graph:       req.Graph,
	}
	ok, err := dal.UpdateRuleChain(updated)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if !ok {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	}
	invalidateRuleChainCache(tenantID)
	return dal.GetRuleChainByID(req.ID, tenantID)
}

// DeleteChain 删除规则链。
func (*RuleChain) DeleteChain(id string, claims *utils.UserClaims) error {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "id is required")
	}
	ok, err := dal.DeleteRuleChain(id, tenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if !ok {
		return errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	}
	invalidateRuleChainCache(tenantID)
	return nil
}

// GetChain 读取规则链详情。
func (*RuleChain) GetChain(id string, claims *utils.UserClaims) (*model.RuleChain, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	chain, err := dal.GetRuleChainByID(id, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if chain == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	}
	return chain, nil
}

// ListChains 分页列表；keyword 按名称模糊。
func (*RuleChain) ListChains(keyword string, page, pageSize int, claims *utils.UserClaims) (map[string]interface{}, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	count, chains, err := dal.ListRuleChainsByTenant(tenantID, keyword, page, pageSize)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	return map[string]interface{}{
		"total": count,
		"list":  chains,
	}, nil
}

// ---- 执行入口与缓存 ----

type ruleChainCacheEntry struct {
	graphs    []*RuleChainGraph
	expiresAt time.Time
}

var (
	ruleChainCacheMu      sync.Mutex
	ruleChainCacheByTenant = map[string]ruleChainCacheEntry{}
)

func invalidateRuleChainCache(tenantID string) {
	ruleChainCacheMu.Lock()
	defer ruleChainCacheMu.Unlock()
	delete(ruleChainCacheByTenant, tenantID)
}

func enabledGraphsForTenant(tenantID string) []*RuleChainGraph {
	if strings.TrimSpace(tenantID) == "" {
		return nil
	}
	ruleChainCacheMu.Lock()
	entry, ok := ruleChainCacheByTenant[tenantID]
	ruleChainCacheMu.Unlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.graphs
	}
	graphsRaw, err := dal.ListEnabledRuleChainGraphs(tenantID)
	if err != nil {
		logrus.WithError(err).Warn("rule chain cache load failed")
		return nil
	}
	graphs := make([]*RuleChainGraph, 0, len(graphsRaw))
	for _, raw := range graphsRaw {
		graph, perr := ParseRuleChainGraph(raw)
		if perr != nil {
			logrus.WithError(perr).Warn("skip invalid rule chain graph")
			continue
		}
		graphs = append(graphs, graph)
	}
	ruleChainCacheMu.Lock()
	ruleChainCacheByTenant[tenantID] = ruleChainCacheEntry{graphs: graphs, expiresAt: time.Now().Add(ruleChainCacheTTL)}
	ruleChainCacheMu.Unlock()
	return graphs
}

// OnTelemetry 遥测上行触发入口（调用方以 goroutine 调用）。
func (*RuleChain) OnTelemetry(device model.Device, values map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("device_id", device.ID).Errorf("rule chain telemetry panic: %v", r)
		}
	}()
	if len(values) == 0 || strings.TrimSpace(device.TenantID) == "" {
		return
	}
	ctx := context.Background()
	for _, graph := range enabledGraphsForTenant(device.TenantID) {
		hasTrigger := false
		for _, root := range graph.Roots() {
			if root.Type == RuleChainTriggerTelemetry {
				hasTrigger = true
				break
			}
		}
		if !hasTrigger {
			continue
		}
		rcc := &RuleChainContext{
			DeviceID:     device.ID,
			DeviceNumber: device.DeviceNumber,
			TenantID:     device.TenantID,
			Timestamp:    time.Now().UnixMilli(),
		}
		for _, execErr := range ExecuteRuleChainGraph(ctx, graph, rcc, values) {
			logrus.WithField("chain", graph.Nodes[0].ID).Warn(execErr)
		}
	}
}

// OnDeviceOnline 设备上线触发入口（调用方以 goroutine 调用）。
func (*RuleChain) OnDeviceOnline(device model.Device) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("device_id", device.ID).Errorf("rule chain online panic: %v", r)
		}
	}()
	if strings.TrimSpace(device.TenantID) == "" {
		return
	}
	ctx := context.Background()
	values := map[string]any{"status": float64(1)}
	for _, graph := range enabledGraphsForTenant(device.TenantID) {
		hasTrigger := false
		for _, root := range graph.Roots() {
			if root.Type == RuleChainTriggerOnline {
				hasTrigger = true
				break
			}
		}
		if !hasTrigger {
			continue
		}
		rcc := &RuleChainContext{
			DeviceID:     device.ID,
			DeviceNumber: device.DeviceNumber,
			TenantID:     device.TenantID,
			Timestamp:    time.Now().UnixMilli(),
		}
		for _, execErr := range ExecuteRuleChainGraph(ctx, graph, rcc, values) {
			logrus.Warn(execErr)
		}
	}
}
