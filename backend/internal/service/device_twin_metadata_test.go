package service

import (
	"testing"
	"time"
)

func TestBuildDeviceTwinRowsCarriesPersistedMetadata(t *testing.T) {
	desiredAt := time.Date(2026, 7, 19, 1, 2, 3, 0, time.FixedZone("offset", 8*60*60))
	expiresAt := desiredAt.Add(time.Hour)
	reportedAt := desiredAt.Add(5 * time.Minute)
	revision := "expected-data-1"

	rows := buildDeviceTwinRows(
		[]twinExpectedRow{{
			key:              "temperature",
			label:            "temperature",
			source:           "telemetry",
			desired:          22.0,
			status:           "pending",
			comparable:       true,
			desiredUpdatedAt: twinTimePointer(desiredAt),
			desiredExpiresAt: twinTimePointer(expiresAt),
			desiredRevision:  &revision,
		}},
		map[string]twinReportedEntry{
			"temperature": {value: 21.0, reportedAt: twinTimePointer(reportedAt)},
		},
		nil,
	)

	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.DesiredUpdatedAt == nil || !row.DesiredUpdatedAt.Equal(desiredAt.UTC()) {
		t.Fatalf("desired_updated_at = %v, want %v", row.DesiredUpdatedAt, desiredAt.UTC())
	}
	if row.DesiredExpiresAt == nil || !row.DesiredExpiresAt.Equal(expiresAt.UTC()) {
		t.Fatalf("desired_expires_at = %v, want %v", row.DesiredExpiresAt, expiresAt.UTC())
	}
	if row.ReportedAt == nil || !row.ReportedAt.Equal(reportedAt.UTC()) {
		t.Fatalf("reported_at = %v, want %v", row.ReportedAt, reportedAt.UTC())
	}
	if row.DesiredRevision == nil || *row.DesiredRevision != revision {
		t.Fatalf("desired_revision = %v, want %s", row.DesiredRevision, revision)
	}
	if row.LastWriteSource == nil || *row.LastWriteSource != "reported" {
		t.Fatalf("last_write_source = %v, want reported", row.LastWriteSource)
	}
}

func TestTwinLastWriteSourceDoesNotInventMissingTimestamp(t *testing.T) {
	desiredAt := time.Now().UTC()
	if source := twinLastWriteSource(&desiredAt, nil, true); source != nil {
		t.Fatalf("last write source = %v, want nil when reported timestamp is missing", source)
	}
	if source := twinLastWriteSource(nil, nil, false); source != nil {
		t.Fatalf("last write source = %v, want nil without timestamp evidence", source)
	}
}

func TestTwinReportedTimeAcceptsTelemetryUnixMilliseconds(t *testing.T) {
	want := time.Date(2026, 7, 19, 1, 2, 3, 456*int(time.Millisecond), time.UTC)
	got := twinReportedTime(want.UnixMilli())
	if got == nil || !got.Equal(want) {
		t.Fatalf("reported time = %v, want %v", got, want)
	}
}

func TestBuildDeviceTwinRowsFreshnessControlsMatched(t *testing.T) {
	desiredAt := time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC)
	cases := []struct {
		name       string
		desiredAt  *time.Time
		reportedAt *time.Time
		wantFresh  bool
		wantMatch  bool
	}{
		{name: "reported later", desiredAt: twinTimePointer(desiredAt), reportedAt: twinTimePointer(desiredAt.Add(time.Minute)), wantFresh: true, wantMatch: true},
		{name: "reported equal", desiredAt: twinTimePointer(desiredAt), reportedAt: twinTimePointer(desiredAt), wantFresh: true, wantMatch: true},
		{name: "reported earlier", desiredAt: twinTimePointer(desiredAt), reportedAt: twinTimePointer(desiredAt.Add(-time.Minute)), wantFresh: false, wantMatch: false},
		{name: "reported timestamp missing", desiredAt: twinTimePointer(desiredAt), wantFresh: false, wantMatch: false},
		{name: "desired timestamp missing", reportedAt: twinTimePointer(desiredAt.Add(-time.Minute)), wantFresh: true, wantMatch: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := buildDeviceTwinRows(
				[]twinExpectedRow{{key: "temperature", label: "temperature", source: "telemetry", desired: 22.0, comparable: true, desiredUpdatedAt: tc.desiredAt}},
				map[string]twinReportedEntry{"temperature": {value: 22.0, reportedAt: tc.reportedAt}},
				nil,
			)
			if len(rows) != 1 {
				t.Fatalf("rows = %d, want 1", len(rows))
			}
			if got := rows[0].Matched; got != tc.wantMatch {
				t.Fatalf("matched = %v, want %v", got, tc.wantMatch)
			}
			if got := rows[0].ReportedFresh; got != tc.wantFresh {
				t.Fatalf("reported_fresh = %v, want %v", got, tc.wantFresh)
			}
		})
	}
}

func TestDeviceTwinSummaryOldEqualReportedIsNotReady(t *testing.T) {
	desiredAt := time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC)
	rows := buildDeviceTwinRows(
		[]twinExpectedRow{{key: "temperature", label: "temperature", source: "telemetry", desired: 22.0, comparable: true, desiredUpdatedAt: twinTimePointer(desiredAt)}},
		map[string]twinReportedEntry{"temperature": {value: 22.0, reportedAt: twinTimePointer(desiredAt.Add(-time.Minute))}},
		nil,
	)
	matched, unavailable, delta := 0, 0, 0
	for _, row := range rows {
		if row.Matched {
			matched++
		}
		if row.Comparable && row.Reported == nil {
			unavailable++
		}
		if row.Comparable && !row.Matched {
			delta++
		}
	}
	summary := buildDeviceTwinSummary(len(rows), 1, matched, delta, unavailable, 0)
	if summary.ConvergenceStatus == "ready" || summary.MatchedCount != 0 || summary.DeltaCount != 1 {
		t.Fatalf("summary = %+v, want old equal reported not ready", summary)
	}
}
