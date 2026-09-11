package dal

import (
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestNormalizeReportPage(t *testing.T) {
	tests := []struct {
		page, size         int
		wantPage, wantSize int
	}{
		{0, 0, 1, defaultReportPageSize},
		{2, 7, 2, 7},
		{1, maxReportPageSize + 1, 1, maxReportPageSize},
	}
	for _, test := range tests {
		page, size := normalizeReportPage(test.page, test.size)
		if page != test.wantPage || size != test.wantSize {
			t.Fatalf("normalizeReportPage(%d, %d) = (%d, %d), want (%d, %d)", test.page, test.size, page, size, test.wantPage, test.wantSize)
		}
	}
}

func TestReportIdempotencyHashesAreStableAndSeparated(t *testing.T) {
	first := HashReportIdempotencyKey(" key ")
	second := HashReportIdempotencyKey("key")
	if first != second {
		t.Fatal("idempotency key hashing must trim surrounding whitespace")
	}
	fingerprintA := HashReportRequestFingerprint([]byte("manual:schedule-a"))
	fingerprintB := HashReportRequestFingerprint([]byte("manual:schedule-b"))
	if fingerprintA == fingerprintB {
		t.Fatal("different request shapes must have different fingerprints")
	}
	if !errors.Is(ErrReportIdempotencyConflict, ErrReportIdempotencyConflict) {
		t.Fatal("idempotency conflict must be a typed sentinel")
	}
}

func TestFirstFutureReportOccurrenceCoalescesStaleSlots(t *testing.T) {
	schedule := &model.ReportSchedule{ID: "schedule-1"}
	start := time.Unix(0, 0).UTC()
	now := start.Add(3*time.Hour + 30*time.Minute)
	next, missed, lastMissed, err := firstFutureReportOccurrenceBounded(schedule, start, now, func(_ *model.ReportSchedule, after time.Time) (time.Time, error) {
		return after.Add(time.Hour), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if missed != 4 || !next.Equal(start.Add(4*time.Hour)) || !lastMissed.Equal(start.Add(3*time.Hour)) {
		t.Fatalf("next=%s lastMissed=%s missed=%d, want next=%s lastMissed=%s missed=4", next, lastMissed, missed, start.Add(4*time.Hour), start.Add(3*time.Hour))
	}
}

func TestFirstFutureReportOccurrenceRejectsNonAdvancingParser(t *testing.T) {
	schedule := &model.ReportSchedule{ID: "schedule-1"}
	start := time.Unix(0, 0).UTC()
	_, _, err := firstFutureReportOccurrence(schedule, start, start, func(_ *model.ReportSchedule, after time.Time) (time.Time, error) {
		return after, nil
	})
	if err == nil {
		t.Fatal("non-advancing parser must be rejected")
	}
}

func TestFirstFutureReportOccurrenceRejectsUnboundedStaleScan(t *testing.T) {
	schedule := &model.ReportSchedule{ID: "schedule-1"}
	start := time.Unix(0, 0).UTC()
	_, _, _, err := firstFutureReportOccurrenceBounded(schedule, start, start.Add(reportOccurrenceScanHorizon+time.Hour), func(_ *model.ReportSchedule, after time.Time) (time.Time, error) {
		return after.Add(time.Hour), nil
	})
	if err == nil {
		t.Fatal("stale occurrence beyond scan horizon must be rejected without advancing next_run_at")
	}
}

func TestSnapshotReportRunFreezesScheduleDefinition(t *testing.T) {
	now := time.Unix(200, 0).UTC()
	schedule := &model.ReportSchedule{
		ID: "schedule-1", TenantID: "tenant-1", Name: "Daily", Recipients: "ops@example.test",
		DeviceIDs: []string{"device-1"}, Keys: []string{"temperature"}, Format: "csv",
	}
	run := snapshotReportRun(schedule, "run-1", model.ReportRunTriggerManual, nil, nil, now.Add(-time.Hour), now, 0, 3, now)
	schedule.DeviceIDs[0] = "mutated"
	schedule.Keys[0] = "mutated"
	if run.ConfigSnapshot.DeviceIDs[0] != "device-1" || run.ConfigSnapshot.Keys[0] != "temperature" {
		t.Fatalf("run snapshot aliases mutable schedule slices: %+v", run)
	}
}
