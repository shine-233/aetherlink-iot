// 文件用途：组共享可见性（TB-46 GPE v1）服务层公共 helper——计算当前调用者在
//
//	看板/资产列表查询中应被隐藏的组共享资源集合。
//
// 核心逻辑：管理员（SYS_ADMIN/TENANT_ADMIN）不受组共享限制；普通租户用户取
//
//	「组绑定资源集合 − 组内可见资源集合」= 对其隐藏的集合，列表查询按 ID 排除。
//	语义是"限制默认可见性"：绑定到组的看板/资产对组外成员 fail-closed 不可见，
//	未绑定任何组的资源维持既有租户内可见行为（不回归）。
//
// 关键注意事项：可见性映射的任何 DB 错误必须上抛（列表请求失败）而不是降级放行——
//
//	映射不可用时放行会把组受限资源泄露给组外成员（fail-open）；
//	scopes 为空时返回空集（上游列表本身查不到数据，无需过滤）。
//
// 重构建议：若后续出现"组共享放大可见性"（把资源单独授权给组而默认全员不可见）的
//
//	需求，需把"排除集"模型升级为"可见集白名单"模型并重审所有列表入口，不要在
//	本 helper 上做增量打补丁。
package service

import (
	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	utils "aetherlink-iot/backend/pkg/utils"
)

// groupHiddenResourceIDs 返回当前调用者应被隐藏的组共享资源 ID 列表（kind: board/asset）。
// 返回空切片/nil 表示无隐藏需求；错误必须由调用方上抛，不得降级为"不过滤"。
func groupHiddenResourceIDs(scopes []string, claims *utils.UserClaims, kind string) ([]string, error) {
	if claims == nil || claims.ID == "" || len(scopes) == 0 {
		return nil, nil
	}
	if authz.HasRole(claims, authz.ManagerRoles...) {
		// 管理员是组共享的配置者，不受其限制（组管理本身即管理员能力）。
		return nil, nil
	}
	bound, err := dal.GetGroupBoundResourceIDs(scopes, kind)
	if err != nil {
		return nil, err
	}
	if len(bound) == 0 {
		return nil, nil
	}
	visible, err := dal.GetGroupSharedResourceIDs(scopes, claims.ID, kind)
	if err != nil {
		return nil, err
	}
	visibleSet := make(map[string]struct{}, len(visible))
	for _, id := range visible {
		visibleSet[id] = struct{}{}
	}
	hidden := make([]string, 0)
	for _, id := range bound {
		if _, ok := visibleSet[id]; !ok {
			hidden = append(hidden, id)
		}
	}
	return hidden, nil
}

// groupHiddenResourceIDSet 同 groupHiddenResourceIDs，返回集合便于树剪枝等 O(1) 判定。
func groupHiddenResourceIDSet(scopes []string, claims *utils.UserClaims, kind string) (map[string]struct{}, error) {
	hidden, err := groupHiddenResourceIDs(scopes, claims, kind)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(hidden))
	for _, id := range hidden {
		set[id] = struct{}{}
	}
	return set, nil
}
