package dal

import "gorm.io/gorm"

const (
	// legacyListDefaultPageSize / legacyListMaxPageSize 资源类列表（媒体库、应用包、数据转换器、集成）
	// 的历史分页约定：pageSize 不在 [1, 200] 内时回落为 20。
	legacyListDefaultPageSize = 20
	legacyListMaxPageSize     = 200
)

// normalizeLegacyListPage 归一 page/pageSize：page < 1 取 1；pageSize 越界取默认 20。
func normalizeLegacyListPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > legacyListMaxPageSize {
		pageSize = legacyListDefaultPageSize
	}
	return page, pageSize
}

// countAndFindLegacyPage 在已施加筛选的 db 上 Count，再按 order（调用方代码常量）+ 归一后的分页 Find 到 dest。
// 计数失败返回 (0, err)；查询失败返回 (count, err)，与各调用方原有返回形状一致。
func countAndFindLegacyPage(db *gorm.DB, order string, page, pageSize int, dest interface{}) (int64, error) {
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	page, pageSize = normalizeLegacyListPage(page, pageSize)
	err := db.Order(order).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(dest).Error
	return count, err
}
