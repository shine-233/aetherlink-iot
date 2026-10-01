// 文件用途：遥测统计的时间窗口/聚合函数校验与单设备单指标统计查询。
// 核心逻辑：校验请求参数与聚合窗口最小间隔，调用 dal 取数；需要导出时转交 exportToCSV。
// 关键注意事项：统计口径变化会影响用户报表，聚合函数、空值和时间边界必须稳定。
// 相关文件：CSV 导出见 telemetry_statistic_csv.go；多设备统计见 telemetry_statistic_by_device.go。
package service

import (
	"fmt"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// Telemetry statistic requests support raw and aggregated windows.
// The validation helpers below enforce the same minimum aggregate intervals used by the UI.
const maxNoAggregateTelemetryStatisticPoints = 10000

func (*TelemetryData) GetTelemetrServeStatisticData(req *model.GetTelemetryStatisticReq, claims *utils.UserClaims) (any, error) {
	if err := validateTelemetryStatisticReq(req); err != nil {
		return nil, err
	}

	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims); err != nil {
		return nil, err
	}

	if err := processTimeRange(req); err != nil {
		return nil, err
	}

	rspData, err := fetchTelemetryData(req)
	if err != nil {
		return nil, err
	}

	if !req.IsExport {
		if len(rspData) == 0 {
			return []map[string]interface{}{}, nil
		}
		return rspData, nil
	}

	data, err := exportToCSV(req, rspData)
	if err != nil {
		return nil, telemetryStatisticDBError(err)
	}
	return data, nil
}

func validateTelemetryStatisticReq(req *model.GetTelemetryStatisticReq) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "request is required")
	}
	req.DeviceId = strings.TrimSpace(req.DeviceId)
	req.Key = strings.TrimSpace(req.Key)
	req.TimeRange = strings.TrimSpace(req.TimeRange)
	req.AggregateWindow = strings.TrimSpace(req.AggregateWindow)
	req.AggregateFunction = strings.TrimSpace(req.AggregateFunction)

	if req.DeviceId == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	if req.Key == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "key is required")
	}
	if req.TimeRange == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "time_range is required")
	}
	if req.AggregateWindow == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "aggregate_window is required")
	}
	return normalizeTelemetryStatisticAggregateFunction(req)
}

func normalizeTelemetryStatisticAggregateFunction(req *model.GetTelemetryStatisticReq) error {
	if req.AggregateWindow == "no_aggregate" {
		req.AggregateFunction = ""
		return nil
	}
	if req.AggregateFunction == "" {
		req.AggregateFunction = "avg"
		return nil
	}
	switch req.AggregateFunction {
	case "avg", "max", "min", "sum", "diff":
		return nil
	default:
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported aggregate_function")
	}
}

func processTimeRange(req *model.GetTelemetryStatisticReq) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "request is required")
	}

	if req.AggregateWindow == "no_aggregate" && req.EndTime-req.StartTime > 24*time.Hour.Milliseconds() {
		return errcode.New(207001)
	}

	if req.TimeRange == "custom" {
		if req.StartTime == 0 || req.EndTime == 0 || req.StartTime > req.EndTime {
			return errcode.New(207002)
		}
		return nil
	}

	duration, ok := telemetryStatisticTimeRangeDuration(req.TimeRange)
	if !ok {
		return errcode.WithVars(207003, map[string]interface{}{
			"time_range": req.TimeRange,
		})
	}

	now := time.Now()
	req.EndTime = now.UnixNano() / 1e6
	req.StartTime = now.Add(-duration).UnixNano() / 1e6
	return nil
}

func telemetryStatisticTimeRangeDuration(timeRange string) (time.Duration, bool) {
	timeRanges := map[string]time.Duration{
		"last_5m":  5 * time.Minute,
		"last_15m": 15 * time.Minute,
		"last_30m": 30 * time.Minute,
		"last_1h":  time.Hour,
		"last_3h":  3 * time.Hour,
		"last_6h":  6 * time.Hour,
		"last_12h": 12 * time.Hour,
		"last_24h": 24 * time.Hour,
		"last_3d":  72 * time.Hour,
		"last_7d":  7 * 24 * time.Hour,
		"last_15d": 15 * 24 * time.Hour,
		"last_30d": 30 * 24 * time.Hour,
		"last_60d": 60 * 24 * time.Hour,
		"last_90d": 90 * 24 * time.Hour,
		"last_6m":  180 * 24 * time.Hour,
		"last_1y":  365 * 24 * time.Hour,
	}
	duration, ok := timeRanges[timeRange]
	return duration, ok
}

func fetchTelemetryData(req *model.GetTelemetryStatisticReq) ([]map[string]interface{}, error) {
	if req.AggregateWindow == "no_aggregate" {
		data, err := dal.GetTelemetrStatisticDataWithLimit(
			req.DeviceId,
			req.Key,
			req.StartTime,
			req.EndTime,
			maxNoAggregateTelemetryStatisticPoints+1,
		)
		if err != nil {
			return nil, telemetryStatisticDBError(err)
		}
		if len(data) > maxNoAggregateTelemetryStatisticPoints {
			return nil, errcode.NewWithMessage(
				errcode.CodeParamError,
				fmt.Sprintf(
					"no_aggregate telemetry statistic is limited to %d points; narrow the time range or choose an aggregate_window",
					maxNoAggregateTelemetryStatisticPoints,
				),
			)
		}
		return data, nil
	}

	if err := validateAggregateWindow(req.StartTime, req.EndTime, req.AggregateWindow); err != nil {
		return nil, err
	}

	return dal.GetTelemetrStatisticaAgregationData(
		req.DeviceId,
		req.Key,
		req.StartTime,
		req.EndTime,
		dal.StatisticAggregateWindowMillisecond[req.AggregateWindow],
		req.AggregateFunction,
	)
}

// AggregateRule defines the minimum aggregate interval for a time range.
type AggregateRule struct {
	Days         int
	MinInterval  string
	FriendlyDesc string
}

func validateAggregateWindow(startTime, endTime int64, aggregateWindow string) error {
	if !isSupportedAggregateWindow(aggregateWindow) {
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported aggregate_window")
	}

	days := int((endTime - startTime) / (24 * 60 * 60 * 1000))
	rules := []AggregateRule{
		{365, "7d", "1 year"},
		{180, "1d", "6 months"},
		{90, "6h", "90 days"},
		{60, "3h", "60 days"},
		{30, "1h", "30 days"},
		{15, "30m", "15 days"},
		{7, "10m", "7 days"},
		{3, "5m", "3 days"},
		{1, "2m", "1 day"},
	}

	for _, rule := range rules {
		if days > rule.Days && !isValidInterval(aggregateWindow, rule.MinInterval) {
			return errcode.WithVars(207004, map[string]interface{}{
				"time_range":         rule.FriendlyDesc,
				"min_interval":       rule.MinInterval,
				"current_time_range": fmt.Sprintf("%s to %s (%d days)", formatTime(startTime), formatTime(endTime), days),
				"aggregate_window":   aggregateWindow,
			})
		}
	}

	return nil
}

func isSupportedAggregateWindow(aggregateWindow string) bool {
	_, ok := dal.StatisticAggregateWindowMillisecond[aggregateWindow]
	return ok
}

func isValidInterval(current, minInterval string) bool {
	weights := map[string]int{
		"30s": 1,
		"1m":  2,
		"2m":  3,
		"5m":  4,
		"10m": 5,
		"30m": 6,
		"1h":  7,
		"3h":  8,
		"6h":  9,
		"1d":  10,
		"7d":  11,
		"1mo": 12,
	}

	currentWeight, exists := weights[current]
	if !exists {
		return false
	}

	minWeight, exists := weights[minInterval]
	if !exists {
		return false
	}

	return currentWeight >= minWeight
}

func formatTime(timestamp int64) string {
	return time.Unix(timestamp/1000, 0).Format("2006-01-02 15:04:05")
}
