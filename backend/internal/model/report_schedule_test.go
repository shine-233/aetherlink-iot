package model

import (
	"reflect"
	"testing"
	"time"
)

func TestCreateReportScheduleRequestEnabledDefaultsTrue(t *testing.T) {
	if !((CreateReportScheduleRequest{}).EnabledOrDefault()) {
		t.Fatal("omitted enabled must default true")
	}
	value := false
	if (CreateReportScheduleRequest{Enabled: &value}).EnabledOrDefault() {
		t.Fatal("explicit false must be preserved")
	}
}

func TestPublicReportDTOsExcludeInternalEvidence(t *testing.T) {
	forbidden := map[string]bool{
		"ClaimToken":         true,
		"LeaseUntil":         true,
		"Payload":            true,
		"PayloadDigest":      true,
		"IdempotencyKeyHash": true,
		"RequestFingerprint": true,
		"LastError":          true,
		"RawError":           true,
	}
	for _, value := range []any{CreateReportScheduleRequest{}, UpdateReportScheduleRequest{}, ReportRunResponse{}, ReportRunActionResponse{}} {
		typeOf := reflect.TypeOf(value)
		for index := 0; index < typeOf.NumField(); index++ {
			if forbidden[typeOf.Field(index).Name] {
				t.Fatalf("public DTO %s exposes internal field %s", typeOf.Name(), typeOf.Field(index).Name)
			}
		}
	}
}

func TestReportRunToResponseProjectsCanonicalOverallStatuses(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	for _, test := range []struct {
		name       string
		generation string
		delivery   string
		want       string
	}{
		{name: "queued", generation: ReportGenerationStatusPending, delivery: ReportDeliveryStatusPending, want: "queued"},
		{name: "generation running", generation: ReportGenerationStatusProcessing, delivery: ReportDeliveryStatusPending, want: "running"},
		{name: "delivery running", generation: ReportGenerationStatusSucceeded, delivery: ReportDeliveryStatusRetrying, want: "running"},
		{name: "succeeded", generation: ReportGenerationStatusSucceeded, delivery: ReportDeliveryStatusAccepted, want: "succeeded"},
		{name: "generation failed", generation: ReportGenerationStatusFailed, delivery: ReportDeliveryStatusPending, want: "failed"},
		{name: "delivery failed", generation: ReportGenerationStatusSucceeded, delivery: ReportDeliveryStatusFailed, want: "failed"},
		{name: "ambiguous", generation: ReportGenerationStatusSucceeded, delivery: ReportDeliveryStatusAmbiguous, want: "ambiguous"},
	} {
		t.Run(test.name, func(t *testing.T) {
			run := &ReportScheduleRun{ID: "run-1", ScheduleID: "schedule-1", Trigger: ReportRunTriggerManual, WindowStartAt: now.Add(-time.Hour), WindowEndAt: now, GenerationStatus: test.generation, CreatedAt: now}
			delivery := &ReportScheduleDelivery{RunID: run.ID, Status: test.delivery}
			if got := run.ToResponse(delivery).OverallStatus; got != test.want {
				t.Fatalf("overall status = %q, want %q", got, test.want)
			}
		})
	}
}

func TestReportRunToResponseUsesCodedErrorsOnly(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	code := "SMTP_TIMEOUT"
	run := &ReportScheduleRun{
		ID: "run-1", ScheduleID: "schedule-1", Trigger: ReportRunTriggerManual,
		WindowStartAt: now.Add(-time.Hour), WindowEndAt: now,
		GenerationStatus: ReportGenerationStatusSucceeded, CreatedAt: now,
	}
	delivery := &ReportScheduleDelivery{
		RunID: run.ID, Status: ReportDeliveryStatusAmbiguous, AttemptCount: 1, LastError: &code,
	}
	response := run.ToResponse(delivery)
	if response.OverallStatus != "ambiguous" || !response.DuplicateDeliveryRisk || response.DeliveryErrorCode != code {
		t.Fatalf("unexpected response: %+v", response)
	}
}
