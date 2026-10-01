package storage

import (
	"strconv"
	"testing"
)

// benchTelemetryPoints builds a typical device report: mixed numeric, bool and
// string values, n distinct keys.
func benchTelemetryPoints(n int) []TelemetryDataPoint {
	points := make([]TelemetryDataPoint, 0, n)
	for i := 0; i < n; i++ {
		var value interface{}
		switch i % 4 {
		case 0, 1:
			value = float64(i) + 0.5
		case 2:
			value = i%2 == 0
		default:
			value = "v" + strconv.Itoa(i)
		}
		points = append(points, TelemetryDataPoint{Key: "k" + strconv.Itoa(i), Value: value})
	}
	return points
}

func benchConvertItem(b *testing.B, n int) {
	points := benchTelemetryPoints(n)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := &telemetryBatchItem{deviceID: "dev-1", tenantID: "t1", timestamp: 1700000000000, points: points}
		rows, _ := item.convertedRows()
		if len(rows) != n {
			b.Fatalf("rows=%d", len(rows))
		}
	}
}

func BenchmarkTelemetryConvertedRows4(b *testing.B)  { benchConvertItem(b, 4) }
func BenchmarkTelemetryConvertedRows16(b *testing.B) { benchConvertItem(b, 16) }

// BenchmarkTelemetryDeduplicateBatch mimics one flush: 200 messages from 50
// devices, 8 keys each.
func BenchmarkTelemetryDeduplicateBatch(b *testing.B) {
	points := benchTelemetryPoints(8)
	w := &telemetryWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]*telemetryBatchItem, 0, 200)
		for m := 0; m < 200; m++ {
			batch = append(batch, &telemetryBatchItem{
				deviceID:  "dev-" + strconv.Itoa(m%50),
				tenantID:  "t1",
				timestamp: 1700000000000 + int64(m/50),
				points:    points,
			})
		}
		history, current, _ := w.deduplicateAndConvert(batch)
		if len(history) != 1600 || len(current) != 400 {
			b.Fatalf("history=%d current=%d", len(history), len(current))
		}
	}
}
