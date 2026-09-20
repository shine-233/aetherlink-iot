// File purpose: TB-19 行业方案 DAL——方案 CRUD 与安装流水写入。
// Core logic: 全部查询显式携带 tenant_id；安装流水 append-only，不提供更新。
package dal

import (
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InsertIndustrySolution 写入方案定义。
func InsertIndustrySolution(tx *gorm.DB, s *model.IndustrySolution) error {
	if tx == nil {
		tx = global.DB
	}
	return tx.Create(s).Error
}

// GetIndustrySolutionByIDAndTenant 按 ID + 租户读方案（不存在/不属于返回同一错误）。
func GetIndustrySolutionByIDAndTenant(tenantID, id string) (*model.IndustrySolution, error) {
	var s model.IndustrySolution
	err := global.DB.Where("tenant_id = ? AND id = ?", tenantID, id).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetIndustrySolutionByNameAndTenant 同租户按名查询（唯一索引一致性校验用）。
func GetIndustrySolutionByNameAndTenant(tenantID, name string) (*model.IndustrySolution, error) {
	var s model.IndustrySolution
	err := global.DB.Where("tenant_id = ? AND name = ?", tenantID, name).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListIndustrySolutionsByTenant 租户内方案分页列表。
func ListIndustrySolutionsByTenant(tenantID string, page, pageSize int) (int64, []model.IndustrySolution, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	db := global.DB.Model(&model.IndustrySolution{}).Where("tenant_id = ?", tenantID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var list []model.IndustrySolution
	err := db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return total, list, err
}

// DeleteIndustrySolution 条件删除：只有属于该租户的方案会被删。
func DeleteIndustrySolution(tenantID, id string) error {
	res := global.DB.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&model.IndustrySolution{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// InsertIndustrySolutionInstalls 批量写安装流水。
func InsertIndustrySolutionInstalls(tx *gorm.DB, rows []model.IndustrySolutionInstall) error {
	if len(rows) == 0 {
		return nil
	}
	if tx == nil {
		tx = global.DB
	}
	return tx.Create(&rows).Error
}

// ListIndustrySolutionInstalls 方案安装流水回查（最新在前）。
func ListIndustrySolutionInstalls(tenantID, solutionID string, limit int) ([]model.IndustrySolutionInstall, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []model.IndustrySolutionInstall
	err := global.DB.
		Where("tenant_id = ? AND solution_id = ?", tenantID, solutionID).
		Clauses(clause.OrderBy{Columns: []clause.OrderByColumn{{Column: clause.Column{Name: "created_at"}, Desc: true}}}).
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
