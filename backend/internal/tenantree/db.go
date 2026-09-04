// 文件用途：tenantree 的默认数据库数据源（DBSource）。
// 核心逻辑：全量读取 public.tenants(id, parent_tenant_id)，并把 NULL 与空串两种"根"
//
//	约定统一归一为 Parent=""（与 internal/hierarchy 的根语义对齐），供 Tree 校验建树。
//
// 关键注意事项：
//   - 依赖 60.sql：public.tenants 表与其 parent_tenant_id 列由迁移提供；表/列缺失时
//     LoadTenantNodes 返回底层错误（不吞错、不在启动期假装可用）；
//   - 迁移只登记显式层级；未登记进该表的既有租户（users.tenant_id 隐式维度）在
//     Scope/Descendants 解析时自然退化为"仅自身"，与历史隔离行为兼容。
package tenantree

import (
	"context"

	"gorm.io/gorm"
)

// tenantNodesSQL 读取全量租户父子边。
// COALESCE(parent_tenant_id, ”)：兼容"NULL=根"（新建表风格）与"”=根"（60.sql ALTER 风格）。
const tenantNodesSQL = `SELECT id, COALESCE(parent_tenant_id, '') AS parent FROM public.tenants`

// DBSource 基于 *gorm.DB 的租户树数据源。
type DBSource struct {
	db *gorm.DB
}

// NewDBSource 构建 DB 数据源。
func NewDBSource(db *gorm.DB) *DBSource {
	return &DBSource{db: db}
}

// LoadTenantNodes 实现 Source：返回全量 (id, parent) 边；parent 已归一为空串表示根。
func (s *DBSource) LoadTenantNodes(ctx context.Context) ([]TenantNode, error) {
	if s == nil || s.db == nil {
		return nil, ErrNilSource
	}
	rows := make([]TenantNode, 0)
	err := s.db.WithContext(ctx).Raw(tenantNodesSQL).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// NewDBTree 便捷构造：以 DBSource 构建租户树缓存。
// 调用方需在服务装配期执行一次 Refresh（或依赖读路径的惰性首载）。
func NewDBTree(db *gorm.DB) *Tree {
	return New(NewDBSource(db))
}
