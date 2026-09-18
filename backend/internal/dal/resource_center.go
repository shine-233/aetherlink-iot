// 文件用途：资源中心（TP-5）数据访问层（DAL）。
// 核心逻辑：
// 1. 聚合 device_templates 与 boards 两张表，实现跨形态资源分类目录（Catalog）；
// 2. 提供物模型模板与大屏/看板模板统一分页检索（List）；
// 3. 支持看板版本扫描（用于导入冲突预览）、按行业打包查询及下载量原子累加。
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

// catalogCountRow 内部目录统计扫描结构
type catalogCountRow struct {
	TypeKey       string `gorm:"column:type_key"`
	Count         int64  `gorm:"column:cnt"`
	DownloadCount int64  `gorm:"column:dl_cnt"`
}

// ListResourceCenterCatalog 获取资源中心全貌目录（同时统计设备模板与大屏看板）。
func ListResourceCenterCatalog(ctx context.Context, tenantID string) ([]model.ResourceCenterCatalogEntry, error) {
	if global.DB == nil {
		return nil, errResourceCenterDBNotReady
	}

	// 1. 统计设备模板
	var tplRows []catalogCountRow
	err := global.DB.WithContext(ctx).
		Table(model.TableNameDeviceTemplate).
		Select("COALESCE(type_key, '') AS type_key, COUNT(*) AS cnt, COALESCE(SUM(download_count), 0) AS dl_cnt").
		Where("tenant_id = ?", tenantID).
		Group("type_key").
		Find(&tplRows).Error
	if err != nil {
		return nil, err
	}

	// 2. 统计大屏看板
	var boardRows []catalogCountRow
	err = global.DB.WithContext(ctx).
		Table(model.TableNameBoard).
		Select("COALESCE(type_key, '') AS type_key, COUNT(*) AS cnt, COALESCE(SUM(download_count), 0) AS dl_cnt").
		Where("tenant_id = ?", tenantID).
		Group("type_key").
		Find(&boardRows).Error
	if err != nil {
		return nil, err
	}

	// 3. 归并两表统计
	type combinedEntry struct {
		deviceCount   int64
		boardCount    int64
		downloadCount int64
	}
	agg := make(map[string]*combinedEntry)
	for _, r := range tplRows {
		k := strings.TrimSpace(r.TypeKey)
		if _, ok := agg[k]; !ok {
			agg[k] = &combinedEntry{}
		}
		agg[k].deviceCount += r.Count
		agg[k].downloadCount += r.DownloadCount
	}
	for _, r := range boardRows {
		k := strings.TrimSpace(r.TypeKey)
		if _, ok := agg[k]; !ok {
			agg[k] = &combinedEntry{}
		}
		agg[k].boardCount += r.Count
		agg[k].downloadCount += r.DownloadCount
	}

	// 映射为响应列表
	result := make([]model.ResourceCenterCatalogEntry, 0, len(agg))
	for k, v := range agg {
		name := k
		if name == "" {
			name = "通用/默认"
		}
		result = append(result, model.ResourceCenterCatalogEntry{
			TypeKey:       k,
			Name:          name,
			DeviceCount:   v.deviceCount,
			BoardCount:    v.boardCount,
			TotalCount:    v.deviceCount + v.boardCount,
			DownloadCount: v.downloadCount,
		})
	}
	return result, nil
}

// ListResourceCenterItems 资源中心跨类型统一分页检索。
func ListResourceCenterItems(ctx context.Context, req model.ResourceCenterListReq, tenantID string) (int64, []model.ResourceCenterItem, error) {
	if global.DB == nil {
		return 0, nil, errResourceCenterDBNotReady
	}

	items := make([]model.ResourceCenterItem, 0)
	keyword := strings.TrimSpace(req.Keyword)
	typeKey := strings.TrimSpace(req.TypeKey)
	rType := strings.TrimSpace(req.ResourceType)
	if rType == "" {
		rType = "all"
	}

	includeTemplates := rType == "all" || rType == "device_template"
	includeBoards := rType == "all" || rType == "board_template"

	// 1. 查询符合条件的设备模板
	if includeTemplates {
		var tpls []model.DeviceTemplate
		q := global.DB.WithContext(ctx).Table(model.TableNameDeviceTemplate).Where("tenant_id = ?", tenantID)
		if typeKey != "" {
			q = q.Where("type_key = ?", typeKey)
		}
		if keyword != "" {
			pattern := "%" + keyword + "%"
			q = q.Where("name LIKE ? OR description LIKE ?", pattern, pattern)
		}
		if err := q.Find(&tpls).Error; err != nil {
			return 0, nil, err
		}
		for _, t := range tpls {
			author := ""
			if t.Author != nil {
				author = *t.Author
			}
			version := "1.0.0"
			if t.Version != nil && *t.Version != "" {
				version = *t.Version
			}
			desc := ""
			if t.Description != nil {
				desc = *t.Description
			}
			tk := ""
			if t.TypeKey != nil {
				tk = *t.TypeKey
			}
			path := ""
			if t.Path != nil {
				path = *t.Path
			}
			items = append(items, model.ResourceCenterItem{
				ID:            t.ID,
				ResourceType:  "device_template",
				Name:          t.Name,
				Version:       version,
				Author:        author,
				Description:   desc,
				TypeKey:       tk,
				Path:          path,
				VisType:       "",
				DownloadCount: 0,
				CreatedAt:     t.CreatedAt,
				UpdatedAt:     t.UpdatedAt,
			})
		}
	}

	// 2. 查询符合条件的大屏看板
	if includeBoards {
		var boards []model.Board
		q := global.DB.WithContext(ctx).Table(model.TableNameBoard).Where("tenant_id = ?", tenantID)
		if typeKey != "" {
			q = q.Where("type_key = ?", typeKey)
		}
		if keyword != "" {
			pattern := "%" + keyword + "%"
			q = q.Where("name LIKE ? OR description LIKE ?", pattern, pattern)
		}
		if err := q.Find(&boards).Error; err != nil {
			return 0, nil, err
		}
		for _, b := range boards {
			author := ""
			if b.Author != nil {
				author = *b.Author
			}
			version := "1.0.0"
			if b.Version != nil && *b.Version != "" {
				version = *b.Version
			}
			desc := ""
			if b.Description != nil {
				desc = *b.Description
			}
			tk := ""
			if b.TypeKey != nil {
				tk = *b.TypeKey
			}
			path := ""
			if b.PreviewURL != nil {
				path = *b.PreviewURL
			}
			visType := "native"
			if b.VisType != nil && *b.VisType != "" {
				visType = *b.VisType
			}
			items = append(items, model.ResourceCenterItem{
				ID:            b.ID,
				ResourceType:  "board_template",
				Name:          b.Name,
				Version:       version,
				Author:        author,
				Description:   desc,
				TypeKey:       tk,
				Path:          path,
				VisType:       visType,
				DownloadCount: b.DownloadCount,
				CreatedAt:     b.CreatedAt,
				UpdatedAt:     b.UpdatedAt,
			})
		}
	}

	total := int64(len(items))

	// 分页截取（在内存中按 UpdatedAt 倒序排列后切片）
	// 简单稳定的冒泡/插入排序，数量通常百级别以内
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].UpdatedAt.After(items[i].UpdatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	offset := (req.Page - 1) * req.PageSize
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return total, []model.ResourceCenterItem{}, nil
	}
	end := offset + req.PageSize
	if end > len(items) {
		end = len(items)
	}
	return total, items[offset:end], nil
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
	latestByName := make(map[string]item)
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
