package aetherlink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestNormalizePostgresSSLMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty keeps local default", input: "", want: "disable"},
		{name: "whitespace keeps local default", input: "  ", want: "disable"},
		{name: "explicit mode", input: " verify-full ", want: "verify-full"},
		{name: "case is normalized", input: "REQUIRE", want: "require"},
		{name: "invalid mode is rejected", input: "insecure", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePostgresSSLMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizePostgresSSLMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("normalizePostgresSSLMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildPostgresDSNIncludesConfiguredSSLMode(t *testing.T) {
	got := buildPostgresDSN("user", "pass", "db", "postgres", 5432, "verify-ca")
	want := "user=user password=pass dbname=db host=postgres port=5432 sslmode=verify-ca"
	if got != want {
		t.Fatalf("buildPostgresDSN() = %q, want %q", got, want)
	}
}

func TestPostgresLogTargetDoesNotContainPassword(t *testing.T) {
	password := "secret-password"
	got := postgresLogTarget("user", "db", "postgres", 5432)
	if strings.Contains(got, password) {
		t.Fatalf("postgresLogTarget() leaked password: %q", got)
	}
}

func TestDeviceVoucherLookupCandidatesSupportBothJSONKeyOrders(t *testing.T) {
	got := deviceVoucherLookupCandidates(`{"password":"device-pass","username":"device-user"}`)
	want := map[string]bool{
		`{"password":"device-pass","username":"device-user"}`: false,
		`{"username":"device-user","password":"device-pass"}`: false,
	}
	for _, candidate := range got {
		if _, ok := want[candidate]; ok {
			want[candidate] = true
		}
	}
	for candidate, found := range want {
		if !found {
			t.Fatalf("deviceVoucherLookupCandidates() missing %q; got %v", candidate, got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("deviceVoucherLookupCandidates() returned duplicate/unexpected candidates: %v", got)
	}
}

func TestConfigureRuntimeEnvironmentLetsEnvironmentOverrideConfigFileValues(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	configPath := filepath.Join(t.TempDir(), "aetherlink.yml")
	if err := os.WriteFile(configPath, []byte("db:\n  psql:\n    psqldb: database-from-file\n  redis:\n    password: redis-password-from-file\n"), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	viper.SetConfigFile(configPath)
	t.Setenv("GMQTT_DB_PSQL_PSQLDB", "database-from-environment")
	t.Setenv("GMQTT_DB_REDIS_PASSWORD", "")

	if err := configureRuntimeEnvironment(); err != nil {
		t.Fatalf("configureRuntimeEnvironment() error = %v", err)
	}
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("read test config: %v", err)
	}

	if got := viper.GetString("db.psql.psqldb"); got != "database-from-environment" {
		t.Fatalf("database name = %q, want environment override", got)
	}
	if got := viper.GetString("db.redis.password"); got != "" {
		t.Fatalf("redis password = %q, want explicit empty environment override", got)
	}
}
