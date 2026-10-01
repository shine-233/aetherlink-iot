package storage

import (
	"cmp"
	"math/rand"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// referenceDeduplicate is the original map-based algorithm: history keeps the
// first row per (device,key,ts); current keeps the newest row per (device,key),
// earliest arrival on ties; both sorted.
func referenceDeduplicate(batch []*telemetryBatchItem) ([]TelemetryData, []TelemetryCurrentData, int) {
	duplicates := 0
	for _, item := range batch {
		_, d := item.convertedRows()
		duplicates += d
	}
	var history []TelemetryData
	seen := map[telemetryPointIdentity]struct{}{}
	latest := map[telemetrySeriesKey]int{}
	for _, item := range batch {
		if item == nil {
			continue
		}
		for _, row := range item.rows {
			id := telemetryIdentityOf(row)
			if _, ok := seen[id]; ok {
				duplicates++
				continue
			}
			seen[id] = struct{}{}
			history = append(history, row)
			s := telemetrySeriesKey{row.DeviceID, row.Key}
			if idx, ok := latest[s]; !ok || row.TS > history[idx].TS {
				latest[s] = len(history) - 1
			}
		}
	}
	var current []TelemetryCurrentData
	for _, idx := range latest {
		current = append(current, telemetryCurrentFromHistory(history[idx]))
	}
	slices.SortFunc(history, func(a, b TelemetryData) int {
		if c := compareTelemetrySeries(a.DeviceID, a.Key, b.DeviceID, b.Key); c != 0 {
			return c
		}
		return cmp.Compare(a.TS, b.TS)
	})
	slices.SortFunc(current, func(a, b TelemetryCurrentData) int {
		return compareTelemetrySeries(a.DeviceID, a.Key, b.DeviceID, b.Key)
	})
	return history, current, duplicates
}

func randomTelemetryBatch(r *rand.Rand) []*telemetryBatchItem {
	n := r.Intn(40)
	batch := make([]*telemetryBatchItem, 0, n)
	for m := 0; m < n; m++ {
		if r.Intn(15) == 0 {
			batch = append(batch, nil)
			continue
		}
		points := make([]TelemetryDataPoint, r.Intn(12))
		for i := range points {
			// Distinct value per point so a wrong winner shows up in the comparison.
			points[i] = TelemetryDataPoint{Key: "k" + strconv.Itoa(r.Intn(6)), Value: float64(m*100 + i)}
		}
		batch = append(batch, &telemetryBatchItem{
			deviceID:  "d" + strconv.Itoa(r.Intn(4)),
			tenantID:  "t",
			timestamp: int64(r.Intn(4)),
			points:    points,
		})
	}
	return batch
}

func cloneTelemetryBatch(batch []*telemetryBatchItem) []*telemetryBatchItem {
	out := make([]*telemetryBatchItem, len(batch))
	for i, item := range batch {
		if item != nil {
			c := *item
			out[i] = &c
		}
	}
	return out
}

func TestDeduplicateAndConvertMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	w := &telemetryWriter{}
	for iter := 0; iter < 2000; iter++ {
		batch := randomTelemetryBatch(r)
		wantH, wantC, wantD := referenceDeduplicate(cloneTelemetryBatch(batch))
		gotH, gotC, gotD := w.deduplicateAndConvert(batch)
		if gotD != wantD || len(gotH) != len(wantH) || len(gotC) != len(wantC) {
			t.Fatalf("iter %d: dup %d/%d history %d/%d current %d/%d", iter, gotD, wantD, len(gotH), len(wantH), len(gotC), len(wantC))
		}
		for i := range wantH {
			if !reflect.DeepEqual(gotH[i], wantH[i]) {
				t.Fatalf("iter %d history[%d]: got %+v want %+v", iter, i, gotH[i], wantH[i])
			}
		}
		for i := range wantC {
			if !reflect.DeepEqual(gotC[i], wantC[i]) {
				t.Fatalf("iter %d current[%d]: got %+v want %+v", iter, i, gotC[i], wantC[i])
			}
		}
	}
}
