// 文件用途：集中管理遥测消息转换、批次去重和 current 表查找辅助逻辑。
//
// telemetry_writer.go 负责批处理和持久化编排；本文件保留这些纯转换辅助函数，
// 不改变 writer 对外行为或数据库写入契约。
package storage

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
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

// telemetryPointIdentity is the (device_id,key,ts) history identity. A struct
// key avoids building a fmt.Sprintf string per point on the hot path.
type telemetryPointIdentity struct {
	deviceID string
	key      string
	ts       int64
}

func telemetryIdentityOf(row TelemetryData) telemetryPointIdentity {
	return telemetryPointIdentity{deviceID: row.DeviceID, key: row.Key, ts: row.TS}
}

// telemetrySeriesKey is the (device_id,key) identity of telemetry_current_datas.
type telemetrySeriesKey struct {
	deviceID string
	key      string
}

// telemetryConversionHook is a test seam counting item conversions.
var telemetryConversionHook func(*telemetryBatchItem)

// convertedRows converts the item exactly once and caches the result. Within
// one item every point shares the message timestamp, so duplicate keys are
// duplicate identities and the first occurrence wins (the historical
// first-writer-wins rule). The raw points are released after conversion.
//
// An item is owned by exactly one goroutine at a time (the producer before it
// is appended under bufferMu, the flusher after it is taken), so the cache
// needs no extra synchronization.
func (item *telemetryBatchItem) convertedRows() ([]TelemetryData, int) {
	if item == nil {
		return nil, 0
	}
	if item.converted {
		return item.rows, item.duplicates
	}
	if hook := telemetryConversionHook; hook != nil {
		hook(item)
	}
	rows := make([]TelemetryData, 0, len(item.points))
	duplicates := 0
	var seen map[string]struct{}
	if len(item.points) > 1 {
		seen = make(map[string]struct{}, len(item.points))
	}
	for _, point := range item.points {
		if seen != nil {
			if _, exists := seen[point.Key]; exists {
				duplicates++
				continue
			}
			seen[point.Key] = struct{}{}
		}
		boolV, numberV, stringV := convertValue(point.Value)
		rows = append(rows, TelemetryData{
			DeviceID: item.deviceID,
			Key:      point.Key,
			TS:       item.timestamp,
			BoolV:    boolV,
			NumberV:  numberV,
			StringV:  stringV,
			TenantID: item.tenantID,
		})
	}
	item.rows = rows
	item.duplicates = duplicates
	item.converted = true
	item.points = nil
	return rows, duplicates
}

// deduplicateAndConvert merges the per-item converted rows of a batch into
// history and current rows. History keeps the first row per (device,key,ts);
// current keeps the newest row per (device,key), earliest arrival on ties.
//
// Both outputs are sorted (history by device,key,ts; current by device,key) so
// every flush acquires PostgreSQL row locks in the same order. Two
// transactions touching overlapping series can then only wait, never deadlock.
func (w *telemetryWriter) deduplicateAndConvert(batch []*telemetryBatchItem) (
	[]TelemetryData, []TelemetryCurrentData, int,
) {
	total := 0
	duplicates := 0
	for _, item := range batch {
		rows, itemDuplicates := item.convertedRows()
		total += len(rows)
		duplicates += itemDuplicates
	}

	historyData := make([]TelemetryData, 0, total)
	seen := make(map[telemetryPointIdentity]struct{}, total)
	latest := make(map[telemetrySeriesKey]int, total)
	for _, item := range batch {
		if item == nil {
			continue
		}
		for _, row := range item.rows {
			identity := telemetryIdentityOf(row)
			if _, exists := seen[identity]; exists {
				duplicates++
				continue
			}
			seen[identity] = struct{}{}
			historyData = append(historyData, row)

			series := telemetrySeriesKey{deviceID: row.DeviceID, key: row.Key}
			if index, ok := latest[series]; !ok || row.TS > historyData[index].TS {
				latest[series] = len(historyData) - 1
			}
		}
	}

	currentData := make([]TelemetryCurrentData, 0, len(latest))
	for _, index := range latest {
		currentData = append(currentData, telemetryCurrentFromHistory(historyData[index]))
	}
	sortTelemetryHistory(historyData)
	sortTelemetryCurrent(currentData)
	return historyData, currentData, duplicates
}

func compareTelemetrySeries(aDevice, aKey, bDevice, bKey string) int {
	if c := cmp.Compare(aDevice, bDevice); c != 0 {
		return c
	}
	return cmp.Compare(aKey, bKey)
}

func sortTelemetryHistory(rows []TelemetryData) {
	slices.SortFunc(rows, func(a, b TelemetryData) int {
		if c := compareTelemetrySeries(a.DeviceID, a.Key, b.DeviceID, b.Key); c != 0 {
			return c
		}
		return cmp.Compare(a.TS, b.TS)
	})
}

func sortTelemetryCurrent(rows []TelemetryCurrentData) {
	slices.SortFunc(rows, func(a, b TelemetryCurrentData) int {
		return compareTelemetrySeries(a.DeviceID, a.Key, b.DeviceID, b.Key)
	})
}

func telemetryCurrentLookupKey(deviceID, key string) telemetrySeriesKey {
	return telemetrySeriesKey{deviceID: deviceID, key: key}
}

func buildTelemetryCurrentLookup(currentData []TelemetryCurrentData) map[telemetrySeriesKey]TelemetryCurrentData {
	currentByKey := make(map[telemetrySeriesKey]TelemetryCurrentData, len(currentData))
	for _, row := range currentData {
		key := telemetryCurrentLookupKey(row.DeviceID, row.Key)
		if existing, ok := currentByKey[key]; !ok || row.TS.After(existing.TS) {
			currentByKey[key] = row
		}
	}
	return currentByKey
}

// buildTelemetryCurrentChunk keeps one current row per device/key while using
// the newest value already selected by buildTelemetryCurrentLookup. Input
// history is sorted, so the output keeps the deterministic lock order.
func buildTelemetryCurrentChunk(
	historyData []TelemetryData,
	currentByKey map[telemetrySeriesKey]TelemetryCurrentData,
) []TelemetryCurrentData {
	seen := make(map[telemetrySeriesKey]struct{})
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
