// 文件用途：规则链服务层（ROADMAP B2）——CRUD、图校验与上行执行入口。
// 核心逻辑：CRUD 带租户守卫与 DAG 校验；执行入口按租户拉启用链（60s 缓存），
//
//	写操作失效缓存；OnTelemetry/OnDeviceOnline 供上行钩子以 goroutine 调用。
//
// 关键注意事项：空租户 fail-closed；执行错误只记录不阻断上行主流程；
//
//	单设备单次执行的节点扇出由 maxNodes 上限约束，防止放大。
package service

import (
	"bytes"
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
	"github.com/go-playground/validator/v10"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

const ruleChainCacheTTL = 60 * time.Second

// RuleChain 规则链服务入口。
type RuleChain struct{}

var ruleChainRequestValidator = validator.New()

type createRuleChainReq struct {
	Name        string          `json:"name" validate:"required,max=128"`
	Description *string         `json:"description"`
	TenantID    string          `json:"tenant_id"` // 仅 SYS_ADMIN 可指定
	Enabled     *bool           `json:"enabled"`
	Graph       json.RawMessage `json:"graph" validate:"required"`
}

type updateRuleChainReq struct {
	ID          string          `json:"id" validate:"required,max=36"`
	Name        string          `json:"name" validate:"required,max=128"`
	Description *string         `json:"description"`
	Enabled     bool            `json:"enabled"`
	Graph       json.RawMessage `json:"graph" validate:"required"`
}

func validateRuleChainRequest(req interface{}) error {
	if err := ruleChainRequestValidator.Struct(req); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "invalid rule chain request: "+err.Error())
	}
	return nil
}

func normalizeRuleChainGraph(raw json.RawMessage) ([]byte, error) {
	graphJSON := bytes.TrimSpace([]byte(raw))
	if len(graphJSON) > 0 && graphJSON[0] == '"' {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "graph string is not valid json")
		}
		graphJSON = []byte(encoded)
	}
	if _, err := ParseRuleChainGraph(string(graphJSON)); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "invalid graph: "+err.Error())
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, graphJSON); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "graph is not valid json")
	}
	return compacted.Bytes(), nil
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
	req.Name = strings.TrimSpace(req.Name)
	if err := validateRuleChainRequest(&req); err != nil {
		return nil, err
	}
	tenantID, err := normalizeRuleChainTenant(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	graphJSON, err := normalizeRuleChainGraph(req.Graph)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	now := time.Now().UTC()
	chain := &model.RuleChain{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     enabled,
		Graph:       graphJSON,
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
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if err := validateRuleChainRequest(&req); err != nil {
		return nil, err
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
	graphJSON, err := normalizeRuleChainGraph(req.Graph)
	if err != nil {
		return nil, err
	}
	updated := &model.RuleChain{
		ID:          existing.ID,
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		Graph:       graphJSON,
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

// ruleChainListScopes 将调用方 claims 映射为规则链列表读作用域（ROADMAP C2 自上而下三态约定的服务层入口）：
//   - nil claims → nil（fail-closed，由 ListChains 返回无权限）；
//   - TENANT_USER → 仅自身 [claims.TenantID]；
//   - 空租户（SYS_ADMIN 平台态）→ [""]（保留 legacy 平台行可读语义）；
//   - 非空管理员（TENANT_ADMIN / 指定租户的 SYS_ADMIN）→ expandTenantIDScope（self∪子孙，无链接回退 self-only）。
//
// 说明：规则链是租户级资源，无按用户维度的可见性拆分（不同于 notification_history 的设备 Owner EXISTS 钳制），
// 故 TENANT_USER 直接 self-only；作用域展开只放宽租户维，不触碰 Owner/User 维。
func ruleChainListScopes(claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	if claims.Authority == constant.TENANT_USER {
		if tenantID := strings.TrimSpace(claims.TenantID); tenantID != "" {
			return []string{tenantID}
		}
		return nil
	}
	if strings.TrimSpace(claims.TenantID) == "" {
		return []string{""}
	}
	return expandTenantIDScope(claims.TenantID)
}

// ListChains 分页列表；keyword 按名称模糊。读路径接入 C2 自上而下作用域。
func (*RuleChain) ListChains(keyword string, page, pageSize int, claims *utils.UserClaims) (map[string]interface{}, error) {
	scopes := ruleChainListScopes(claims)
	if len(scopes) == 0 {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	count, chains, err := dal.ListRuleChainsByTenant(scopes, keyword, page, pageSize)
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
	ruleChainCacheMu       sync.Mutex
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
		for _, execErr := range ExecuteRuleChainGraphForTrigger(ctx, graph, rcc, values, RuleChainTriggerTelemetry) {
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
		for _, execErr := range ExecuteRuleChainGraphForTrigger(ctx, graph, rcc, values, RuleChainTriggerOnline) {
			logrus.Warn(execErr)
		}
	}
}
