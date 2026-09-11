// 文件用途：通用实体关系服务层（ROADMAP P1.1）。
// 核心逻辑：租户 Scope 守卫、写入校验、幂等创建、条件查询、删除保护与显式级联。
// 关键注意事项：
//  1. 跨租户一律表现为 not found（CodeNotFound），不用 403——403 会泄漏"该 ID 存在于别处"。
//  2. 关系是有向的：反向必须显式写入，本层绝不自动生成反向边。
//  3. 成环策略（明确声明）：
//     - 自环（同类型且同 ID）拒绝写入，由模型校验拦截；
//     - 多跳环允许写入。关系图在工程上允许成环（如 A 管理 B、B 备份 A），
//     是否阻断由调用方用 HasRelationPath 自行判断，本层不静默拒绝也不静默"修复"。
//  4. 删除实体默认保护（有关系则拒绝），级联必须由调用方显式声明。
//  5. 租户层级不可用时 fail closed：不降级成"只允许查自己"，因为那会把
//     暂时性故障伪装成"确实没有更多数据"。
package service

import (
	"context"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
)

const (
	entityRelationDefaultLimit  = 200
	entityRelationMaxLimit      = 1000
	entityRelationDuplicateMsg  = "重复键违反唯一约束"
	entityRelationCycleMaxDepth = 8
)

// EntityDeletionPolicy 删除实体时对其关系的处理策略。
type EntityDeletionPolicy string

const (
	// EntityDeletionProtect 有关系则拒绝删除（默认，防止悄悄制造悬挂引用）。
	EntityDeletionProtect EntityDeletionPolicy = "protect"
	// EntityDeletionCascade 显式级联删除该实体的全部关系。
	EntityDeletionCascade EntityDeletionPolicy = "cascade"
)

// TenantScopeProvider 租户可视范围提供者。*tenantree.Tree 天然满足。
type TenantScopeProvider interface {
	Scope(ctx context.Context, tenantID string) ([]string, error)
}

// entityRelationScopeProvider 供测试与显式装配覆盖；为空时回落到 global.TenantTree。
var entityRelationScopeProvider TenantScopeProvider

// SetEntityRelationScopeProvider 显式注入租户层级来源（测试用；传 nil 表示回落默认）。
func SetEntityRelationScopeProvider(p TenantScopeProvider) {
	entityRelationScopeProvider = p
}

// listEntityRelations 关系查询落点，可注入。
// Scope 守卫是纯逻辑，本不该依赖数据库：把它与查询落点解耦后，
// 租户越权的判定可以脱离数据库验证（否则单机跑测试必挂）。
var listEntityRelations = dal.ListEntityRelations

func currentEntityRelationScopeProvider() (TenantScopeProvider, bool) {
	if entityRelationScopeProvider != nil {
		return entityRelationScopeProvider, true
	}
	if global.TenantTree != nil {
		return global.TenantTree, true
	}
	return nil, false
}

// assertTenantInScope 校验 target 是否在 scopeRoot 的可视范围内。
// 不在范围内返回 CodeNotFound；层级不可用返回系统错误（fail closed）。
func assertTenantInScope(ctx context.Context, scopeRoot, target string) error {
	if strings.TrimSpace(scopeRoot) == "" || strings.TrimSpace(target) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "租户作用域不能为空")
	}
	if scopeRoot == target {
		return nil
	}
	provider, ok := currentEntityRelationScopeProvider()
	if !ok {
		return errcode.NewWithMessage(errcode.CodeSystemError, "租户层级不可用")
	}
	allowed, err := provider.Scope(ctx, scopeRoot)
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeSystemError, "租户层级不可用")
	}
	for _, t := range allowed {
		if t == target {
			return nil
		}
	}
	return errcode.NewWithMessage(errcode.CodeNotFound, "关系不存在")
}

// CreateRelation 在租户内创建关系。
// 幂等：若同端点同类型的关系已存在，不报错，而是把既有记录回填到 r 并返回 nil，
// 便于调用方拿到既有 ID 而不是拿到一个冲突错误自己再去查一次。
func CreateRelation(ctx context.Context, r *model.EntityRelation) error {
	if r == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "关系不能为空")
	}
	if strings.TrimSpace(r.TenantID) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "租户不能为空")
	}
	if r.Metadata == nil {
		empty := "{}"
		r.Metadata = &empty
	}
	if err := model.ValidateEntityRelation(r, len(*r.Metadata)); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}

	err := dal.CreateEntityRelation(r)
	if err == nil {
		return nil
	}
	if isPostgresUniqueViolation(err) {
		existing, getErr := dal.FindEntityRelationInTenant(
			r.TenantID, r.FromType, r.FromID, r.RelationType, r.ToType, r.ToID,
		)
		if getErr == nil && existing != nil {
			*r = *existing
			return nil
		}
		// 冲突但读不回既有记录：不能当作成功，返回脱敏后的冲突错误。
		return errcode.NewWithMessage(errcode.CodeSystemError, entityRelationDuplicateMsg)
	}
	// 其余错误原样上抛，不吞掉数据库故障。
	return err
}

// ListRelations 按租户 Scope 查询关系，返回列表与总数。
// scopeRoot 是调用方所属租户；q.TenantID 是目标租户，必须是 scopeRoot 自身或其后代。
func ListRelations(ctx context.Context, scopeRoot string, q model.EntityRelationQuery) ([]*model.EntityRelation, int64, error) {
	if err := q.Validate(); err != nil {
		return nil, 0, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	if err := assertTenantInScope(ctx, scopeRoot, q.TenantID); err != nil {
		return nil, 0, err
	}
	if q.Limit <= 0 {
		q.Limit = entityRelationDefaultLimit
	}
	if q.Limit > entityRelationMaxLimit {
		q.Limit = entityRelationMaxLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	return listEntityRelations(q)
}

// DeleteRelation 删除租户内的单条关系，返回受影响行数（0=未命中或越权）。
func DeleteRelation(ctx context.Context, id, tenantID string) (int64, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(tenantID) == "" {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "关系 ID 与租户不能为空")
	}
	return dal.DeleteEntityRelationInTenant(id, tenantID)
}

// DeleteRelationsForEntity 删除某实体在租户内挂载的全部关系。
// 策略为 protect 且存在关系时拒绝删除并返回 CodeOpDenied，绝不静默级联。
func DeleteRelationsForEntity(ctx context.Context, tenantID, entityType, entityID string, policy EntityDeletionPolicy) (int64, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(entityID) == "" {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "租户与实体 ID 不能为空")
	}
	if !model.IsAllowedEntityType(entityType) {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, model.ErrEntityRelationUnknownType.Error())
	}

	count, err := dal.CountEntityRelations(tenantID, entityType, entityID)
	if err != nil {
		return 0, err
	}
	if policy != EntityDeletionCascade && count > 0 {
		return 0, errcode.NewWithMessage(errcode.CodeOpDenied, "实体仍存在关联关系，需显式级联后才能删除")
	}
	if count == 0 {
		return 0, nil
	}
	return dal.DeleteEntityRelationsForEntity(tenantID, entityType, entityID)
}

// HasRelationPath 判断从 from 到 to 是否存在深度不超过 maxDepth 的有向关系路径。
// 用于调用方在写入前自行判断是否会产生环；本函数不做任何写入。
// maxDepth<=0 时取 entityRelationMaxDepth 上限。
func HasRelationPath(ctx context.Context, tenantID, fromType, fromID, toType, toID string, relationType string, maxDepth int) (bool, error) {
	if !model.IsAllowedEntityType(fromType) || !model.IsAllowedEntityType(toType) {
		return false, errcode.NewWithMessage(errcode.CodeParamError, model.ErrEntityRelationUnknownType.Error())
	}
	if maxDepth <= 0 {
		maxDepth = entityRelationCycleMaxDepth
	}

	visited := map[string]bool{}
	frontier := []struct{ typ, id string }{{fromType, fromID}}
	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		next := make([]struct{ typ, id string }, 0)
		for _, node := range frontier {
			key := node.typ + ":" + node.id
			if visited[key] {
				continue
			}
			visited[key] = true

			list, _, err := dal.ListEntityRelations(model.EntityRelationQuery{
				TenantID:     tenantID,
				FromType:     node.typ,
				FromID:       node.id,
				RelationType: relationType,
			})
			if err != nil {
				return false, err
			}
			for _, rel := range list {
				if rel.ToType == toType && rel.ToID == toID {
					return true, nil
				}
				next = append(next, struct{ typ, id string }{rel.ToType, rel.ToID})
			}
		}
		frontier = next
	}
	return false, nil
}
