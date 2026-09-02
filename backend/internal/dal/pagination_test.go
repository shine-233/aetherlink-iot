// 文件用途：验证统一分页收敛 helper 的行为契约。
// 核心逻辑：以 sqlite 内存库 + DryRun 断言生成 SQL 的 LIMIT/OFFSET 形状，
//   覆盖有效分页、超上限 clamp、缺省兜底三条路径。
// 关键注意事项：本测试锁定的是"查询必有界"这一安全属性，不锁定具体 SQL 文案。

package dal

import (
	"strconv"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPaginationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func paginationSQL(t *testing.T, db *gorm.DB) string {
	t.Helper()
	dryRun := db.Session(&gorm.Session{DryRun: true})
	q := applyListPagination(dryRun.Model(&modelUserRow{}), pageArg, sizeArg)
	// DryRun 下只有调用 finisher（Find/Scan 等）才会真正构建 SQL。
	res := q.Find(&[]modelUserRow{})
	if res.Statement == nil {
		t.Fatal("dry-run statement is nil")
	}
	return res.Statement.SQL.String()
}

type modelUserRow struct {
	ID   string
	Name string
}

var (
	pageArg int
	sizeArg int
)

func TestClampListPageSize(t *testing.T) {
	if got := clampListPageSize(maxListLimit); got != maxListLimit {
		t.Fatalf("clamp at boundary = %d, want %d", got, maxListLimit)
	}
	if got := clampListPageSize(maxListLimit * 10); got != maxListLimit {
		t.Fatalf("oversized page size = %d, want %d", got, maxListLimit)
	}
	if got := clampListPageSize(10); got != 10 {
		t.Fatalf("normal page size changed: %d", got)
	}
}

func TestApplyListPaginationValidPage(t *testing.T) {
	pageArg, sizeArg = 3, 50
	defer func() { pageArg, sizeArg = 0, 0 }()
	sql := strings.ToUpper(paginationSQL(t, newPaginationTestDB(t)))
	if !strings.Contains(sql, "LIMIT 50") || !strings.Contains(sql, "OFFSET 100") {
		t.Fatalf("expected LIMIT 50 OFFSET 100, got: %s", sql)
	}
}

func TestApplyListPaginationClampsOversizedPage(t *testing.T) {
	pageArg, sizeArg = 1, maxListLimit*4
	defer func() { pageArg, sizeArg = 0, 0 }()
	sql := strings.ToUpper(paginationSQL(t, newPaginationTestDB(t)))
	if !strings.Contains(sql, "LIMIT "+itoa(maxListLimit)) {
		t.Fatalf("expected clamped LIMIT %d, got: %s", maxListLimit, sql)
	}
}

func TestApplyListPaginationDefaultsToBoundedWhenMissing(t *testing.T) {
	pageArg, sizeArg = 0, 0
	sql := strings.ToUpper(paginationSQL(t, newPaginationTestDB(t)))
	if !strings.Contains(sql, "LIMIT "+itoa(defaultListLimit)) {
		t.Fatalf("expected default LIMIT %d instead of unbounded scan, got: %s", defaultListLimit, sql)
	}
	if strings.Contains(sql, "OFFSET") {
		t.Fatalf("default path should not carry OFFSET, got: %s", sql)
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
