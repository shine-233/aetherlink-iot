// 文件用途：SCADA / Widget 基础层数据访问（ROADMAP P1.3）。
// 核心逻辑：项目、画布文档、发布版本快照、控制审计的租户内读写。
// 关键注意事项：
//  1. 每个查询都强制带 tenant_id。跨租户数据对本层而言"不存在"，
//     一律表现为未命中，不返回数据交给上层判断。
//  2. 画布保存走**条件更新**（WHERE current_version = expectedVersion），
//     由 RowsAffected 判定成败：这是乐观并发的唯一可靠依据。
//     "先查版本号再更新"在高并发下会漏判，等于没有并发保护。
//  3. RowsAffected=0 同时涵盖"版本不匹配"与"根本不存在"两种原因，
//     本层只返回行数，由服务层显式再查一次来区分——不猜。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 项目
// ---------------------------------------------------------------------------

// CreateScadaProject 写入项目；同名冲突由数据库唯一约束拒绝。
func CreateScadaProject(p *model.ScadaProject) error {
	return global.DB.Create(p).Error
}

// GetScadaProjectInTenant 租户内读取项目；未命中返回 gorm.ErrRecordNotFound。
func GetScadaProjectInTenant(id, tenantID string) (*model.ScadaProject, error) {
	var m model.ScadaProject
	if err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListScadaProjects 列出租户内项目，按创建时间升序。
func ListScadaProjects(tenantID string, limit int) ([]model.ScadaProject, error) {
	if limit <= 0 {
		limit = 200
	}
	var rows []model.ScadaProject
	err := global.DB.Where("tenant_id = ?", tenantID).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

// DeleteScadaProjectInTenant 租户内删除项目，返回受影响行数（0=未命中）。
func DeleteScadaProjectInTenant(id, tenantID string) (int64, error) {
	res := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.ScadaProject{})
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// 画布文档
// ---------------------------------------------------------------------------

// CreateScadaDocument 写入文档；同名冲突由数据库唯一约束拒绝。
func CreateScadaDocument(d *model.ScadaDocument) error {
	return global.DB.Create(d).Error
}

// GetScadaDocumentInTenant 租户内读取文档；未命中返回 gorm.ErrRecordNotFound。
func GetScadaDocumentInTenant(id, tenantID string) (*model.ScadaDocument, error) {
	var m model.ScadaDocument
	if err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListScadaDocumentsByProject 列出项目下文档。
func ListScadaDocumentsByProject(tenantID, projectID string, limit int) ([]model.ScadaDocument, error) {
	if limit <= 0 {
		limit = 200
	}
	var rows []model.ScadaDocument
	err := global.DB.Where("tenant_id = ? AND project_id = ?", tenantID, projectID).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

// SaveScadaDocumentInTenant 乐观并发保存画布。
// 仅在 current_version 与 expectedVersion 一致**且文档未归档**时写入并把版本 +1。
// 归档是终态：少了这个条件，"归档"就只是个标签，数据照样能被改，
// 而归档的本意正是"这份画布不再变化"。
// 返回受影响行数：0 表示版本不匹配、已归档或文档不存在（由服务层区分）。
func SaveScadaDocumentInTenant(id, tenantID string, expectedVersion int32, canvas string, updatedBy *string) (int64, error) {
	res := global.DB.Model(&model.ScadaDocument{}).
		Where("id = ? AND tenant_id = ? AND current_version = ?", id, tenantID, expectedVersion).
		Where("status <> ?", model.ScadaStatusArchived).
		Updates(map[string]interface{}{
			"json_data":       canvas,
			"current_version": gorm.Expr("current_version + 1"),
			"updated_by":      updatedBy,
		})
	return res.RowsAffected, res.Error
}

// MarkScadaDocumentPublished 将文档标记为已发布的指定版本。
// 同样按 expectedVersion 做条件更新，避免并发保存把发布结果冲掉。
func MarkScadaDocumentPublished(id, tenantID string, expectedVersion, publishedVersion int32, updatedBy *string) (int64, error) {
	res := global.DB.Model(&model.ScadaDocument{}).
		Where("id = ? AND tenant_id = ? AND current_version = ?", id, tenantID, expectedVersion).
		Updates(map[string]interface{}{
			"status":            model.ScadaStatusPublished,
			"published_version": publishedVersion,
			"updated_by":        updatedBy,
		})
	return res.RowsAffected, res.Error
}

// MarkScadaDocumentArchived 归档文档（终态），返回受影响行数。
func MarkScadaDocumentArchived(id, tenantID string, updatedBy *string) (int64, error) {
	res := global.DB.Model(&model.ScadaDocument{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{
			"status":     model.ScadaStatusArchived,
			"updated_by": updatedBy,
		})
	return res.RowsAffected, res.Error
}

// DeleteScadaDocumentInTenant 租户内删除文档，返回受影响行数（0=未命中）。
func DeleteScadaDocumentInTenant(id, tenantID string) (int64, error) {
	res := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.ScadaDocument{})
	return res.RowsAffected, res.Error
}

// ---------------------------------------------------------------------------
// 发布版本快照
// ---------------------------------------------------------------------------

// InsertScadaDocumentVersion 写入一条发布快照；同版本重复发布由唯一约束拒绝。
func InsertScadaDocumentVersion(v *model.ScadaDocumentVersion) error {
	return global.DB.Create(v).Error
}

// GetScadaDocumentVersion 读取指定版本的快照；未命中返回 gorm.ErrRecordNotFound。
func GetScadaDocumentVersion(tenantID, documentID string, version int32) (*model.ScadaDocumentVersion, error) {
	var m model.ScadaDocumentVersion
	err := global.DB.Where("tenant_id = ? AND document_id = ? AND version = ?", tenantID, documentID, version).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListScadaDocumentVersions 列出文档已发布版本，按版本号倒序（最新在前）。
func ListScadaDocumentVersions(tenantID, documentID string, limit int) ([]model.ScadaDocumentVersion, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []model.ScadaDocumentVersion
	err := global.DB.Where("tenant_id = ? AND document_id = ?", tenantID, documentID).
		Order("version DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// ---------------------------------------------------------------------------
// 控制审计
// ---------------------------------------------------------------------------

// CreateScadaControlAudit 写入一条控制审计（成功与拒绝都落库）。
func CreateScadaControlAudit(a *model.ScadaControlAudit) error {
	return global.DB.Create(a).Error
}

// UpdateScadaControlAuditOutcome 将 pending 审计推进到终态（success/failed）。
// 只允许从 pending 推进：重复结算同一次下发会产生两条终态记录，
// 让一次控制动作看起来发生了两次。
func UpdateScadaControlAuditOutcome(id, tenantID, fromOutcome, toOutcome, detail string) (int64, error) {
	updates := map[string]interface{}{"outcome": toOutcome}
	if detail != "" {
		updates["detail"] = detail
	}
	res := global.DB.Model(&model.ScadaControlAudit{}).
		Where("id = ? AND tenant_id = ? AND outcome = ?", id, tenantID, fromOutcome).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// ListScadaControlAudits 列出文档的控制审计，按时间倒序。
func ListScadaControlAudits(tenantID, documentID string, limit int) ([]model.ScadaControlAudit, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []model.ScadaControlAudit
	err := global.DB.Where("tenant_id = ? AND document_id = ?", tenantID, documentID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
