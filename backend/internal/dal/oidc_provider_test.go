package dal

import (
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOidcDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open oidc sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OidcProvider{}); err != nil {
		t.Fatalf("migrate oidc: %v", err)
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

func seedOidcProvider(t *testing.T, p *model.OidcProvider) {
	t.Helper()
	if err := CreateOidcProvider(p); err != nil {
		t.Fatalf("seed oidc provider: %v", err)
	}
}

func TestOidcProviderCRUDAndTenantScope(t *testing.T) {
	setupOidcDB(t)
	seedOidcProvider(t, &model.OidcProvider{ID: "p1", TenantID: "t1", Name: "GitHub", Issuer: "https://github.com", ClientID: "cid", Enabled: true})
	seedOidcProvider(t, &model.OidcProvider{ID: "p2", TenantID: "t2", Name: "GitLab", Issuer: "https://gitlab.com", ClientID: "cid2", Enabled: true})
	seedOidcProvider(t, &model.OidcProvider{ID: "p3", TenantID: "t1", Name: "Disabled", Issuer: "https://x.example", ClientID: "cid3", Enabled: false})

	// 租户隔离读取
	if _, err := GetOidcProviderOwned("p2", "t1"); err != gorm.ErrRecordNotFound {
		t.Fatalf("cross-tenant owned read must fail, got %v", err)
	}
	got, err := GetOidcProviderOwned("p1", "t1")
	if err != nil || got.Name != "GitHub" {
		t.Fatalf("owned read p1 err=%v", err)
	}
	// 列表（含 disabled）
	list, err := ListOidcProvidersByTenant("t1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list t1 len=%d err=%v", len(list), err)
	}
	// 公开入口：disabled 不可见
	if _, err := GetOidcProviderByID("p3"); err != gorm.ErrRecordNotFound {
		t.Fatalf("disabled provider must be hidden from public sso, got %v", err)
	}
	pub, err := GetOidcProviderByID("p1")
	if err != nil || pub.ID != "p1" {
		t.Fatalf("public read err=%v", err)
	}
	// 更新（租户维度守卫）
	ok, err := UpdateOidcProvider(&model.OidcProvider{ID: "p2", TenantID: "t1", Name: "hack", ClientID: "x"})
	if err != nil || ok {
		t.Fatalf("cross-tenant update must not match ok=%v err=%v", ok, err)
	}
	// 删除
	if err := DeleteOidcProvider("p2", "t2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := GetOidcProviderOwned("p2", "t2"); err != gorm.ErrRecordNotFound {
		t.Fatalf("deleted provider should be gone, got %v", err)
	}
}
