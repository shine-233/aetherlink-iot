// 文件用途：租户级列表读作用域的统一实现处（ROADMAP C2 自上而下）。
// 核心逻辑：原先 ota / fleet_command_job / notification_history / scene_automation /
//
//	email_template / fleet_saved_filter 各自复制了一份 15 行相同的作用域解析，本文件收敛为
//	两个共享 helper，各聚合保留自己的包装函数（名字与签名不变，既有测试无需改动）。
//
// 关键注意事项：角色判定一律走 internal/authz（authz.HasRole），禁止再写
//
//	claims.Authority == constant.XXX；空租户的 [""] 平台行语义与 fail-closed nil 是
//	既有 DAL 契约，任何"顺手清理"都会把平台默认行查丢或把空租户放行，禁止改动。
package service

import (
	"strings"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/pkg/utils"
)

// platformOrExpandedScopes 解析"平台行或自上而下"作用域：
// 空租户（SYS_ADMIN 维护的 tenant_id 为空串的平台默认行）→ [""]，保持旧行为；
// 非空租户 → expandTenantIDScope（self∪子孙，链接缺失回退 self-only）。
func platformOrExpandedScopes(tenantID string) []string {
	if tenantID == "" {
		return []string{""}
	}
	return expandTenantIDScope(tenantID)
}

// tenantReadListScopes 解析租户级资源的列表读作用域：
//   - nil 声明 → nil（fail-closed，DAL 查不到数据）；
//   - TENANT_USER 保持 self-only：资源无 per-user 维度，或其可见性由 DAL 的 owner
//     关系 EXISTS 钳制，跨层展开无意义；空租户 → nil fail-closed；
//   - 其余（TENANT_ADMIN / SYS_ADMIN）走 platformOrExpandedScopes：
//     空租户 → [""]（平台行），非空 → expandTenantIDScope。
func tenantReadListScopes(tenantID string, claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	if authz.HasRole(claims, authz.TenantUser) {
		if self := strings.TrimSpace(tenantID); self != "" {
			return []string{self}
		}
		return nil
	}
	if strings.TrimSpace(tenantID) == "" {
		return []string{""}
	}
	return platformOrExpandedScopes(tenantID)
}

// claimsTenantReadListScopes 是 tenantReadListScopes 的"取声明自身租户"简写。
func claimsTenantReadListScopes(claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	return tenantReadListScopes(claims.TenantID, claims)
}
