// 文件用途: 覆盖遥测统计聚合查询（单设备按时间窗口）的回归测试，确保批量 CTE 改造
// 后的结果形状、排序和空窗口过滤行为与旧的逐窗口实现保持一致。
// 核心逻辑: 用内存 sqlite 构造跨多个时间窗口的遥测数据，断言 getAggregatedDataWithTime
// 返回按时间升序排列的聚合结果，且没有数据落入的窗口被跳过。
// 关键注意事项: 断言需覆盖多窗口批量路径（曾经是 N 次 Raw 查询）以及空结果、单窗口边界。
package dal

import (
	"fmt"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTelemetryStatisticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { global.DB = oldDB })
	global.DB = db

	require.NoError(t, db.AutoMigrate(&model.TelemetryData{}))
	return db
}

func numberV(v float64) *float64 { return &v }

func TestGetAggregatedDataWithTimeBatchesAcrossWindows(t *testing.T) {
	db := setupTelemetryStatisticsTestDB(t)

	// getAggregatedDataWithTime 内部按 time.Local 对齐窗口边界，测试数据和期望值
	// 必须用同一个时区构造，否则窗口边界会和本机时区错位（CI/本机时区不保证是 UTC）。
	loc := time.Local
	day := func(y int, m time.Month, d int) int64 {
		return time.Date(y, m, d, 12, 0, 0, 0, loc).UnixMilli()
	}

	rows := []*model.TelemetryData{
		{DeviceID: "device-a", Key: "temperature", T: day(2026, 3, 1), NumberV: numberV(10)},
		{DeviceID: "device-a", Key: "temperature", T: day(2026, 3, 1) + 1000, NumberV: numberV(20)},
		// 2026-03-02 没有数据，对应窗口应被跳过（与旧实现的 result[0]["value"]==nil 行为一致）。
		{DeviceID: "device-a", Key: "temperature", T: day(2026, 3, 3), NumberV: numberV(30)},
		// 不同设备/不同 key 的数据不能混入结果。
		{DeviceID: "device-b", Key: "temperature", T: day(2026, 3, 3), NumberV: numberV(999)},
		{DeviceID: "device-a", Key: "humidity", T: day(2026, 3, 3), NumberV: numberV(888)},
	}
	require.NoError(t, db.Create(rows).Error)

	startTime := time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UnixMilli()
	endTime := time.Date(2026, 3, 4, 0, 0, 0, 0, loc).UnixMilli()
	limit := 4

	results, err := getAggregatedDataWithTime("device-a", "temperature", startTime, endTime, "avg", &limit, "day")
	require.NoError(t, err)

	// aggregateTimeWindows 按"最新窗口在前"排列（从 endTime 往回数 limit 个窗口），
	// getAggregatedDataWithTime 保持这个顺序不重排。3/2 没有数据的窗口被跳过，
	// 只剩 [3/3,3/4) 和 [3/1,3/2) 两个有数据的窗口，顺序是时间降序，
	// timestamp 取的是窗口起点（window.startMS），不是数据点自身的时间。
	dayStart := func(y int, m time.Month, d int) int64 {
		return time.Date(y, m, d, 0, 0, 0, 0, loc).UnixMilli()
	}
	require.Len(t, results, 2)
	require.Equal(t, dayStart(2026, 3, 3), results[0]["timestamp"])
	require.InDelta(t, 30.0, results[0]["value"], 0.0001)
	require.Equal(t, dayStart(2026, 3, 1), results[1]["timestamp"])
	require.InDelta(t, 15.0, results[1]["value"], 0.0001)
}

func TestGetAggregatedDataWithTimeSumAndMaxAggregateFunctions(t *testing.T) {
	db := setupTelemetryStatisticsTestDB(t)

	loc := time.Local
	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, loc).UnixMilli()

	rows := []*model.TelemetryData{
		{DeviceID: "device-a", Key: "power", T: ts, NumberV: numberV(5)},
		{DeviceID: "device-a", Key: "power", T: ts + 1000, NumberV: numberV(7)},
	}
	require.NoError(t, db.Create(rows).Error)

	// aggregateTimeWindows 以 endTime 所在自然日的下一天 00:00 为锚点往回切窗口，
	// 所以 endTime 必须落在 3/1 当天（而不是 3/1 24:00 之后）才能让 limit=1 的窗口覆盖 ts。
	startTime := time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UnixMilli()
	endTime := time.Date(2026, 3, 1, 23, 0, 0, 0, loc).UnixMilli()
	limit := 1

	sumResults, err := getAggregatedDataWithTime("device-a", "power", startTime, endTime, "sum", &limit, "day")
	require.NoError(t, err)
	require.Len(t, sumResults, 1)
	require.InDelta(t, 12.0, sumResults[0]["value"], 0.0001)

	maxResults, err := getAggregatedDataWithTime("device-a", "power", startTime, endTime, "max", &limit, "day")
	require.NoError(t, err)
	require.Len(t, maxResults, 1)
	require.InDelta(t, 7.0, maxResults[0]["value"], 0.0001)
}

func TestGetAggregatedDataWithTimeNoDataReturnsEmpty(t *testing.T) {
	setupTelemetryStatisticsTestDB(t)

	loc := time.Local
	startTime := time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UnixMilli()
	endTime := time.Date(2026, 3, 4, 0, 0, 0, 0, loc).UnixMilli()
	limit := 3

	results, err := getAggregatedDataWithTime("device-missing", "temperature", startTime, endTime, "avg", &limit, "day")
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestQueryAggregateTimeWindowsBatchSingleRoundTrip(t *testing.T) {
	db := setupTelemetryStatisticsTestDB(t)

	loc := time.UTC
	windows := []telemetryWindow{
		{startMS: time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UnixMilli(), endMS: time.Date(2026, 3, 2, 0, 0, 0, 0, loc).UnixMilli()},
		{startMS: time.Date(2026, 3, 2, 0, 0, 0, 0, loc).UnixMilli(), endMS: time.Date(2026, 3, 3, 0, 0, 0, 0, loc).UnixMilli()},
		{startMS: time.Date(2026, 3, 3, 0, 0, 0, 0, loc).UnixMilli(), endMS: time.Date(2026, 3, 4, 0, 0, 0, 0, loc).UnixMilli()},
	}

	rows := []*model.TelemetryData{
		{DeviceID: "device-a", Key: "temperature", T: windows[0].startMS + 1000, NumberV: numberV(1)},
		{DeviceID: "device-a", Key: "temperature", T: windows[2].startMS + 1000, NumberV: numberV(3)},
	}
	require.NoError(t, db.Create(rows).Error)

	aggregateFunc, err := aggregateSQLFunction("avg")
	require.NoError(t, err)

	result, err := queryAggregateTimeWindowsBatch("device-a", "temperature", aggregateFunc, windows)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, windows[0].startMS, result[0].Timestamp)
	require.InDelta(t, 1.0, result[0].Value, 0.0001)
	require.Equal(t, windows[2].startMS, result[1].Timestamp)
	require.InDelta(t, 3.0, result[1].Value, 0.0001)
}
