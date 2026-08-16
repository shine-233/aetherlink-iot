// telemetry_statistics_queries.go owns batch telemetry statistic query assembly.
package dal

import (
	"fmt"
	"strings"
	"time"

	query "aetherlink-iot/backend/internal/query"
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

func getDataRange(deviceId, key string, startTime, endTime int64, limit *int) ([]map[string]interface{}, error) {
	q := query.TelemetryData
	queryBuilder := telemetryDataRangeQuery(deviceId, key, startTime, endTime)
	queryBuilder = queryBuilder.Order(q.T.Desc())

	if limit != nil {
		queryBuilder = queryBuilder.Limit(*limit)
	}

	var data []map[string]interface{}
	err := queryBuilder.Select(q.T.As("timestamp"), q.NumberV.As("value")).Scan(&data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getAggregatedData(deviceId, key string, startTime, endTime int64, aggregateMethod string, limit *int) (interface{}, error) {
	q := query.TelemetryData
	queryBuilder := telemetryDataRangeQuery(deviceId, key, startTime, endTime)

	var result []map[string]interface{}
	var err error

	switch aggregateMethod {
	case "avg":
		err = queryBuilder.Select(q.NumberV.Avg().As("value")).Scan(&result)
	case "sum":
		err = queryBuilder.Select(q.NumberV.Sum().As("value")).Scan(&result)
	case "max":
		err = queryBuilder.Select(q.NumberV.Max().As("value")).Scan(&result)
	case "min":
		err = queryBuilder.Select(q.NumberV.Min().As("value")).Scan(&result)
	default:
		return nil, fmt.Errorf("unsupported telemetry aggregate method: %s", aggregateMethod)
	}

	if err != nil {
		return nil, err
	}

	if len(result) > 0 && result[0]["value"] != nil {
		return result[0]["value"], nil
	}

	return 0, nil
}

const (
	telemetryDataRangeWhereSQL = "device_id = ? AND key = ? AND ts BETWEEN ? AND ?"
)

func getAggregatedDataWithTime(deviceId, key string, startTime, endTime int64, aggregateMethod string, limit *int, timeType string) ([]map[string]interface{}, error) {
	aggregateFunc, err := aggregateSQLFunction(aggregateMethod)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, window := range aggregateTimeWindows(startTime, endTime, telemetryWindowLimit(limit), timeType, time.Local) {
		row, ok, err := queryAggregateTimeWindow(deviceId, key, aggregateFunc, window)
		if err != nil {
			return nil, err
		}
		if ok {
			results = append(results, row)
		}
	}

	return results, nil
}

func queryAggregateTimeWindow(deviceId, key, aggregateFunc string, window telemetryWindow) (map[string]interface{}, bool, error) {
	sql := fmt.Sprintf(`
			SELECT
				%s as value,
				%d as timestamp
			FROM telemetry_datas
			WHERE %s
		`, aggregateFunc, window.startMS, telemetryDataRangeWhereSQL)

	var result []map[string]interface{}
	err := global.DB.Raw(sql, telemetryDataRangeSQLArgs(deviceId, key, window.startMS, window.endMS)...).Scan(&result)
	if err.Error != nil {
		return nil, false, err.Error
	}
	if len(result) == 0 || result[0]["value"] == nil {
		return nil, false, nil
	}
	return result[0], true, nil
}

func telemetryDataRangeSQLArgs(deviceId, key string, startTime, endTime int64) []interface{} {
	return []interface{}{deviceId, key, startTime, endTime}
}
