// 文件用途：方言枚举与配置解析单测（TP-20）——Parse/FromTptodbType/Effective/Capabilities 全量表驱动。
// 核心逻辑：钉死 db.dialect 归一化口径、tptodb_type→方言映射（与 internal/app、internal/dal 开关口径一致）、
// Effective 优先级（显式 > tptodb 开关 > postgres 默认）与 fail-closed（未注册方言一律报错不猜默认）。
// 关键注意事项："tsdb" 等开关值不是合法 db.dialect 输入，两者刻意分开，本测试防止语义混串回归。
package dialect

import (
	"errors"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    Dialect
		wantErr bool
	}{
		{"postgres", "postgres", DialectPostgres, false},
		{"postgresql", "postgresql", DialectPostgres, false},
		{"pg", "pg", DialectPostgres, false},
		{"case-insensitive", " PostgreSQL ", DialectPostgres, false},
		{"tdengine", "tdengine", DialectTDengine, false},
		{"td", "TD", DialectTDengine, false},
		{"kingbase", "kingbase", DialectKingbase, false},
		{"kingbasees", "KingBaseES", DialectKingbase, false},
		{"polardb", "polardb", DialectPolardb, false},
		{"empty", "", "", true},
		{"whitespace-only", "   ", "", true},
		{"tptodb-switch-not-a-dialect", "TSDB", "", true},
		{"unknown", "mysql", "", true},
		{"injection-attempt", "postgres; DROP TABLE users", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) error = nil, want error", tc.raw)
				}
				if !errors.Is(err, ErrUnknownDialect) {
					t.Fatalf("Parse(%q) error = %v, want ErrUnknownDialect", tc.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error = %v, want nil", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestFromTptodbType(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		want      Dialect
		wantKnown bool
	}{
		{"none", "NONE", "", false},
		{"empty", "", "", false},
		{"whitespace", "  ", "", false},
		{"tsdb", "TSDB", DialectTDengine, true},
		{"tsdb-case-space", " tsdb ", DialectTDengine, true},
		{"kingbase", "KINGBASE", DialectKingbase, true},
		{"kingbase-case", "kingbase", DialectKingbase, true},
		{"polardb", "POLARDB", DialectPolardb, true},
		{"polardb-case", "polardb", DialectPolardb, true},
		{"unknown", "MYSQL", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FromTptodbType(tc.raw)
			if ok != tc.wantKnown {
				t.Fatalf("FromTptodbType(%q) ok = %v, want %v", tc.raw, ok, tc.wantKnown)
			}
			if got != tc.want {
				t.Fatalf("FromTptodbType(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestEffective(t *testing.T) {
	cases := []struct {
		name          string
		dialectConfig string
		tptodbType    string
		want          Dialect
		wantErr       bool
	}{
		{"explicit-wins", "tdengine", "KINGBASE", DialectTDengine, false},
		{"explicit-pg", " postgresql ", "", DialectPostgres, false},
		{"fallback-tptodb-tsdb", "", "TSDB", DialectTDengine, false},
		{"fallback-tptodb-kingbase", "", "KINGBASE", DialectKingbase, false},
		{"fallback-tptodb-polardb", "", "POLARDB", DialectPolardb, false},
		{"fallback-none", "", "NONE", DialectPostgres, false},
		{"fallback-empty", "", "", DialectPostgres, false},
		{"explicit-unknown-fails-closed", "mysql", "TSDB", "", true},
		{"explicit-empty-string-is-unset", "  ", "TSDB", DialectTDengine, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Effective(tc.dialectConfig, tc.tptodbType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Effective(%q,%q) error = nil, want error", tc.dialectConfig, tc.tptodbType)
				}
				return
			}
			if err != nil {
				t.Fatalf("Effective(%q,%q) error = %v, want nil", tc.dialectConfig, tc.tptodbType, err)
			}
			if got != tc.want {
				t.Fatalf("Effective(%q,%q) = %q, want %q", tc.dialectConfig, tc.tptodbType, got, tc.want)
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	cases := []struct {
		dialect              Dialect
		wantPageStyle        PageStyle
		wantMetadataStyle    MetadataStyle
		wantQuoteChar        byte
		wantUpperIdentifier  bool
		wantMaxIdentifierLen int
	}{
		{DialectPostgres, PageStyleLimitOffset, MetadataInformationSchema, '"', false, 63},
		{DialectTDengine, PageStyleLimitOffset, MetadataInsSchema, '`', false, 192},
		{DialectKingbase, PageStyleRownum, MetadataOracleDict, '"', true, 63},
		{DialectPolardb, PageStyleLimitOffset, MetadataInformationSchema, '`', false, 64},
	}
	for _, tc := range cases {
		t.Run(string(tc.dialect), func(t *testing.T) {
			caps := tc.dialect.Capabilities()
			if caps.PageStyle != tc.wantPageStyle {
				t.Fatalf("PageStyle = %q, want %q", caps.PageStyle, tc.wantPageStyle)
			}
			if caps.MetadataStyle != tc.wantMetadataStyle {
				t.Fatalf("MetadataStyle = %q, want %q", caps.MetadataStyle, tc.wantMetadataStyle)
			}
			if caps.QuoteChar != tc.wantQuoteChar {
				t.Fatalf("QuoteChar = %q, want %q", caps.QuoteChar, tc.wantQuoteChar)
			}
			if caps.UpperIdentifier != tc.wantUpperIdentifier {
				t.Fatalf("UpperIdentifier = %v, want %v", caps.UpperIdentifier, tc.wantUpperIdentifier)
			}
			if caps.MaxIdentifierLen != tc.wantMaxIdentifierLen {
				t.Fatalf("MaxIdentifierLen = %d, want %d", caps.MaxIdentifierLen, tc.wantMaxIdentifierLen)
			}
			if !tc.dialect.IsKnown() {
				t.Fatalf("%q should be known", tc.dialect)
			}
		})
	}
}

func TestIsKnownRejectsUnknownDialect(t *testing.T) {
	for _, d := range []Dialect{"", "mysql", "oracle", "POSTGRES", "TDENGINE"} {
		if d.IsKnown() {
			t.Fatalf("IsKnown(%q) = true, want false（大小写敏感，未归一化的值必须拒绝）", d)
		}
	}
}

func TestValidateIdentifier(t *testing.T) {
	caps := DialectPostgres.Capabilities()
	valid := []string{"telemetry_datas", "_tmp", "T1", "a", "device_id_2"}
	for _, name := range valid {
		if err := ValidateIdentifier(name, caps); err != nil {
			t.Fatalf("ValidateIdentifier(%q) error = %v, want nil", name, err)
		}
	}
	invalid := []struct {
		name string
		caps Capabilities
	}{
		{"", caps},
		{"1abc", caps},
		{"telemetry-datas", caps},
		{"telemetry;drop", caps},
		{"telemetry datas", caps},
		{"telemetry\"datas", caps},
		{"telemetry`datas", caps},
		{strings.Repeat("a", 64), caps}, // 64 字符超出 PG 上限 63
		{strings.Repeat("a", 193), DialectTDengine.Capabilities()}, // 193 字符超出 TDengine 上限 192
	}
	for _, tc := range invalid {
		if err := ValidateIdentifier(tc.name, tc.caps); err == nil {
			t.Fatalf("ValidateIdentifier(%q...) error = nil, want error", tc.name)
		}
	}
	// 同名长度在不同上限方言下判定相反：100 字符在 TDengine（192）下合法，PG（63）下非法。
	long100 := strings.Repeat("a", 100)
	if err := ValidateIdentifier(long100, DialectTDengine.Capabilities()); err != nil {
		t.Fatalf("ValidateIdentifier(100 chars, tdengine) error = %v, want nil", err)
	}
	if err := ValidateIdentifier(long100, caps); err == nil {
		t.Fatal("ValidateIdentifier(100 chars, postgres) error = nil, want error")
	}
}

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct {
		dialect Dialect
		name    string
		want    string
		wantErr bool
	}{
		{DialectPostgres, "telemetry_datas", `"telemetry_datas"`, false},
		{DialectKingbase, "telemetry_datas", `"telemetry_datas"`, false},
		{DialectTDengine, "telemetry_datas", "`telemetry_datas`", false},
		{DialectPolardb, "telemetry_datas", "`telemetry_datas`", false},
		{DialectPostgres, "telemetry-datas", "", true},
		{Dialect("mysql"), "t", "", true},
	}
	for _, tc := range cases {
		t.Run(string(tc.dialect)+"/"+tc.name, func(t *testing.T) {
			got, err := QuoteIdentifier(tc.dialect, tc.name)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("QuoteIdentifier(%q,%q) error = nil, want error", tc.dialect, tc.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("QuoteIdentifier(%q,%q) error = %v, want nil", tc.dialect, tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("QuoteIdentifier(%q,%q) = %q, want %q", tc.dialect, tc.name, got, tc.want)
			}
		})
	}
}
