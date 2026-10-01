// telemetry_statistics_queries.go owns batch telemetry statistic query assembly.
package dal

import (
	"fmt"
	"strings"
	"time"

	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

var StatisticAggregateWindowMicrosecond = map[string]int64{
	"30s": int64(time.Second * 30 / time.Microsecond),
	"1m":  int64(time.Minute / time.Microsecond),
	"2m":  int64(time.Minute * 2 / time.Microsecond),
	"5m":  int64(time.Minute * 5 / time.Microsecond),
	"10m": int64(time.Minute * 10 / time.Microsecond),
	"30m": int64(time.Minute * 30 / time.Microsecond),
	"1h":  int64(time.Hour / time.Microsecond),
	"3h":  int64(time.Hour * 3 / time.Microsecond),
	"6h":  int64(time.Hour * 6 / time.Microsecond),
	"1d":  int64(time.Hour * 24 / time.Microsecond),
	"7d":  int64(time.Hour * 24 * 7 / time.Microsecond),
	"1mo": int64(time.Hour * 24 * 30 / time.Microsecond),
}

var StatisticAggregateWindowMillisecond = map[string]int64{
	"30s": int64(time.Second * 30 / time.Millisecond),
	"1m":  int64(time.Minute / time.Millisecond),
	"2m":  int64(time.Minute * 2 / time.Millisecond),
	"5m":  int64(time.Minute * 5 / time.Millisecond),
	"10m": int64(time.Minute * 10 / time.Millisecond),
	"30m": int64(time.Minute * 30 / time.Millisecond),
	"1h":  int64(time.Hour / time.Millisecond),
	"3h":  int64(time.Hour * 3 / time.Millisecond),
	"6h":  int64(time.Hour * 6 / time.Millisecond),
	"1d":  int64(time.Hour * 24 / time.Millisecond),
	"7d":  int64(time.Hour * 24 * 7 / time.Millisecond),
	"1mo": int64(time.Hour * 24 * 30 / time.Millisecond),
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetTelemetryStatisticDataByDeviceIds(deviceIds []string, keys []string, timeType string, limit *int, aggregateMethod string) ([]map[string]interface{}, error) {
	if len(deviceIds) != len(keys) {
		return nil, fmt.Errorf("device ID count does not match key count")
	}

	startTime, endTime, err := telemetryStatisticTimeRange(timeType, limit)
	if err != nil {
		return nil, err
	}

	if aggregateMethod == "count" {
		return getTelemetryStatisticCountRowsByBatch(deviceIds, keys, startTime, endTime)
	}

	if aggregateMethod == "diff" {
		return getTelemetryStatisticDiffRowsByBatch(deviceIds, keys, startTime, endTime, timeType)
	}

	if isTelemetryStatisticBatchAggregateMethod(aggregateMethod) {
		return getTelemetryStatisticAggregateRowsByBatch(deviceIds, keys, startTime, endTime, timeType, limit, aggregateMethod)
	}

	var results []map[string]interface{}

	for i, deviceId := range deviceIds {
		row, ok := getTelemetryStatisticDataForDevice(deviceId, keys[i], startTime, endTime, timeType, limit, aggregateMethod)
		if ok {
			results = append(results, row)
		}
	}

	return results, nil
}

func isTelemetryStatisticBatchAggregateMethod(aggregateMethod string) bool {
	switch aggregateMethod {
	case "avg", "sum", "max", "min":
		return true
	default:
		return false
	}
}

func telemetryStatisticTimeRange(timeType string, limit *int) (int64, int64, error) {
	endTime := time.Now().UnixNano() / 1e6
	actualLimit := telemetryWindowLimit(limit)

	switch timeType {
	case "hour":
		return endTime - int64(actualLimit*int(time.Hour.Milliseconds())), endTime, nil
	case "day":
		return endTime - int64(actualLimit*int(24*time.Hour.Milliseconds())), endTime, nil
	case "week":
		return endTime - int64(actualLimit*int(7*24*time.Hour.Milliseconds())), endTime, nil
	case "month":
		return endTime - int64(actualLimit*int(30*24*time.Hour.Milliseconds())), endTime, nil
	case "year":
		return endTime - int64(actualLimit*int(365*24*time.Hour.Milliseconds())), endTime, nil
	default:
		return 0, 0, fmt.Errorf("unsupported telemetry statistic time type: %s", timeType)
	}
}

func getTelemetryStatisticDataForDevice(deviceId, key string, startTime, endTime int64, timeType string, limit *int, aggregateMethod string) (map[string]interface{}, bool) {
	if aggregateMethod == "count" {
		return getTelemetryStatisticCountRow(deviceId, key, startTime, endTime)
	}
	if aggregateMethod == "diff" {
		return getTelemetryStatisticDiffRow(deviceId, key, startTime, endTime, timeType)
	}
	return getTelemetryStatisticAggregateRow(deviceId, key, startTime, endTime, aggregateMethod, limit, timeType)
}

func getTelemetryStatisticCountRow(deviceId, key string, startTime, endTime int64) (map[string]interface{}, bool) {
	count, err := getDataCount(deviceId, key, startTime, endTime)
	if err != nil {
		logrus.Error("query telemetry statistic count failed")
		return nil, false
	}
	return map[string]interface{}{
		"device_id": deviceId,
		"key":       key,
		"count":     count,
	}, true
}

type telemetryStatisticBatchCountRow struct {
	PairOrdinal int   `gorm:"column:pair_ordinal"`
	Count       int64 `gorm:"column:count"`
}

func getTelemetryStatisticCountRowsByBatch(deviceIds []string, keys []string, startTime, endTime int64) ([]map[string]interface{}, error) {
	rows, err := queryTelemetryStatisticBatchCountRows(deviceIds, keys, startTime, endTime)
	if err != nil {
		return nil, err
	}

	counts := make([]int64, len(deviceIds))
	for _, row := range rows {
		if row.PairOrdinal < 0 || row.PairOrdinal >= len(counts) {
			continue
		}
		counts[row.PairOrdinal] = row.Count
	}

	results := make([]map[string]interface{}, 0, len(deviceIds))
	for i := range deviceIds {
		results = append(results, map[string]interface{}{
			"device_id": deviceIds[i],
			"key":       keys[i],
			"count":     counts[i],
		})
	}
	return results, nil
}

func queryTelemetryStatisticBatchCountRows(deviceIds []string, keys []string, startTime, endTime int64) ([]telemetryStatisticBatchCountRow, error) {
	var sql strings.Builder
	args := make([]interface{}, 0, len(deviceIds)*3+2)

	sql.WriteString("WITH requested_pairs(device_id, key, pair_ordinal) AS (VALUES ")
	for i := range deviceIds {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(?, ?, CAST(? AS integer))")
		args = append(args, deviceIds[i], keys[i], i)
	}
	sql.WriteString(") ")
	sql.WriteString("SELECT p.pair_ordinal, COUNT(td.device_id) AS count FROM requested_pairs p ")
	sql.WriteString("LEFT JOIN telemetry_datas td ON td.device_id = p.device_id AND td.key = p.key ")
	sql.WriteString("AND td.ts BETWEEN ? AND ? ")
	sql.WriteString("GROUP BY p.pair_ordinal ORDER BY p.pair_ordinal ASC")
	args = append(args, startTime, endTime)

	var rows []telemetryStatisticBatchCountRow
	err := global.DB.Raw(sql.String(), args...).Scan(&rows).Error
	return rows, err
}

func getTelemetryStatisticDiffRow(deviceId, key string, startTime, endTime int64, timeType string) (map[string]interface{}, bool) {
	diffData, err := getDiffData(deviceId, key, startTime, endTime, timeType)
	if err != nil {
		logrus.Error("query telemetry statistic diff data failed")
		return nil, false
	}
	return map[string]interface{}{
		"device_id": deviceId,
		"key":       key,
		"data":      diffData,
	}, true
}

func getTelemetryStatisticAggregateRow(deviceId, key string, startTime, endTime int64, aggregateMethod string, limit *int, timeType string) (map[string]interface{}, bool) {
	aggregatedData, err := getAggregatedDataWithTime(deviceId, key, startTime, endTime, aggregateMethod, limit, timeType)
	if err != nil {
		logrus.Error("query telemetry aggregate data failed")
		return nil, false
	}
	return map[string]interface{}{
		"device_id": deviceId,
		"key":       key,
		"data":      aggregatedData,
	}, true
}

type telemetryStatisticBatchAggregateRow struct {
	PairOrdinal int     `gorm:"column:pair_ordinal"`
	Timestamp   int64   `gorm:"column:timestamp"`
	Value       float64 `gorm:"column:value"`
}

func getTelemetryStatisticAggregateRowsByBatch(deviceIds []string, keys []string, startTime, endTime int64, timeType string, limit *int, aggregateMethod string) ([]map[string]interface{}, error) {
	aggregateFunc, err := aggregateSQLFunction(aggregateMethod)
	if err != nil {
		return nil, err
	}

	windows := aggregateTimeWindows(startTime, endTime, telemetryWindowLimit(limit), timeType, time.Local)
	resultData := make([][]map[string]interface{}, len(deviceIds))
	for i := range deviceIds {
		resultData[i] = make([]map[string]interface{}, 0, len(windows))
	}

	if len(deviceIds) == 0 || len(windows) == 0 {
		return buildTelemetryStatisticBatchAggregateResults(deviceIds, keys, resultData), nil
	}

	rows, err := queryTelemetryStatisticBatchAggregateRows(deviceIds, keys, windows, aggregateFunc)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.PairOrdinal < 0 || row.PairOrdinal >= len(resultData) {
			continue
		}
		resultData[row.PairOrdinal] = append(resultData[row.PairOrdinal], map[string]interface{}{
			"timestamp": row.Timestamp,
			"value":     row.Value,
		})
	}

	return buildTelemetryStatisticBatchAggregateResults(deviceIds, keys, resultData), nil
}

func queryTelemetryStatisticBatchAggregateRows(deviceIds []string, keys []string, windows []telemetryWindow, aggregateFunc string) ([]telemetryStatisticBatchAggregateRow, error) {
	var sql strings.Builder
	args := make([]interface{}, 0, len(deviceIds)*3+len(windows)*3)

	sql.WriteString("WITH requested_pairs(device_id, key, pair_ordinal) AS (VALUES ")
	for i := range deviceIds {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(?, ?, CAST(? AS integer))")
		args = append(args, deviceIds[i], keys[i], i)
	}
	sql.WriteString("), statistic_windows(window_start, window_end, window_ordinal) AS (VALUES ")
	for i, window := range windows {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(CAST(? AS bigint), CAST(? AS bigint), CAST(? AS integer))")
		args = append(args, window.startMS, window.endMS, i)
	}
	sql.WriteString(") ")
	sql.WriteString("SELECT p.pair_ordinal, w.window_start AS timestamp, ")
	sql.WriteString(aggregateFunc)
	sql.WriteString(" AS value FROM requested_pairs p CROSS JOIN statistic_windows w ")
	sql.WriteString("JOIN telemetry_datas td ON td.device_id = p.device_id AND td.key = p.key AND td.ts BETWEEN w.window_start AND w.window_end ")
	sql.WriteString("AND td.number_v IS NOT NULL AND abs(td.number_v) < 1e15 ")
	sql.WriteString("GROUP BY p.pair_ordinal, w.window_ordinal, w.window_start ")
	sql.WriteString("HAVING ")
	sql.WriteString(aggregateFunc)
	sql.WriteString(" IS NOT NULL ")
	sql.WriteString("ORDER BY p.pair_ordinal ASC, w.window_ordinal ASC")

	var rows []telemetryStatisticBatchAggregateRow
	err := global.DB.Raw(sql.String(), args...).Scan(&rows).Error
	return rows, err
}

func buildTelemetryStatisticBatchAggregateResults(deviceIds []string, keys []string, data [][]map[string]interface{}) []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(deviceIds))
	for i := range deviceIds {
		results = append(results, map[string]interface{}{
			"device_id": deviceIds[i],
			"key":       keys[i],
			"data":      data[i],
		})
	}
	return results
}

func getDataCount(deviceId, key string, startTime, endTime int64) (int64, error) {
	queryBuilder := telemetryDataRangeQuery(deviceId, key, startTime, endTime)

	count, err := queryBuilder.Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// getAggregatedDataWithTime 按 timeType 切出的时间窗口聚合单设备单 key 的遥测数据。
// 窗口之间彼此独立，之前的实现逐窗口下发一次 Raw 查询（例如按天粒度查一个月就是 30 次
// 串行往返），这里改成一次批量 CTE 查询，一次往返拿到所有窗口的聚合结果，
// 复用 queryTelemetryStatisticBatchAggregateRows 已验证过的 VALUES-CTE 思路。
// 结果顺序沿用 aggregateTimeWindows 本身的顺序（从最新窗口到最旧窗口，即时间降序），
// 与旧的逐窗口实现完全一致，调用方不需要跟着改。
func getAggregatedDataWithTime(deviceId, key string, startTime, endTime int64, aggregateMethod string, limit *int, timeType string) ([]map[string]interface{}, error) {
	aggregateFunc, err := aggregateSQLFunction(aggregateMethod)
	if err != nil {
		return nil, err
	}

	windows := aggregateTimeWindows(startTime, endTime, telemetryWindowLimit(limit), timeType, time.Local)
	if len(windows) == 0 {
		return nil, nil
	}

	rows, err := queryAggregateTimeWindowsBatch(deviceId, key, aggregateFunc, windows)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		results = append(results, map[string]interface{}{
			"value":     row.Value,
			"timestamp": row.Timestamp,
		})
	}
	return results, nil
}

type telemetryAggregateWindowRow struct {
	WindowOrdinal int     `gorm:"column:window_ordinal"`
	Timestamp     int64   `gorm:"column:timestamp"`
	Value         float64 `gorm:"column:value"`
}

// queryAggregateTimeWindowsBatch 把单设备单 key 的全部时间窗口打包进一条
// VALUES CTE，一次往返返回每个窗口的聚合值，按 windows 入参的原始顺序排列
// （window_ordinal 就是 windows 的下标，不对时间重新排序）；没有数据落入的
// 窗口不会出现在结果里（HAVING 过滤掉聚合值为 NULL 的分组），与逐窗口查询时
// "result[0]["value"]==nil 则跳过" 的行为保持一致。
func queryAggregateTimeWindowsBatch(deviceId, key, aggregateFunc string, windows []telemetryWindow) ([]telemetryAggregateWindowRow, error) {
	var sql strings.Builder
	args := make([]interface{}, 0, len(windows)*3+2)

	sql.WriteString("WITH statistic_windows(window_start, window_end, window_ordinal) AS (VALUES ")
	for i, window := range windows {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(CAST(? AS bigint), CAST(? AS bigint), CAST(? AS integer))")
		args = append(args, window.startMS, window.endMS, i)
	}
	sql.WriteString(") ")
	sql.WriteString("SELECT w.window_ordinal, w.window_start AS timestamp, ")
	sql.WriteString(aggregateFunc)
	sql.WriteString(" AS value FROM statistic_windows w ")
	sql.WriteString("JOIN telemetry_datas td ON td.device_id = ? AND td.key = ? AND td.ts BETWEEN w.window_start AND w.window_end ")
	sql.WriteString("GROUP BY w.window_ordinal, w.window_start ")
	sql.WriteString("HAVING ")
	sql.WriteString(aggregateFunc)
	sql.WriteString(" IS NOT NULL ")
	sql.WriteString("ORDER BY w.window_ordinal ASC")
	args = append(args, deviceId, key)

	var rows []telemetryAggregateWindowRow
	err := global.DB.Raw(sql.String(), args...).Scan(&rows).Error
	return rows, err
}
