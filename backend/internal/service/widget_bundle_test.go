// 文件用途：部件库服务（TB-04）单元测试。
// 核心逻辑：widgets JSON 结构校验、内置四部件导出描述、种子 bundle 幂等落库、
//
//	资源中心部件库维度的依赖检查与冲突预览、租户幂等导入（sqlite 内存库）。
//
// 关键注意事项：全部 DB 用例走 sqlite 内存库并 AutoMigrate 仅 WidgetBundle 表，
//
//	不依赖真实 PostgreSQL；跨租户隔离由 (id, tenant_id) 双条件查询保证。
//
// 重构建议：若内置部件定义扩充（如新增 circle 部件），只需同步
//
//	scada_mobile_wiring.go 的 builtinWidgetDefinitions，本测试按数量与类型名断言。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupWidgetBundleTestDB 建立仅含 widget_bundles 表的 sqlite 内存库。
func setupWidgetBundleTestDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.WidgetBundle{}))
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
}

const validGaugeDef = `{"type":"gauge","version":"1","schema":"{\"type\":\"object\"}","capabilities":["2d"],"commands":[{"name":"refresh","requires_confirmation":false}]}`

func TestValidateWidgetsJSON(t *testing.T) {
	// 空数组与合法定义通过
	require.NoError(t, validateWidgetsJSON("[]"))
	require.NoError(t, validateWidgetsJSON("["+validGaugeDef+"]"))

	// 非法 JSON / 非数组 / 非对象数组
	require.Error(t, validateWidgetsJSON("not-json"))
	require.Error(t, validateWidgetsJSON(`{"type":"gauge"}`))
	require.Error(t, validateWidgetsJSON(""))

	// 缺 type / 缺 version / 缺 schema / 非法 capability / 重复命令
	require.Error(t, validateWidgetsJSON(`[{"version":"1","schema":"{}","capabilities":["2d"]}]`))
	require.Error(t, validateWidgetsJSON(`[{"type":"gauge","schema":"{}","capabilities":["2d"]}]`))
	require.Error(t, validateWidgetsJSON(`[{"type":"gauge","version":"1","capabilities":["2d"]}]`))
	require.Error(t, validateWidgetsJSON(`[{"type":"gauge","version":"1","schema":"{}","capabilities":["flat"]}]`))
	require.Error(t, validateWidgetsJSON(`[{"type":"gauge","version":"1","schema":"{}","capabilities":["2d"],"commands":[{"name":"a"},{"name":"a"}]}]`))

	// 服务入口归一：空/缺失落为 "[]"，非法内容报错
	out, err := normalizeWidgetsInput(nil)
	require.NoError(t, err)
	require.Equal(t, "[]", out)
	empty := "  "
	out, err = normalizeWidgetsInput(&empty)
	require.NoError(t, err)
	require.Equal(t, "[]", out)
	bad := "not-json"
	_, err = normalizeWidgetsInput(&bad)
	require.Error(t, err)
}

func TestExportBuiltinWidgetBundle(t *testing.T) {
	svc := &WidgetBundleService{}
	exported, err := svc.ExportBuiltinWidgetBundle()
	require.NoError(t, err)
	require.Equal(t, "aetherlink-widget-bundle", exported.Kind)
	require.Equal(t, BuiltinWidgetBundleName, exported.Name)
	require.Equal(t, "1.0.0", *exported.Version)
	require.Equal(t, BuiltinWidgetBundleTypeKey, *exported.TypeKey)

	// 内置四部件：gauge/chart/valve/twin3d，逐个回读并复用注册表校验。
	var defs []WidgetDefinition
	require.NoError(t, json.Unmarshal([]byte(exported.Widgets), &defs))
	require.Len(t, defs, 4)
	types := map[string]bool{}
	for i := range defs {
		require.NoError(t, ValidateWidgetDefinition(&defs[i]), "widget[%d] must pass registry validation", i)
		types[defs[i].Type] = true
	}
	require.True(t, types["gauge"])
	require.True(t, types["chart"])
	require.True(t, types["valve"])
	require.True(t, types["twin3d"])
}

func TestSeedBuiltinWidgetBundleIdempotency(t *testing.T) {
	setupWidgetBundleTestDB(t)
	ctx := context.Background()
	svc := &WidgetBundleService{}
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"}

	first, err := svc.SeedBuiltinWidgetBundle(ctx, claims)
	require.NoError(t, err)
	require.False(t, first.Idempotent)
	require.NotNil(t, first.Bundle)
	require.Equal(t, BuiltinWidgetBundleName, first.Bundle.Name)
	require.Equal(t, "tenant-1", first.Bundle.TenantID)

	// 内容一致 → 幂等命中，不重复建
	second, err := svc.SeedBuiltinWidgetBundle(ctx, claims)
	require.NoError(t, err)
	require.True(t, second.Idempotent)
	require.Equal(t, first.Bundle.ID, second.Bundle.ID)

	// 租户隔离：另一租户种子不受影响
	claimsB := &utils.UserClaims{ID: "user-2", TenantID: "tenant-2"}
	other, err := svc.SeedBuiltinWidgetBundle(ctx, claimsB)
	require.NoError(t, err)
	require.False(t, other.Idempotent)
	require.NotEqual(t, first.Bundle.ID, other.Bundle.ID)

	// 同名但内容不同 → fail closed，绝不静默覆盖用户手工 bundle
	conflict := &model.CreateWidgetBundleReq{Name: BuiltinWidgetBundleName}
	_, err = svc.CreateWidgetBundle(ctx, conflict, &utils.UserClaims{ID: "user-3", TenantID: "tenant-3"})
	require.NoError(t, err)
	_, err = svc.SeedBuiltinWidgetBundle(ctx, &utils.UserClaims{ID: "user-3", TenantID: "tenant-3"})
	require.Error(t, err)
}

func TestWidgetBundleCRUDAndTenantIsolation(t *testing.T) {
	setupWidgetBundleTestDB(t)
	ctx := context.Background()
	svc := &WidgetBundleService{}
	claimsA := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a"}
	claimsB := &utils.UserClaims{ID: "user-b", TenantID: "tenant-b"}

	widgets := "[" + validGaugeDef + "]"
	created, err := svc.CreateWidgetBundle(ctx, &model.CreateWidgetBundleReq{
		Name:    "工业仪表盘部件",
		Widgets: &widgets,
		Version: strPtrForWidgetTest("2.0.0"),
	}, claimsA)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", created.Version)
	require.Equal(t, widgets, created.Widgets)

	// 同租户重名拒绝
	_, err = svc.CreateWidgetBundle(ctx, &model.CreateWidgetBundleReq{Name: "工业仪表盘部件"}, claimsA)
	require.Error(t, err)

	// 非法 widgets 拒绝
	badWidgets := `[{"type":"","version":"1","schema":"{}","capabilities":["2d"]}]`
	_, err = svc.CreateWidgetBundle(ctx, &model.CreateWidgetBundleReq{Name: "坏定义", Widgets: &badWidgets}, claimsA)
	require.Error(t, err)

	// 租户 B 查询/更新/删除租户 A 的 bundle 一律拒绝
	_, err = svc.GetWidgetBundleByID(ctx, created.ID, claimsB)
	require.Error(t, err)
	_, err = svc.UpdateWidgetBundle(ctx, &model.UpdateWidgetBundleReq{ID: created.ID, Name: strPtrForWidgetTest("劫持")}, claimsB)
	require.Error(t, err)
	err = svc.DeleteWidgetBundle(ctx, created.ID, claimsB)
	require.Error(t, err)
	// 且未被删掉
	still, err := svc.GetWidgetBundleByID(ctx, created.ID, claimsA)
	require.NoError(t, err)
	require.NotNil(t, still)

	// 更新走异名去重 + 内容更新
	updated, err := svc.UpdateWidgetBundle(ctx, &model.UpdateWidgetBundleReq{
		ID:      created.ID,
		Widgets: &badWidgets,
	}, claimsA)
	require.Error(t, err) // 非法 widgets 更新被拒
	require.Nil(t, updated)

	okWidgets := "[]"
	updated, err = svc.UpdateWidgetBundle(ctx, &model.UpdateWidgetBundleReq{
		ID:      created.ID,
		Widgets: &okWidgets,
		Version: strPtrForWidgetTest("2.1.0"),
	}, claimsA)
	require.NoError(t, err)
	require.Equal(t, "[]", updated.Widgets)
	require.Equal(t, "2.1.0", updated.Version)

	// 列表租户隔离：B 看不到 A 的 bundle
	listA, err := svc.ListWidgetBundles(ctx, &model.GetWidgetBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}, claimsA)
	require.NoError(t, err)
	require.EqualValues(t, 1, listA["total"])
	listB, err := svc.ListWidgetBundles(ctx, &model.GetWidgetBundleListReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}, claimsB)
	require.NoError(t, err)
	require.EqualValues(t, 0, listB["total"])

	// 删除后不可再查
	require.NoError(t, svc.DeleteWidgetBundle(ctx, created.ID, claimsA))
	_, err = svc.GetWidgetBundleByID(ctx, created.ID, claimsA)
	require.Error(t, err)
}

func TestImportWidgetBundleWithTenantIdempotent(t *testing.T) {
	setupWidgetBundleTestDB(t)
	svc := &WidgetBundleService{}
	const tenantID = "tenant-import"
	widgets := "[" + validGaugeDef + "]"

	payload := model.WidgetBundleExport{
		Kind:    "aetherlink-widget-bundle",
		Name:    "市场部件包",
		Version: strPtrForWidgetTest("1.0.0"),
		Widgets: widgets,
	}
	saved, created, err := svc.ImportWidgetBundleWithTenant(payload, tenantID)
	require.NoError(t, err)
	require.True(t, created)

	// 同名同版本同内容 → 幂等
	again := payload
	saved2, created2, err := svc.ImportWidgetBundleWithTenant(again, tenantID)
	require.NoError(t, err)
	require.False(t, created2)
	require.Equal(t, saved.ID, saved2.ID)

	// 同名异版本 → 覆盖更新（不新建）
	updated := payload
	updated.Version = strPtrForWidgetTest("1.1.0")
	saved3, created3, err := svc.ImportWidgetBundleWithTenant(updated, tenantID)
	require.NoError(t, err)
	require.False(t, created3)
	require.Equal(t, saved.ID, saved3.ID)
	require.Equal(t, "1.1.0", saved3.Version)

	// 非法部件内容整条拒绝，不落半截 bundle
	broken := payload
	broken.Name = "坏包"
	broken.Version = strPtrForWidgetTest("1.0.0")
	broken.Widgets = `{"type":"gauge"}`
	_, _, err = svc.ImportWidgetBundleWithTenant(broken, tenantID)
	require.Error(t, err)

	// 空租户 fail closed
	_, _, err = svc.ImportWidgetBundleWithTenant(payload, "")
	require.Error(t, err)
}

func TestCheckMarketBundleDependenciesWidgets(t *testing.T) {
	widgetA := &model.WidgetBundleExport{Name: " 部件A ", Version: strPtrForWidgetTest("1.0.0")}
	widgetB := &model.WidgetBundleExport{Name: "部件A", Version: strPtrForWidgetTest("2.0.0")} // 与 A 去空格后重名

	bundle := &model.MarketBundle{
		TypeKey:    "automation",
		ExportedAt: 1,
		Count:      2,
		Widgets:    []*model.WidgetBundleExport{widgetA, widgetB},
	}
	issues := CheckMarketBundleDependencies(bundle)
	require.NotEmpty(t, issues)
	joined := strings.Join(issues, ";")
	require.Contains(t, joined, "duplicate widget bundle name")

	// count 与三资源总和不符
	bundle2 := &model.MarketBundle{
		TypeKey:    "automation",
		ExportedAt: 1,
		Count:      3,
		Widgets:    []*model.WidgetBundleExport{{Name: "部件B"}},
	}
	issues2 := CheckMarketBundleDependencies(bundle2)
	require.NotEmpty(t, issues2)
	require.Contains(t, strings.Join(issues2, ";"), "does not match")

	// type_key 不自洽
	mismatch := "industry"
	bundle3 := &model.MarketBundle{
		TypeKey:    "automation",
		ExportedAt: 1,
		Count:      1,
		Widgets:    []*model.WidgetBundleExport{{Name: "部件C", TypeKey: &mismatch}},
	}
	issues3 := CheckMarketBundleDependencies(bundle3)
	require.NotEmpty(t, issues3)
	require.Contains(t, strings.Join(issues3, ";"), "does not match bundle type_key")
}

func TestAppendWidgetBundlePreview(t *testing.T) {
	widgets := []*model.WidgetBundleExport{
		{Name: "新建部件", Version: strPtrForWidgetTest("1.0.0")},
		{Name: "覆盖部件", Version: strPtrForWidgetTest("2.0.0")},
		{Name: "幂等部件", Version: strPtrForWidgetTest("1.0.0")},
	}
	existing := map[string]string{
		"覆盖部件": "1.0.0", // 异版本 → 覆盖
		"幂等部件": "1.0.0", // 同版本 → 幂等跳过
	}

	preview := &model.MarketBundleImportPreview{
		Create:    []string{},
		Overwrite: []string{},
		Blocking:  []string{},
	}
	AppendWidgetBundlePreview(preview, widgets, existing)

	require.Equal(t, []string{"新建部件"}, preview.WidgetCreate)
	require.Equal(t, []string{"覆盖部件"}, preview.WidgetOverwrite)
	require.Contains(t, preview.Create, "新建部件")
	require.Contains(t, preview.Overwrite, "覆盖部件")
	require.Equal(t, 2, preview.Total)

	// 空 widgets 不动预览
	preview2 := &model.MarketBundleImportPreview{Total: 7}
	AppendWidgetBundlePreview(preview2, nil, existing)
	require.Equal(t, 7, preview2.Total)
}

func TestMarketBundleCanonicalCoversWidgets(t *testing.T) {
	// Widgets 必须进入签名规范 JSON：改动部件内容后摘要必须变化。
	signed := &model.MarketBundle{
		TypeKey:    "k",
		ExportedAt: 1,
		Count:      1,
		Widgets:    []*model.WidgetBundleExport{{Name: "部件", Widgets: "[]"}},
	}
	digest1, err := ComputeMarketBundleDigest(signed)
	require.NoError(t, err)
	signed.Widgets[0].Widgets = `[{"type":"gauge"}]`
	digest2, err := ComputeMarketBundleDigest(signed)
	require.NoError(t, err)
	require.NotEqual(t, digest1, digest2)
}

func strPtrForWidgetTest(s string) *string {
	out := s
	return &out
}
