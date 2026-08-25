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
	total, list, err := ListCalculatedFieldsByPage("tenant-a", &model.CalculatedFieldListReq{
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
	total, _, err = ListCalculatedFieldsByPage("tenant-a", &model.CalculatedFieldListReq{
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
	_, list, err = ListCalculatedFieldsByPage("tenant-cap", &model.CalculatedFieldListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 5000},
	})
	if err != nil {
		t.Fatalf("capped page list: %v", err)
	}
	if len(list) != maxCalculatedFieldListLimit {
		t.Fatalf("oversized page returned %d rows, want capped at %d", len(list), maxCalculatedFieldListLimit)
	}

	nameFilter := "field-other"
	total, _, err = ListCalculatedFieldsByPage("tenant-b", &model.CalculatedFieldListReq{
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

	fields, err := ListEnabledCalculatedFieldsByTemplate("tpl-on")
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(fields) != 1 || fields[0].ID != "on-1" {
		t.Fatalf("enabled fields = %#v, want only on-1", fields)
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

	got, err := GetDeviceTemplateIDByDeviceID("dev-bound")
	if err != nil || got != "tpl-9" {
		t.Fatalf("template for dev-bound = %q err=%v, want tpl-9", got, err)
	}
	got, err = GetDeviceTemplateIDByDeviceID("dev-unbound")
	if err != nil || got != "" {
		t.Fatalf("template for dev-unbound = %q err=%v, want empty", got, err)
	}
	got, err = GetDeviceTemplateIDByDeviceID("missing-device")
	if err != nil || got != "" {
		t.Fatalf("missing device must resolve to empty without error, got %q err=%v", got, err)
	}
}

func ptrCalcString(value string) *string { return &value }
