package initialize

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// 含空格、单引号、反斜杠或 "k=v" 片段的密码必须原样到达驱动，且不能注入额外连接参数。
func TestBuildPgDSNQuotesSpecialValues(t *testing.T) {
	cases := []string{
		"plain",
		"with space",
		"it's",
		`back\slash`,
		"x sslmode=require",
		"",
	}
	for _, password := range cases {
		cfg := &DbConfig{
			Host: "127.0.0.1", Port: 5432, DbName: "aether db", Username: "postgres",
			Password: password, TimeZone: "Asia/Shanghai",
		}
		parsed, err := pgconn.ParseConfig(buildPgDSN(cfg))
		if err != nil {
			t.Fatalf("password %q: parse dsn: %v", password, err)
		}
		if parsed.Password != password {
			t.Fatalf("password round-trip = %q, want %q", parsed.Password, password)
		}
		if parsed.Database != "aether db" || parsed.User != "postgres" || parsed.Host != "127.0.0.1" || parsed.Port != 5432 {
			t.Fatalf("unexpected parsed config: %+v", parsed)
		}
		if parsed.TLSConfig != nil {
			t.Fatalf("password %q must not change sslmode (TLS config injected)", password)
		}
		if parsed.RuntimeParams["TimeZone"] != "Asia/Shanghai" {
			t.Fatalf("TimeZone = %q", parsed.RuntimeParams["TimeZone"])
		}
	}
}

func TestQuotePgDSNValueLeavesSimpleValuesUnchanged(t *testing.T) {
	if got := quotePgDSNValue("localhost"); got != "localhost" {
		t.Fatalf("got %q", got)
	}
	if got := quotePgDSNValue(""); got != "''" {
		t.Fatalf("empty value = %q, want ''", got)
	}
}
