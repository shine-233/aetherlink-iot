package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

type reportTelemetryReaderFunc func(context.Context, string, string, string, int64, int64, int) ([]*model.TelemetryData, error)

func (function reportTelemetryReaderFunc) Read(ctx context.Context, tenantID, deviceID, key string, start, end int64, limit int) ([]*model.TelemetryData, error) {
	return function(ctx, tenantID, deviceID, key, start, end, limit)
}

func TestReportGeneratorUsesSnapshotWindowAndDeterministicOrdering(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	end := start.Add(time.Hour)
	valueOne, valueTwo := 1.5, 2.5
	var calls []string
	processor := &ReportRunProcessor{Telemetry: reportTelemetryReaderFunc(func(_ context.Context, tenantID, deviceID, key string, gotStart, gotEnd int64, limit int) ([]*model.TelemetryData, error) {
		if tenantID != "tenant-1" || gotStart != start.UnixMilli() || gotEnd != end.UnixMilli() {
			t.Fatalf("unexpected snapshot query: tenant=%q start=%d end=%d", tenantID, gotStart, gotEnd)
		}
		if limit < 1 || limit > reportMaxRows+1 {
			t.Fatalf("query limit = %d, want a bounded positive limit", limit)
		}
		calls = append(calls, deviceID+"/"+key)
		if deviceID == "device-a" {
			return []*model.TelemetryData{{T: start.Add(2 * time.Second).UnixMilli(), NumberV: &valueTwo}, {T: start.Add(time.Second).UnixMilli(), NumberV: &valueOne}}, nil
		}
		return nil, nil
	})}
	run := &model.ReportScheduleRun{TenantID: "tenant-1", WindowStartAt: start, WindowEndAt: end, ConfigSnapshot: model.ReportRunConfigSnapshot{DeviceIDs: []string{"device-b", "device-a"}, Keys: []string{"temperature"}, Format: "csv"}}

	payload, rows, code, err := processor.generate(context.Background(), run)
	if err != nil || code != "" || rows != 2 {
		t.Fatalf("generate = rows %d code %q error %v", rows, code, err)
	}
	if strings.Join(calls, ",") != "device-a/temperature,device-b/temperature" {
		t.Fatalf("query order = %v", calls)
	}
	want := "\xEF\xBB\xBFtimestamp,device_id,key,value\n2023-11-14T22:13:21Z,device-a,temperature,1.5\n2023-11-14T22:13:22Z,device-a,temperature,2.5\n"
	if string(payload) != want {
		t.Fatalf("CSV = %q, want %q", string(payload), want)
	}
}

func TestReportGeneratorFailsWholeArtifactOnTelemetryError(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	processor := &ReportRunProcessor{Telemetry: reportTelemetryReaderFunc(func(context.Context, string, string, string, int64, int64, int) ([]*model.TelemetryData, error) {
		return nil, errors.New("database detail that must not become an API error")
	})}
	payload, _, code, err := processor.generate(context.Background(), &model.ReportScheduleRun{TenantID: "tenant", WindowStartAt: start, WindowEndAt: start.Add(time.Hour), ConfigSnapshot: model.ReportRunConfigSnapshot{DeviceIDs: []string{"device"}, Keys: []string{"key"}, Format: "csv"}})
	if err == nil || code != "telemetry_query_failed" || payload != nil {
		t.Fatalf("generate = payload %v code %q error %v", payload, code, err)
	}
}

func TestReportGeneratorDetectsGlobalRowOverflow(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	processor := &ReportRunProcessor{Telemetry: reportTelemetryReaderFunc(func(_ context.Context, _, _, _ string, _, _ int64, limit int) ([]*model.TelemetryData, error) {
		return make([]*model.TelemetryData, limit), nil
	})}
	payload, _, code, err := processor.generate(context.Background(), &model.ReportScheduleRun{TenantID: "tenant", WindowStartAt: start, WindowEndAt: start.Add(time.Hour), ConfigSnapshot: model.ReportRunConfigSnapshot{DeviceIDs: []string{"device"}, Keys: []string{"key"}, Format: "csv"}})
	if err == nil || code != "row_limit_exceeded" || payload != nil {
		t.Fatalf("generate = payload %v code %q error %v", payload, code, err)
	}
}

// The byte cap (reportMaxBytes) is enforced by reportLimitedBuffer but had no
// covering test, so a regression could silently emit truncated artifacts under
// an "ok" code. A single device/key pair emits at most reportMaxRows rows, so
// each row must be wide enough for the CSV payload to cross reportMaxBytes
// before the row limit trips.
func TestReportGeneratorDetectsByteLimit(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	value := strings.Repeat("x", 512)
	processor := &ReportRunProcessor{Telemetry: reportTelemetryReaderFunc(func(_ context.Context, _, _, _ string, _, _ int64, limit int) ([]*model.TelemetryData, error) {
		rows := make([]*model.TelemetryData, limit-1) // stay at, not above, the row limit
		for index := range rows {
			rowValue := value
			rows[index] = &model.TelemetryData{
				T:       start.Add(time.Duration(index) * time.Millisecond).UnixMilli(),
				StringV: &rowValue,
			}
		}
		return rows, nil
	})}
	payload, _, code, err := processor.generate(context.Background(), &model.ReportScheduleRun{TenantID: "tenant", WindowStartAt: start, WindowEndAt: start.Add(time.Hour), ConfigSnapshot: model.ReportRunConfigSnapshot{DeviceIDs: []string{"device"}, Keys: []string{"key"}, Format: "csv"}})
	if err == nil || code != "byte_limit_exceeded" || payload != nil {
		t.Fatalf("generate = payload %v code %q error %v", payload, code, err)
	}
}

func TestReportGenerationEnvelopeClassifiesConfigurationAndSnapshotErrors(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	run := &model.ReportScheduleRun{
		WindowStartAt: start, WindowEndAt: start.Add(time.Hour),
		ConfigSnapshot: model.ReportRunConfigSnapshot{ScheduleName: "Daily", Recipients: "ops@example.test"},
	}
	configurationErr := errors.New("configuration storage unavailable")
	processor := &ReportRunProcessor{EnvelopeFrom: func(context.Context) (string, error) { return "", configurationErr }}
	if _, code, err := processor.generationEnvelope(context.Background(), run, []byte("payload")); !errors.Is(err, configurationErr) || code != "envelope_config_unavailable" {
		t.Fatalf("configuration envelope = code %q error %v", code, err)
	}
	processor.EnvelopeFrom = func(context.Context) (string, error) { return "sender@example.test", nil }
	run.ConfigSnapshot.Recipients = "invalid address"
	if _, code, err := processor.generationEnvelope(context.Background(), run, []byte("payload")); err == nil || code != "invalid_envelope" {
		t.Fatalf("snapshot envelope = code %q error %v", code, err)
	}
}

func reportGenerationClaimFixture() dal.ReportGenerationClaim {
	start := time.Unix(1700000000, 0).UTC()
	return dal.ReportGenerationClaim{
		Token: "claim-token",
		Run: &model.ReportScheduleRun{
			ID: "run-1", TenantID: "tenant-1", WindowStartAt: start, WindowEndAt: start.Add(time.Hour),
			ConfigSnapshot: model.ReportRunConfigSnapshot{
				ScheduleName: "Daily", Recipients: "ops@example.test, audit@example.test",
				DeviceIDs: []string{"device-1"}, Keys: []string{"temperature"}, Format: "csv",
			},
		},
	}
}

func successfulReportTelemetryReader() ReportTelemetryReader {
	return reportTelemetryReaderFunc(func(_ context.Context, _, _, _ string, start, _ int64, _ int) ([]*model.TelemetryData, error) {
		value := 21.5
		return []*model.TelemetryData{{T: start + 1000, NumberV: &value}}, nil
	})
}

func TestGenerateClaimRetriesEnvelopeConfigurationFailure(t *testing.T) {
	claim := reportGenerationClaimFixture()
	configurationErr := errors.New("configuration storage unavailable")
	var retryCalls, failCalls, completeCalls int
	processor := &ReportRunProcessor{
		Telemetry:    successfulReportTelemetryReader(),
		EnvelopeFrom: func(context.Context) (string, error) { return "", configurationErr },
		RetryGeneration: func(_ context.Context, runID, token, code, detail string) error {
			retryCalls++
			if runID != claim.Run.ID || token != claim.Token || code != "envelope_config_unavailable" || detail != configurationErr.Error() {
				t.Fatalf("retry settlement = run %q token %q code %q detail %q", runID, token, code, detail)
			}
			return nil
		},
		FailGeneration: func(context.Context, string, string, string) error {
			failCalls++
			return nil
		},
		CompleteGeneration: func(context.Context, string, string, string, []string, string, string, []byte, int64) error {
			completeCalls++
			return nil
		},
	}

	err := processor.GenerateClaim(context.Background(), claim)
	if !errors.Is(err, configurationErr) {
		t.Fatalf("GenerateClaim error = %v, want configuration error", err)
	}
	if retryCalls != 1 || failCalls != 0 || completeCalls != 0 {
		t.Fatalf("settlement calls = retry %d fail %d complete %d", retryCalls, failCalls, completeCalls)
	}
}

func TestGenerateClaimFailsInvalidImmutableEnvelope(t *testing.T) {
	claim := reportGenerationClaimFixture()
	claim.Run.ConfigSnapshot.Recipients = "invalid address"
	var retryCalls, failCalls, completeCalls int
	processor := &ReportRunProcessor{
		Telemetry:    successfulReportTelemetryReader(),
		EnvelopeFrom: func(context.Context) (string, error) { return "sender@example.test", nil },
		RetryGeneration: func(context.Context, string, string, string, string) error {
			retryCalls++
			return nil
		},
		FailGeneration: func(_ context.Context, runID, token, code string) error {
			failCalls++
			if runID != claim.Run.ID || token != claim.Token || code != "invalid_envelope" {
				t.Fatalf("failure settlement = run %q token %q code %q", runID, token, code)
			}
			return nil
		},
		CompleteGeneration: func(context.Context, string, string, string, []string, string, string, []byte, int64) error {
			completeCalls++
			return nil
		},
	}

	if err := processor.GenerateClaim(context.Background(), claim); err == nil {
		t.Fatal("GenerateClaim accepted an invalid immutable envelope")
	}
	if retryCalls != 0 || failCalls != 1 || completeCalls != 0 {
		t.Fatalf("settlement calls = retry %d fail %d complete %d", retryCalls, failCalls, completeCalls)
	}
}

func TestGenerateClaimCompletesWithImmutableEnvelopeAndPayload(t *testing.T) {
	claim := reportGenerationClaimFixture()
	var completeCalls int
	processor := &ReportRunProcessor{
		Telemetry:    successfulReportTelemetryReader(),
		EnvelopeFrom: func(context.Context) (string, error) { return " sender@example.test ", nil },
		RetryGeneration: func(context.Context, string, string, string, string) error {
			t.Fatal("unexpected retry settlement")
			return nil
		},
		FailGeneration: func(context.Context, string, string, string) error {
			t.Fatal("unexpected failure settlement")
			return nil
		},
		CompleteGeneration: func(_ context.Context, runID, token, from string, recipients []string, messageID, subject string, payload []byte, rows int64) error {
			completeCalls++
			if runID != claim.Run.ID || token != claim.Token || from != "sender@example.test" {
				t.Fatalf("completion identity = run %q token %q from %q", runID, token, from)
			}
			if strings.Join(recipients, ",") != "audit@example.test,ops@example.test" || messageID != reportMessageID(claim.Run.ID) {
				t.Fatalf("completion envelope = recipients %v message ID %q", recipients, messageID)
			}
			if subject != "[AetherLink report] Daily" || rows != 1 || !strings.Contains(string(payload), "device-1,temperature,21.5") {
				t.Fatalf("completion artifact = subject %q rows %d payload %q", subject, rows, payload)
			}
			return nil
		},
	}

	if err := processor.GenerateClaim(context.Background(), claim); err != nil {
		t.Fatalf("GenerateClaim error = %v", err)
	}
	if completeCalls != 1 {
		t.Fatalf("completion calls = %d, want 1", completeCalls)
	}
}

func TestGenerateClaimReturnsSettlementError(t *testing.T) {
	claim := reportGenerationClaimFixture()
	queryErr := errors.New("telemetry unavailable")
	settleErr := errors.New("claim ownership lost")
	processor := &ReportRunProcessor{
		Telemetry: reportTelemetryReaderFunc(func(context.Context, string, string, string, int64, int64, int) ([]*model.TelemetryData, error) {
			return nil, queryErr
		}),
		RetryGeneration: func(_ context.Context, _, _, code, detail string) error {
			if code != "telemetry_query_failed" || detail != queryErr.Error() {
				t.Fatalf("retry settlement = code %q detail %q", code, detail)
			}
			return settleErr
		},
	}

	if err := processor.GenerateClaim(context.Background(), claim); !errors.Is(err, settleErr) {
		t.Fatalf("GenerateClaim error = %v, want settlement error", err)
	}
}

func TestReportScheduleCronAndTimezoneValidation(t *testing.T) {
	if _, _, err := parseReportSchedule("0 8 * * *", "America/New_York"); err != nil {
		t.Fatalf("valid five-field schedule: %v", err)
	}
	if _, _, err := parseReportSchedule("0 0 8 * * *", "Asia/Shanghai"); err != nil {
		t.Fatalf("valid six-field schedule: %v", err)
	}
	for _, test := range []struct{ cron, zone string }{{"0 8 * *", "UTC"}, {"0 8 * * * * *", "UTC"}, {"0 8 * * *", "Not/AZone"}} {
		if _, _, err := parseReportSchedule(test.cron, test.zone); err == nil {
			t.Fatalf("parseReportSchedule(%q, %q) accepted invalid input", test.cron, test.zone)
		}
	}
}
