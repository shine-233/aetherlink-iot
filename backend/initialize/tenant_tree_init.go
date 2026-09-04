// 文件用途：装配进程级共享的租户父子树单例（C2 ① 的实接收口）。
// 核心逻辑：基于已建立的 GORM 连接创建 tenantree.DBSource 并挂入 global.TenantTree；
// 只做内存装配、不做任何查询——tenants 表在 60.sql 合入 main 前不存在时，
// 首次 Scope/Descendants 会按 tenantree 语义快速失败，调用方回退 self-only（旧隔离行为不变）。
// 关键注意事项：该单例被 RBAC 角色集扩展（②）与 DAL Scope 级联（③）共享，
// 写路径失效必须在真实租户写入处显式调用 global.TenantTree.Invalidate()，本文件不承载该职责。
// 重构建议：如需预加载或周期刷新，可在此追加 best-effort Refresh（失败仅告警，不阻断启动）。
package initialize

import (
	"fmt"
	"log"

	"aetherlink-iot/backend/internal/tenantree"
	global "aetherlink-iot/backend/pkg/global"
)

// TenantTreeInit 装配共享租户树单例。DB 必须已就绪；树本身懒加载。
// 写路径说明：截至本提交，全仓没有任何 Go 代码写入 public.tenants（建表+回填在 60.sql，
// 读取仅在 tenantree）。因此不存在可挂 Invalidate 的租户 CRUD/parent 变更代码；
// 未来新增租户写入点时，应在事务提交成功后调用 global.TenantTree.Invalidate()。
func TenantTreeInit() error {
	if global.DB == nil {
		return fmt.Errorf("tenant tree init: global.DB is nil")
	}
	global.TenantTree = tenantree.NewDBTree(global.DB)
	log.Println("tenant tree ready (lazy load; tenants table optional until sql/60.sql applied)")
	return nil
}
