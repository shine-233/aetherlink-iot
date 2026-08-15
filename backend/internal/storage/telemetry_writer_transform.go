// 文件用途：集中管理遥测消息转换、批次去重和 current 表查找辅助逻辑。
//
// telemetry_writer.go 负责批处理和持久化编排；本文件保留这些纯转换辅助函数，
// 不改变 writer 对外行为或数据库写入契约。
package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// telemetryBatchItemFromMessage converts the wire-compatible telemetry payload
// into the writer's internal batch item without changing the existing contract.
func telemetryBatchItemFromMessage(msg *Message) (*telemetryBatchItem, error) {
	if msg == nil {
		return nil, fmt.Errorf("telemetry message is nil")
	}

	points, ok := msg.Data.([]TelemetryDataPoint)
	if !ok {
		if dataSlice, sliceOK := msg.Data.([]interface{}); sliceOK {
			points = make([]TelemetryDataPoint, 0, len(dataSlice))
			for _, value := range dataSlice {
				if point, pointOK := value.(TelemetryDataPoint); pointOK {
					points = append(points, point)
				}
			}
		}
		if len(points) == 0 {
			return nil, fmt.Errorf("invalid telemetry data format")
		}
	}

	return &telemetryBatchItem{
		deviceID:           msg.DeviceID,
		tenantID:           msg.TenantID,
		timestamp:          msg.Timestamp,
		points:             points,
		writeAheadPrepared: msg.telemetryWriteAheadPrepared,
	}, nil
}

// deduplicateAndConvert turns writer batches into history and current rows.
// History uses device/key/timestamp identity; current retains the newest value.
func (w *telemetryWriter) deduplicateAndConvert(batch []*telemetryBatchItem) (
	[]TelemetryData, []TelemetryCurrentData, int,
) {
	seen := make(map[string]struct{})
	historyData := make([]TelemetryData, 0, len(batch)*2)
	currentMap := make(map[string]*TelemetryCurrentData)
	duplicates := 0

	for _, item := range batch {
		for _, point := range item.points {
			identity := fmt.Sprintf("%s|%s|%d", item.deviceID, point.Key, item.timestamp)
			if _, exists := seen[identity]; exists {
				duplicates++
				continue
			}
			seen[identity] = struct{}{}

			boolV, numberV, stringV := convertValue(point.Value)
			historyData = append(historyData, TelemetryData{
				DeviceID: item.deviceID,
				Key:      point.Key,
				TS:       item.timestamp,
				BoolV:    boolV,
				NumberV:  numberV,
				StringV:  stringV,
				TenantID: item.tenantID,
			})

			currentKey := telemetryCurrentLookupKey(item.deviceID, point.Key)
			ts := time.UnixMilli(item.timestamp)
			if existing, ok := currentMap[currentKey]; !ok || ts.After(existing.TS) {
				currentMap[currentKey] = &TelemetryCurrentData{
					DeviceID: item.deviceID,
					Key:      point.Key,
					TS:       ts,
					BoolV:    boolV,
					NumberV:  numberV,
					StringV:  stringV,
					TenantID: item.tenantID,
				}
			}
		}
	}

	currentData := make([]TelemetryCurrentData, 0, len(currentMap))
	for _, data := range currentMap {
		currentData = append(currentData, *data)
	}
	return historyData, currentData, duplicates
}

func telemetryCurrentLookupKey(deviceID, key string) string {
	return deviceID + "|" + key
}

func buildTelemetryCurrentLookup(currentData []TelemetryCurrentData) map[string]TelemetryCurrentData {
	currentByKey := make(map[string]TelemetryCurrentData, len(currentData))
	for _, row := range currentData {
		key := telemetryCurrentLookupKey(row.DeviceID, row.Key)
		if existing, ok := currentByKey[key]; !ok || row.TS.After(existing.TS) {
			currentByKey[key] = row
		}
	}
	return currentByKey
}

// buildTelemetryCurrentChunk keeps one current row per device/key while using
// the newest value already selected by buildTelemetryCurrentLookup.
func buildTelemetryCurrentChunk(
	historyData []TelemetryData,
	currentByKey map[string]TelemetryCurrentData,
) []TelemetryCurrentData {
	seen := make(map[string]struct{})
	currentData := make([]TelemetryCurrentData, 0, len(historyData))
	for _, history := range historyData {
		key := telemetryCurrentLookupKey(history.DeviceID, history.Key)
		if _, ok := seen[key]; ok {
			continue
		}
		current, ok := currentByKey[key]
		if !ok {
			continue
		}
		seen[key] = struct{}{}
		currentData = append(currentData, current)
	}
	return currentData
}

func telemetryHistoryPreviewRows(historyData []TelemetryData, limit int) []map[string]interface{} {
	if limit > len(historyData) {
		limit = len(historyData)
	}
	previewRows := make([]map[string]interface{}, 0, limit)
	for i := 0; i < limit; i++ {
		previewRows = append(previewRows, map[string]interface{}{
			"device_id": historyData[i].DeviceID,
			"key":       historyData[i].Key,
			"ts":        historyData[i].TS,
			"tenant_id": historyData[i].TenantID,
		})
	}
	return previewRows
}

// convertValue maps telemetry values to the nullable database columns used by
// history, current-value, attribute-event, and direct-write storage paths.
func convertValue(value interface{}) (*bool, *float64, *string) {
	switch v := value.(type) {
	case bool:
		return &v, nil, nil
	case int:
		f := float64(v)
		return nil, &f, nil
	case int32:
		f := float64(v)
		return nil, &f, nil
	case int64:
		f := float64(v)
		return nil, &f, nil
	case float32:
		f := float64(v)
		return nil, &f, nil
	case float64:
		return nil, &v, nil
	case string:
		return nil, nil, &v
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			s := fmt.Sprintf("%v", v)
			return nil, nil, &s
		}
		s := string(jsonBytes)
		return nil, nil, &s
	}
}
