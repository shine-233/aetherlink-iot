package dal

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestListLatestIDsByTypeKey 同名取最高版本、版本相同取最晚创建、nil 版本视为空串、
// 租户与 type_key 过滤生效。
func TestListLatestIDsByTypeKey(t *testing.T) {
	oldDB := global.DB
	t.Cleanup(func() { global.DB = oldDB })
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE res (id TEXT PRIMARY KEY, name TEXT, version TEXT NULL,
		created_at DATETIME, tenant_id TEXT, type_key TEXT)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	global.DB = db

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seed := []struct {
		id, name string
		version  any
		offset   int
		tenant   string
		typeKey  string
	}{
		{"a1", "A", "1.0", 0, "t1", "k"},
		{"a2", "A", "2.0", 1, "t1", "k"},
		{"a3", "A", "1.5", 2, "t1", "k"},
		{"b1", "B", "1.0", 0, "t1", "k"},
		{"b2", "B", "1.0", 5, "t1", "k"}, // 同版本，更晚创建
		{"c1", "C", nil, 0, "t1", "k"},
		{"c2", "C", "0.1", 1, "t1", "k"}, // 非空版本胜过 nil
		{"d1", "D", "9.0", 0, "t1", "other"},
		{"e1", "E", "9.0", 0, "t2", "k"},
	}
	for _, s := range seed {
		if err := db.Exec(`INSERT INTO res VALUES (?,?,?,?,?,?)`, s.id, s.name, s.version,
			base.Add(time.Duration(s.offset)*time.Hour), s.tenant, s.typeKey).Error; err != nil {
			t.Fatalf("seed %s: %v", s.id, err)
		}
	}

	check := func(typeKey string, want []string) {
		t.Helper()
		got, err := listLatestIDsByTypeKey(context.Background(), "res", "t1", typeKey)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("typeKey=%q got %v want %v", typeKey, got, want)
		}
	}
	check("k", []string{"a2", "b2", "c2"})
	check("", []string{"a2", "b2", "c2", "d1"})
}
