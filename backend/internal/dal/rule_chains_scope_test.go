// 文件用途：验证规则链列表读路径的 tenant scopes 三态契约（ROADMAP C2 自上而下）：
// 0→fail-closed 空结果、1→tenant_id =（与旧单租户等价）、>1→tenant_id IN（self∪子孙）；
// 含空租户 [""] 平台链、名称过滤与作用域组合用例。
package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestListRuleChainsByTenantScopes(t *testing.T) {
	db := setupRuleChainScopeTestDB(t)
	now := time.Now().UTC()
	hqTenant := "tenant-hq"
	childTenant := "tenant-child"
	foreignTenant := "tenant-x"
	platformTenant := ""
	chains := []model.RuleChain{
		{ID: "rc-hq-1", TenantID: hqTenant, Name: "hq chain 1", Enabled: true, Graph: []byte("{}"), CreatedAt: &now, UpdatedAt: &now},
		{ID: "rc-hq-2", TenantID: hqTenant, Name: "hq chain 2", Enabled: true, Graph: []byte("{}"), CreatedAt: &now, UpdatedAt: &now},
		{ID: "rc-child", TenantID: childTenant, Name: "child chain", Enabled: true, Graph: []byte("{}"), CreatedAt: &now, UpdatedAt: &now},
		{ID: "rc-foreign", TenantID: foreignTenant, Name: "foreign chain", Enabled: true, Graph: []byte("{}"), CreatedAt: &now, UpdatedAt: &now},
		{ID: "rc-platform", TenantID: platformTenant, Name: "platform chain", Enabled: true, Graph: []byte("{}"), CreatedAt: &now, UpdatedAt: &now},
	}
	if err := db.Create(&chains).Error; err != nil {
		t.Fatalf("create rule chains: %v", err)
	}

	t.Run("parent scope returns self and descendants only", func(t *testing.T) {
		total, list, err := ListRuleChainsByTenant([]string{hqTenant, childTenant}, "", 1, 20)
		if err != nil {
			t.Fatalf("ListRuleChainsByTenant(): %v", err)
		}
		if total != 3 || len(list) != 3 {
			t.Fatalf("total = %d, rows = %#v, want 3 in-scope chains", total, list)
		}
		seen := map[string]bool{}
		for _, rc := range list {
			seen[rc.ID] = true
			if rc.TenantID != hqTenant && rc.TenantID != childTenant {
				t.Fatalf("row %q escaped scope with tenant %q", rc.ID, rc.TenantID)
			}
		}
		if !seen["rc-hq-1"] || !seen["rc-hq-2"] || !seen["rc-child"] {
			t.Fatalf("in-scope rows = %v, want rc-hq-1, rc-hq-2 and rc-child", seen)
		}
	})

	t.Run("single scope keeps legacy tenant filter", func(t *testing.T) {
		total, list, err := ListRuleChainsByTenant([]string{childTenant}, "", 1, 20)
		if err != nil {
			t.Fatalf("ListRuleChainsByTenant(): %v", err)
		}
		if total != 1 || len(list) != 1 || list[0].ID != "rc-child" {
			t.Fatalf("total = %d, rows = %#v, want only rc-child", total, list)
		}
	})

	t.Run("keyword filter combines with scopes", func(t *testing.T) {
		total, list, err := ListRuleChainsByTenant([]string{hqTenant, childTenant}, "hq", 1, 20)
		if err != nil {
			t.Fatalf("ListRuleChainsByTenant(): %v", err)
		}
		if total != 2 || len(list) != 2 {
			t.Fatalf("name filter total = %d, rows = %#v, want 2 hq chains", total, list)
		}
		for _, rc := range list {
			if rc.TenantID != hqTenant {
				t.Fatalf("out-of-scope row %q (tenant %q) leaked past scope", rc.ID, rc.TenantID)
			}
		}
	})

	t.Run("empty tenant scope maps platform rows", func(t *testing.T) {
		total, list, err := ListRuleChainsByTenant([]string{""}, "", 1, 20)
		if err != nil {
			t.Fatalf("ListRuleChainsByTenant(): %v", err)
		}
		if total != 1 || len(list) != 1 || list[0].ID != "rc-platform" {
			t.Fatalf("total = %d, rows = %#v, want only rc-platform", total, list)
		}
	})

	t.Run("nil and empty scopes fail closed", func(t *testing.T) {
		for _, scopes := range [][]string{nil, []string{}} {
			total, list, err := ListRuleChainsByTenant(scopes, "", 1, 20)
			if err != nil {
				t.Fatalf("ListRuleChainsByTenant(scopes=%v): %v", scopes, err)
			}
			if total != 0 || list == nil || len(list) != 0 {
				t.Fatalf("fail-closed result = (%d, %#v, %v), want (0, non-nil empty, nil)", total, list, err)
			}
		}
	})
}

func setupRuleChainScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open rule chain scope sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.RuleChain{}); err != nil {
		t.Fatalf("migrate rule chain scope table: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}
