package dal

// 文件用途：资源中心目录（Catalog）聚合统计。
// 核心逻辑：对 device_templates / boards / widget_bundles 三张表各跑一条 GROUP BY type_key 的
//   聚合查询，再在内存里按 type_key 归并成"行业目录"条目（数量 + 下载量）。
// 关键注意事项：
//   - widget_bundles 没有 download_count 列，投影里恒写 0——不要试图给它 SUM(download_count)。
//   - 三条聚合各自独立，任一条失败必须整体返回错误，不能返回半张目录。

import (
	"context"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
)

// catalogCountRow 内部目录统计扫描结构
type catalogCountRow struct {
	TypeKey       string `gorm:"column:type_key"`
	Count         int64  `gorm:"column:cnt"`
	DownloadCount int64  `gorm:"column:dl_cnt"`
}

// catalogAggregate 按 type_key 归并的累计器。
type catalogAggregate struct {
	deviceCount   int64
	boardCount    int64
	widgetCount   int64
	downloadCount int64
}

// ListResourceCenterCatalog 获取资源中心全貌目录（同时统计设备模板、大屏看板与部件库）。
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

	// 3. 统计部件库（TB-04；widget_bundles 无 download_count 列，dl_cnt 恒 0）
	var widgetRows []catalogCountRow
	err = global.DB.WithContext(ctx).
		Table(model.TableNameWidgetBundle).
		Select("COALESCE(type_key, '') AS type_key, COUNT(*) AS cnt, 0 AS dl_cnt").
		Where("tenant_id = ?", tenantID).
		Group("type_key").
		Find(&widgetRows).Error
	if err != nil {
		return nil, err
	}

	// 4. 归并三表统计
	agg := make(map[string]*catalogAggregate)
	entryFor := func(key string) *catalogAggregate {
		if _, ok := agg[key]; !ok {
			agg[key] = &catalogAggregate{}
		}
		return agg[key]
	}
	for _, r := range tplRows {
		e := entryFor(strings.TrimSpace(r.TypeKey))
		e.deviceCount += r.Count
		e.downloadCount += r.DownloadCount
	}
	for _, r := range boardRows {
		e := entryFor(strings.TrimSpace(r.TypeKey))
		e.boardCount += r.Count
		e.downloadCount += r.DownloadCount
	}
	for _, r := range widgetRows {
		entryFor(strings.TrimSpace(r.TypeKey)).widgetCount += r.Count
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
			WidgetCount:   v.widgetCount,
			TotalCount:    v.deviceCount + v.boardCount + v.widgetCount,
			DownloadCount: v.downloadCount,
		})
	}
	return result, nil
}
