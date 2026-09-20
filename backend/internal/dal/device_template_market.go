package dal

import (
	"context"
	"errors"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// PHASE-D-D10 BEGIN 模板市场运营化 DAL

// ListTemplateIDsByTypeKey 按租户+行业类型列出模板 ID。
// tenant-scope: tenant_id 硬过滤，仅返回调用者租户模板。
// 依模板名称去重并取最新版本，保证导出的资源包内不包含重名模板。
func ListTemplateIDsByTypeKey(ctx context.Context, tenantID, typeKey string) ([]string, error) {
	if global.DB == nil {
		return nil, errTemplateMarketDBNotReady
	}
	type item struct {
		ID        string    `gorm:"column:id"`
		Name      string    `gorm:"column:name"`
		Version   *string   `gorm:"column:version"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	query := global.DB.WithContext(ctx).
		Table(model.TableNameDeviceTemplate).
		Select("id, name, version, created_at").
		Where("tenant_id = ?", tenantID)
	if typeKey != "" {
		query = query.Where("type_key = ?", typeKey)
	}
	var rows []item
	if err := query.Order("created_at ASC").Scan(&rows).Error; err != nil {
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

// IncrementTemplateDownloadCounts 导出计数（打包下载按包含模板逐个 +1）。
// tenant-scope: ids 已由调用方按租户导出流程取得，更新按主键精确匹配。
func IncrementTemplateDownloadCounts(ctx context.Context, ids []string) error {
	if global.DB == nil {
		return errTemplateMarketDBNotReady
	}
	if len(ids) == 0 {
		return nil
	}
	return global.DB.WithContext(ctx).
		Table(model.TableNameDeviceTemplate).
		Where("id IN ?", ids).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

// ListMarketCatalog 行业分类目录：租户内 distinct type_key + 模板数。
// tenant-scope: tenant_id 硬过滤。
func ListMarketCatalog(ctx context.Context, tenantID string) ([]model.MarketCatalogEntry, error) {
	if global.DB == nil {
		return nil, errTemplateMarketDBNotReady
	}
	rows := make([]model.MarketCatalogEntry, 0)
	err := global.DB.WithContext(ctx).
		Table(model.TableNameDeviceTemplate).
		Select("COALESCE(type_key, '') AS type_key, COUNT(*) AS template_count, COALESCE(SUM(download_count),0) AS download_count").
		Where("tenant_id = ?", tenantID).
		Group("type_key").
		Order("template_count DESC").
		Find(&rows).Error
	return rows, err
}

var errTemplateMarketDBNotReady = errors.New("db is not initialized")

// PHASE-D-D10 END
