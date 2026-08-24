package aetherlink

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

// setupVoucherDualModeTestDB 用 AETHERLINK_TEST_PSQL_DSN 指向的真实 PostgreSQL 替换
// 包级 db 句柄（与 backend/internal/dal 的 DSN 门控回归同一约定），并确保
// voucher_hash 列存在（50.sql 由 backend 负责迁移，这里自建以保证测试可独立运行）。
func setupVoucherDualModeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; voucher dual-mode lookup requires PostgreSQL")
	}

	pg, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err := pg.Exec(`ALTER TABLE devices ADD COLUMN IF NOT EXISTS voucher_hash varchar(64)`).Error; err != nil {
		t.Fatalf("ensure voucher_hash column: %v", err)
	}
	if err := pg.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_voucher_hash ON devices (voucher_hash)`).Error; err != nil {
		t.Fatalf("ensure voucher_hash index: %v", err)
	}

	prevDB := db
	db = pg
	t.Cleanup(func() { db = prevDB })
	return pg
}

// seedVoucherDualModeDevice 直插 devices 行并按用例控制 voucher/voucher_hash 组合。
func seedVoucherDualModeDevice(t *testing.T, pg *gorm.DB, id string, voucher string, voucherHash *string) {
	t.Helper()

	if err := pg.Exec(
		`INSERT INTO devices (id, voucher, tenant_id, device_number, is_enabled, activate_flag, is_online, voucher_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET voucher = EXCLUDED.voucher, voucher_hash = EXCLUDED.voucher_hash`,
		id, voucher, "tenant-voucher-dual-mode", id, "enabled", "active", 0, voucherHash,
	).Error; err != nil {
		t.Fatalf("seed device %s: %v", id, err)
	}
	t.Cleanup(func() { pg.Exec(`DELETE FROM devices WHERE id = ?`, id) })
}

// TestLookupDeviceByVoucherDualMode 锁定双模式匹配顺序：
//  1. canonical JSON 命中 hash 列——行内明文为哨兵值，只有 voucher_hash 能命中；
//  2. 键序不同候选经候选展开命中明文兜底——行内 voucher_hash 为 NULL；
//  3. 两列均无 → gorm.ErrRecordNotFound。
func TestLookupDeviceByVoucherDualMode(t *testing.T) {
	pg := setupVoucherDualModeTestDB(t)

	canonical := `{"username":"device-user","password":"device-pass"}`
	hashOfCanonical := voucherCacheKey(canonical)

	// Case 1：明文列是哨兵值，canonical 只能经 voucher_hash 命中。
	sentinel := "sentinel-not-a-real-voucher"
	seedVoucherDualModeDevice(t, pg, "voucher-dual-hash-hit", sentinel, &hashOfCanonical)
	got, err := lookupDeviceByVoucherFromDB(canonical)
	if err != nil || got == nil {
		t.Fatalf("hash-column lookup = (%v, %v), want device", got, err)
	}
	if got.ID != "voucher-dual-hash-hit" {
		t.Fatalf("hash-column lookup returned %q, want voucher-dual-hash-hit", got.ID)
	}

	// Case 2：存量行为明文 + voucher_hash NULL；presented 为 canonical 键序，
	// 候选展开后的 lexical 键序经明文兜底命中。
	lexical := `{"password":"device-pass-2","username":"device-user-2"}`
	seedVoucherDualModeDevice(t, pg, "voucher-dual-plaintext-fallback", lexical, nil)
	presented := `{"username":"device-user-2","password":"device-pass-2"}`
	got, err = lookupDeviceByVoucherFromDB(presented)
	if err != nil || got == nil {
		t.Fatalf("plaintext fallback lookup = (%v, %v), want device", got, err)
	}
	if got.ID != "voucher-dual-plaintext-fallback" {
		t.Fatalf("plaintext fallback returned %q, want voucher-dual-plaintext-fallback", got.ID)
	}

	// Case 3：两列均未命中 → gorm.ErrRecordNotFound。
	if _, err := lookupDeviceByVoucherFromDB(`{"username":"no-such-device"}`); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unknown voucher error = %v, want gorm.ErrRecordNotFound", err)
	}
}
