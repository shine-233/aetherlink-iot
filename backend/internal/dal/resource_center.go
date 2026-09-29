// 文件用途：资源中心（TP-5）跨形态资源检索与看板版本/下载量读取。
// 核心逻辑：资源中心把 device_templates / boards / widget_bundles 三张表投影成同一种
//   ResourceCenterItem，用一条 UNION ALL + ORDER BY + LIMIT/OFFSET 在库内完成排序分页；
//   total 另用三条 COUNT 取，避免为了数数把整表读进内存。
// 关键注意事项：
//   - 各分支的 SELECT 列序必须与 resourceCenterUnionRow 严格对齐，错位不会报错只会静默取错值。
//   - 排序用 (updated_at IS NULL) ASC 兜底：PG 的 DESC 默认把 NULL 排最前，SQLite 排最后，
//     不显式处理会让"未设更新时间"的部件库在两套库上出现在相反位置。
//   - 分页上限走 maxListLimit，与其余列表接口一致。

package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

var errResourceCenterDBNotReady = errors.New("database is not initialized")

// resourceCenterBranch 描述 UNION ALL 里的一个形态分支。
// projection 的列序必须与 resourceCenterUnionRow 一致。
type resourceCenterBranch struct {
	resourceType string
	table        string
	projection   string
	textColumns  []string // 关键词命中列
}

var resourceCenterBranches = map[string]resourceCenterBranch{
	"device_template": {
		resourceType: "device_template",
		table:        model.TableNameDeviceTemplate,
		projection: `id, 'device_template' AS resource_type, name,
			COALESCE(NULLIF(version, ''), '1.0.0') AS version,
			COALESCE(author, '') AS author, COALESCE(description, '') AS description,
			COALESCE(type_key, '') AS type_key, COALESCE(path, '') AS path,
			'' AS vis_type, 0 AS download_count, created_at, updated_at`,
		textColumns: []string{"name", "description"},
	},
	"board_template": {
		resourceType: "board_template",
		table:        model.TableNameBoard,
		projection: `id, 'board_template' AS resource_type, name,
			COALESCE(NULLIF(version, ''), '1.0.0') AS version,
			COALESCE(author, '') AS author, COALESCE(description, '') AS description,
			COALESCE(type_key, '') AS type_key, COALESCE(preview_url, '') AS path,
			COALESCE(NULLIF(vis_type, ''), 'native') AS vis_type,
			COALESCE(download_count, 0) AS download_count, created_at, updated_at`,
		textColumns: []string{"name", "description"},
	},
	"widget_bundle": {
		resourceType: "widget_bundle",
		table:        model.TableNameWidgetBundle,
		projection: `id, 'widget_bundle' AS resource_type, name,
			COALESCE(NULLIF(version, ''), '1.0.0') AS version,
			'' AS author, COALESCE(description, '') AS description,
			COALESCE(type_key, '') AS type_key, '' AS path,
			'' AS vis_type, 0 AS download_count, created_at, updated_at`,
		textColumns: []string{"name", "description"},
	},
}

// resourceCenterUnionRow 是 UNION ALL 的单行扫描结构。
// created_at / updated_at 用指针：widget_bundles 两列可空，直接扫进 time.Time 会因 NULL 报错。
type resourceCenterUnionRow struct {
	ID            string     `gorm:"column:id"`
	ResourceType  string     `gorm:"column:resource_type"`
	Name          string     `gorm:"column:name"`
	Version       string     `gorm:"column:version"`
	Author        string     `gorm:"column:author"`
	Description   string     `gorm:"column:description"`
	TypeKey       string     `gorm:"column:type_key"`
	Path          string     `gorm:"column:path"`
	VisType       string     `gorm:"column:vis_type"`
	DownloadCount int64      `gorm:"column:download_count"`
	CreatedAt     *time.Time `gorm:"column:created_at"`
	UpdatedAt     *time.Time `gorm:"column:updated_at"`
}

// ListResourceCenterItems 资源中心跨类型统一分页检索。
func ListResourceCenterItems(ctx context.Context, req model.ResourceCenterListReq, tenantID string) (int64, []model.ResourceCenterItem, error) {
	if global.DB == nil {
		return 0, nil, errResourceCenterDBNotReady
	}

	keyword := strings.TrimSpace(req.Keyword)
	typeKey := strings.TrimSpace(req.TypeKey)
	rType := strings.TrimSpace(req.ResourceType)
	if rType == "" {
		rType = "all"
	}
	page, pageSize := normalizePageParams(req.Page, req.PageSize, 10, maxListLimit)

	branches := make([]resourceCenterBranch, 0, 3)
	if rType == "all" {
		for _, key := range []string{"device_template", "board_template", "widget_bundle"} {
			branches = append(branches, resourceCenterBranches[key])
		}
	} else if branch, ok := resourceCenterBranches[rType]; ok {
		branches = append(branches, branch)
	}
	if len(branches) == 0 {
		return 0, []model.ResourceCenterItem{}, nil
	}

	// 1. total：每形态一条 COUNT，只数不取行（旧实现把三张表整表读进内存再 len()）。
	var total int64
	for _, branch := range branches {
		q := global.DB.WithContext(ctx).Table(branch.table).Where("tenant_id = ?", tenantID)
		if typeKey != "" {
			q = q.Where("type_key = ?", typeKey)
		}
		q = whereKeywordContains(q, opLike, keyword, branch.textColumns...)
		var count int64
		if err := q.Count(&count).Error; err != nil {
			return 0, nil, err
		}
		total += count
	}
	if total == 0 {
		return 0, []model.ResourceCenterItem{}, nil
	}

	// 2. 当前页：一条 UNION ALL，排序与分页都在库内完成。
	// 复合 SELECT（UNION ALL）的 ORDER BY 只能引用输出列，不能写表达式，
	// 因此整体包一层子查询后再做表达式排序与分页（SQLite 与 PG 语义一致）。
	var union strings.Builder
	args := make([]interface{}, 0, len(branches)*3+2)
	for i, branch := range branches {
		if i > 0 {
			union.WriteString(" UNION ALL ")
		}
		union.WriteString("SELECT " + branch.projection + " FROM " + branch.table + " WHERE tenant_id = ?")
		args = append(args, tenantID)
		if typeKey != "" {
			union.WriteString(" AND type_key = ?")
			args = append(args, typeKey)
		}
		if keyword != "" {
			pattern := ContainsLikePattern(keyword)
			union.WriteString(" AND (" + branch.textColumns[0] + " LIKE ? ESCAPE '\\' OR " + branch.textColumns[1] + " LIKE ? ESCAPE '\\')")
			args = append(args, pattern, pattern)
		}
	}
	var sb strings.Builder
	sb.WriteString("SELECT * FROM (")
	sb.WriteString(union.String())
	sb.WriteString(") AS resource_center_union")
	// (updated_at IS NULL) ASC 让"无更新时间"的行在两套库上都排最后（PG 的 DESC 默认 NULL 最前）。
	sb.WriteString(" ORDER BY (updated_at IS NULL) ASC, updated_at DESC, resource_type ASC, id ASC LIMIT ? OFFSET ?")
	args = append(args, pageSize, (page-1)*pageSize)

	var rows []resourceCenterUnionRow
	if err := global.DB.WithContext(ctx).Raw(sb.String(), args...).Scan(&rows).Error; err != nil {
		return 0, nil, err
	}

	items := make([]model.ResourceCenterItem, 0, len(rows))
	for _, row := range rows {
		createdAt := time.Time{}
		if row.CreatedAt != nil {
			createdAt = *row.CreatedAt
		}
		updatedAt := createdAt
		if row.UpdatedAt != nil {
			updatedAt = *row.UpdatedAt
		}
		items = append(items, model.ResourceCenterItem{
			ID:            row.ID,
			ResourceType:  row.ResourceType,
			Name:          row.Name,
			Version:       row.Version,
			Author:        row.Author,
			Description:   row.Description,
			TypeKey:       row.TypeKey,
			Path:          row.Path,
			VisType:       row.VisType,
			DownloadCount: row.DownloadCount,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		})
	}
	return total, items, nil
}

// ListBoardTemplateVersionsInTenant 列出租户内全部看板的 名称→版本。
func ListBoardTemplateVersionsInTenant(ctx context.Context, tenantID string) (map[string]string, error) {
	if global.DB == nil {
		return nil, errResourceCenterDBNotReady
	}
	var rows []struct {
		Name    string  `gorm:"column:name"`
		Version *string `gorm:"column:version"`
	}
	err := global.DB.WithContext(ctx).
		Table(model.TableNameBoard).
		Select("name, version").
		Where("tenant_id = ?", tenantID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	versions := make(map[string]string, len(rows))
	for _, row := range rows {
		ver := "1.0.0"
		if row.Version != nil && *row.Version != "" {
			ver = *row.Version
		}
		if prev, ok := versions[row.Name]; !ok || ver > prev {
			versions[row.Name] = ver
		}
	}
	return versions, nil
}

// ListBoardIDsByTypeKey 按租户和行业类型查询看板 ID 列表。
// 依看板名称去重并取最新版本，保证导出的资源包内不包含重名看板。
func ListBoardIDsByTypeKey(ctx context.Context, tenantID, typeKey string) ([]string, error) {
	if global.DB == nil {
		return nil, errResourceCenterDBNotReady
	}
	type item struct {
		ID        string    `gorm:"column:id"`
		Name      string    `gorm:"column:name"`
		Version   *string   `gorm:"column:version"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	q := global.DB.WithContext(ctx).
		Table(model.TableNameBoard).
		Select("id, name, version, created_at").
		Where("tenant_id = ?", tenantID)
	if typeKey != "" {
		q = q.Where("type_key = ?", typeKey)
	}
	var rows []item
	if err := q.Order("created_at ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	latestByName := make(map[string]item, len(rows))
	for _, r := range rows {
		prev, exists := latestByName[r.Name]
		if !exists {
			latestByName[r.Name] = r
			continue
		}
		ver := ""
		if r.Version != nil {
			ver = *r.Version
		}
		prevVer := ""
		if prev.Version != nil {
			prevVer = *prev.Version
		}
		if ver > prevVer || (ver == prevVer && r.CreatedAt.After(prev.CreatedAt)) {
			latestByName[r.Name] = r
		}
	}
	ids := make([]string, 0, len(latestByName))
	for _, item := range latestByName {
		ids = append(ids, item.ID)
	}
	return ids, nil
}

// IncrementBoardDownloadCounts 批量累计看板导出/下载次数。
func IncrementBoardDownloadCounts(ctx context.Context, ids []string) error {
	if global.DB == nil {
		return errResourceCenterDBNotReady
	}
	if len(ids) == 0 {
		return nil
	}
	return global.DB.WithContext(ctx).
		Table(model.TableNameBoard).
		Where("id IN ?", ids).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

// GetBoardByNameInTenant 按租户和名称查询看板。
func GetBoardByNameInTenant(ctx context.Context, tenantID, name string) (*model.Board, error) {
	if global.DB == nil {
		return nil, errResourceCenterDBNotReady
	}
	var board model.Board
	err := global.DB.WithContext(ctx).
		Table(model.TableNameBoard).
		Where("tenant_id = ? AND name = ?", tenantID, name).
		First(&board).Error
	if err != nil {
		return nil, err
	}
	return &board, nil
}
