// 文件用途：通用实体关系数据访问层（ROADMAP P1.1）。
// 核心逻辑：租户内关系的写入、精确查找、条件查询、计数与删除。
// 关键注意事项：
//  1. 每一个查询都强制带 tenant_id 条件。跨租户数据对本层而言"不存在"，
//     不是"存在但没权限"，因此一律表现为未命中而不是返回数据交给上层判断。
//  2. 重复关系由数据库唯一约束（含 tenant_id）拒绝，本层不自己做"先查再插"，
//     那在高并发下会漏判；冲突判定统一交给 service 层的 isPostgresUniqueViolation。
package dal

import (
	"errors"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// ErrEntityRelationStoreUnavailable 存储句柄未就绪。
// 此前每个函数直接解引用 global.DB，nil 时直接 panic 崩进程；
// 这里改为显式错误（fail closed），与项目内其它 DAL 的 nil 防护约定一致
// （见 calcfield_recompute.go / device_template_market.go）。
var ErrEntityRelationStoreUnavailable = errors.New("entity relation store is unavailable")

func entityRelationDB() (*gorm.DB, error) {
	if global.DB == nil {
		return nil, ErrEntityRelationStoreUnavailable
	}
	return global.DB, nil
}

// CreateEntityRelation 写入一条关系。重复关系返回唯一约束冲突错误。
func CreateEntityRelation(r *model.EntityRelation) error {
	db, err := entityRelationDB()
	if err != nil {
		return err
	}
	return db.Create(r).Error
}

// FindEntityRelationInTenant 按完整端点定位关系，用于幂等写入与重复检测。
// 未命中返回 gorm.ErrRecordNotFound。
func FindEntityRelationInTenant(tenantID, fromType, fromID, relationType, toType, toID string) (*model.EntityRelation, error) {
	db, dbErr := entityRelationDB()
	if dbErr != nil {
		return nil, dbErr
	}
	var m model.EntityRelation
	err := db.
		Where("tenant_id = ?", tenantID).
		Where("from_type = ? AND from_id = ?", fromType, fromID).
		Where("relation_type = ?", relationType).
		Where("to_type = ? AND to_id = ?", toType, toID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetEntityRelationInTenant 按租户定位单条关系；未命中返回 gorm.ErrRecordNotFound。
func GetEntityRelationInTenant(id, tenantID string) (*model.EntityRelation, error) {
	db, dbErr := entityRelationDB()
	if dbErr != nil {
		return nil, dbErr
	}
	var m model.EntityRelation
	err := db.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DeleteEntityRelationInTenant 租户内删除关系，返回受影响行数（0=未命中）。
func DeleteEntityRelationInTenant(id, tenantID string) (int64, error) {
	db, err := entityRelationDB()
	if err != nil {
		return 0, err
	}
	res := db.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.EntityRelation{})
	return res.RowsAffected, res.Error
}

// ListEntityRelations 按条件查询租户内关系，返回列表与总数。
// total 为忽略分页后的总条数，便于前端分页。
func ListEntityRelations(query model.EntityRelationQuery) ([]*model.EntityRelation, int64, error) {
	handle, dbErr := entityRelationDB()
	if dbErr != nil {
		return nil, 0, dbErr
	}
	db := handle.Model(&model.EntityRelation{}).Where("tenant_id = ?", query.TenantID)

	if query.FromType != "" {
		db = db.Where("from_type = ?", query.FromType)
	}
	if query.FromID != "" {
		db = db.Where("from_id = ?", query.FromID)
	}
	if query.ToType != "" {
		db = db.Where("to_type = ?", query.ToType)
	}
	if query.ToID != "" {
		db = db.Where("to_id = ?", query.ToID)
	}
	if query.RelationType != "" {
		db = db.Where("relation_type = ?", query.RelationType)
	}

	// Entity* + Direction：按"某实体作为哪一端"匹配。
	if query.EntityType != "" && query.EntityID != "" {
		switch query.NormalizedDirection() {
		case model.RelationDirectionOut:
			db = db.Where("from_type = ? AND from_id = ?", query.EntityType, query.EntityID)
		case model.RelationDirectionIn:
			db = db.Where("to_type = ? AND to_id = ?", query.EntityType, query.EntityID)
		default:
			db = db.Where(
				"(from_type = ? AND from_id = ?) OR (to_type = ? AND to_id = ?)",
				query.EntityType, query.EntityID, query.EntityType, query.EntityID,
			)
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	db = db.Order("created_at DESC")
	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	}
	if query.Offset > 0 {
		db = db.Offset(query.Offset)
	}

	var list []*model.EntityRelation
	if err := db.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// CountEntityRelations 统计某实体在租户内挂载的关系数（任一端点）。
// 用于"删除实体时的关系保护"：调用方据此决定拒绝还是级联。
func CountEntityRelations(tenantID, entityType, entityID string) (int64, error) {
	db, dbErr := entityRelationDB()
	if dbErr != nil {
		return 0, dbErr
	}
	var total int64
	err := db.Model(&model.EntityRelation{}).
		Where("tenant_id = ?", tenantID).
		Where(
			"(from_type = ? AND from_id = ?) OR (to_type = ? AND to_id = ?)",
			entityType, entityID, entityType, entityID,
		).
		Count(&total).Error
	return total, err
}

// DeleteEntityRelationsForEntity 删除某实体在租户内的全部关系（任一端点）。
// 这是显式级联入口：只有调用方确认过策略后才应调用，绝不作为默认行为。
func DeleteEntityRelationsForEntity(tenantID, entityType, entityID string) (int64, error) {
	db, err := entityRelationDB()
	if err != nil {
		return 0, err
	}
	res := db.
		Where("tenant_id = ?", tenantID).
		Where(
			"(from_type = ? AND from_id = ?) OR (to_type = ? AND to_id = ?)",
			entityType, entityID, entityType, entityID,
		).
		Delete(&model.EntityRelation{})
	return res.RowsAffected, res.Error
}
