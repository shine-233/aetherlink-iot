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
	n := len(item.points)
	rows := make([]TelemetryData, 0, n)
	// One backing slab per item holds every column value, so the nullable
	// *bool/*float64/*string fields point into it instead of each value
	// escaping to its own heap allocation. Rows never mutate the values, and
	// the slab lives exactly as long as the rows that reference it.
	slab := make([]telemetryValueCell, n)
	duplicates := 0
	// Small reports (the common case) dedup by scanning the rows built so far;
	// that beats hashing for a handful of keys and allocates nothing.
	var seen map[string]struct{}
	if n > telemetryLinearDedupLimit {
		seen = make(map[string]struct{}, n)
	}
	for i, point := range item.points {
		if seen != nil {
			if _, exists := seen[point.Key]; exists {
				duplicates++
				continue
			}
			seen[point.Key] = struct{}{}
		} else if telemetryRowsContainKey(rows, point.Key) {
			duplicates++
			continue
		}
		boolV, numberV, stringV := slab[i].set(point.Value)
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
//
// Implementation: sort lightweight row references by (device,key,ts,arrival)
// and scan once. Duplicate identities become adjacent with the earliest arrival
// first (first-writer-wins), and after dedup the newest row of a series is the
// last of its run, so no per-batch hash maps are needed.
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

	refs := make([]telemetryRowRef, 0, total)
	for _, item := range batch {
		if item == nil {
			continue
		}
		for i := range item.rows {
			refs = append(refs, telemetryRowRef{row: &item.rows[i], seq: len(refs)})
		}
	}
	slices.SortFunc(refs, func(a, b telemetryRowRef) int {
		if c := compareTelemetrySeries(a.row.DeviceID, a.row.Key, b.row.DeviceID, b.row.Key); c != 0 {
			return c
		}
		if c := cmp.Compare(a.row.TS, b.row.TS); c != 0 {
			return c
		}
		return cmp.Compare(a.seq, b.seq)
	})

	historyData := make([]TelemetryData, 0, len(refs))
	series := 0
	for _, ref := range refs {
		if n := len(historyData); n > 0 {
			prev := &historyData[n-1]
			if prev.DeviceID == ref.row.DeviceID && prev.Key == ref.row.Key {
				if prev.TS == ref.row.TS {
					duplicates++
					continue
				}
			} else {
				series++
			}
		} else {
			series++
		}
		historyData = append(historyData, *ref.row)
	}

	currentData := make([]TelemetryCurrentData, 0, series)
	for i := range historyData {
		row := &historyData[i]
		if i+1 < len(historyData) && historyData[i+1].DeviceID == row.DeviceID && historyData[i+1].Key == row.Key {
			continue
		}
		currentData = append(currentData, telemetryCurrentFromHistory(*row))
	}
	return historyData, currentData, duplicates
}

// telemetryRowRef points at a converted row; seq is its arrival order in the
// batch and breaks ties so the sort is total and deterministic.
type telemetryRowRef struct {
	row *TelemetryData
	seq int
}

func compareTelemetrySeries(aDevice, aKey, bDevice, bKey string) int {
	if c := cmp.Compare(aDevice, bDevice); c != 0 {
		return c
	}
	return cmp.Compare(aKey, bKey)
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

// telemetryLinearDedupLimit is the point count up to which convertedRows
// deduplicates keys with a linear scan instead of a map.
const telemetryLinearDedupLimit = 8

func telemetryRowsContainKey(rows []TelemetryData, key string) bool {
	for i := range rows {
		if rows[i].Key == key {
			return true
		}
	}
	return false
}

// telemetryValueCell is slab storage for one converted value. set has the
// same mapping as convertValue but returns pointers into the cell.
type telemetryValueCell struct {
	b bool
	f float64
	s string
}

func (c *telemetryValueCell) set(value interface{}) (*bool, *float64, *string) {
	switch v := value.(type) {
	case bool:
		c.b = v
		return &c.b, nil, nil
	case int:
		c.f = float64(v)
	case int32:
		c.f = float64(v)
	case int64:
		c.f = float64(v)
	case float32:
		c.f = float64(v)
	case float64:
		c.f = v
	case string:
		c.s = v
		return nil, nil, &c.s
	default:
		_, _, stringV := convertValue(v)
		c.s = *stringV
		return nil, nil, &c.s
	}
	return nil, &c.f, nil
}
