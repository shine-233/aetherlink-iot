// 文件用途：多设备遥测统计查询与图表数据装配。
// 核心逻辑：校验设备/指标配对与时间类型，做租户与设备级访问鉴权，取回聚合结果后按
// count / diff / 时序三种口径装成 model.ChartValue。
// 关键注意事项：DeviceIds 与 Keys 必须一一对应；鉴权用 requireTelemetryClaims 与
// hasTelemetryTenantAccess，任何口径变化都会影响前端图表，格式化函数不得随意改动。
package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func (*TelemetryData) ServeMsgCountByTenantId(tenantId string) (int64, error) {
	cnt, err := dal.GetTelemetryDataCountByTenantId(tenantId)
	if err != nil {
		return 0, dbError(err)
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
	return dbError(err)
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
