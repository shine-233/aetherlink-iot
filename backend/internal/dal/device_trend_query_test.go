// 文件用途：钉住 GetDeviceTrend SQL 的热路径形态与 144.sql 索引。
//
// device_trend.go 依赖 PG 专有语法，SQLite 单测跑不了，所以这里只断言源码形态：
// 结果等价性已在 PG 17.5（5 万设备 / 180 万状态历史，含 owner 过滤）上用
// EXCEPT ALL 双向比对验证为 0 差异。回退到"GROUP BY MAX(id) 再按 id 回表"会让
// 执行计划重新整表扫描 device_status_history，本测试会失败。
package dal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/global"
)

func readDeviceTrendSource(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "device_trend.go"))
	if err != nil {
		t.Fatalf("read device_trend.go: %v", err)
	}
	return string(raw)
}

func TestDeviceTrendSQLAvoidsHistoryJoinBack(t *testing.T) {
	src := readDeviceTrendSource(t)
	for _, banned := range []string{"MAX(id) AS max_id", "ON dsh.id = latest.max_id", "COUNT(DISTINCT dsh.device_id)"} {
		if strings.Contains(src, banned) {
			t.Fatalf("device_trend.go reintroduced full-history join-back pattern %q", banned)
		}
	}
	for _, want := range []string{
		"CROSS JOIN LATERAL",
		"ORDER BY dsh.id DESC\n        LIMIT 1",
		"SELECT DISTINCT ON (dsh.device_id, date_trunc('hour', dsh.change_time))",
		"ORDER BY dsh.device_id, date_trunc('hour', dsh.change_time), dsh.id DESC",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("device_trend.go missing hot-path form %q", want)
		}
	}
}

func TestMigration144DeviceStatusHistoryIndexIsIdempotent(t *testing.T) {
	if global.VERSION_NUMBER < 144 {
		t.Fatalf("VERSION_NUMBER = %d, want at least 144", global.VERSION_NUMBER)
	}
	assertIdempotentIndexMigration(t, "144.sql", []string{
		"public.device_status_history (tenant_id, device_id, id DESC)",
		"INCLUDE (status, change_time)",
	})
}
