// 文件用途: 覆盖计算字段服务层校验——output_key 正则、govaluate 表达式解析提示、不存在映射 100404。
// 核心逻辑: 纯校验函数直测 + sqlite 内存库驱动 Create/Update/Toggle/Delete 的作用域行为。
// 关键注意事项: 断言错误码使用 errcode 常量,message 契约含 "calculated field not found"。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCalculatedFieldServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "calcfield_service_" + t.Name()
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
	if err := db.AutoMigrate(&model.CalculatedField{}, &model.DeviceTemplate{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func seedTemplateForScope(t *testing.T, db *gorm.DB, id, tenantID string) {
	t.Helper()
	template := &model.DeviceTemplate{ID: id, Name: "tpl", TenantID: tenantID}
	if err := db.Create(template).Error; err != nil {
		t.Fatalf("seed template %s: %v", id, err)
	}
}

func tenantClaims() *utils.UserClaims {
	return &utils.UserClaims{ID: "user-1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
}

func TestValidateCalculatedFieldValue(t *testing.T) {
	cases := []struct {
		name      string
		outputKey string
		expr      string
		wantErr   bool
	}{
		{name: "valid arithmetic", outputKey: "power_w", expr: "(voltage * current) / 1000"},
		{name: "valid boolean", outputKey: "is_hot", expr: "temperature > 80"},
		{name: "underscore leading key rejected", outputKey: "_power", expr: "a + b", wantErr: true},
		{name: "digit leading key rejected", outputKey: "1power", expr: "a + b", wantErr: true},
		{name: "dash rejected", outputKey: "power-w", expr: "a + b", wantErr: true},
		{name: "empty expression rejected", outputKey: "power_w", expr: "   ", wantErr: true},
		{name: "broken expression rejected", outputKey: "power_w", expr: "voltage * * current", wantErr: true},
	}
	for _, tc := range cases {
		err := validateCalculatedFieldValue(tc.outputKey, tc.expr)
		if tc.wantErr && err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		}
		if tc.wantErr && err != nil {
			if codeErr, ok := err.(*errcode.Error); !ok || codeErr.Code != errcode.CodeParamError {
				t.Fatalf("%s: want CodeParamError, got %#v", tc.name, err)
			}
		}
	}
}

func TestBrokenExpressionMessageHintsReferencedVariables(t *testing.T) {
	err := validateCalculatedFieldValue("power_w", "voltage * * current")
	codeErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("want errcode.Error, got %#v", err)
	}
	if codeErr.Code != errcode.CodeParamError {
		t.Fatalf("code = %d, want %d", codeErr.Code, errcode.CodeParamError)
	}
	for _, variable := range []string{"voltage", "current"} {
		if !containsSubstring(codeErr.CustomMsg, variable) {
			t.Fatalf("message must hint referenced variable %q, got %q", variable, codeErr.CustomMsg)
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestCreateValidatesExpressionAndTemplateOwnership(t *testing.T) {
	db := setupCalculatedFieldServiceTestDB(t)
	seedTemplateForScope(t, db, "tpl-1", "tenant-a")
	svc := &CalculatedFieldService{}

	enabled := true
	created, err := svc.CreateCalculatedField(&model.CalculatedFieldCreateReq{
		Name:             "power",
		DeviceTemplateID: "tpl-1",
		OutputKey:        "power_w",
		Expression:       "voltage * current",
		Enabled:          &enabled,
	}, tenantClaims())
	if err != nil {
		t.Fatalf("create calculated field: %v", err)
	}
	if created.ID == "" || created.TenantID != "tenant-a" || !created.Enabled {
		t.Fatalf("unexpected created row %#v", created)
	}

	if _, err := svc.CreateCalculatedField(&model.CalculatedFieldCreateReq{
		Name:             "bad",
		DeviceTemplateID: "tpl-1",
		OutputKey:        "bad_key",
		Expression:       "voltage * ",
	}, tenantClaims()); err == nil {
		t.Fatal("invalid expression must fail creation")
	}

	if _, err := svc.CreateCalculatedField(&model.CalculatedFieldCreateReq{
		Name:             "foreign",
		DeviceTemplateID: "tpl-other-tenant",
		OutputKey:        "power_w",
		Expression:       "voltage * current",
	}, tenantClaims()); err == nil {
		t.Fatal("template outside tenant must fail creation")
	}

	if _, err := svc.CreateCalculatedField(&model.CalculatedFieldCreateReq{
		Name:             "no-claims",
		DeviceTemplateID: "tpl-1",
		OutputKey:        "power_w",
		Expression:       "voltage * current",
	}, nil); err == nil {
		t.Fatal("missing claims must be rejected")
	}
}

func TestUpdateToggleDeleteMapMissingToNotFound(t *testing.T) {
	db := setupCalculatedFieldServiceTestDB(t)
	seedTemplateForScope(t, db, "tpl-1", "tenant-a")
	svc := &CalculatedFieldService{}
	claims := tenantClaims()

	created, err := svc.CreateCalculatedField(&model.CalculatedFieldCreateReq{
		Name:             "power",
		DeviceTemplateID: "tpl-1",
		OutputKey:        "power_w",
		Expression:       "voltage * current",
	}, claims)
	if err != nil {
		t.Fatalf("create calculated field: %v", err)
	}

	updated, err := svc.UpdateCalculatedField(created.ID, &model.CalculatedFieldUpdateReq{
		Name:             "power-v2",
		DeviceTemplateID: "tpl-1",
		OutputKey:        "power_kw",
		Expression:       "(voltage * current) / 1000",
	}, claims)
	if err != nil {
		t.Fatalf("update calculated field: %v", err)
	}
	if updated.OutputKey != "power_kw" {
		t.Fatalf("update not applied, output_key=%s", updated.OutputKey)
	}

	toggled, err := svc.ToggleCalculatedField(created.ID, &model.CalculatedFieldToggleReq{}, claims)
	if err != nil {
		t.Fatalf("toggle without explicit state should flip: %v", err)
	}
	if !toggled.Enabled {
		t.Fatal("flip from default-disabled must enable the field")
	}
	explicit := true
	toggled, err = svc.ToggleCalculatedField(created.ID, &model.CalculatedFieldToggleReq{Enabled: &explicit}, claims)
	if err != nil || !toggled.Enabled {
		t.Fatalf("explicit toggle failed: err=%v enabled=%v", err, toggled != nil && toggled.Enabled)
	}

	if err := svc.DeleteCalculatedField(created.ID, claims); err != nil {
		t.Fatalf("delete calculated field: %v", err)
	}
	if err := svc.DeleteCalculatedField(created.ID, claims); err == nil {
		t.Fatal("second delete must fail")
	} else if codeErr, ok := err.(*errcode.Error); ok {
		if codeErr.Code != errcode.CodeNotFound {
			t.Fatalf("delete missing code=%d, want %d", codeErr.Code, errcode.CodeNotFound)
		}
		if !containsSubstring(codeErr.CustomMsg, "calculated field not found") {
			t.Fatalf("not-found message contract broken: %q", codeErr.CustomMsg)
		}
	} else {
		t.Fatalf("delete missing must return errcode.Error, got %#v", err)
	}

	if _, err := svc.UpdateCalculatedField("fake-id", &model.CalculatedFieldUpdateReq{
		Name: "x", DeviceTemplateID: "tpl-1", OutputKey: "k", Expression: "a + 1",
	}, claims); err == nil {
		t.Fatal("update on fake-id must report calculated field not found")
	}
	if _, err := svc.GetCalculatedField("fake-id", claims); err == nil {
		t.Fatal("get on fake-id must report calculated field not found")
	}
}
