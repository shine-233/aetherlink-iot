// 文件用途：租户层级数据访问层（ROADMAP C2 客户层级）。
// 核心逻辑：TenantScope 返回"自身 + 全部后代"租户 ID 集合（BFS），供数据级联过滤
//           （WHERE tenant_id IN scope）与权限守卫复用；新增/更新/删除租户在此收口。
// 关键注意事项：
//   1. tenants 表是唯一层级事实源；未登记租户 ID 的 scope 退化为 [自身]，保持向后兼容。
//   2. 本层只做数据操作，层级可见性/循环校验等业务规则由 service 层守卫把关。
//   3. BFS 图解析提炼为无 DB 依赖的 ResolveTenantScopeFromMap，便于纯单测锁定语义。
// 重构建议：租户规模增长后可将 scope 计算下沉 CTE（WITH RECURSIVE），现阶段内存 BFS 足够。
package dal

import (
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// TenantScope 返回 tenantID 自身及其全部后代租户 ID 集合（无序）。
// 若 tenants 表中不存在该记录，退化为只含自身 ID 的单元素集合。
// 父租户用它作为数据可见范围，子租户语义上等价于旧版精确等值过滤。
func TenantScope(tenantID string) ([]string, error) {
	all, err := ListAllTenants()
	if err != nil {
		return nil, err
	}
	childrenByParent := make(map[string][]string, len(all))
	for _, t := range all {
		if t.ParentTenantID == nil || *t.ParentTenantID == "" {
			continue
		}
		childrenByParent[*t.ParentTenantID] = append(childrenByParent[*t.ParentTenantID], t.ID)
	}
	return ResolveTenantScopeFromMap(childrenByParent, tenantID), nil
}

// ResolveTenantScopeFromMap 从 childrenByParent 邻接表解析 root 的"自身+后代"集合。
// 无 DB 依赖的纯函数：图缺失（未登记）时退化为只含 root。
func ResolveTenantScopeFromMap(childrenByParent map[string][]string, root string) []string {
	scope := []string{root}
	seen := map[string]struct{}{root: {}}
	queue := []string{root}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, child := range childrenByParent[node] {
			if _, dup := seen[child]; dup {
				continue
			}
			seen[child] = struct{}{}
			scope = append(scope, child)
			queue = append(queue, child)
		}
	}
	return scope
}

// TenantAncestors 返回从 tenantID 沿父链向上直达根租户的祖先 ID 列表（不含自身，含根）。
// 用于守卫"目标租户是否落在当前管理员可管辖子树内"。
func TenantAncestors(tenantID string) ([]string, error) {
	all, err := ListAllTenants()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Tenant, len(all))
	for _, t := range all {
		byID[t.ID] = t
	}
	var ancestors []string
	cur := byID[tenantID]
	for cur != nil && cur.ParentTenantID != nil {
		parentID := *cur.ParentTenantID
		ancestors = append(ancestors, parentID)
		cur = byID[parentID]
	}
	return ancestors, nil
}

// ListAllTenants 读取全部租户登记（层级关系量级小，一次性装载供内存建图）。
// tenant-scope: all-tenants 层级托管元数据（非业务租户数据），导出给
// service 层后必须按可管辖子树裁剪再对外返回（见 TenantService.GetTenantTree）。
func ListAllTenants() ([]*model.Tenant, error) {
	var list []*model.Tenant
	if err := global.DB.Order("created_at ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetTenantByID 按 ID 查询单个租户登记；不存在返回 gorm.ErrRecordNotFound。
// tenant-scope: caller-enforced 目标租户的层级托管行，调用方（TenantService）
// 必须先经过 assertTenantManageability 可管辖子树守卫再落库/返回。
func GetTenantByID(id string) (*model.Tenant, error) {
	var t model.Tenant
	if err := global.DB.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTenant 新增租户登记；code 唯一冲突由数据库唯一约束兜底。
func CreateTenant(t *model.Tenant) (*model.Tenant, error) {
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := global.DB.Create(t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateTenant 更新租户登记（仅更新可编辑字段）。
func UpdateTenant(t *model.Tenant) error {
	now := time.Now().UTC()
	t.UpdatedAt = now
	return global.DB.Model(&model.Tenant{}).
		Where("id = ?", t.ID).
		Select("name", "code", "status", "remark", "updated_at").
		Updates(t).Error
}

// DeleteTenant 删除租户登记；调用方须先保证无子租户与可管辖性。
func DeleteTenant(id string) error {
	return global.DB.Where("id = ?", id).Delete(&model.Tenant{}).Error
}

// CountChildren 统计指定租户的直接子租户数量（删除守卫用）。
func CountChildren(parentTenantID string) (int64, error) {
	var count int64
	if err := global.DB.Model(&model.Tenant{}).
		Where("parent_tenant_id = ?", parentTenantID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}