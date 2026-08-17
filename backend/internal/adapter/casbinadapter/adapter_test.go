package casbinadapter

import (
	"errors"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const testModel = `
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
g2 = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

func testDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&rule{}); err != nil {
		t.Fatalf("migrate test table: %v", err)
	}
	return db
}

func testEnforcer(t *testing.T, adapter *Adapter) *casbin.Enforcer {
	t.Helper()
	casbinModel, err := model.NewModelFromString(testModel)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	enforcer, err := casbin.NewEnforcer(casbinModel, adapter)
	if err != nil {
		t.Fatalf("create enforcer: %v", err)
	}
	return enforcer
}

func TestAdapterAutoSaveAndReload(t *testing.T) {
	adapter, err := New(testDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	enforcer := testEnforcer(t, adapter)
	if _, err := enforcer.AddPolicy("role-admin", "/devices", "allow"); err != nil {
		t.Fatal(err)
	}
	if _, err := enforcer.AddGroupingPolicy("user-1", "role-admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := enforcer.AddNamedGroupingPolicy("g2", "/devices", "device-resource"); err != nil {
		t.Fatal(err)
	}

	reloaded := testEnforcer(t, adapter)
	if err := reloaded.LoadPolicy(); err != nil {
		t.Fatal(err)
	}
	allowed, err := reloaded.Enforce("user-1", "/devices", "allow")
	if err != nil || !allowed {
		t.Fatalf("reloaded policy allowed=%v err=%v", allowed, err)
	}
	hasDevicePolicy, err := reloaded.HasNamedGroupingPolicy("g2", "/devices", "device-resource")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDevicePolicy {
		t.Fatal("g2 policy was not reloaded")
	}
}

func TestAdapterSaveAndRemovalOperations(t *testing.T) {
	db := testDatabase(t)
	adapter, _ := New(db)
	enforcer := testEnforcer(t, adapter)
	_, _ = enforcer.AddPolicies([][]string{{"role-a", "/a", "allow"}, {"role-b", "/b", "allow"}})
	_, _ = enforcer.AddGroupingPolicies([][]string{{"user-a", "role-a"}, {"user-b", "role-b"}})
	if err := enforcer.SavePolicy(); err != nil {
		t.Fatal(err)
	}

	if err := adapter.RemovePolicy("p", "p", []string{"role-a", "/a", "allow"}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RemovePolicies("g", "g", [][]string{{"user-a", "role-a"}, {"user-b", "role-b"}}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RemoveFilteredPolicy("p", "p", 0, "role-b", "", "allow"); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&rule{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("remaining rules=%d, want 0", count)
	}
}

func TestAdapterLoadsNullTrailingFields(t *testing.T) {
	db := testDatabase(t)
	ptype, first, second := "g", "user", "role"
	if err := db.Create(&rule{Ptype: &ptype, V0: &first, V1: &second}).Error; err != nil {
		t.Fatal(err)
	}
	adapter, _ := New(db)
	enforcer := testEnforcer(t, adapter)
	if err := enforcer.LoadPolicy(); err != nil {
		t.Fatal(err)
	}
	hasGroupingPolicy, err := enforcer.HasGroupingPolicy("user", "role")
	if err != nil {
		t.Fatal(err)
	}
	if !hasGroupingPolicy {
		t.Fatal("historical NULL trailing fields were not loaded")
	}
	if err := adapter.RemovePolicy("g", "g", []string{"user", "role"}); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterValidatesInputsAndRollsBackBatch(t *testing.T) {
	db := testDatabase(t)
	adapter, _ := New(db)
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) succeeded")
	}
	if err := adapter.AddPolicy("p", "p", make([]string, 7)); err == nil {
		t.Fatal("oversized policy succeeded")
	}
	if err := adapter.RemoveFilteredPolicy("p", "p", 5, "a", "b"); err == nil {
		t.Fatal("invalid filter range succeeded")
	}

	callbackName := "reject-second-casbin-row"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if rows, ok := tx.Statement.Dest.(*[]rule); ok && len(*rows) == 2 {
			tx.AddError(errors.New("injected batch failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })
	if err := adapter.AddPolicies("p", "p", [][]string{{"a", "/a", "allow"}, {"b", "/b", "allow"}}); err == nil {
		t.Fatal("injected batch failure was ignored")
	}
	var count int64
	if err := db.Model(&rule{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed batch persisted %d rows", count)
	}
}
