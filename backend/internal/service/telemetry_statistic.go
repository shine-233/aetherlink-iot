// 文件用途：维护遥测统计聚合和多设备指标查询服务。
// 核心逻辑：按设备、指标、时间窗口和聚合函数计算统计结果，供图表和报表使用。
// 关键注意事项：统计口径变化会影响用户报表，聚合函数、空值和时间边界必须稳定。
// 重构建议：拆出聚合参数校验和查询仓储，补齐权限、时区、空数据和大窗口测试。
// telemetry_statistic.go owns telemetry statistics and export behavior.
//
// It builds aggregate telemetry views and CSV/export artifacts for dashboards,
// reports, and device-analysis pages. Keep time-window and file-output behavior
// documented before changing it.
package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// Telemetry statistic requests support raw and aggregated windows.
// The validation helpers below enforce the same minimum aggregate intervals used by the UI.
const maxNoAggregateTelemetryStatisticPoints = 10000
const telemetryStatisticExportDir = "./files/excel/telemetry/"

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
func exportToCSV(req *model.GetTelemetryStatisticReq, data []map[string]interface{}) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, errcode.New(202100) // 没有可导出的遥测统计数据。
	}

	fileName := telemetryStatisticCSVFileName(req)
	filePath := filepath.Join(telemetryStatisticExportDir, fileName)

	file, err := createTelemetryStatisticCSVFile(filePath)
	if err != nil {
		return nil, err
	}
	fileClosed := false
	defer func() {
		if !fileClosed {
			_ = file.Close()
		}
	}()

	writer := csv.NewWriter(file)
	if err := writeTelemetryStatisticCSV(writer, data); err != nil {
		return nil, err
	}
	if err := finalizeTelemetryStatisticCSV(writer, file); err != nil {
		return nil, err
	}
	fileClosed = true

	logrus.Info("CSV export completed")

	return map[string]interface{}{
		"file_name": fileName,
		"file_path": filePath,
	}, nil
}

func telemetryStatisticCSVFileName(req *model.GetTelemetryStatisticReq) string {
	return fmt.Sprintf(
		"%s_%s_%d_%d.csv",
		sanitizeTelemetryStatisticFileToken(req.DeviceId),
		sanitizeTelemetryStatisticFileToken(req.Key),
		req.StartTime,
		req.EndTime,
	)
}

func sanitizeTelemetryStatisticFileToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "unknown"
	}

	var builder strings.Builder
	for _, char := range token {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}

	cleaned := strings.Trim(builder.String(), "._-")
	if cleaned == "" {
		return "unknown"
	}
	if len(cleaned) > 80 {
		return cleaned[:80]
	}
	return cleaned
}

func createTelemetryStatisticCSVFile(filePath string) (*os.File, error) {
	if err := os.MkdirAll(telemetryStatisticExportDir, os.ModePerm); err != nil {
		return nil, errcode.WithVars(202101, map[string]interface{}{
			"error": err.Error(),
		})
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, errcode.WithVars(202102, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return file, nil
}

func writeTelemetryStatisticCSV(writer *csv.Writer, data []map[string]interface{}) error {
	if err := writer.Write([]string{"timestamp", "value"}); err != nil {
		return errcode.WithVars(202103, map[string]interface{}{
			"error": err.Error(),
		})
	}

	for _, row := range data {
		values, err := telemetryStatisticCSVRow(row)
		if err != nil {
			return err
		}
		if err := writer.Write(values); err != nil {
			return errcode.WithVars(202104, map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
	return nil
}

func telemetryStatisticCSVRow(row map[string]interface{}) ([]string, error) {
	timestamp, ok := row["x"].(int64)
	if !ok {
		return nil, errcode.New(202105) // 时间戳字段类型异常。
	}

	value, ok := row["y"].(float64)
	if !ok {
		return nil, errcode.New(202106)
	}

	t := time.Unix(0, timestamp*int64(time.Millisecond))
	return []string{t.Format("2006-01-02 15:04:05.000"), fmt.Sprintf("%.3f", value)}, nil
}

func finalizeTelemetryStatisticCSV(writer *csv.Writer, file *os.File) error {
	writer.Flush()
	if err := writer.Error(); err != nil {
		return errcode.WithVars(202104, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if err := file.Sync(); err != nil {
		return errcode.WithVars(202104, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if err := file.Close(); err != nil {
		return errcode.WithVars(202104, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return nil
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

func (*TelemetryData) ServeMsgCountByTenantId(tenantId string) (int64, error) {
	cnt, err := dal.GetTelemetryDataCountByTenantId(tenantId)
	if err != nil {
		return 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return cnt, err
}

// GetTelemetryStatisticDataByDeviceIds returns chart values for multiple device/key telemetry statistic queries.
func (*TelemetryData) GetTelemetryStatisticDataByDeviceIds(req *model.GetTelemetryStatisticByDeviceIdReq, claims *utils.UserClaims) (interface{}, error) {
	if err := validateTelemetryStatisticByDeviceIDReq(req); err != nil {
		return nil, err
	}

	if err := ensureTelemetryStatisticDeviceAccess(req.DeviceIds, claims); err != nil {
		return nil, err
	}

	results, err := fetchTelemetryStatisticByDeviceIDs(req)
	if err != nil {
		return nil, telemetryStatisticDBError(err)
	}

	return buildTelemetryStatisticChartData(results, req), nil
}

func validateTelemetryStatisticByDeviceIDReq(req *model.GetTelemetryStatisticByDeviceIdReq) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "request is required")
	}

	if len(req.DeviceIds) != len(req.Keys) {
		return errcode.WithVars(errcode.CodeParamError, map[string]interface{}{
			"error":            "device id count must match key count",
			"device_ids_count": len(req.DeviceIds),
			"keys_count":       len(req.Keys),
		})
	}

	if len(req.DeviceIds) == 0 {
		return errcode.WithVars(errcode.CodeParamError, map[string]interface{}{
			"error": "device ids and keys cannot be empty",
		})
	}

	for i := range req.DeviceIds {
		req.DeviceIds[i] = strings.TrimSpace(req.DeviceIds[i])
		req.Keys[i] = strings.TrimSpace(req.Keys[i])
		if req.DeviceIds[i] == "" {
			return errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
		}
		if req.Keys[i] == "" {
			return errcode.NewWithMessage(errcode.CodeParamError, "key is required")
		}
	}

	req.TimeType = strings.TrimSpace(req.TimeType)
	if !isSupportedStatisticTimeType(req.TimeType) {
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported time_type")
	}

	req.AggregateMethod = strings.TrimSpace(req.AggregateMethod)
	if !isSupportedStatisticAggregateMethod(req.AggregateMethod) {
		return errcode.NewWithMessage(errcode.CodeParamError, "unsupported aggregate_method")
	}

	return nil
}

func isSupportedStatisticTimeType(timeType string) bool {
	switch timeType {
	case "hour", "day", "week", "month", "year":
		return true
	default:
		return false
	}
}

func isSupportedStatisticAggregateMethod(aggregateMethod string) bool {
	switch aggregateMethod {
	case "avg", "sum", "max", "min", "count", "diff":
		return true
	default:
		return false
	}
}

func ensureTelemetryStatisticDeviceAccess(deviceIDs []string, claims *utils.UserClaims) error {
	if err := requireTelemetryClaims(claims, telemetryReadPermissionMessage); err != nil {
		return err
	}

	normalizedIDs := make([]string, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		normalizedID, err := requireTelemetryDeviceID(deviceID)
		if err != nil {
			return err
		}
		normalizedIDs = append(normalizedIDs, normalizedID)
	}

	devicesByID, err := dal.GetDevicesByIDsUnscoped(normalizedIDs)
	if err != nil {
		return err
	}

	for _, deviceID := range normalizedIDs {
		deviceInfo := devicesByID[deviceID]
		if !hasTelemetryTenantAccess(deviceInfo, claims, true) {
			return errcode.NewWithMessage(errcode.CodeNoPermission, telemetryReadPermissionMessage)
		}
	}
	return nil
}

func fetchTelemetryStatisticByDeviceIDs(req *model.GetTelemetryStatisticByDeviceIdReq) ([]map[string]interface{}, error) {
	return dal.GetTelemetryStatisticDataByDeviceIds(
		req.DeviceIds,
		req.Keys,
		req.TimeType,
		req.Limit,
		req.AggregateMethod,
	)
}

func telemetryStatisticDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func buildTelemetryStatisticChartData(results []map[string]interface{}, req *model.GetTelemetryStatisticByDeviceIdReq) []model.ChartValue {
	chartData := make([]model.ChartValue, 0)
	for _, result := range results {
		key, _ := result["key"].(string)
		chartData = append(chartData, chartValuesForStatisticResult(key, result, req)...)
	}
	return chartData
}

func chartValuesForStatisticResult(key string, result map[string]interface{}, req *model.GetTelemetryStatisticByDeviceIdReq) []model.ChartValue {
	switch req.AggregateMethod {
	case "count":
		return countStatisticChartValues(key, result, req.TimeType)
	case "diff":
		return diffStatisticChartValues(key, result)
	default:
		return timeSeriesStatisticChartValues(key, result, req.TimeType)
	}
}

func countStatisticChartValues(key string, result map[string]interface{}, timeType string) []model.ChartValue {
	countVal, ok := result["count"].(int64)
	if !ok {
		return nil
	}
	return []model.ChartValue{{
		Key:   key,
		Time:  formatCountStatisticTime(time.Now(), timeType),
		Value: float64(countVal),
	}}
}

func diffStatisticChartValues(key string, result map[string]interface{}) []model.ChartValue {
	dataSlice, ok := result["data"].([]map[string]interface{})
	if !ok {
		return nil
	}

	values := make([]model.ChartValue, 0, len(dataSlice))
	for _, item := range dataSlice {
		timeStr, _ := item["time"].(string)
		value, _ := item["value"].(float64)
		values = append(values, model.ChartValue{
			Key:   key,
			Time:  timeStr,
			Value: value,
		})
	}
	return values
}

func timeSeriesStatisticChartValues(key string, result map[string]interface{}, timeType string) []model.ChartValue {
	dataSlice, ok := result["data"].([]map[string]interface{})
	if !ok {
		return nil
	}

	values := make([]model.ChartValue, 0, len(dataSlice))
	for _, item := range dataSlice {
		timestamp, _ := item["timestamp"].(int64)
		values = append(values, model.ChartValue{
			Key:   key,
			Time:  formatStatisticTimestamp(timestamp, timeType),
			Value: statisticValueAsFloat(item["value"]),
		})
	}
	return values
}

func formatCountStatisticTime(now time.Time, timeType string) string {
	switch timeType {
	case "hour":
		return now.Format("2006-01-02 15:00:00")
	case "day", "week":
		return now.Format("2006-01-02")
	case "month":
		return now.Format("2006-01")
	case "year":
		return now.Format("2006")
	default:
		return now.Format("2006-01-02 15:04:05")
	}
}

func formatStatisticTimestamp(timestamp int64, timeType string) string {
	if timestamp == 0 {
		return ""
	}

	t := time.Unix(0, timestamp*int64(time.Millisecond))
	switch timeType {
	case "hour":
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).Format("2006-01-02T15:04:05.000-07:00")
	case "day", "week":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Format("2006-01-02T15:04:05.000-07:00")
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02T15:04:05.000-07:00")
	case "year":
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02T15:04:05.000-07:00")
	default:
		return t.Format("2006-01-02T15:04:05.000-07:00")
	}
}

func statisticValueAsFloat(value interface{}) float64 {
	switch val := value.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	default:
		return 0
	}
}
