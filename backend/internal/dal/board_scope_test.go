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

func setupBoardScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open board sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Board{}); err != nil {
		t.Fatalf("migrate board: %v", err)
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

func TestGetBoardListByPageForScopesFiltersAcrossTenantScope(t *testing.T) {
	db := setupBoardScopeTestDB(t)
	now := time.Now().UTC()
	seed := func(id, tenant string) {
		board := model.Board{
			ID:        id,
			Name:      "Board " + id,
			TenantID:  tenant,
			CreatedAt: now,
			UpdatedAt: now,
			HomeFlag:  "N",
		}
		if err := db.Create(&board).Error; err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	seed("b-t1-a", "tenant-1")
	seed("b-t1-b", "tenant-1")
	seed("b-t2-a", "tenant-2")
	seed("b-t3-a", "tenant-3")

	req := &model.GetBoardListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}
	idsOf := func(list interface{}) map[string]bool {
		m := map[string]bool{}
		if rows, ok := list.([]*model.Board); ok {
			for _, r := range rows {
				m[r.ID] = true
			}
		}
		return m
	}

	// 单作用域与旧行为等价。
	count, list, err := GetBoardListByPageForScopes(req, []string{"tenant-1"})
	if err != nil {
		t.Fatalf("single scope err: %v", err)
	}
	got := idsOf(list)
	if count != 2 || !got["b-t1-a"] || got["b-t2-a"] {
		t.Fatalf("single scope got count=%d ids=%#v", count, got)
	}

	// 级联作用域 {tenant-3, tenant-1}：包含 self+祖先，隔离 tenant-2。
	count, list, err = GetBoardListByPageForScopes(req, []string{"tenant-3", "tenant-1"})
	if err != nil {
		t.Fatalf("scoped err: %v", err)
	}
	got = idsOf(list)
	if count != 3 || !got["b-t1-a"] || !got["b-t3-a"] || got["b-t2-a"] {
		t.Fatalf("scoped {t3,t1} got count=%d ids=%#v", count, got)
	}

	// nil 作用域 = 管理员全量（与旧空 tenantId 语义一致）。
	count, list, err = GetBoardListByPageForScopes(req, nil)
	if err != nil || count != 4 {
		t.Fatalf("nil scopes should mean all, got count=%d err=%v", count, err)
	}
}
