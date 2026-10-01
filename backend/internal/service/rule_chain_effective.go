// 文件用途：设备 Profile 档案级默认规则链解析（TB-18，ROADMAP §4.1/§5）。
// 核心逻辑：以 enabledGraphsForTenant 的租户级启用链为兜底，叠加档案（device_configs）
//
//	绑定的 default_rule_chain_id：档案链优先、租户链兜底、按 ChainID 去重、顺序稳定；
//	GetEffectiveRuleChainsForDevice 供 OnTelemetry/OnDeviceOnline 执行面消费，
//	ResolveEffectiveRuleChainsForDevice 供 GET /rule-chains/device-effective/:deviceId 出参。
//
// 关键注意事项：档案缺失/跨租户/未绑定/链停用一律回落租户级链（fail-open 到租户级执行，
//
//	绝不因档案侧异常中断上行）；空租户 fail-closed 返回空；档案链图带 60s 负缓存，
//	失效钩子复用 invalidateRuleChainCache（链增删改时清空）。
//
// 重构建议：若后续档案维度扩展更多执行面字段（队列、告警收敛），把档案解析抽成独立聚合器。
package service

import (
	"strings"
	"sync"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// 生效链来源标记（EffectiveRuleChainRef.Source）。
const (
	effectiveRuleChainSourceProfile = "profile"
	effectiveRuleChainSourceTenant  = "tenant"
)

// profileRuleChainCacheEntry 档案链图缓存条目：graph 为 nil 表示负缓存
// （链不存在/停用/图非法），避免坏绑定每条遥测都打一次 DB。
type ruleChainProfileCacheEntry struct {
	graph     *RuleChainGraph
	expiresAt time.Time
}

var (
	ruleChainProfileCacheMu   sync.Mutex
	ruleChainProfileCacheByID = map[string]ruleChainProfileCacheEntry{} // key: tenantID + "\x00" + chainID
)

// purgeProfileRuleChainCacheForTenant 按租户前缀清空档案链图缓存；
// 由 invalidateRuleChainCache 调用，与租户级链缓存的失效时机保持一致。
func purgeProfileRuleChainCacheForTenant(tenantID string) {
	prefix := tenantID + "\x00"
	for key := range ruleChainProfileCacheByID {
		if strings.HasPrefix(key, prefix) {
			delete(ruleChainProfileCacheByID, key)
		}
	}
}

// cachedProfileRuleChainGraph 读取（并按需加载）档案绑定链的解析图，仅缓存启用链；
// 停用/不存在/非法图负缓存为 nil。调用方需保证 chainID 已通过租户归属解析。
func cachedProfileRuleChainGraph(tenantID, chainID string) *RuleChainGraph {
	key := tenantID + "\x00" + chainID
	ruleChainProfileCacheMu.Lock()
	entry, ok := ruleChainProfileCacheByID[key]
	ruleChainProfileCacheMu.Unlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.graph
	}
	graph := loadEnabledProfileRuleChainGraph(tenantID, chainID)
	ruleChainProfileCacheMu.Lock()
	ruleChainProfileCacheByID[key] = ruleChainProfileCacheEntry{graph: graph, expiresAt: time.Now().Add(ruleChainCacheTTL)}
	ruleChainProfileCacheMu.Unlock()
	return graph
}

// loadEnabledProfileRuleChainGraph 从 DAL 加载档案绑定链并解析；租户过滤在
// GetRuleChainByID 内完成（id+tenant 双条件），查不到/停用/图非法统一返回 nil。
func loadEnabledProfileRuleChainGraph(tenantID, chainID string) *RuleChainGraph {
	chain, err := dal.GetRuleChainByID(chainID, tenantID)
	if err != nil {
		logrus.WithError(err).Warn("load profile rule chain failed")
		return nil
	}
	if chain == nil || !chain.Enabled {
		return nil
	}
	graph, perr := ParseRuleChainGraph(string(chain.Graph))
	if perr != nil {
		logrus.WithError(perr).Warn("skip invalid profile rule chain graph")
		return nil
	}
	graph.ChainID = chain.ID
	return graph
}

// profileDefaultRuleChainIDForDevice 解析设备档案绑定的默认规则链 ID；未绑定/异常返回空串。
// fail-closed 点：档案与设备不同租户时绑定一律不生效（防止跨租户链借档案挂载）。
func profileDefaultRuleChainIDForDevice(device model.Device) string {
	if device.DeviceConfigID == nil {
		return ""
	}
	configID := strings.TrimSpace(*device.DeviceConfigID)
	if configID == "" {
		return ""
	}
	// 进程内 TTL 缓存（dal.GetDeviceConfigRouting）：每条上行不再反序列化整份档案 JSON。
	routing, found, err := dal.GetDeviceConfigRouting(configID)
	if err != nil || !found {
		if err != nil {
			logrus.WithError(err).WithField("device_id", device.ID).Warn("load device config for rule chain resolution failed")
		}
		return ""
	}
	tenantID := strings.TrimSpace(device.TenantID)
	if tenantID == "" || routing.TenantID != tenantID {
		logrus.WithFields(logrus.Fields{
			"device_id":        device.ID,
			"device_config_id": configID,
		}).Warn("device config tenant mismatch; profile rule chain ignored")
		return ""
	}
	return routing.DefaultRuleChainID
}

// resolveEffectiveRuleChainGraphs 纯函数（单测锚点）：档案链优先、租户链兜底、
// 按 ChainID 去重、顺序稳定——输入两个切片的相对顺序即输出的相对顺序，
// 不做排序、不遍历 map 产出顺序，同输入必同输出。
func resolveEffectiveRuleChainGraphs(profileGraphs, tenantGraphs []*RuleChainGraph) []*RuleChainGraph {
	merged := make([]*RuleChainGraph, 0, len(profileGraphs)+len(tenantGraphs))
	seen := make(map[string]bool, len(profileGraphs)+len(tenantGraphs))
	for _, group := range [][]*RuleChainGraph{profileGraphs, tenantGraphs} {
		for _, graph := range group {
			if graph == nil {
				continue
			}
			if graph.ChainID != "" {
				if seen[graph.ChainID] {
					continue
				}
				seen[graph.ChainID] = true
			}
			merged = append(merged, graph)
		}
	}
	return merged
}

// GetEffectiveRuleChainsForDevice 解析单设备生效规则链（TB-18 执行面入口）：
// 档案绑定的默认链优先、租户级启用链兜底，按 ChainID 去重、顺序稳定。
// 任何档案侧解析失败都退化为纯租户级链（不中断上行）；空租户 fail-closed 返回空。
func GetEffectiveRuleChainsForDevice(device model.Device) []*RuleChainGraph {
	tenantID := strings.TrimSpace(device.TenantID)
	if tenantID == "" {
		return nil
	}
	tenantGraphs := enabledGraphsForTenant(tenantID)
	chainID := profileDefaultRuleChainIDForDevice(device)
	if chainID == "" {
		return tenantGraphs
	}
	profileGraphs := make([]*RuleChainGraph, 0, 1)
	if graph := cachedProfileRuleChainGraph(tenantID, chainID); graph != nil {
		profileGraphs = append(profileGraphs, graph)
	}
	return resolveEffectiveRuleChainGraphs(profileGraphs, tenantGraphs)
}

// ResolveEffectiveRuleChainsForDevice 解析端点服务（TB-18）：按读权限校验设备归属后，
// 返回档案绑定元数据与实际生效链清单（档案链在前、租户链在后、去重）。
func (*RuleChain) ResolveEffectiveRuleChainsForDevice(deviceID string, claims *utils.UserClaims) (*model.DeviceEffectiveRuleChainsRes, error) {
	if claims == nil {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	device, err := loadTelemetryDeviceForAccess(deviceID, claims, "no permission to resolve device rule chains")
	if err != nil {
		return nil, err
	}
	// 不放开共享只读：规则链清单是租户级元数据，共享接收方不得借设备视图枚举链名。
	if !hasTelemetryTenantAccess(device, claims, false) {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to resolve device rule chains")
	}
	tenantID := strings.TrimSpace(device.TenantID)
	if tenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant id in claims")
	}

	res := &model.DeviceEffectiveRuleChainsRes{DeviceID: device.ID, Chains: make([]model.EffectiveRuleChainRef, 0, 4)}
	if device.DeviceConfigID != nil && strings.TrimSpace(*device.DeviceConfigID) != "" {
		configID := strings.TrimSpace(*device.DeviceConfigID)
		res.DeviceConfigID = &configID
		if bound := profileDefaultRuleChainIDForDevice(*device); bound != "" {
			res.DefaultRuleChainID = &bound
		}
	}
	// 档案链（若有效启用）：按档案来源置于队首。
	if res.DefaultRuleChainID != nil {
		chain, chainErr := dal.GetRuleChainByID(*res.DefaultRuleChainID, tenantID)
		if chainErr != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": chainErr.Error()})
		}
		if chain != nil && chain.Enabled {
			res.Chains = append(res.Chains, model.EffectiveRuleChainRef{ID: chain.ID, Name: chain.Name, Source: effectiveRuleChainSourceProfile})
		}
	}
	// 租户级启用链兜底：与执行面同一查询（ListEnabledRuleChains），顺序天然一致。
	chains, err := dal.ListEnabledRuleChains(tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	for _, c := range chains {
		if res.DefaultRuleChainID != nil && c.ID == *res.DefaultRuleChainID {
			// 已按档案来源列出，去重（停用绑定则不在这里重复出现）。
			continue
		}
		res.Chains = append(res.Chains, model.EffectiveRuleChainRef{ID: c.ID, Name: c.Name, Source: effectiveRuleChainSourceTenant})
	}
	return res, nil
}
