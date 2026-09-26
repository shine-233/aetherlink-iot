// 文件用途：元查询方言映射单测（TP-20）——表存在性/列清单 SQL 的精确断言与参数归一/注入校验全量表驱动。
// 核心逻辑：postgres 走 information_schema（current_schema() 限定）；tdengine 走 ins_tables/ins_columns
// （DATABASE() 限定当前库，表名区分大小写原样绑定）；kingbase 走 user_tables/user_tab_columns 且绑定参数大写归一。
// 关键注意事项：断言是精确字符串相等——元查询 SQL 是方言契约；绑定参数与 SQL 成对断言，防止归一口径漂移。
package dialect

import (
	"strings"
	"testing"
)

func TestTableExistsSQL(t *testing.T) {
	cases := []struct {
		name     string
		dialect  Dialect
		table    string
		wantSQL  string
		wantArgs []any
	}{
		{"postgres", DialectPostgres, "telemetry_datas",
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?",
			[]any{"telemetry_datas"}},
		{"polardb", DialectPolardb, "telemetry_datas",
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?",
			[]any{"telemetry_datas"}},
		{"tdengine-case-preserved", DialectTDengine, "telemetry_datas",
			"SELECT COUNT(*) FROM information_schema.ins_tables WHERE db_name = DATABASE() AND table_name = ?",
			[]any{"telemetry_datas"}},
		{"kingbase-uppercased", DialectKingbase, "telemetry_datas",
			"SELECT COUNT(*) FROM user_tables WHERE table_name = ?",
			[]any{"TELEMETRY_DATAS"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args, err := TableExistsSQL(tc.dialect, tc.table)
			if err != nil {
				t.Fatalf("TableExistsSQL(%q,%q) error = %v, want nil", tc.dialect, tc.table, err)
			}
			if sql != tc.wantSQL {
				t.Fatalf("TableExistsSQL(%q,%q) sql =\n  %s\nwant\n  %s", tc.dialect, tc.table, sql, tc.wantSQL)
			}
			if len(args) != len(tc.wantArgs) || args[0] != tc.wantArgs[0] {
				t.Fatalf("TableExistsSQL(%q,%q) args = %v, want %v", tc.dialect, tc.table, args, tc.wantArgs)
			}
		})
	}
}

func TestListColumnsSQL(t *testing.T) {
	cases := []struct {
		name     string
		dialect  Dialect
		table    string
		wantSQL  string
		wantArgs []any
	}{
		{"postgres", DialectPostgres, "telemetry_datas",
			"SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? ORDER BY ordinal_position",
			[]any{"telemetry_datas"}},
		{"polardb", DialectPolardb, "telemetry_datas",
			"SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? ORDER BY ordinal_position",
			[]any{"telemetry_datas"}},
		{"tdengine", DialectTDengine, "telemetry_datas",
			"SELECT column_name, data_type FROM information_schema.ins_columns WHERE db_name = DATABASE() AND table_name = ? ORDER BY ordinal_position",
			[]any{"telemetry_datas"}},
		{"kingbase-uppercased", DialectKingbase, "telemetry_datas",
			"SELECT column_name, data_type, nullable FROM user_tab_columns WHERE table_name = ? ORDER BY column_id",
			[]any{"TELEMETRY_DATAS"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args, err := ListColumnsSQL(tc.dialect, tc.table)
			if err != nil {
				t.Fatalf("ListColumnsSQL(%q,%q) error = %v, want nil", tc.dialect, tc.table, err)
			}
			if sql != tc.wantSQL {
				t.Fatalf("ListColumnsSQL(%q,%q) sql =\n  %s\nwant\n  %s", tc.dialect, tc.table, sql, tc.wantSQL)
			}
			if len(args) != len(tc.wantArgs) || args[0] != tc.wantArgs[0] {
				t.Fatalf("ListColumnsSQL(%q,%q) args = %v, want %v", tc.dialect, tc.table, args, tc.wantArgs)
			}
		})
	}
}

func TestMetadataRejectsInvalidIdentifiers(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		table   string
		wantErr string
	}{
		{"unknown-dialect", Dialect("mysql"), "t", "unknown dialect"},
		{"empty-dialect", "", "t", "unknown dialect"},
		{"empty-table", DialectPostgres, "", "identifier is empty"},
		{"injection", DialectPostgres, "t; DROP TABLE users", "invalid identifier"},
		{"quote-injection", DialectKingbase, `t"`, "invalid identifier"},
		{"starts-with-digit", DialectTDengine, "1t", "invalid identifier"},
		{"too-long-for-pg", DialectPostgres, strings.Repeat("a", 64), "exceeds max length"},
		{"too-long-for-kingbase", DialectKingbase, strings.Repeat("a", 64), "exceeds max length"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := TableExistsSQL(tc.dialect, tc.table)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("TableExistsSQL(%q,%q) error = %v, want contains %q", tc.dialect, tc.table, err, tc.wantErr)
			}
			_, _, err = ListColumnsSQL(tc.dialect, tc.table)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ListColumnsSQL(%q,%q) error = %v, want contains %q", tc.dialect, tc.table, err, tc.wantErr)
			}
		})
	}
}

func TestMetadataLengthBoundariesAccepted(t *testing.T) {
	// 上界接受：63 字符在 PG/KingBase、192 字符在 TDengine 下合法。
	pgMax := strings.Repeat("a", 63)
	if _, _, err := TableExistsSQL(DialectPostgres, pgMax); err != nil {
		t.Fatalf("TableExistsSQL(postgres, 63 chars) error = %v, want nil", err)
	}
	if _, _, err := ListColumnsSQL(DialectKingbase, pgMax); err != nil {
		t.Fatalf("ListColumnsSQL(kingbase, 63 chars) error = %v, want nil", err)
	}
	tdMax := strings.Repeat("a", 192)
	if _, _, err := TableExistsSQL(DialectTDengine, tdMax); err != nil {
		t.Fatalf("TableExistsSQL(tdengine, 192 chars) error = %v, want nil", err)
	}
}
