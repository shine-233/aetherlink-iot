package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readDependencyContractFile(t *testing.T, relativePath string) string {
	t.Helper()
	content, err := os.ReadFile(relativePath)
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}

// TestOptionalMarketDefaultsDisabled ensures a fresh private deployment does
// not silently turn the external Market service into a startup dependency.
func TestOptionalMarketDefaultsDisabled(t *testing.T) {
	for _, configPath := range []string{
		filepath.Join("..", "..", "configs", "conf.yml"),
		filepath.Join("..", "..", "configs", "conf.example.yml"),
	} {
		config := readDependencyContractFile(t, configPath)
		if !strings.Contains(config, "market:\n  # 外部市场是可选能力") ||
			!strings.Contains(config, "\n  enabled: false\n  base_url: \"\"") {
			t.Errorf("%s must keep Market explicitly disabled and unconfigured by default", configPath)
		}
	}
}

// TestMultiDBStackIsExplicitLocalOnly keeps the optional database compatibility
// fixture reproducible and prevents its example credentials from being exposed
// beyond the developer machine.
func TestDefaultPostgresSupportsOptionalTimescaleDB(t *testing.T) {
	compose := readDependencyContractFile(t, filepath.Join("..", "..", "..", "docker-compose.yml"))
	if !strings.Contains(compose, "image: postgres:18-alpine") {
		t.Error("default deployment must use the pinned PostgreSQL 18 image")
	}

	schema := readDependencyContractFile(t, filepath.Join("..", "..", "sql", "1.sql"))
	for _, fallbackClause := range []string{
		"IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN",
		"PERFORM create_hypertable('telemetry_datas', 'ts'",
	} {
		if !strings.Contains(schema, fallbackClause) {
			t.Errorf("base schema must keep the optional TimescaleDB fallback clause %q", fallbackClause)
		}
	}
}

func TestMultiDBStackIsExplicitLocalOnly(t *testing.T) {
	composePath := filepath.Join("..", "..", "test", "multidb", "docker-compose.yml")
	compose := readDependencyContractFile(t, composePath)

	for _, image := range []string{"mysql:26.7", "postgres:18-alpine", "adminer:5.5.1"} {
		if !strings.Contains(compose, "image: "+image) {
			t.Errorf("multidb test stack must pin image %s", image)
		}
	}
	if strings.Count(compose, "profiles: [multidb-test]") != 3 {
		t.Error("every multidb service must require the explicit multidb-test profile")
	}
	for _, port := range []string{"3306", "5432", "8080"} {
		mapping := "\"127.0.0.1:" + port + ":" + port + "\""
		if !strings.Contains(compose, mapping) {
			t.Errorf("test port %s must bind only to loopback", port)
		}
	}
	if strings.Contains(compose, "mysql_native_password") {
		t.Error("multidb test stack must not force the deprecated MySQL authentication plugin")
	}
}
