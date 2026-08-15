package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/pkg/global"
)

type attributeEventMigrationField struct {
	name       string
	column     string
	typeOf     reflect.Type
	primaryKey bool
}

func TestAttributeEventDurabilityMigrationContract(t *testing.T) {
	if global.VERSION_NUMBER < 39 {
		t.Fatalf("VERSION_NUMBER = %d, want at least 39", global.VERSION_NUMBER)
	}

	sqlDir := attributeEventMigrationSQLDir(t)
	for _, version := range []int{35, 36, 37, 38, 39} {
		path := filepath.Join(sqlDir, strconv.Itoa(version)+".sql")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required migration %d is missing: %v", version, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(sqlDir, "37.sql"))
	if err != nil {
		t.Fatalf("read 37.sql: %v", err)
	}
	sql := compactAttributeEventMigrationSQL(string(raw))
	claimRaw, err := os.ReadFile(filepath.Join(sqlDir, "39.sql"))
	if err != nil {
		t.Fatalf("read 39.sql: %v", err)
	}
	claimSQL := compactAttributeEventMigrationSQL(string(claimRaw))

	if got, want := strings.Count(sql, "create table "), strings.Count(sql, "create table if not exists "); got != want {
		t.Fatalf("37.sql has %d CREATE TABLE statements but only %d are idempotent", got, want)
	}
	if got, want := strings.Count(sql, "create index "), strings.Count(sql, "create index if not exists "); got != want {
		t.Fatalf("37.sql has %d CREATE INDEX statements but only %d are idempotent", got, want)
	}

	receiptDDL := attributeEventMigrationTableDDL(t, sql, "uplink_storage_receipts")
	assertAttributeEventMigrationContains(t, receiptDDL, []string{
		"id varchar(36) not null",
		"fingerprint char(64) not null",
		"data_type varchar(16) not null",
		"device_id varchar(36) not null",
		"tenant_id varchar(36) not null",
		"ts bigint not null",
		"payload jsonb not null",
		"created_at timestamptz not null default now()",
		"constraint uplink_storage_receipts_pkey primary key (id)",
		"check (data_type = 'attribute')",
	})
	if strings.Contains(receiptDDL, "'event'") {
		t.Fatal("uplink_storage_receipts must remain attribute-only")
	}

	deadLetterDDL := attributeEventMigrationTableDDL(t, sql, "uplink_storage_dead_letters")
	assertAttributeEventMigrationContains(t, deadLetterDDL, []string{
		"id varchar(36) not null",
		"data_type varchar(16) not null",
		"device_id varchar(36) not null",
		"tenant_id varchar(36) not null",
		"ts bigint not null",
		"payload jsonb not null",
		"status varchar(16) not null default 'pending'",
		"attempts int4 not null default 0",
		"last_error text null",
		"next_retry_at timestamptz null",
		"created_at timestamptz not null default now()",
		"updated_at timestamptz not null default now()",
		"constraint uplink_storage_dead_letters_pkey primary key (id)",
		"check (data_type in ('attribute', 'event'))",
		"check (status in ('pending', 'retrying', 'processing', 'resolved', 'dead'))",
		"check (attempts >= 0)",
	})

	assertAttributeEventMigrationContains(t, sql, []string{
		"create index if not exists uplink_storage_dead_letters_replay_idx on public.uplink_storage_dead_letters using btree (status, next_retry_at, created_at, id)",
		"create index if not exists uplink_storage_dead_letters_device_ts_idx on public.uplink_storage_dead_letters using btree (device_id, ts desc)",
		"create index if not exists uplink_storage_receipts_device_ts_idx on public.uplink_storage_receipts using btree (device_id, ts desc)",
	})
	assertAttributeEventMigrationContains(t, claimSQL, []string{
		"add column if not exists claim_token varchar(36) null",
		"add column if not exists lease_until timestamptz null",
		"where status = 'processing'",
		"status = 'processing' and claim_token is not null and char_length(claim_token) = 36 and lease_until is not null",
		"status <> 'processing' and claim_token is null and lease_until is null",
		"create index if not exists uplink_storage_dead_letters_claim_idx on public.uplink_storage_dead_letters using btree ( status, next_retry_at, lease_until, created_at, id )",
	})

	assertAttributeEventMigrationModel(t, reflect.TypeOf(uplinkStorageReceipt{}), []attributeEventMigrationField{
		{name: "ID", column: "id", typeOf: reflect.TypeOf(""), primaryKey: true},
		{name: "Fingerprint", column: "fingerprint", typeOf: reflect.TypeOf("")},
		{name: "DataType", column: "data_type", typeOf: reflect.TypeOf(DataType(""))},
		{name: "DeviceID", column: "device_id", typeOf: reflect.TypeOf("")},
		{name: "TenantID", column: "tenant_id", typeOf: reflect.TypeOf("")},
		{name: "TS", column: "ts", typeOf: reflect.TypeOf(int64(0))},
		{name: "Payload", column: "payload", typeOf: reflect.TypeOf(json.RawMessage(nil))},
		{name: "CreatedAt", column: "created_at", typeOf: reflect.TypeOf(time.Time{})},
	})
	assertAttributeEventMigrationModel(t, reflect.TypeOf(uplinkStorageDeadLetter{}), []attributeEventMigrationField{
		{name: "ID", column: "id", typeOf: reflect.TypeOf(""), primaryKey: true},
		{name: "DataType", column: "data_type", typeOf: reflect.TypeOf(DataType(""))},
		{name: "DeviceID", column: "device_id", typeOf: reflect.TypeOf("")},
		{name: "TenantID", column: "tenant_id", typeOf: reflect.TypeOf("")},
		{name: "TS", column: "ts", typeOf: reflect.TypeOf(int64(0))},
		{name: "Payload", column: "payload", typeOf: reflect.TypeOf(json.RawMessage(nil))},
		{name: "Status", column: "status", typeOf: reflect.TypeOf("")},
		{name: "Attempts", column: "attempts", typeOf: reflect.TypeOf(int(0))},
		{name: "LastError", column: "last_error", typeOf: reflect.TypeOf("")},
		{name: "NextRetryAt", column: "next_retry_at", typeOf: reflect.TypeOf((*time.Time)(nil))},
		{name: "ClaimToken", column: "claim_token", typeOf: reflect.TypeOf((*string)(nil))},
		{name: "LeaseUntil", column: "lease_until", typeOf: reflect.TypeOf((*time.Time)(nil))},
		{name: "CreatedAt", column: "created_at", typeOf: reflect.TypeOf(time.Time{})},
		{name: "UpdatedAt", column: "updated_at", typeOf: reflect.TypeOf(time.Time{})},
	})
}

func attributeEventMigrationSQLDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve attribute/event migration test source path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "sql")
}

func compactAttributeEventMigrationSQL(sql string) string {
	return strings.Join(strings.Fields(strings.ToLower(sql)), " ")
}

func attributeEventMigrationTableDDL(t *testing.T, sql string, table string) string {
	t.Helper()
	prefix := "create table if not exists public." + table + " ("
	start := strings.Index(sql, prefix)
	if start < 0 {
		t.Fatalf("37.sql is missing idempotent table %s", table)
	}
	remainder := sql[start:]
	end := strings.Index(remainder, ");")
	if end < 0 {
		t.Fatalf("37.sql table %s has no closing statement", table)
	}
	return remainder[:end+2]
}

func assertAttributeEventMigrationContains(t *testing.T, haystack string, contracts []string) {
	t.Helper()
	for _, contract := range contracts {
		if !strings.Contains(haystack, contract) {
			t.Errorf("attribute/event migration is missing contract %q", contract)
		}
	}
}

func assertAttributeEventMigrationModel(
	t *testing.T,
	model reflect.Type,
	contracts []attributeEventMigrationField,
) {
	t.Helper()
	if model.NumField() != len(contracts) {
		t.Fatalf("%s has %d fields, migration contract describes %d", model.Name(), model.NumField(), len(contracts))
	}
	for _, contract := range contracts {
		field, ok := model.FieldByName(contract.name)
		if !ok {
			t.Errorf("%s is missing model field %s", model.Name(), contract.name)
			continue
		}
		if field.Type != contract.typeOf {
			t.Errorf("%s.%s type = %s, want %s", model.Name(), field.Name, field.Type, contract.typeOf)
		}
		options := attributeEventMigrationGORMOptions(field.Tag.Get("gorm"))
		if options["column"] != contract.column {
			t.Errorf("%s.%s column = %q, want %q", model.Name(), field.Name, options["column"], contract.column)
		}
		_, primaryKey := options["primaryKey"]
		if primaryKey != contract.primaryKey {
			t.Errorf("%s.%s primaryKey = %t, want %t", model.Name(), field.Name, primaryKey, contract.primaryKey)
		}
	}
}

func attributeEventMigrationGORMOptions(tag string) map[string]string {
	options := make(map[string]string)
	for _, option := range strings.Split(tag, ";") {
		parts := strings.SplitN(option, ":", 2)
		if len(parts) == 1 {
			options[parts[0]] = ""
			continue
		}
		options[parts[0]] = parts[1]
	}
	return options
}
