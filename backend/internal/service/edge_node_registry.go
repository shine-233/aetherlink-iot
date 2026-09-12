// 文件用途：P1.5 边缘节点注册的决策层（校验 + 与既有节点比对）。
// 核心逻辑：先规范化并校验注册请求，再与既有节点比对，判定拒绝 / 无变化 / 更新。
// 关键注意事项：
//   - **同一 NodeID 换租户是安全事件，必须拒绝**。若静默覆盖，
//     一个租户就能把别人的边缘节点抢过来，后续所有下发都会打到错误的地方。
//   - **能力列表必须规范化**（去空白、去重、排序）后再比对，
//     否则同一节点重复注册会因顺序或空白差异被判成"变了"，产生无意义的变更记录。
//   - **版本为空或含非数值段一律拒绝注册**：后续的版本兼容判定依赖它，
//     放一个无法解析的版本进去，等于让后面的兼容检查全部失效。
//   - 无法判断（缺 NodeID / TenantID）直接拒绝，不做任何推测。
//   - 本文件只做决策，不落库；落库与证书签发待迁移排期。
package service

import (
	"sort"
	"strings"
)

// EdgeNodeRegistration 一次节点注册请求。
type EdgeNodeRegistration struct {
	NodeID       string
	TenantID     string
	Version      string
	Capabilities []string
}

// EdgeNodeRegistrationOutcome 注册判定结果。
type EdgeNodeRegistrationOutcome string

const (
	// EdgeNodeRegistrationRejected 拒绝：请求非法，或与既有节点冲突。
	EdgeNodeRegistrationRejected EdgeNodeRegistrationOutcome = "rejected"
	// EdgeNodeRegistrationUnchanged 幂等：与既有节点一致，无变化。
	EdgeNodeRegistrationUnchanged EdgeNodeRegistrationOutcome = "unchanged"
	// EdgeNodeRegistrationUpdated 更新：同租户下版本或能力发生变化。
	EdgeNodeRegistrationUpdated EdgeNodeRegistrationOutcome = "updated"
)

// normalizeCapabilities 去空白、丢空项、去重、排序。
// 排序保证同一组能力无论以什么顺序上报，规范化后都相同。
func normalizeCapabilities(capabilities []string) []string {
	seen := make(map[string]struct{}, len(capabilities))
	cleaned := make([]string, 0, len(capabilities))
	for _, raw := range capabilities {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	sort.Strings(cleaned)
	return cleaned
}

// isDottedNumericVersion 版本必须是点分数字（如 1.2.3）。
// 空串或含非数值段一律不合法——后续兼容判定无法处理，放行等于让检查失效。
func isDottedNumericVersion(version string) bool {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return false
	}
	for _, part := range strings.Split(trimmed, ".") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// EvaluateEdgeNodeRegistration 校验注册请求并与既有节点比对。
// existing 为 nil 表示首次注册（合法时返回 updated，表示"要写入新节点"）。
// 返回的 registration 是规范化后的结果，可直接用于落库。
func EvaluateEdgeNodeRegistration(req EdgeNodeRegistration, existing *EdgeNodeRegistration) (EdgeNodeRegistrationOutcome, string, EdgeNodeRegistration) {
	normalized := EdgeNodeRegistration{
		NodeID:       strings.TrimSpace(req.NodeID),
		TenantID:     strings.TrimSpace(req.TenantID),
		Version:      strings.TrimSpace(req.Version),
		Capabilities: normalizeCapabilities(req.Capabilities),
	}

	switch {
	case normalized.NodeID == "":
		return EdgeNodeRegistrationRejected, "node_id is required", normalized
	case normalized.TenantID == "":
		return EdgeNodeRegistrationRejected, "tenant_id is required", normalized
	case !isDottedNumericVersion(normalized.Version):
		return EdgeNodeRegistrationRejected, "version must be dotted numeric (e.g. 1.2.3)", normalized
	}

	if existing == nil {
		return EdgeNodeRegistrationUpdated, "first registration", normalized
	}

	// 同一 NodeID 归属别的租户：安全事件，拒绝而不是覆盖。
	if existing.TenantID != "" && existing.TenantID != normalized.TenantID {
		return EdgeNodeRegistrationRejected,
			"node id already registered to another tenant; refusing to reassign", normalized
	}

	sameVersion := existing.Version == normalized.Version
	sameCapabilities := capabilitiesEqual(existing.Capabilities, normalized.Capabilities)
	if sameVersion && sameCapabilities {
		return EdgeNodeRegistrationUnchanged, "no change", normalized
	}

	reason := "version changed"
	if !sameCapabilities {
		if sameVersion {
			reason = "capabilities changed"
		} else {
			reason = "version and capabilities changed"
		}
	}
	return EdgeNodeRegistrationUpdated, reason, normalized
}

// capabilitiesEqual 比较两端已规范化的能力列表。
// 调用方应传入规范化结果；这里逐项比对，长度不同即不同。
func capabilitiesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
