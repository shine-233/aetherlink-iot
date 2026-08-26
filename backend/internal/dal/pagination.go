// 文件用途：统一 DAL 列表查询的分页收敛逻辑。
// 核心逻辑：page/pageSize 有效时按 clamp 后的页大小 + offset 分页；未传分页参数时
//   以 defaultListLimit 兜底——历史"Page=0 不加 LIMIT"的条件分页会退化为全表扫描，
//   本 helper 是对该模式的结构性收口（范式来源：email_templates/fleet_command_jobs）。
// 关键注意事项：仅适用于交互式列表查询；确需全量的导出/后台任务路径不得使用，
//   应显式写明边界并单独评审。新列表查询一律走本 helper，不要再手写 if page != 0 分支。
// 重构建议：后续可为 default/max 上限增加配置面，并补充 gen 构建器侧的行为测试。

package dal

import "gorm.io/gorm"

const (
	// defaultListLimit 未传分页参数时的兜底行数上限。
	defaultListLimit = 200

	// maxListLimit 单页大小硬上限。
	maxListLimit = 500
)

// limitOffsetQuery 抽象两类构建器的共同形状：*gorm.DB 与 gen 生成的 IDo 接口的
// Limit/Offset 都返回自身类型，因此用自引用约束让链式调用保持可组合。
type limitOffsetQuery[Q any] interface {
	Limit(int) Q
	Offset(int) Q
}

// clampListPageSize 把单页大小收敛到 maxListLimit 内（非正数原样返回，
// 由调用方语义决定：applyListPagination 已保证仅在有效入参时走分页分支）。
func clampListPageSize(pageSize int) int {
	if pageSize > maxListLimit {
		return maxListLimit
	}
	return pageSize
}

// applyListPagination 返回施加了有界分页的查询构建器。
func applyListPagination[Q limitOffsetQuery[Q]](q Q, page, pageSize int) Q {
	if page > 0 && pageSize > 0 {
		return q.Limit(clampListPageSize(pageSize)).Offset((page - 1) * pageSize)
	}
	return q.Limit(defaultListLimit)
}

// 编译期锚定 *gorm.DB 满足约束。
var _ limitOffsetQuery[*gorm.DB] = (*gorm.DB)(nil)
