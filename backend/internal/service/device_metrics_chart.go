package service

import (
	"strconv"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

// 读取单设备指标图表数据。
func (*Device) GetDeviceMetricsChart(param *model.GetDeviceMetricsChartReq, userClaims *utils.UserClaims) (any, error) {
	if _, err := ensureTelemetryDeviceReadAccess(param.DeviceID, userClaims); err != nil {
		return nil, err
	}

	data := newDeviceMetricsChartData(param)
	switch param.DataType {
	case "telemetry":
		if err := fillTelemetryMetricChart(&data, param, userClaims); err != nil {
			return nil, err
		}
	case "attribute":
		if err := fillAttributeMetricChart(&data, param); err != nil {
			return nil, err
		}
	case "event":
		if err := fillEventMetricChart(&data, param); err != nil {
			return nil, err
		}
	case "command":
		data.Value = nil
	}

	return data, nil
}

func newDeviceMetricsChartData(param *model.GetDeviceMetricsChartReq) model.DeviceMetricsChartData {
	return model.DeviceMetricsChartData{
		DeviceID:          param.DeviceID,
		DataType:          param.DataType,
		Key:               param.Key,
		AggregateWindow:   param.AggregateWindow,
		AggregateFunction: param.AggregateFunction,
		TimeRange:         param.TimeRange,
	}
}

func fillTelemetryMetricChart(data *model.DeviceMetricsChartData, param *model.GetDeviceMetricsChartReq, userClaims *utils.UserClaims) error {
	telemetryCurrentDataList, err := dal.GetCurrentTelemetryDataEvolutionByKeys(param.DeviceID, []string{param.Key})
	if err != nil {
		return deviceMetricLatestValueError(param.DeviceID, err)
	}
	if len(telemetryCurrentDataList) > 0 {
		setMetricTypedValueAndTimestamp(data, telemetryCurrentDataList[0].BoolV, telemetryCurrentDataList[0].NumberV, telemetryCurrentDataList[0].StringV, telemetryCurrentDataList[0].T)
	}
	if param.DataMode != "history" {
		return nil
	}

	historyData, err := GroupApp.TelemetryData.GetTelemetrServeStatisticData(buildTelemetryMetricHistoryReq(data, param), userClaims)
	if err != nil {
		return err
	}
	points := convertTelemetryMetricHistoryPoints(historyData)
	data.Points = &points
	return nil
}

func fillAttributeMetricChart(data *model.DeviceMetricsChartData, param *model.GetDeviceMetricsChartReq) error {
	attributeData, err := dal.GetAttributeOneKeysByDeviceId(param.DeviceID, param.Key)
	if err != nil {
		return deviceMetricLatestValueError(param.DeviceID, err)
	}
	if attributeData != nil {
		setMetricTypedValueAndTimestamp(data, attributeData.BoolV, attributeData.NumberV, attributeData.StringV, attributeData.T)
	}
	return nil
}

func fillEventMetricChart(data *model.DeviceMetricsChartData, param *model.GetDeviceMetricsChartReq) error {
	eventData, err := dal.GetEventDataOneKeysByDeviceId(param.DeviceID, param.Key)
	if err != nil {
		return deviceMetricLatestValueError(param.DeviceID, err)
	}
	if eventData != nil {
		var v interface{} = *eventData.Datum
		data.Value = &v
	}
	return nil
}

func setMetricTypedValueAndTimestamp(data *model.DeviceMetricsChartData, boolV *bool, numberV *float64, stringV *string, timestampSource time.Time) {
	if boolV != nil {
		var v interface{} = *boolV
		data.Value = &v
	} else if numberV != nil {
		var v interface{} = *numberV
		data.Value = &v
	} else if stringV != nil {
		var v interface{} = *stringV
		data.Value = &v
	}
	timestamp := timestampSource.Unix() * 1000
	data.Timestamp = &timestamp
}

func buildTelemetryMetricHistoryReq(data *model.DeviceMetricsChartData, param *model.GetDeviceMetricsChartReq) *model.GetTelemetryStatisticReq {
	req := &model.GetTelemetryStatisticReq{
		DeviceId: param.DeviceID,
		Key:      param.Key,
	}
	if param.AggregateWindow != nil {
		req.AggregateWindow = *param.AggregateWindow
		if req.AggregateWindow != "no_aggregate" {
			if param.AggregateFunction != nil {
				req.AggregateFunction = *param.AggregateFunction
			} else {
				req.AggregateFunction = "avg"
				data.AggregateFunction = &req.AggregateFunction
			}
		}
	} else {
		req.AggregateWindow = "no_aggregate"
		data.AggregateWindow = &req.AggregateWindow
	}
	if param.TimeRange != nil {
		req.TimeRange = *param.TimeRange
	} else {
		req.TimeRange = "last_1h"
		data.TimeRange = &req.TimeRange
	}
	return req
}

func convertTelemetryMetricHistoryPoints(historyData any) []model.DataPoint {
	hData, ok := historyData.([]map[string]interface{})
	if !ok {
		return []model.DataPoint{}
	}
	points := make([]model.DataPoint, 0, len(hData))
	for _, value := range hData {
		point, ok := convertTelemetryMetricHistoryPoint(value)
		if ok {
			points = append(points, point)
		}
	}
	return points
}

func convertTelemetryMetricHistoryPoint(value map[string]interface{}) (model.DataPoint, bool) {
	timestamp, ok := value["x"].(int64)
	if !ok {
		return model.DataPoint{}, false
	}
	point := model.DataPoint{T: timestamp}
	if yVal, ok := value["y"]; ok {
		point.V = metricPointFloatValue(yVal)
	}
	return point, true
}

func metricPointFloatValue(value interface{}) float64 {
	switch val := value.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

func deviceMetricLatestValueError(deviceID string, err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"error": "get device metrics latest value failed:" + err.Error(),
		"id":    deviceID,
	})
}
