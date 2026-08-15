package dal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

func getDiffData(deviceId, key string, startTime, endTime int64, timeType string) ([]map[string]interface{}, error) {
	loc := time.Local
	alignedEndTime := diffAlignedEndTime(endTime, timeType, loc)
	windowCount, err := diffWindowCount(startTime, alignedEndTime, timeType, loc)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, window := range diffTimeWindows(startTime, endTime, alignedEndTime, windowCount, timeType) {
		row, ok, err := queryDiffTimeWindow(deviceId, key, window)
		if err != nil {
			logrus.Error("query telemetry diff time window failed", err)
			continue
		}
		if ok {
			results = append(results, row)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		ti, _ := results[i]["timestamp"].(int64)
		tj, _ := results[j]["timestamp"].(int64)
		return ti < tj
	})

	return results, nil
}

func queryDiffTimeWindow(deviceId, key string, window telemetryWindow) (map[string]interface{}, bool, error) {
	diffValue, err := getDiffValueInTimeWindow(deviceId, key, window.queryStartMS, window.queryEndMS)
	if err != nil {
		return nil, false, err
	}
	if diffValue == nil {
		return nil, false, nil
	}

	return map[string]interface{}{
		"timestamp": window.startMS,
		"time":      window.startAt.Format("2006-01-02T15:04:05.000+08:00"),
		"value":     *diffValue,
	}, true, nil
}

type telemetryStatisticBatchDiffBoundaryRow struct {
	PairOrdinal   int      `gorm:"column:pair_ordinal"`
	WindowOrdinal int      `gorm:"column:window_ordinal"`
	Timestamp     int64    `gorm:"column:timestamp"`
	LatestNumberV *float64 `gorm:"column:latest_number_v"`
	LatestStringV *string  `gorm:"column:latest_string_v"`
	OldestNumberV *float64 `gorm:"column:oldest_number_v"`
	OldestStringV *string  `gorm:"column:oldest_string_v"`
}

func getTelemetryStatisticDiffRowsByBatch(deviceIds []string, keys []string, startTime, endTime int64, timeType string) ([]map[string]interface{}, error) {
	loc := time.Local
	alignedEndTime := diffAlignedEndTime(endTime, timeType, loc)
	windowCount, err := diffWindowCount(startTime, alignedEndTime, timeType, loc)
	if err != nil {
		return nil, err
	}

	windows := diffTimeWindows(startTime, endTime, alignedEndTime, windowCount, timeType)
	resultData := make([][]map[string]interface{}, len(deviceIds))
	for i := range resultData {
		resultData[i] = make([]map[string]interface{}, 0, len(windows))
	}

	if len(deviceIds) == 0 || len(windows) == 0 {
		return buildTelemetryStatisticBatchDiffResults(deviceIds, keys, resultData), nil
	}

	rows, err := queryTelemetryStatisticBatchDiffRows(deviceIds, keys, windows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.PairOrdinal < 0 || row.PairOrdinal >= len(resultData) {
			continue
		}
		if row.WindowOrdinal < 0 || row.WindowOrdinal >= len(windows) {
			continue
		}

		latestValue, err := extractNumericBoundaryValue(row.LatestNumberV, row.LatestStringV)
		if err != nil {
			logrus.Error("failed to extract latest telemetry value", err)
			continue
		}

		oldestValue, err := extractNumericBoundaryValue(row.OldestNumberV, row.OldestStringV)
		if err != nil {
			logrus.Error("failed to extract oldest telemetry value", err)
			continue
		}

		window := windows[row.WindowOrdinal]
		resultData[row.PairOrdinal] = append(resultData[row.PairOrdinal], map[string]interface{}{
			"timestamp": row.Timestamp,
			"time":      window.startAt.Format("2006-01-02T15:04:05.000+08:00"),
			"value":     latestValue - oldestValue,
		})
	}

	for i := range resultData {
		sort.Slice(resultData[i], func(left, right int) bool {
			leftTime, _ := resultData[i][left]["timestamp"].(int64)
			rightTime, _ := resultData[i][right]["timestamp"].(int64)
			return leftTime < rightTime
		})
	}

	return buildTelemetryStatisticBatchDiffResults(deviceIds, keys, resultData), nil
}

func queryTelemetryStatisticBatchDiffRows(deviceIds []string, keys []string, windows []telemetryWindow) ([]telemetryStatisticBatchDiffBoundaryRow, error) {
	var sql strings.Builder
	args := make([]interface{}, 0, len(deviceIds)*3+len(windows)*5)

	sql.WriteString("WITH requested_pairs(device_id, key, pair_ordinal) AS (VALUES ")
	for i := range deviceIds {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(?, ?, ?)")
		args = append(args, deviceIds[i], keys[i], i)
	}
	sql.WriteString("), statistic_windows(window_start, window_end, query_start, query_end, window_ordinal) AS (VALUES ")
	for i, window := range windows {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(?, ?, ?, ?, ?)")
		args = append(args, window.startMS, window.endMS, window.queryStartMS, window.queryEndMS, i)
	}
	sql.WriteString("), boundary_rows AS (")
	sql.WriteString("SELECT p.pair_ordinal, w.window_ordinal, w.window_start AS timestamp, td.number_v, td.string_v, ")
	sql.WriteString("ROW_NUMBER() OVER (PARTITION BY p.pair_ordinal, w.window_ordinal ORDER BY td.ts DESC) AS latest_rank, ")
	sql.WriteString("ROW_NUMBER() OVER (PARTITION BY p.pair_ordinal, w.window_ordinal ORDER BY td.ts ASC) AS oldest_rank ")
	sql.WriteString("FROM requested_pairs p CROSS JOIN statistic_windows w ")
	sql.WriteString("JOIN telemetry_datas td ON td.device_id = p.device_id AND td.key = p.key AND td.ts BETWEEN w.query_start AND w.query_end")
	sql.WriteString(") ")
	sql.WriteString("SELECT pair_ordinal, window_ordinal, timestamp, ")
	sql.WriteString("MAX(CASE WHEN latest_rank = 1 THEN number_v END) AS latest_number_v, ")
	sql.WriteString("MAX(CASE WHEN latest_rank = 1 THEN string_v END) AS latest_string_v, ")
	sql.WriteString("MAX(CASE WHEN oldest_rank = 1 THEN number_v END) AS oldest_number_v, ")
	sql.WriteString("MAX(CASE WHEN oldest_rank = 1 THEN string_v END) AS oldest_string_v ")
	sql.WriteString("FROM boundary_rows WHERE latest_rank = 1 OR oldest_rank = 1 ")
	sql.WriteString("GROUP BY pair_ordinal, window_ordinal, timestamp ")
	sql.WriteString("ORDER BY pair_ordinal ASC, window_ordinal ASC")

	var rows []telemetryStatisticBatchDiffBoundaryRow
	err := global.DB.Raw(sql.String(), args...).Scan(&rows).Error
	return rows, err
}

func buildTelemetryStatisticBatchDiffResults(deviceIds []string, keys []string, data [][]map[string]interface{}) []map[string]interface{} {
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

func extractNumericBoundaryValue(numberV *float64, stringV *string) (float64, error) {
	if numberV != nil {
		return *numberV, nil
	}
	if stringV != nil && *stringV != "" {
		value, err := strconv.ParseFloat(*stringV, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to number: %v", *stringV, err)
		}
		return value, nil
	}
	return 0, fmt.Errorf("no valid numeric telemetry value found")
}

func getDiffValueInTimeWindow(deviceId, key string, startTime, endTime int64) (*float64, error) {
	queryBuilder := telemetryDataRangeQuery(deviceId, key, startTime, endTime)

	latestData, err := scanTelemetryDiffBoundary(queryBuilder, true)
	if err != nil {
		return nil, err
	}

	oldestData, err := scanTelemetryDiffBoundary(queryBuilder, false)
	if err != nil {
		return nil, err
	}

	if len(latestData) == 0 || len(oldestData) == 0 {
		return nil, nil
	}

	latestValue, err := extractNumericValue(latestData[0])
	if err != nil {
		logrus.Error("failed to extract latest telemetry value", err)
		return nil, nil
	}

	oldestValue, err := extractNumericValue(oldestData[0])
	if err != nil {
		logrus.Error("failed to extract oldest telemetry value", err)
		return nil, nil
	}

	diff := latestValue - oldestValue
	return &diff, nil
}

func scanTelemetryDiffBoundary(queryBuilder query.ITelemetryDataDo, latest bool) ([]map[string]interface{}, error) {
	q := query.TelemetryData
	orderedQuery := queryBuilder.Select(q.NumberV.As("number_v"), q.StringV.As("string_v"))
	if latest {
		orderedQuery = orderedQuery.Order(q.T.Desc())
	} else {
		orderedQuery = orderedQuery.Order(q.T.Asc())
	}

	var data []map[string]interface{}
	err := orderedQuery.Limit(1).Scan(&data)
	return data, err
}

func extractNumericValue(data map[string]interface{}) (float64, error) {
	if numberV, exists := data["number_v"]; exists && numberV != nil {
		if val, ok := numberV.(float64); ok {
			return val, nil
		}
	}

	if stringV, exists := data["string_v"]; exists && stringV != nil {
		if strVal, ok := stringV.(string); ok && strVal != "" {
			if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
				return floatVal, nil
			} else {
				return 0, fmt.Errorf("cannot convert string %q to number: %v", strVal, err)
			}
		}
	}

	return 0, fmt.Errorf("no valid numeric telemetry value found")
}
