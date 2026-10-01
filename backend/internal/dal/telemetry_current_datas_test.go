// 文件用途：锁定 GetCurrentTelemetrDetailData 的"设备最新一行（不按 key 筛选）"契约。
// 核心逻辑：对多 key 设备写入不同 ts 的若干行，断言返回的是 ts 最大的那一行，
// 而不是任意一个 key 的最新值；并断言跨设备行不泄漏。
// 关键注意事项：该契约由 automation_tests/tests/12_telemetry_extra.test.js 锚定
// （"current detail is a latest-row diagnostic endpoint without a key selector"），
// backend/sql/143.sql 新增的 (device_id, ts DESC) 索引不改变此行为，只去掉无索引排序。
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
)

func setupTelemetryCurrentDetailDB(t *testing.T) *gorm.DB {
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

// TestGetCurrentTelemetrDetailData_MultiKeyDeviceReturnsLatestRow 锁定多 key 设备的契约：
// 返回值是 ts 最大的那一行，即最近一次上报命中的具体 key，而不是"每 key 当前值"的
// 任意聚合。该测试在修复前就应通过（本次改动不改变返回语义，只补索引），
// 用于防止后续有人把 Order("ts DESC") 误改成按 key 分组的窗口语义。
func TestGetCurrentTelemetrDetailData_MultiKeyDeviceReturnsLatestRow(t *testing.T) {
	db := setupTelemetryCurrentDetailDB(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	numVal := 42.0
	strVal := "latest-humidity"
	otherNum := 10.0

	rows := []*model.TelemetryCurrentData{
		// 同设备下更早上报的 key：temperature。
		{DeviceID: "device-multi", Key: "temperature", T: now.Add(-time.Minute), NumberV: &otherNum},
		// 同设备下最近一次上报的 key：humidity，ts 最大，应为返回行。
		{DeviceID: "device-multi", Key: "humidity", T: now, StringV: &strVal},
		// 其他设备的行必须不泄漏。
		{DeviceID: "device-other", Key: "temperature", T: now.Add(time.Minute), NumberV: &numVal},
	}
	for i := range rows {
		if err := db.Create(rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	got, err := GetCurrentTelemetrDetailData("device-multi")
	if err != nil {
		t.Fatalf("GetCurrentTelemetrDetailData returned error: %v", err)
	}
	if got.DeviceID != "device-multi" {
		t.Fatalf("device_id leaked: got %+v", got)
	}
	if got.Key != "humidity" {
		t.Fatalf("expected latest-row key %q (ts=%v), got key %q (ts=%v); multi-key device must return the max-ts row, not an arbitrary key",
			"humidity", now, got.Key, got.T)
	}
	if got.StringV == nil || *got.StringV != strVal {
		t.Fatalf("expected value %q from latest row, got %+v", strVal, got)
	}
}

// TestGetCurrentTelemetrDetailData_SingleKeyDevice 覆盖单 key 设备（最常见场景）：
// 等值+排序取一退化为等值查询，仍应命中新复合索引并返回唯一行。
func TestGetCurrentTelemetrDetailData_SingleKeyDevice(t *testing.T) {
	db := setupTelemetryCurrentDetailDB(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	boolVal := true
	if err := db.Create(&model.TelemetryCurrentData{
		DeviceID: "device-single", Key: "switch", T: now, BoolV: &boolVal,
	}).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	got, err := GetCurrentTelemetrDetailData("device-single")
	if err != nil {
		t.Fatalf("GetCurrentTelemetrDetailData returned error: %v", err)
	}
	if got.DeviceID != "device-single" || got.Key != "switch" {
		t.Fatalf("unexpected row: %+v", got)
	}
	if got.BoolV == nil || *got.BoolV != true {
		t.Fatalf("expected bool_v=true, got %+v", got)
	}
}

// TestGetCurrentTelemetrDetailData_NoRows 覆盖设备无当前值时的既有错误语义
// （ErrRecordNotFound 经 First() 原样向上传播，调用方 service 层据此判空）。
func TestGetCurrentTelemetrDetailData_NoRows(t *testing.T) {
	setupTelemetryCurrentDetailDB(t)

	_, err := GetCurrentTelemetrDetailData("device-missing")
	if err == nil {
		t.Fatal("expected error for device with no current telemetry rows")
	}
}

// TestGetCurrentTelemetrDetailData_SameTimestampTieBreaksByKey 锁定同毫秒多 key 并列时的
// 确定性：按 key 升序决胜。旧实现用 First()，其隐式 ORDER BY 主键（device_id）已被 WHERE
// 固定，并列行返回顺序依赖存储/计划，不可复现。同时断言就绪探测与明细查询返回同一行。
func TestGetCurrentTelemetrDetailData_SameTimestampTieBreaksByKey(t *testing.T) {
	db := setupTelemetryCurrentDetailDB(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	a, b, c := 1.0, 2.0, 3.0
	// 故意按非字母序插入，避免插入顺序恰好等于期望顺序。
	rows := []*model.TelemetryCurrentData{
		{DeviceID: "device-tie", Key: "zeta", T: now, NumberV: &c},
		{DeviceID: "device-tie", Key: "alpha", T: now, NumberV: &a},
		{DeviceID: "device-tie", Key: "mid", T: now, NumberV: &b},
		{DeviceID: "device-tie", Key: "older", T: now.Add(-time.Second), NumberV: &a},
	}
	for i := range rows {
		if err := db.Create(rows[i]).Error; err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	got, err := GetCurrentTelemetrDetailData("device-tie")
	if err != nil {
		t.Fatalf("GetCurrentTelemetrDetailData returned error: %v", err)
	}
	if got.Key != "alpha" || got.NumberV == nil || *got.NumberV != a {
		t.Fatalf("expected tie-break winner key %q, got %+v", "alpha", got)
	}

	count, latest, err := getCurrentTelemetryReadinessFromDB("device-tie")
	if err != nil {
		t.Fatalf("getCurrentTelemetryReadinessFromDB returned error: %v", err)
	}
	if count != int64(len(rows)) {
		t.Fatalf("expected count %d, got %d", len(rows), count)
	}
	if latest == nil || latest.Key != got.Key {
		t.Fatalf("readiness latest row %+v disagrees with detail row key %q", latest, got.Key)
	}
}
