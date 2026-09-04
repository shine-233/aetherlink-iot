// 文件用途: 覆盖 calculated_field DAL 的租户隔离 CRUD、分页封顶与设备→模板解析。
// 核心逻辑: sqlite 内存库 + AutoMigrate 构造隔离环境,断言 RowsAffected 守卫与作用域边界。
// 关键注意事项: 时间字段参数化写入;sqlite 不支持 timestamptz 默认值,统一由代码赋值。
package dal

import (
	"fmt"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCalculatedFieldTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "calcfield_dal_" + t.Name()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(
		&model.CalculatedField{},
		&model.Device{},
		&model.DeviceConfig{},
	); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func createCalculatedFieldRow(t *testing.T, id, tenantID, templateID, outputKey string, enabled bool) {
	t.Helper()
	now := time.Now().UTC()
	field := &model.CalculatedField{
		ID:               id,
		TenantID:         tenantID,
		Name:             "field-" + id,
		DeviceTemplateID: templateID,
		OutputKey:        outputKey,
		Expression:       "voltage * current",
		Enabled:          enabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := global.DB.Create(field).Error; err != nil {
		t.Fatalf("seed calculated field %s: %v", id, err)
	}
}

func TestCalculatedFieldCRUDRespectsTenantScope(t *testing.T) {
	db := setupCalculatedFieldTestDB(t)
	createCalculatedFieldRow(t, "cf-1", "tenant-a", "tpl-1", "power_w", false)
	createCalculatedFieldRow(t, "cf-2", "tenant-b", "tpl-1", "power_w", false)

	field, err := GetCalculatedFieldForScope("cf-1", "tenant-a")
	if err != nil {
		t.Fatalf("get scoped field: %v", err)
	}
	if field.OutputKey != "power_w" || field.Enabled {
		t.Fatalf("unexpected row %#v", field)
	}
	if _, err := GetCalculatedFieldForScope("cf-1", "tenant-b"); err != gorm.ErrRecordNotFound {
		t.Fatalf("cross-tenant read must miss with ErrRecordNotFound, got %v", err)
	}

	if err := UpdateCalculatedFieldForScope("cf-1", "tenant-b", map[string]interface{}{"enabled": true}); err != gorm.ErrRecordNotFound {
		t.Fatalf("cross-tenant update must be a guarded no-hit, got %v", err)
	}
	if err := UpdateCalculatedFieldForScope("cf-1", "tenant-a", map[string]interface{}{"enabled": true}); err != nil {
		t.Fatalf("scoped update: %v", err)
	}
	var reloaded model.CalculatedField
	if err := db.First(&reloaded, "id = ?", "cf-1").Error; err != nil || !reloaded.Enabled {
		t.Fatalf("update not applied: err=%v enabled=%v", err, reloaded.Enabled)
	}

	if err := DeleteCalculatedFieldForScope("cf-1", "tenant-a"); err != nil {
		t.Fatalf("scoped delete: %v", err)
	}
	if err := DeleteCalculatedFieldForScope("cf-1", "tenant-a"); err != gorm.ErrRecordNotFound {
		t.Fatalf("second delete must report not found, got %v", err)
	}
}

func TestListCalculatedFieldsByPageFiltersAndCaps(t *testing.T) {
	db := setupCalculatedFieldTestDB(t)
	for i := 0; i < 12; i++ {
		createCalculatedFieldRow(t, string(rune('a'+i)), "tenant-a", "tpl-1", "key_a", i%2 == 0)
	}
	createCalculatedFieldRow(t, "other", "tenant-b", "tpl-1", "key_b", true)

	templateFilter := "tpl-1"
	total, list, err := ListCalculatedFieldsByPage([]string{"tenant-a"}, &model.CalculatedFieldListReq{
		PageReq:          model.PageReq{Page: 1, PageSize: 5},
		DeviceTemplateID: &templateFilter,
	})
	if err != nil {
		t.Fatalf("paged list: %v", err)
	}
	if total != 12 || len(list) != 5 {
		t.Fatalf("total=%d rows=%d, want total=12 rows=5", total, len(list))
	}

	// pageSize 超上限/负数时必须收敛到具名上限而不是取消 LIMIT。
	total, _, err = ListCalculatedFieldsByPage([]string{"tenant-a"}, &model.CalculatedFieldListReq{
		PageReq: model.PageReq{Page: 1, PageSize: -7},
	})
	if err != nil {
		t.Fatalf("negative page size list: %v", err)
	}
	if total != 12 {
		t.Fatalf("negative page size total=%d, want 12", total)
	}

	// 超过单页上限的存量行会被 LIMIT 截断：505 行只返回具名上限内的 500 行。
	now := time.Now().UTC()
	rows := make([]*model.CalculatedField, 0, 505)
	for i := 0; i < 505; i++ {
		rows = append(rows, &model.CalculatedField{
			ID:               fmt.Sprintf("cf-cap-%03d", i),
			TenantID:         "tenant-cap",
			Name:             "cap",
			DeviceTemplateID: "tpl-cap",
			OutputKey:        "power_w",
			Expression:       "voltage * current",
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	if err := db.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed capped rows: %v", err)
	}
	_, list, err = ListCalculatedFieldsByPage([]string{"tenant-cap"}, &model.CalculatedFieldListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 5000},
	})
	if err != nil {
		t.Fatalf("capped page list: %v", err)
	}
	if len(list) != maxCalculatedFieldListLimit {
		t.Fatalf("oversized page returned %d rows, want capped at %d", len(list), maxCalculatedFieldListLimit)
	}

	nameFilter := "field-other"
	total, _, err = ListCalculatedFieldsByPage([]string{"tenant-b"}, &model.CalculatedFieldListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
		Name:    &nameFilter,
	})
	if err != nil {
		t.Fatalf("name filtered list: %v", err)
	}
	if total != 1 {
		t.Fatalf("name filter total=%d, want 1", total)
	}
}

func TestListEnabledCalculatedFieldsByTemplateOnlyReturnsEnabled(t *testing.T) {
	setupCalculatedFieldTestDB(t)
	createCalculatedFieldRow(t, "on-1", "tenant-a", "tpl-on", "power_w", true)
	createCalculatedFieldRow(t, "off-1", "tenant-a", "tpl-on", "energy_wh", false)
	// 异租户同模板 id 的启用规则不得泄漏进结果（tenant-scope 棘轮契约）。
	createCalculatedFieldRow(t, "on-foreign", "tenant-b", "tpl-on", "power_w_foreign", true)

	fields, err := ListEnabledCalculatedFieldsByTemplate("tenant-a", "tpl-on")
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(fields) != 1 || fields[0].ID != "on-1" {
		t.Fatalf("enabled fields = %#v, want only on-1", fields)
	}

	foreign, err := ListEnabledCalculatedFieldsByTemplate("tenant-b", "tpl-on")
	if err != nil {
		t.Fatalf("list enabled for foreign tenant: %v", err)
	}
	if len(foreign) != 1 || foreign[0].ID != "on-foreign" {
		t.Fatalf("foreign tenant fields = %#v, want only on-foreign", foreign)
	}
}

func TestGetDeviceTemplateIDByDeviceIDJoinsConfigs(t *testing.T) {
	db := setupCalculatedFieldTestDB(t)
	now := time.Now().UTC()
	boundConfigID := "config-1"
	blankConfigID := "config-blank"
	if err := db.Create(&model.DeviceConfig{
		ID: boundConfigID, Name: "config-1", DeviceType: "1", TenantID: "tenant-a",
		AutoRegister: 0, DeviceTemplateID: ptrCalcString("tpl-9"), CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed device config: %v", err)
	}
	if err := db.Create(&model.DeviceConfig{
		ID: blankConfigID, Name: "config-blank", DeviceType: "1", TenantID: "tenant-a",
		AutoRegister: 0, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed blank device config: %v", err)
	}

	devices := []*model.Device{
		{ID: "dev-bound", TenantID: "tenant-a", Voucher: "v-1", DeviceNumber: "n-1", ActivateFlag: "active", IsEnabled: "enabled", DeviceConfigID: &boundConfigID},
		{ID: "dev-unbound", TenantID: "tenant-a", Voucher: "v-2", DeviceNumber: "n-2", ActivateFlag: "active", IsEnabled: "enabled"},
		{ID: "dev-blank-config", TenantID: "tenant-a", Voucher: "v-3", DeviceNumber: "n-3", ActivateFlag: "active", IsEnabled: "enabled", DeviceConfigID: &blankConfigID},
	}
	for _, device := range devices {
		if err := db.Create(device).Error; err != nil {
			t.Fatalf("seed device %s: %v", device.ID, err)
		}
	}

	got, err := GetDeviceTemplateIDByDeviceID("tenant-a", "dev-bound")
	if err != nil || got != "tpl-9" {
		t.Fatalf("template for dev-bound = %q err=%v, want tpl-9", got, err)
	}
	got, err = GetDeviceTemplateIDByDeviceID("tenant-a", "dev-unbound")
	if err != nil || got != "" {
		t.Fatalf("template for dev-unbound = %q err=%v, want empty", got, err)
	}
	got, err = GetDeviceTemplateIDByDeviceID("tenant-a", "missing-device")
	if err != nil || got != "" {
		t.Fatalf("missing device must resolve to empty without error, got %q err=%v", got, err)
	}
}

// TestListCalculatedFieldsByPageScopeDown 自上而下作用域真实结果集：
// [hq, child] 含两租户、[hq] 只含本租户、空作用域 fail-closed、模板过滤与作用域叠加。
func TestListCalculatedFieldsByPageScopeDown(t *testing.T) {
	setupCalculatedFieldTestDB(t)
	createCalculatedFieldRow(t, "cf-hq-1", "hq", "tpl-hq", "power_w", true)
	createCalculatedFieldRow(t, "cf-child-1", "child", "tpl-child", "energy_wh", true)
	createCalculatedFieldRow(t, "cf-x-1", "tenant-x", "tpl-x", "voltage_v", false)

	templateFilter := "tpl-child"
	total, list, err := ListCalculatedFieldsByPage([]string{"hq", "child"}, &model.CalculatedFieldListReq{
		PageReq:          model.PageReq{Page: 1, PageSize: 10},
		DeviceTemplateID: &templateFilter,
	})
	if err != nil {
		t.Fatalf("scoped+template list: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "cf-child-1" {
		t.Fatalf("template-filtered scope [hq child] total=%d rows=%#v", total, list)
	}

	total, list, err = ListCalculatedFieldsByPage([]string{"hq", "child"}, nil)
	if err != nil {
		t.Fatalf("scope [hq child]: %v", err)
	}
	if total != 2 {
		t.Fatalf("scope [hq child] total=%d, want 2", total)
	}
	seen := map[string]bool{}
	for _, f := range list {
		seen[f.ID] = true
	}
	if !seen["cf-hq-1"] || !seen["cf-child-1"] || seen["cf-x-1"] {
		t.Fatalf("scope [hq child] leaked/missing rows: %#v", seen)
	}

	// 单元素作用域等价旧单租户：hq 看不到 child 与 tenant-x。
	total, list, err = ListCalculatedFieldsByPage([]string{"hq"}, nil)
	if err != nil {
		t.Fatalf("scope [hq]: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "cf-hq-1" {
		t.Fatalf("scope [hq] total=%d rows=%#v", total, list)
	}

	// 空作用域 fail-closed。
	total, list, err = ListCalculatedFieldsByPage(nil, nil)
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("empty scope total=%d rows=%d", total, len(list))
	}
}

// TestGetCalculatedFieldForScopesScopeDown 单条读作用域成员判定：父可读子、子不可读父、空作用域不存在。
func TestGetCalculatedFieldForScopesScopeDown(t *testing.T) {
	setupCalculatedFieldTestDB(t)
	createCalculatedFieldRow(t, "cf-hq-1", "hq", "tpl-hq", "power_w", true)
	createCalculatedFieldRow(t, "cf-child-1", "child", "tpl-child", "energy_wh", true)

	if _, err := GetCalculatedFieldForScopes("cf-child-1", []string{"hq", "child"}); err != nil {
		t.Fatalf("parent in-scope read child: %v", err)
	}
	if _, err := GetCalculatedFieldForScopes("cf-child-1", []string{"hq"}); err != gorm.ErrRecordNotFound {
		t.Fatalf("child out-of-scope read must miss, got %v", err)
	}
	if _, err := GetCalculatedFieldForScopes("cf-hq-1", []string{"hq"}); err != nil {
		t.Fatalf("single-scope own read: %v", err)
	}
	if _, err := GetCalculatedFieldForScopes("cf-hq-1", nil); err != gorm.ErrRecordNotFound {
		t.Fatalf("empty scope must fail closed as not found, got %v", err)
	}
	// 写路径守卫仍是严格单租户：hq 无法按 child 租户命中。
	if err := UpdateCalculatedFieldForScope("cf-child-1", "hq", map[string]interface{}{"enabled": false}); err != gorm.ErrRecordNotFound {
		t.Fatalf("write guard must stay strict-tenant, got %v", err)
	}
}

func ptrCalcString(value string) *string { return &value }
