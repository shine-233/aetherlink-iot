package service

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/authz"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/storage"
	"aetherlink-iot/backend/pkg/utils"
)

func TestNormalizeTelemetryDeadLetterPage(t *testing.T) {
	tests := []struct {
		page, size         int
		wantPage, wantSize int
	}{
		{0, 0, 1, telemetryDeadLetterDefaultPageSize},
		{-3, -1, 1, telemetryDeadLetterDefaultPageSize},
		{2, 50, 2, 50},
		{1, telemetryDeadLetterMaxPageSize + 1, 1, telemetryDeadLetterMaxPageSize},
	}
	for _, tt := range tests {
		page, size := normalizeTelemetryDeadLetterPage(tt.page, tt.size)
		if page != tt.wantPage || size != tt.wantSize {
			t.Fatalf("normalize(%d,%d) = (%d,%d), want (%d,%d)", tt.page, tt.size, page, size, tt.wantPage, tt.wantSize)
		}
	}
}

func TestBuildTelemetryDeadLetterRspFormatsTimes(t *testing.T) {
	created := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	next := created.Add(time.Minute)
	rsp := buildTelemetryDeadLetterRsp(storage.TelemetryDeadLetter{
		ID: "dl-1", Status: storage.TelemetryDeadLetterStatusRetrying, Attempts: 2,
		NextRetryAt: &next, CreatedAt: created, UpdatedAt: created,
	})
	if rsp.ID != "dl-1" || rsp.Attempts != 2 || rsp.Status != storage.TelemetryDeadLetterStatusRetrying {
		t.Fatalf("rsp = %#v", rsp)
	}
	if rsp.NextRetryAt == nil || *rsp.NextRetryAt != "2026-09-30T08:01:00Z" {
		t.Fatalf("next_retry_at = %v", rsp.NextRetryAt)
	}
	if rsp.CreatedAt != "2026-09-30T08:00:00Z" {
		t.Fatalf("created_at = %q", rsp.CreatedAt)
	}
	if got := buildTelemetryDeadLetterRsp(storage.TelemetryDeadLetter{}); got.NextRetryAt != nil {
		t.Fatalf("nil next_retry_at must stay nil, got %v", *got.NextRetryAt)
	}
	if list := buildTelemetryDeadLetterList(nil); list == nil || len(list) != 0 {
		t.Fatalf("empty list must be non-nil empty slice, got %#v", list)
	}
}

func TestTelemetryDeadLetterScopedQueryIsolatesTenants(t *testing.T) {
	db := setupTelemetryDeadLetterServiceTestDB(t)
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	for _, row := range []storage.TelemetryDeadLetter{
		{ID: "a-1", TenantID: "tenant-a", DeviceID: "dev-a", Key: "t", Status: storage.TelemetryDeadLetterStatusPending, CreatedAt: now},
		{ID: "a-2", TenantID: "tenant-a", DeviceID: "dev-a2", Key: "h", Status: storage.TelemetryDeadLetterStatusDead, CreatedAt: now},
		{ID: "b-1", TenantID: "tenant-b", DeviceID: "dev-b", Key: "t", Status: storage.TelemetryDeadLetterStatusPending, CreatedAt: now},
	} {
		createTelemetryDeadLetterRow(t, db, row)
	}

	count := func(req *model.GetTelemetryDeadLetterListReq, claims *utils.UserClaims) int64 {
		t.Helper()
		var n int64
		if err := telemetryDeadLetterScopedQuery(req, claims).Count(&n).Error; err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	tenantAdmin := &utils.UserClaims{ID: "u-a", TenantID: "tenant-a", Authority: authz.TenantAdmin}
	if n := count(&model.GetTelemetryDeadLetterListReq{}, tenantAdmin); n != 2 {
		t.Fatalf("tenant admin sees %d rows, want 2", n)
	}
	// A non-sysadmin's tenant filter must not escape their own tenant.
	if n := count(&model.GetTelemetryDeadLetterListReq{TenantID: "tenant-b"}, tenantAdmin); n != 2 {
		t.Fatalf("tenant admin with foreign tenant filter sees %d rows, want own 2", n)
	}
	if n := count(&model.GetTelemetryDeadLetterListReq{Status: " dead ", Key: "h"}, tenantAdmin); n != 1 {
		t.Fatalf("filtered count = %d, want 1", n)
	}

	sysAdmin := &utils.UserClaims{ID: "root", Authority: authz.SysAdmin}
	if n := count(&model.GetTelemetryDeadLetterListReq{}, sysAdmin); n != 3 {
		t.Fatalf("sysadmin sees %d rows, want 3", n)
	}
	if n := count(&model.GetTelemetryDeadLetterListReq{TenantID: " tenant-b "}, sysAdmin); n != 1 {
		t.Fatalf("sysadmin tenant filter sees %d rows, want 1", n)
	}
}
