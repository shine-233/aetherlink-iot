package initialize

import (
	"os"
	"path/filepath"
	"testing"

	"aetherlink-iot/backend/pkg/global"
)

func TestDecideBaseline(t *testing.T) {
	ok := baselineInputs{DataVersion: 0, Mode: "auto", TimescaleMode: "off", BaselineNumber: 143, SourceMatches: true,
		BaselineMajor: 17, ServerMajor: 17}
	cases := []struct {
		name string
		mut  func(*baselineInputs)
		want bool
	}{
		{"全新库+auto+ts off", func(*baselineInputs) {}, true},
		{"ts auto 且扩展缺失", func(in *baselineInputs) { in.TimescaleMode = "auto" }, true},
		{"ts auto 但扩展已装", func(in *baselineInputs) { in.TimescaleMode = "auto"; in.TimescaleInstalled = true }, false},
		{"ts on 一律增量", func(in *baselineInputs) { in.TimescaleMode = "on"; in.TimescaleInstalled = true }, false},
		{"ts off 即使扩展已装", func(in *baselineInputs) { in.TimescaleInstalled = true }, true},
		{"开关 off", func(in *baselineInputs) { in.Mode = "off" }, false},
		{"已有版本的库", func(in *baselineInputs) { in.DataVersion = 5 }, false},
		{"没有基线文件", func(in *baselineInputs) { in.BaselineNumber = 0 }, false},
		{"脏库", func(in *baselineInputs) { in.BusinessTables = 1 }, false},
		{"基线已过期", func(in *baselineInputs) { in.SourceMatches = false }, false},
		{"服务器主版本低于生成版本", func(in *baselineInputs) { in.ServerMajor = 16 }, false},
		{"服务器主版本更高", func(in *baselineInputs) { in.ServerMajor = 18 }, true},
		{"头部缺 postgres-major", func(in *baselineInputs) { in.BaselineMajor = 0 }, false},
	}
	for _, c := range cases {
		in := ok
		c.mut(&in)
		if got := decideBaseline(in); got != c.want {
			t.Errorf("%s: decideBaseline(%+v)=%v, want %v", c.name, in, got, c.want)
		}
	}
}

func TestLatestBaseline(t *testing.T) {
	dir := t.TempDir()
	if n, _, err := latestBaseline(filepath.Join(dir, "missing"), 143); err != nil || n != 0 {
		t.Fatalf("目录不存在应返回 0,nil，得到 %d,%v", n, err)
	}
	for _, f := range []string{"100.sql", "142.sql", "150.sql", "abc.sql", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("--"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "143.sql"), 0o755); err != nil {
		t.Fatal(err)
	}
	n, p, err := latestBaseline(dir, 143)
	if err != nil || n != 142 || filepath.Base(p) != "142.sql" {
		t.Fatalf("应选 <=143 的最大文件 142.sql，得到 %d %q %v", n, p, err)
	}
	if n, _, _ := latestBaseline(dir, 99); n != 0 {
		t.Fatalf("没有 <=99 的基线应返回 0，得到 %d", n)
	}
}

func TestReadMigrationBaselineMode(t *testing.T) {
	for raw, want := range map[string]string{"": "off", "off": "off", " AUTO ": "auto"} {
		t.Setenv("AETHERLINK_MIGRATION_BASELINE", raw)
		got, err := readMigrationBaselineMode()
		if err != nil || got != want {
			t.Errorf("%q → %q,%v，期望 %q", raw, got, err, want)
		}
	}
	t.Setenv("AETHERLINK_MIGRATION_BASELINE", "on")
	if _, err := readMigrationBaselineMode(); err == nil {
		t.Error("未知取值应 fail-fast")
	}
}

// TestCommittedBaselineMatchesSources 守卫：已提交的基线编号不得超过 VERSION_NUMBER，
// 且头部 source-sha256 必须与当前 sql/1..B.sql 一致（改了旧迁移却没重新生成基线会在这里红）。
func TestCommittedBaselineMatchesSources(t *testing.T) {
	const sqlDir = "../sql"
	n, path, err := latestBaseline(filepath.Join(sqlDir, "baseline"), 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Skip("没有已提交的基线")
	}
	if n > global.VERSION_NUMBER {
		t.Fatalf("基线 %d.sql 超过 VERSION_NUMBER=%d", n, global.VERSION_NUMBER)
	}
	want, err := BaselineSourceSHA256(sqlDir, n)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadBaselineHeaderSHA(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s 已过期：头部 sha256=%s，当前 sql/1..%d.sql=%s；请运行 go run ./cmd/migbaseline -dsn-admin <dsn> -verify 重新生成", path, got, n, want)
	}
}
