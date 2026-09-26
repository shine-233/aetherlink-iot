// 文件用途：分页方言适配单测（TP-20）——LIMIT/OFFSET 直列与 KingBase ROWNUM 包裹的精确 SQL 断言、
// 参数校验边界与 1 基页码换算全量表驱动。
// 核心逻辑：Paginate 对 postgres/tdengine/polardb 产出相同的 "LIMIT n OFFSET m" 尾缀（TDengine 要求
// OFFSET 与 LIMIT 同现，本函数恒同现）；kingbase 产出标准 Oracle 双层 ROWNUM 包裹；非法输入 fail closed。
// 关键注意事项：断言是精确字符串相等（不是子串匹配）——分页 SQL 是契约，改格式必须显式改用例。
package dialect

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestPaginatePerDialect(t *testing.T) {
	const inner = "SELECT * FROM telemetry_datas WHERE device_id = ?"
	cases := []struct {
		name    string
		dialect Dialect
		limit   int
		offset  int
		want    string
	}{
		{"postgres", DialectPostgres, 20, 40, inner + " LIMIT 20 OFFSET 40"},
		{"tdengine", DialectTDengine, 20, 40, inner + " LIMIT 20 OFFSET 40"},
		{"polardb", DialectPolardb, 20, 40, inner + " LIMIT 20 OFFSET 40"},
		{"first-page", DialectPostgres, 20, 0, inner + " LIMIT 20 OFFSET 0"},
		{"kingbase-rownum", DialectKingbase, 20, 40,
			"SELECT * FROM (SELECT page_src.*, ROWNUM AS page_rn FROM (" + inner + ") page_src WHERE ROWNUM <= 60) WHERE page_rn > 40"},
		{"kingbase-first-page", DialectKingbase, 50, 0,
			"SELECT * FROM (SELECT page_src.*, ROWNUM AS page_rn FROM (" + inner + ") page_src WHERE ROWNUM <= 50) WHERE page_rn > 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Paginate(tc.dialect, inner, tc.limit, tc.offset)
			if err != nil {
				t.Fatalf("Paginate(%q,%q,%d,%d) error = %v, want nil", tc.dialect, inner, tc.limit, tc.offset, err)
			}
			if got != tc.want {
				t.Fatalf("Paginate(%q,%q,%d,%d) =\n  %s\nwant\n  %s", tc.dialect, inner, tc.limit, tc.offset, got, tc.want)
			}
		})
	}
}

func TestPaginateStripsTrailingSemicolon(t *testing.T) {
	got, err := Paginate(DialectPostgres, "SELECT * FROM t;", 10, 0)
	if err != nil {
		t.Fatalf("Paginate error = %v, want nil", err)
	}
	if got != "SELECT * FROM t LIMIT 10 OFFSET 0" {
		t.Fatalf("Paginate = %q, want trailing semicolon stripped", got)
	}
}

func TestPaginateErrors(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		inner   string
		limit   int
		offset  int
		wantErr error
	}{
		{"unknown-dialect", Dialect("mysql"), "SELECT 1", 10, 0, ErrUnknownDialect},
		{"empty-dialect", "", "SELECT 1", 10, 0, ErrUnknownDialect},
		{"empty-inner", DialectPostgres, "", 10, 0, ErrEmptyInnerQuery},
		{"whitespace-inner", DialectPostgres, "   ", 10, 0, ErrEmptyInnerQuery},
		{"semicolon-only-inner", DialectPostgres, ";", 10, 0, ErrEmptyInnerQuery},
		{"embedded-semicolon", DialectPostgres, "SELECT 1; DROP TABLE t", 10, 0, ErrInnerSemicolon},
		{"zero-limit", DialectPostgres, "SELECT 1", 0, 0, ErrInvalidLimit},
		{"negative-limit", DialectPostgres, "SELECT 1", -1, 0, ErrInvalidLimit},
		{"negative-offset", DialectPostgres, "SELECT 1", 10, -1, ErrInvalidOffset},
		{"limit-overflow", DialectPostgres, "SELECT 1", math.MaxInt32 + 1, 0, ErrInvalidLimit},
		{"offset-overflow", DialectPostgres, "SELECT 1", 10, math.MaxInt32 + 1, ErrInvalidOffset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Paginate(tc.dialect, tc.inner, tc.limit, tc.offset)
			if err == nil {
				t.Fatalf("Paginate(%q,%q,%d,%d) = %q, want error", tc.dialect, tc.inner, tc.limit, tc.offset, got)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Paginate(%q,%q,%d,%d) error = %v, want %v", tc.dialect, tc.inner, tc.limit, tc.offset, err, tc.wantErr)
			}
			if got != "" {
				t.Fatalf("Paginate error case must not return SQL, got %q", got)
			}
		})
	}
}

func TestPaginateBoundaryAccepted(t *testing.T) {
	// 上界接受：limit/offset 取 int32 上界时不报错（内联字面量仍合法）。
	if _, err := Paginate(DialectPostgres, "SELECT 1", math.MaxInt32, math.MaxInt32); err != nil {
		t.Fatalf("Paginate(max) error = %v, want nil", err)
	}
}

func TestPageToOffset(t *testing.T) {
	cases := []struct {
		name         string
		page         int
		pageSize     int
		wantLimit    int
		wantOffset   int
		wantErr      bool
		wantErrIsErr error
	}{
		{"first-page", 1, 20, 20, 0, false, nil},
		{"third-page", 3, 20, 20, 40, false, nil},
		{"page-2-size-1", 2, 1, 1, 1, false, nil},
		{"zero-page", 0, 20, 0, 0, true, ErrInvalidPage},
		{"negative-page", -3, 20, 0, 0, true, ErrInvalidPage},
		{"zero-size", 2, 0, 0, 0, true, ErrInvalidPageSize},
		{"negative-size", 2, -5, 0, 0, true, ErrInvalidPageSize},
		{"offset-overflow", math.MaxInt32, math.MaxInt32, 0, 0, true, ErrPageRangeOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit, offset, err := PageToOffset(tc.page, tc.pageSize)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("PageToOffset(%d,%d) error = nil, want error", tc.page, tc.pageSize)
				}
				if !errors.Is(err, tc.wantErrIsErr) {
					t.Fatalf("PageToOffset(%d,%d) error = %v, want %v", tc.page, tc.pageSize, err, tc.wantErrIsErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PageToOffset(%d,%d) error = %v, want nil", tc.page, tc.pageSize, err)
			}
			if limit != tc.wantLimit || offset != tc.wantOffset {
				t.Fatalf("PageToOffset(%d,%d) = (%d,%d), want (%d,%d)",
					tc.page, tc.pageSize, limit, offset, tc.wantLimit, tc.wantOffset)
			}
		})
	}
}

func TestPaginateWithPageToOffsetRoundTrip(t *testing.T) {
	// 与 dal applyTelemetryPagination 同口径换算后再生成分页，确认端到端字符串。
	limit, offset, err := PageToOffset(3, 20)
	if err != nil {
		t.Fatalf("PageToOffset error = %v", err)
	}
	got, err := Paginate(DialectKingbase, "SELECT * FROM t", limit, offset)
	if err != nil {
		t.Fatalf("Paginate error = %v", err)
	}
	if !strings.Contains(got, "ROWNUM <= 60") || !strings.Contains(got, "page_rn > 40") {
		t.Fatalf("round-trip SQL = %q, want ROWNUM <= 60 / page_rn > 40", got)
	}
}
