// 文件用途：提供媒体库（media_files，ROADMAP TB-41）持久化存储操作与引用扫描。
// 核心逻辑：媒体登记 CRUD（租户隔离 fail-closed）、分页检索，以及按 file_path 在
//
//	看板 config/preview_url、SCADA 文档 json_data、OTA 升级包 package_url 四个
//	引用面上的包含扫描（删除闸门与引用统计的数据源）。
//
// 关键注意事项：全部查询/删除函数显式携带 tenant_id 条件（tenant-scope: caller-enforced，
//
//	由 service 层注入 claims.TenantID）；引用扫描用 EscapeLikePattern 防通配符注入；
//	扫描面新增表时必须同步带租户过滤，否则引用统计会跨租户误报。
//
// 重构建议：引用关系若需要精确到部件级，落 media_file_refs 明细表替代 LIKE 扫描。
package dal

import (
	"context"
	"errors"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errMediaFileDBNotReady = errors.New("database is not initialized")

// CreateMediaFile 插入一条媒体登记；同租户同路径冲突时幂等跳过（UNIQUE 守卫）。
func CreateMediaFile(record *model.MediaFile) error {
	db := global.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if db.Error != nil {
		return db.Error
	}
	return nil
}

// GetMediaFileForScope 按 id + 租户查询媒体登记（租户隔离 fail-closed）。
func GetMediaFileForScope(ctx context.Context, id, tenantID string) (*model.MediaFile, error) {
	if global.DB == nil {
		return nil, errMediaFileDBNotReady
	}
	var record model.MediaFile
	err := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetMediaFileByPathInTenant 按租户与对外路径查询媒体登记（上传登记幂等键用）。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func GetMediaFileByPathInTenant(ctx context.Context, tenantID, filePath string) (*model.MediaFile, error) {
	if global.DB == nil {
		return nil, errMediaFileDBNotReady
	}
	var record model.MediaFile
	err := global.DB.WithContext(ctx).
		Where("tenant_id = ? AND file_path = ?", tenantID, filePath).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ListMediaFilesForScope 分页查询租户媒体列表；search 模糊匹配原始文件名，mime 精确过滤。
func ListMediaFilesForScope(ctx context.Context, req *model.GetMediaFileListReq, tenantID string) (int64, []*model.MediaFile, error) {
	if global.DB == nil {
		return 0, nil, errMediaFileDBNotReady
	}
	var list []*model.MediaFile

	db := global.DB.WithContext(ctx).Model(&model.MediaFile{}).Where("tenant_id = ?", tenantID)
	if req.Mime != nil && strings.TrimSpace(*req.Mime) != "" {
		db = db.Where("mime = ?", strings.TrimSpace(*req.Mime))
	}
	if req.Search != nil && strings.TrimSpace(*req.Search) != "" {
		// LOWER + LIKE 的写法在 PostgreSQL 与 sqlite 测试库下语义一致（区分大小写由两侧 LOWER 保证）。
		db = db.Where("LOWER(file_name) LIKE LOWER(?)", ContainsLikePattern(*req.Search))
	}

	count, err := countAndFindLegacyPage(db, "created_at DESC, id DESC", req.Page, req.PageSize, &list)
	return count, list, err
}

// UpdateMediaFileReferencedCount 回写最近一次引用扫描的引用方数量。
func UpdateMediaFileReferencedCount(ctx context.Context, id, tenantID string, count int) error {
	if global.DB == nil {
		return errMediaFileDBNotReady
	}
	return global.DB.WithContext(ctx).Model(&model.MediaFile{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Update("referenced_count", count).Error
}

// DeleteMediaFileForScope 按 id + 租户删除登记行；未命中返回 gorm.ErrRecordNotFound。
func DeleteMediaFileForScope(ctx context.Context, id, tenantID string) error {
	if global.DB == nil {
		return errMediaFileDBNotReady
	}
	result := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.MediaFile{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// mediaReferenceSurface 引用扫描面：表名 + 参与 LIKE 匹配的列 + 展示名。
type mediaReferenceSurface struct {
	kind     string
	table    string
	nameCol  string
	matchSQL string
}

// mediaReferenceSurfaces 当前已知的引用面。新增引用面时：每条 matchSQL 必须内嵌
// tenant_id = ? 过滤（fail-closed），并在 service 层注释中同步说明。
// 注意：boards.config 与 scada_documents.json_data 是 json/jsonb 列，
// 真实 PostgreSQL 下必须显式 ::text 转型才能 LIKE（sqlite 测试库无此要求）。
var mediaReferenceSurfaces = []mediaReferenceSurface{
	{
		kind:     "board",
		table:    "boards",
		nameCol:  "name",
		matchSQL: "(config::text LIKE ? OR preview_url LIKE ?)",
	},
	{
		kind:     "scada_document",
		table:    "scada_documents",
		nameCol:  "name",
		matchSQL: "json_data::text LIKE ?",
	},
	{
		kind:     "ota_package",
		table:    "ota_upgrade_packages",
		nameCol:  "name",
		matchSQL: "package_url LIKE ?",
	},
}

// CountMediaReferencesForPath 统计 file_path 在租户内各引用面出现的引用方列表。
// 匹配 needle 用去掉 "./" 前缀的路径，兼容引用方存 "./files/..." 或 "files/..." 两种写法。
func CountMediaReferencesForPath(ctx context.Context, tenantID, filePath string) ([]model.MediaReferencer, error) {
	if global.DB == nil {
		return nil, errMediaFileDBNotReady
	}
	needle := "%" + EscapeLikePattern(strings.TrimPrefix(filePath, "./")) + "%"

	referencers := make([]model.MediaReferencer, 0)
	// ::text 转型仅 PostgreSQL 需要；sqlite 单测库不识别该转型语法。
	useTextCast := global.DB != nil && global.DB.Dialector.Name() == "postgres"
	for _, surface := range mediaReferenceSurfaces {
		matchSQL := surface.matchSQL
		if !useTextCast {
			matchSQL = strings.ReplaceAll(matchSQL, "::text", "")
		}
		placeholders := strings.Count(matchSQL, "?")
		args := make([]interface{}, 0, placeholders+1)
		for i := 0; i < placeholders; i++ {
			args = append(args, needle)
		}
		args = append(args, tenantID)

		var rows []model.MediaReferencer
		query := global.DB.WithContext(ctx).Table(surface.table).
			Select("? AS kind, id AS id, "+surface.nameCol+" AS name", surface.kind).
			Where(matchSQL+" AND tenant_id = ?", args...)
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		referencers = append(referencers, rows...)
	}
	return referencers, nil
}
