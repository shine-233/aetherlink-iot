// 文件用途：锁定 telemetry_current_datas 读侧 gen 链从全新 gorm Statement 出发的收敛回归。
// 核心逻辑：向包级表单例的底层语句注入跨设备残留 WHERE，确定性复现继承链污染前提，
// 断言修复后单设备读、批量读与 readiness 兜底路径都只返回目标设备自己的行。
// 关键注意事项：批次三收敛（2026-08，见 references/gen-inheritance-audit.md）；
// CI 03_data telemetry snapshot 深比较失败的根因即本测试注入的陈旧条件泄漏。
// 重构建议：若后续 gorm/gen 升级引入语句克隆语义变化，优先扩展本测试覆盖其余读函数。
package dal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setupTelemetryCurrentIsolationDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TelemetryCurrentData{}); err != nil {
		t.Fatalf("migrate telemetry_current_datas: %v", err)
	}

	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func seedTelemetryCurrentIsolationRows(t *testing.T, db *gorm.DB) {
	t.Helper()

	now := time.Now()
	rows := []*model.TelemetryCurrentData{
		{DeviceID: "device-stale", Key: "temperature", T: now},
		{DeviceID: "device-target", Key: "humidity", T: now.Add(-time.Minute)},
		{DeviceID: "device-target", Key: "temperature", T: now.Add(-2 * time.Minute)},
	}
	for i := range rows {
		if err := db.Create(rows[i]).Error; err != nil {
			t.Fatalf("seed telemetry row %d: %v", i, err)
		}
	}
}

// poisonTelemetryCurrentSingleton 向包级表单例的底层语句写入跨设备残留 WHERE，
// 模拟高负载下继承式语句根把上一请求的过滤条件带进当前请求的执行语句。
func poisonTelemetryCurrentSingleton(t *testing.T) {
	t.Helper()

	root := query.TelemetryCurrentData.UnderlyingDB()
	if root == nil || root.Statement == nil {
		t.Fatalf("underlying statement unavailable")
	}
	root.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
		clause.Eq{Column: clause.Column{Name: "device_id"}, Value: "device-stale"},
	}})
}

func assertTelemetryRowsAllTarget(t *testing.T, rows []*model.TelemetryCurrentData) {
	t.Helper()

	if len(rows) != 2 {
		t.Fatalf("target device rows = %d (%v), want exactly its own 2 rows", len(rows), rows)
	}
	for _, row := range rows {
		if row.DeviceID != "device-target" {
			t.Fatalf("cross-device stale row leaked into read result: %+v", row)
		}
	}
}

func TestTelemetryCurrentReadsIgnoreStaleSingletonConditions(t *testing.T) {
	db := setupTelemetryCurrentIsolationDB(t)
	seedTelemetryCurrentIsolationRows(t, db)
	poisonTelemetryCurrentSingleton(t)

	// 单设备读（board/twin/details 高频路径）：只允许返回目标设备自己的行。
	rows, err := GetCurrentTelemetryDataEvolution("device-target")
	if err != nil {
		t.Fatalf("GetCurrentTelemetryDataEvolution returned error: %v", err)
	}
	assertTelemetryRowsAllTarget(t, rows)

	// ts DESC 排序契约保持不变。
	if !rows[0].T.After(rows[1].T) {
		t.Fatalf("evolution order broken: %v -> %v", rows[0].T, rows[1].T)
	}

	// 批量读：In 集合过滤不得被残留条件收窄或污染。
	grouped, err := GetCurrentTelemetryDataEvolutionByDeviceIDs([]string{"device-target"})
	if err != nil {
		t.Fatalf("GetCurrentTelemetryDataEvolutionByDeviceIDs returned error: %v", err)
	}
	assertTelemetryRowsAllTarget(t, grouped["device-target"])
	if _, leaked := grouped["device-stale"]; leaked {
		t.Fatal("stale device leaked into batch read result map")
	}
}

func TestGetCurrentTelemetryReadinessIgnoresStaleSingletonConditions(t *testing.T) {
	db := setupTelemetryCurrentIsolationDB(t)
	seedTelemetryCurrentIsolationRows(t, db)
	poisonTelemetryCurrentSingleton(t)

	// readiness（diagnostics 热路径）整体走 raw 链：单例残留条件不得影响计数与最新行。
	count, latest, err := GetCurrentTelemetryReadiness("device-target")
	if err != nil {
		t.Fatalf("GetCurrentTelemetryReadiness returned error: %v", err)
	}
	if latest == nil || latest.DeviceID != "device-target" {
		t.Fatalf("readiness latest = %+v, want device-target row", latest)
	}
	if count != 2 {
		t.Fatalf("readiness count = %d, want 2 for device-target", count)
	}

	// 对照：global.DB 未初始化时返回显式错误而非静默回落继承式 gen 兜底链。
	oldDB := global.DB
	global.DB = nil
	_, _, err = GetCurrentTelemetryReadiness("device-target")
	global.DB = oldDB
	if err == nil {
		t.Fatal("readiness without global.DB should return an explicit error")
	}
}
