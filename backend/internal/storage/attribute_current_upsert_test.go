package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestAttributeCurrentUpsertClauseRequiresNondecreasingTimestamp(t *testing.T) {
	upsert := AttributeCurrentUpsertClause()
	if len(upsert.Columns) != 2 || upsert.Columns[0].Name != "device_id" || upsert.Columns[1].Name != "key" {
		t.Fatalf("conflict columns = %#v, want device_id/key", upsert.Columns)
	}

	wantAssignments := []string{"ts", "bool_v", "number_v", "string_v", "tenant_id"}
	if len(upsert.DoUpdates) != len(wantAssignments) {
		t.Fatalf("update assignments = %#v, want %v", upsert.DoUpdates, wantAssignments)
	}
	for i, want := range wantAssignments {
		if upsert.DoUpdates[i].Column.Name != want {
			t.Fatalf("update assignment %d = %q, want %q", i, upsert.DoUpdates[i].Column.Name, want)
		}
	}

	if len(upsert.Where.Exprs) != 1 {
		t.Fatalf("upsert where expressions = %#v, want one timestamp guard", upsert.Where.Exprs)
	}
	expr, ok := upsert.Where.Exprs[0].(clause.Expr)
	if !ok {
		t.Fatalf("upsert timestamp guard = %T, want clause.Expr", upsert.Where.Exprs[0])
	}
	if expr.SQL != "EXCLUDED.ts >= attribute_datas.ts" {
		t.Fatalf("upsert timestamp guard = %q", expr.SQL)
	}
}

func TestAttributeCurrentWritersKeepTimestampMonotonic(t *testing.T) {
	type writeAttribute func(*gorm.DB, time.Time, float64, string) error

	writers := []struct {
		name  string
		write writeAttribute
	}{
		{
			name: "exported direct writer",
			write: func(db *gorm.DB, ts time.Time, value float64, tenantID string) error {
				return NewDirectWriter(db, logrus.New()).WriteAttributeData(context.Background(), &AttributeData{
					ID:       "incoming",
					DeviceID: "device-1",
					Key:      "firmware-version",
					TS:       ts,
					NumberV:  &value,
					TenantID: tenantID,
				})
			},
		},
		{
			name: "message direct writer",
			write: func(db *gorm.DB, ts time.Time, value float64, tenantID string) error {
				writer := newDirectWriter(db, newMetricsCollector())
				_, err := writer.insertAttribute(&Message{
					DeviceID:  "device-1",
					TenantID:  tenantID,
					Timestamp: ts.UnixMilli(),
				}, AttributeDataPoint{Key: "firmware-version", Value: value})
				return err
			},
		},
	}

	tests := []struct {
		name       string
		incomingTS time.Time
		value      float64
		tenantID   string
		wantTS     time.Time
		wantValue  float64
		wantTenant string
	}{
		{
			name:       "older timestamp is ignored",
			incomingTS: time.UnixMilli(1000),
			value:      10,
			tenantID:   "tenant-older",
			wantTS:     time.UnixMilli(2000),
			wantValue:  20,
			wantTenant: "tenant-seed",
		},
		{
			name:       "newer timestamp replaces current",
			incomingTS: time.UnixMilli(3000),
			value:      30,
			tenantID:   "tenant-newer",
			wantTS:     time.UnixMilli(3000),
			wantValue:  30,
			wantTenant: "tenant-newer",
		},
		{
			name:       "equal timestamp replaces current",
			incomingTS: time.UnixMilli(2000),
			value:      25,
			tenantID:   "tenant-equal",
			wantTS:     time.UnixMilli(2000),
			wantValue:  25,
			wantTenant: "tenant-equal",
		},
	}

	for _, writer := range writers {
		writer := writer
		t.Run(writer.name, func(t *testing.T) {
			for _, tt := range tests {
				tt := tt
				t.Run(tt.name, func(t *testing.T) {
					db := setupAttributeCurrentUpsertTestDB(t)
					seedValue := 20.0
					if err := db.Create(&AttributeData{
						ID:       "seed",
						DeviceID: "device-1",
						Key:      "firmware-version",
						TS:       time.UnixMilli(2000),
						NumberV:  &seedValue,
						TenantID: "tenant-seed",
					}).Error; err != nil {
						t.Fatalf("seed attribute current: %v", err)
					}

					if err := writer.write(db, tt.incomingTS, tt.value, tt.tenantID); err != nil {
						t.Fatalf("write attribute current: %v", err)
					}

					var current AttributeData
					if err := db.First(&current, "device_id = ? AND key = ?", "device-1", "firmware-version").Error; err != nil {
						t.Fatalf("load attribute current: %v", err)
					}
					if !current.TS.Equal(tt.wantTS) {
						t.Fatalf("current timestamp = %s, want %s", current.TS, tt.wantTS)
					}
					if current.NumberV == nil || *current.NumberV != tt.wantValue {
						t.Fatalf("current value = %v, want %v", current.NumberV, tt.wantValue)
					}
					if current.TenantID != tt.wantTenant {
						t.Fatalf("current tenant = %q, want %q", current.TenantID, tt.wantTenant)
					}
				})
			}
		})
	}
}

func TestAttributeCurrentWritersOnlyCountStoredRows(t *testing.T) {
	type writerFactory struct {
		name           string
		returnsDBError bool
		build          func(*gorm.DB) (func(time.Time) error, *metricsCollector)
	}

	writers := []writerFactory{
		{
			name:           "exported direct writer",
			returnsDBError: true,
			build: func(db *gorm.DB) (func(time.Time) error, *metricsCollector) {
				writer := NewDirectWriter(db, logrus.New())
				return func(ts time.Time) error {
					value := 30.0
					return writer.WriteAttributeData(context.Background(), &AttributeData{
						ID:       fmt.Sprintf("incoming-%d", ts.UnixMilli()),
						DeviceID: "device-1",
						Key:      "firmware-version",
						TS:       ts,
						NumberV:  &value,
						TenantID: "tenant-1",
					})
				}, writer.metrics
			},
		},
		{
			name:           "message direct writer",
			returnsDBError: true,
			build: func(db *gorm.DB) (func(time.Time) error, *metricsCollector) {
				metrics := newMetricsCollector()
				writer := newDirectWriter(db, metrics)
				return func(ts time.Time) error {
					return writer.writeAttribute(&Message{
						DeviceID:  "device-1",
						TenantID:  "tenant-1",
						Timestamp: ts.UnixMilli(),
						Data: []AttributeDataPoint{{
							Key:   "firmware-version",
							Value: 30.0,
						}},
					})
				}, metrics
			},
		},
	}

	for _, tt := range writers {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			db := setupAttributeCurrentUpsertTestDB(t)
			seedValue := 20.0
			if err := db.Create(&AttributeData{
				ID:       "seed",
				DeviceID: "device-1",
				Key:      "firmware-version",
				TS:       time.UnixMilli(2000),
				NumberV:  &seedValue,
				TenantID: "tenant-1",
			}).Error; err != nil {
				t.Fatalf("seed attribute current: %v", err)
			}

			write, metrics := tt.build(db)
			if err := write(time.UnixMilli(1000)); err != nil {
				t.Fatalf("write stale attribute current: %v", err)
			}
			got := metrics.GetMetrics()
			if got.AttributeWritten != 0 || got.AttributeFailed != 0 {
				t.Fatalf("metrics after stale no-op = written:%d failed:%d, want 0/0", got.AttributeWritten, got.AttributeFailed)
			}

			if err := write(time.UnixMilli(3000)); err != nil {
				t.Fatalf("write newer attribute current: %v", err)
			}
			got = metrics.GetMetrics()
			if got.AttributeWritten != 1 || got.AttributeFailed != 0 {
				t.Fatalf("metrics after stored row = written:%d failed:%d, want 1/0", got.AttributeWritten, got.AttributeFailed)
			}

			if err := db.Migrator().DropTable(&AttributeData{}); err != nil {
				t.Fatalf("drop attribute table: %v", err)
			}
			err := write(time.UnixMilli(4000))
			if tt.returnsDBError && err == nil {
				t.Fatal("database write error was not returned")
			}
			got = metrics.GetMetrics()
			if got.AttributeWritten != 1 || got.AttributeFailed != 1 {
				t.Fatalf("metrics after database error = written:%d failed:%d, want 1/1", got.AttributeWritten, got.AttributeFailed)
			}
		})
	}
}

func setupAttributeCurrentUpsertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&AttributeData{}); err != nil {
		t.Fatalf("migrate attribute current table: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX attribute_datas_device_key_test ON attribute_datas(device_id, key)").Error; err != nil {
		t.Fatalf("create attribute current identity constraint: %v", err)
	}
	return db
}
