package storage

import (
	"strconv"
	"testing"
)

func TestTelemetryValueCellMatchesConvertValue(t *testing.T) {
	values := []interface{}{true, false, 3, int32(-4), int64(5), float32(1.5), 2.25, "s", "",
		map[string]interface{}{"a": 1}, []interface{}{1, "x"}, nil}
	for _, value := range values {
		wantB, wantF, wantS := convertValue(value)
		var cell telemetryValueCell
		gotB, gotF, gotS := cell.set(value)
		if (wantB == nil) != (gotB == nil) || (wantB != nil && *wantB != *gotB) {
			t.Fatalf("%#v: bool mismatch", value)
		}
		if (wantF == nil) != (gotF == nil) || (wantF != nil && *wantF != *gotF) {
			t.Fatalf("%#v: number mismatch", value)
		}
		if (wantS == nil) != (gotS == nil) || (wantS != nil && *wantS != *gotS) {
			t.Fatalf("%#v: string mismatch", value)
		}
	}
}

// Rows must keep distinct value pointers and first-writer-wins dedup on both
// the linear-scan and map paths.
func TestTelemetryConvertedRowsSlabDedup(t *testing.T) {
	for _, n := range []int{3, telemetryLinearDedupLimit, telemetryLinearDedupLimit + 5} {
		points := make([]TelemetryDataPoint, 0, n+1)
		for i := 0; i < n; i++ {
			points = append(points, TelemetryDataPoint{Key: "k" + strconv.Itoa(i), Value: float64(i)})
		}
		points = append(points, TelemetryDataPoint{Key: "k0", Value: 999.0})
		item := &telemetryBatchItem{deviceID: "d", timestamp: 1, points: points}
		rows, duplicates := item.convertedRows()
		if len(rows) != n || duplicates != 1 {
			t.Fatalf("n=%d rows=%d duplicates=%d", n, len(rows), duplicates)
		}
		for i, row := range rows {
			if row.NumberV == nil || *row.NumberV != float64(i) {
				t.Fatalf("n=%d row %d value=%v", n, i, row.NumberV)
			}
		}
	}
}
