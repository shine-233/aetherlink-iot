// 文件用途：规则链 2.0 Enrichment 富化节点（PHASE-D-D1）。
// 核心逻辑：originator 属性 / 最新遥测 / 关联设备属性 / 租户元数据四类读取，
//
//	结果统一合入 msg metadata（可配 prefix 命名空间），不改写 payload。
//
// 关键注意事项：设备与租户维度全部 fail-closed——关联设备查不到即报错，
//
//	杜绝借富化节点越权读取其他租户数据；数据读取经注入点替换以便 hermetic 测试。
package service

import (
	"context"
	"fmt"
	"strings"
)

// ruleChainEnrichmentFetcher 富化读取统一签名。
type ruleChainEnrichmentFetcher func(ctx context.Context, tenantID, deviceID string, keys []string) (map[string]any, error)

// ruleChainEnrichmentAttributes originator 维度富化：属性/最新遥测 → metadata。
// config: {keys:string[], prefix?:string}
func ruleChainEnrichmentAttributes(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any, deviceID string, fetch ruleChainEnrichmentFetcher) (ruleChainNodeResult, error) {
	if fetch == nil {
		return ruleChainNodeResult{}, fmt.Errorf("enrichment fetcher is not available")
	}
	keys, err := configStringList(node.Config, "keys")
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	if len(keys) == 0 {
		return ruleChainNodeResult{}, fmt.Errorf("enrichment node %s requires keys", node.ID)
	}
	if strings.TrimSpace(rcc.TenantID) == "" {
		return ruleChainNodeResult{}, fmt.Errorf("enrichment node %s requires tenant context", node.ID)
	}
	if strings.TrimSpace(deviceID) == "" {
		return ruleChainNodeResult{}, fmt.Errorf("enrichment node %s requires device context", node.ID)
	}
	values, err := fetch(ctx, rcc.TenantID, deviceID, keys)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	mergePrefixValues(metadata, enrichmentPrefix(node.Config), values)
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// ruleChainEnrichmentRelatedAttributes 关联设备属性富化。
// config: {keys, prefix?, device_id?(显式), device_id_key?(从 payload/metadata 取目标设备)}
func ruleChainEnrichmentRelatedAttributes(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	keys, err := configStringList(node.Config, "keys")
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	if len(keys) == 0 {
		return ruleChainNodeResult{}, fmt.Errorf("enrichment node %s requires keys", node.ID)
	}
	deviceID, err := resolveRelatedDeviceID(node, rcc, payload, metadata)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	if ruleChainDeviceLookup == nil {
		return ruleChainNodeResult{}, fmt.Errorf("device lookup is not available")
	}
	device, err := ruleChainDeviceLookup(ctx, rcc.TenantID, deviceID)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	if device == nil {
		return ruleChainNodeResult{}, fmt.Errorf("related device %q not found in tenant", deviceID)
	}
	if ruleChainAttributeFetcher == nil {
		return ruleChainNodeResult{}, fmt.Errorf("attribute fetcher is not available")
	}
	values, err := ruleChainAttributeFetcher(ctx, rcc.TenantID, device.ID, keys)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	mergePrefixValues(metadata, enrichmentPrefix(node.Config), values)
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}

// resolveRelatedDeviceID 关联设备解析顺序：config.device_id → payload[device_id_key] → metadata[device_id_key]。
func resolveRelatedDeviceID(node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (string, error) {
	if explicit, _ := node.Config["device_id"].(string); strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), nil
	}
	key, _ := node.Config["device_id_key"].(string)
	if key != "" {
		if raw, ok := payload[key]; ok {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s), nil
			}
		}
		if raw, ok := metadata[key]; ok {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s), nil
			}
		}
	}
	return "", fmt.Errorf("related device node %s requires device_id or device_id_key", node.ID)
}

// ruleChainEnrichmentTenantMetadata 租户元数据富化：整组合入 metadata。
func ruleChainEnrichmentTenantMetadata(ctx context.Context, node *RuleChainNode, rcc *RuleChainContext, payload, metadata map[string]any) (ruleChainNodeResult, error) {
	if strings.TrimSpace(rcc.TenantID) == "" {
		return ruleChainNodeResult{}, fmt.Errorf("tenant metadata node %s requires tenant context", node.ID)
	}
	meta, err := ruleChainTenantMetadataFetcher(ctx, rcc.TenantID)
	if err != nil {
		return ruleChainNodeResult{}, err
	}
	mergePrefixValues(metadata, enrichmentPrefix(node.Config), meta)
	return ruleChainNodeResult{pass: true, outputs: []ruleChainNodeOutput{{payload: payload, metadata: metadata, rcc: rcc}}}, nil
}
