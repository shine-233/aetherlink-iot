package storage

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func TestMessageDirectWriterAggregatesAttributeDatabaseFailures(t *testing.T) {
	db := setupAttributeCurrentUpsertTestDB(t)
	if err := db.Exec(`
		CREATE TRIGGER reject_selected_attributes
		BEFORE INSERT ON attribute_datas
		WHEN NEW."key" IN ('humidity', 'pressure')
		BEGIN
			SELECT RAISE(FAIL, 'forced attribute write failure');
		END
	`).Error; err != nil {
		t.Fatalf("create attribute rejection trigger: %v", err)
	}

	metrics := newMetricsCollector(false)
	writer := newDirectWriter(db, metrics)
	err := writer.writeAttribute(&Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		Timestamp: 1000,
		Data: []AttributeDataPoint{
			{Key: "temperature", Value: 21.5},
			{Key: "humidity", Value: 45.0},
			{Key: "pressure", Value: 101.3},
		},
	})
	if err == nil {
		t.Fatal("writeAttribute returned nil after database failures")
	}
	for _, want := range []string{`attribute point 1 ("humidity")`, `attribute point 2 ("pressure")`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("writeAttribute error %q does not contain %q", err, want)
		}
	}

	got := metrics.GetMetrics()
	if got.AttributeWritten != 1 || got.AttributeFailed != 2 {
		t.Fatalf("attribute metrics = written:%d failed:%d, want 1/2", got.AttributeWritten, got.AttributeFailed)
	}
}

func TestStorageProcessMessageReturnsAndReportsDirectWriteFailureOnce(t *testing.T) {
	tests := []struct {
		name          string
		dataType      DataType
		data          interface{}
		prepareFailed func(*testing.T, *gorm.DB)
		wantError     string
		wantWritten   func(Metrics) int64
		wantFailed    func(Metrics) int64
	}{
		{
			name:     "attribute",
			dataType: DataTypeAttribute,
			data: []AttributeDataPoint{
				{Key: "temperature", Value: 21.5},
			},
			prepareFailed: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				if err := db.Migrator().DropTable(&AttributeData{}); err != nil {
					t.Fatalf("drop attribute table: %v", err)
				}
			},
			wantError:   "handle attribute message",
			wantWritten: func(metrics Metrics) int64 { return metrics.AttributeWritten },
			wantFailed:  func(metrics Metrics) int64 { return metrics.AttributeFailed },
		},
		{
			name:     "event",
			dataType: DataTypeEvent,
			data: EventData{
				Identify: "temperature_alarm",
				Data:     json.RawMessage(`{"value": 21.5}`),
			},
			prepareFailed: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				if err := db.AutoMigrate(&EventDataModel{}); err != nil {
					t.Fatalf("migrate event table: %v", err)
				}
				if err := db.Migrator().DropTable(&EventDataModel{}); err != nil {
					t.Fatalf("drop event table: %v", err)
				}
			},
			wantError:   "handle event message",
			wantWritten: func(metrics Metrics) int64 { return metrics.EventWritten },
			wantFailed:  func(metrics Metrics) int64 { return metrics.EventFailed },
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			db := setupAttributeCurrentUpsertTestDB(t)
			tt.prepareFailed(t, db)

			metrics := newMetricsCollector(false)
			logger, logs := newStorageFailureTestLogger()
			service := &storage{
				logger:       logger,
				metrics:      metrics,
				directWriter: newDirectWriter(db, metrics),
			}
			err := service.processMessage(&Message{
				DeviceID:  "device-1",
				TenantID:  "tenant-1",
				DataType:  tt.dataType,
				Timestamp: 1000,
				Data:      tt.data,
			})
			if err == nil {
				t.Fatal("processMessage returned nil after database failure")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("processMessage error = %q, want %q", err, tt.wantError)
			}
			metricsSnapshot := metrics.GetMetrics()
			if got := tt.wantWritten(metricsSnapshot); got != 0 {
				t.Fatalf("written metric = %d after database failure, want 0", got)
			}
			if got := tt.wantFailed(metricsSnapshot); got != 1 {
				t.Fatalf("failed metric = %d, want 1", got)
			}

			logOutput := logs.String()
			if got := strings.Count(logOutput, "storage message handling failed"); got != 1 {
				t.Fatalf("storage boundary failure log count = %d, want 1; logs=%q", got, logOutput)
			}
			if !strings.Contains(logOutput, "data_type="+string(tt.dataType)) {
				t.Fatalf("storage boundary log does not identify data type %q: %q", tt.dataType, logOutput)
			}
			if strings.Contains(logOutput, "insert attribute failed") || strings.Contains(logOutput, "insert event failed") {
				t.Fatalf("direct writer and storage boundary both logged the same failure: %q", logOutput)
			}
		})
	}
}

func newStorageFailureTestLogger() (*logrus.Logger, *bytes.Buffer) {
	logger := logrus.New()
	logs := &bytes.Buffer{}
	logger.SetOutput(logs)
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: true, DisableTimestamp: true})
	return logger, logs
}
