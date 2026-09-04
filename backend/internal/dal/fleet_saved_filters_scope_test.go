// 文件用途：锁定 fleet_saved_filters 列表读路径的 C2 自上而下作用域真实结果集。
// 核心逻辑：sqlite 内存库种子多租户筛选器，验证 scopes 三态（0 fail-closed、
// 1 等价旧单租户、>1 IN）与 user 维度的私有边界：作用域展开只扩大租户范围，
// 同租户/子树其他成员的私有行（shared=false 且非本人）即使落在作用域内也不可见。
// 关键注意事项：本文件只测管理读路径 ListFleetSavedFiltersVisibleToUser；配额
// CountFleetSavedFiltersOwnedByUser 与写守卫 GetFleetSavedFilterInTenant 保持 strict
// 同租户，语义不变，不在此展开。
package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupFleetSavedFilterScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.FleetSavedFilter{}); err != nil {
		t.Fatalf("migrate fleet_saved_filters: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func seedFleetSavedFilter(t *testing.T, db *gorm.DB, id, tenantID, userID string, shared bool) {
	t.Helper()
	now := time.Now().UTC()
	filter := model.FleetSavedFilter{
		ID:           id,
		TenantID:     tenantID,
		UserID:       userID,
		Name:         id,
		DeviceFilter: `{"is_online":1}`,
		Shared:       shared,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&filter).Error; err != nil {
		t.Fatalf("seed saved filter %s: %v", id, err)
	}
}

func fleetSavedFilterIDs(list []*model.FleetSavedFilter) map[string]bool {
	seen := map[string]bool{}
	for _, f := range list {
		if f != nil {
			seen[f.ID] = true
		}
	}
	return seen
}

func assertFleetSavedFilterSubset(t *testing.T, list []*model.FleetSavedFilter, want []string) map[string]bool {
	t.Helper()
	seen := fleetSavedFilterIDs(list)
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("missing %s; got %#v", id, seen)
		}
	}
	return seen
}

// TestListFleetSavedFiltersScopeDown 自上而下作用域真实结果集：
// [hq, child] 可见 hq+child 两租户的本人/共享行；作用域展开绝不把其他成员
// 的私有行带进来；[hq] 等价旧单租户；空作用域 fail-closed。
func TestListFleetSavedFiltersScopeDown(t *testing.T) {
	db := setupFleetSavedFilterScopeTestDB(t)
	// hq（总部）内：hq-admin 本人一条私有、本人一条共享；hq-member 一条共享、一条私有。
	seedFleetSavedFilter(t, db, "hq-own-private", "hq", "hq-admin", false)
	seedFleetSavedFilter(t, db, "hq-own-shared", "hq", "hq-admin", true)
	seedFleetSavedFilter(t, db, "hq-member-shared", "hq", "hq-member", true)
	seedFleetSavedFilter(t, db, "hq-member-private", "hq", "hq-member", false)
	// child（子孙租户）内：child 成员一条共享、一条私有。
	seedFleetSavedFilter(t, db, "child-shared", "child", "child-user", true)
	seedFleetSavedFilter(t, db, "child-private", "child", "child-user", false)
	// tenant-x（无关租户）内共享行，任何合法作用域都不该可见。
	seedFleetSavedFilter(t, db, "x-shared", "tenant-x", "x-user", true)

	// hq-admin 自上而下 [hq, child]：本人全部 + 同租户共享 + child 租户共享可见；
	// hq-member 私有、child 私有、tenant-x 一律不可见。
	list, err := ListFleetSavedFiltersVisibleToUser([]string{"hq", "child"}, "hq-admin", 200)
	if err != nil {
		t.Fatalf("scoped list [hq child]: %v", err)
	}
	seen := assertFleetSavedFilterSubset(t, list, []string{
		"hq-own-private", "hq-own-shared", "hq-member-shared", "child-shared",
	})
	for _, leaked := range []string{"hq-member-private", "child-private", "x-shared"} {
		if seen[leaked] {
			t.Fatalf("scope [hq child] leaked private/out-of-scope row %s: %#v", leaked, seen)
		}
	}

	// child 租户用户只看自身：单元素作用域等价旧单租户。
	list, err = ListFleetSavedFiltersVisibleToUser([]string{"child"}, "child-user", 200)
	if err != nil {
		t.Fatalf("scoped list [child]: %v", err)
	}
	seen = assertFleetSavedFilterSubset(t, list, []string{"child-shared", "child-private"})
	if seen["hq-own-shared"] || seen["x-shared"] {
		t.Fatalf("scope [child] leaked parent/other tenant rows: %#v", seen)
	}

	// 空作用域 fail-closed：不返回任何租户数据。
	list, err = ListFleetSavedFiltersVisibleToUser(nil, "hq-admin", 200)
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("empty scope returned %d rows: %#v", len(list), fleetSavedFilterIDs(list))
	}
}

// TestListFleetSavedFiltersPlatformScope 平台空租户作用域：SYS_ADMIN 以 [""] 管理
// tenant_id 为空串的行（service 层映射），仅该类行可见。
func TestListFleetSavedFiltersPlatformScope(t *testing.T) {
	db := setupFleetSavedFilterScopeTestDB(t)
	seedFleetSavedFilter(t, db, "sys-own", "", "sys-admin", false)
	seedFleetSavedFilter(t, db, "sys-shared", "", "sys-member", true)
	seedFleetSavedFilter(t, db, "hq-row", "hq", "hq-admin", true)

	list, err := ListFleetSavedFiltersVisibleToUser([]string{""}, "sys-admin", 200)
	if err != nil {
		t.Fatalf("platform scope: %v", err)
	}
	// 平台空租户内本人行 + 同租户共享行可见；hq 行因作用域不含而不见。
	seen := assertFleetSavedFilterSubset(t, list, []string{"sys-own", "sys-shared"})
	if seen["hq-row"] {
		t.Fatalf("platform scope [\"\"] leaked tenant rows: %#v", seen)
	}
}
