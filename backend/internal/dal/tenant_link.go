// 文件用途：租户层级（ROADMAP C2）DAL 辅助——读取 tenants.parent_tenant_id 链接。
// 核心逻辑：供 service 层展开 hierarchy.Scope（self∪祖先）；表/列不存在（60.sql 未跑）时
//   优雅回退为仅自身作用域，保证老库启动不崩。
// 关键注意事项：链接结果不缓存跨请求写入；单次查询量小（租户数有限），错误仅记录不阻断。
package dal

import (
	"aetherlink-iot/backend/internal/hierarchy"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// TenantLink tenant parent 链接行。
type TenantLink struct {
	ID             string
	ParentTenantID string
}

// ListTenantParentLinks 读取全部租户的 parent_tenant_id 链接（供 Ancestors/Scope 推导）。
// 若 tenants 表或列不存在（迁移 60.sql 未应用），返回空切片而非错误，由调用方回退 self-only。
func ListTenantParentLinks() []hierarchy.Node {
	var rows []TenantLink
	err := global.DB.Raw("SELECT id, parent_tenant_id FROM tenants").Scan(&rows).Error
	if err != nil {
		logrus.WithError(err).Debug("tenants parent links unavailable; fall back to self-only scope")
		return nil
	}
	nodes := make([]hierarchy.Node, 0, len(rows))
	for _, r := range rows {
		nodes = append(nodes, hierarchy.Node{ID: r.ID, Parent: r.ParentTenantID})
	}
	return nodes
}
