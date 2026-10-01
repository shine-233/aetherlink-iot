package dal

// 文件用途：资源中心跨形态检索（ListResourceCenterItems）的回归测试。
// 核心逻辑：device_templates / boards / widget_bundles 三张表在内存 sqlite 里各造几行，
//   断言"跨类型统一分页"的四条性质：租户隔离、total 与分页、排序、关键词转义。
// 关键注意事项：
//   - 只 AutoMigrate 三个模型：device_templates 的 download_count 列在模型里不存在
//     （生产库有，模型没有），目录聚合那条 SQL 依赖它，故目录接口不在 sqlite 上测。
//   - widget_bundles 的 created_at/updated_at 可空，用例必须覆盖 NULL 分支。

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupResourceCenterDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open resource center sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DeviceTemplate{}, &model.Board{}, &model.WidgetBundle{}); err != nil {
		t.Fatalf("migrate resource center: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
}

// seedResourceCenterItem 按形态插入一行；when 同时写 created_at 与 updated_at，
// pass nil 表示两列留空（部件库允许为 NULL）。
func seedResourceCenterItem(t *testing.T, kind, id, tenant, name string, when *time.Time) {
	t.Helper()
	ctx := context.Background()
	switch kind {
	case "device_template":
		row := &model.DeviceTemplate{ID: id, Name: name, TenantID: tenant}
		if when != nil {
			row.CreatedAt, row.UpdatedAt = *when, *when
		}
		require.NoError(t, global.DB.WithContext(ctx).Create(row).Error)
	case "board_template":
		row := &model.Board{ID: id, Name: name, TenantID: tenant}
		if when != nil {
			row.CreatedAt, row.UpdatedAt = *when, *when
		}
		require.NoError(t, global.DB.WithContext(ctx).Create(row).Error)
	case "widget_bundle":
		if when == nil {
			// 部件库两列时间允许 NULL；但模型的 UpdatedAt 字段会被 gorm Create
			// 自动填充成当前时间，NULL 场景必须绕过模型直插才能落库为 NULL。
			require.NoError(t, global.DB.WithContext(ctx).Exec(
				"INSERT INTO "+model.TableNameWidgetBundle+" (id, name, tenant_id, widgets, version) VALUES (?, ?, ?, '[]', '1.0.0')",
				id, name, tenant).Error)
			return
		}
		row := &model.WidgetBundle{ID: id, Name: name, TenantID: tenant, Widgets: "[]", Version: "1.0.0"}
		row.CreatedAt, row.UpdatedAt = when, when
		require.NoError(t, global.DB.WithContext(ctx).Create(row).Error)
	default:
		t.Fatalf("unknown resource kind %q", kind)
	}
}

func TestResourceCenterItemsPagedAcrossTypes(t *testing.T) {
	setupResourceCenterDB(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := base.Add(time.Hour)
	newest := base.Add(2 * time.Hour)

	seedResourceCenterItem(t, "device_template", "tpl-1", "t1", "模板一", &base)
	seedResourceCenterItem(t, "board_template", "board-1", "t1", "看板一", &newer)
	seedResourceCenterItem(t, "widget_bundle", "bundle-1", "t1", "部件一", &newest)
	// NULL 时间戳：必须排在最后且不报错（PG 的 DESC 默认 NULL 最前，靠 IS NULL 兜底拉平）。
	seedResourceCenterItem(t, "widget_bundle", "bundle-null", "t1", "部件无时间戳", nil)
	// 别的租户：不能出现在 t1 的结果里。
	seedResourceCenterItem(t, "device_template", "tpl-other", "t2", "别家模板", &newest)

	total, page1, err := ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{Page: 1, PageSize: 2}, "t1")
	require.NoError(t, err)
	require.Equal(t, int64(4), total, "total 必须是本租户跨形态总数")
	require.Len(t, page1, 2)
	require.Equal(t, "bundle-1", page1[0].ID)
	require.Equal(t, "widget_bundle", page1[0].ResourceType)
	require.Equal(t, "board-1", page1[1].ID)
	require.Equal(t, "board_template", page1[1].ResourceType)

	_, page2, err := ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{Page: 2, PageSize: 2}, "t1")
	require.NoError(t, err)
	require.Len(t, page2, 2)
	require.Equal(t, "tpl-1", page2[0].ID)
	require.Equal(t, "bundle-null", page2[1].ID, "无时间戳的行必须排在最后")
}

func TestResourceCenterItemsFiltersByTypeAndKeyword(t *testing.T) {
	setupResourceCenterDB(t)
	when := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	seedResourceCenterItem(t, "device_template", "tpl-1", "t1", "水泵模板", &when)
	seedResourceCenterItem(t, "board_template", "board-1", "t1", "水泵看板", &when)
	seedResourceCenterItem(t, "widget_bundle", "bundle-1", "t1", "水泵部件", &when)

	// 形态过滤：只看板。
	total, items, err := ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{
		Page: 1, PageSize: 10, ResourceType: "board_template",
	}, "t1")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "board-1", items[0].ID)

	// 未知形态：空结果而不是退化成全量。
	total, items, err = ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{
		Page: 1, PageSize: 10, ResourceType: "not_a_type",
	}, "t1")
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.Empty(t, items)

	// 关键词：三个形态都命中（转义后仍按包含匹配）。
	total, _, err = ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{
		Page: 1, PageSize: 10, Keyword: "水泵",
	}, "t1")
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
}

func TestResourceCenterItemsKeywordIsEscaped(t *testing.T) {
	setupResourceCenterDB(t)
	when := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	seedResourceCenterItem(t, "device_template", "tpl-percent", "t1", "pump%", &when)
	seedResourceCenterItem(t, "device_template", "tpl-plain", "t1", "pumpX", &when)

	// 关键词 "%" 是字面量：未转义时会把两条都命中。
	total, items, err := ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{
		Page: 1, PageSize: 10, Keyword: "%",
	}, "t1")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "tpl-percent", items[0].ID)
}

func TestResourceCenterItemsPageSizeIsClamped(t *testing.T) {
	setupResourceCenterDB(t)
	when := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	seedResourceCenterItem(t, "device_template", "tpl-1", "t1", "模板一", &when)

	// page/pageSize 非法时归一到默认值，不能产生负 OFFSET 或空列表。
	_, items, err := ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{Page: 0, PageSize: 0}, "t1")
	require.NoError(t, err)
	require.Len(t, items, 1)

	// 超大方页被夹到 maxListLimit，且不会越界 panic。
	_, items, err = ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{Page: 1, PageSize: maxListLimit * 10}, "t1")
	require.NoError(t, err)
	require.Len(t, items, 1)

	// 越界页返回空列表，total 仍为真实总数。
	total, items, err := ListResourceCenterItems(context.Background(), model.ResourceCenterListReq{Page: 99, PageSize: 10}, "t1")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Empty(t, items)
}
