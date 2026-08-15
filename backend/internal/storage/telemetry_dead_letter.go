package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TelemetryDeadLetterStatusPending    = "pending"
	TelemetryDeadLetterStatusRetrying   = "retrying"
	TelemetryDeadLetterStatusProcessing = "processing"
	TelemetryDeadLetterStatusResolved   = "resolved"
	TelemetryDeadLetterStatusDead       = "dead"

	telemetryDeadLetterMaxAttempts = 3
	telemetryDeadLetterBaseBackoff = time.Minute
	telemetryDeadLetterMaxBackoff  = 15 * time.Minute
)

type telemetryDeadLetterDrainItem struct {
	DeadLetter TelemetryDeadLetter
	History    TelemetryData
}

type telemetryDeadLetterDrainSkip struct {
	ID     string
	Reason string
}

type telemetryDeadLetterDrainPlan struct {
	Ready   []telemetryDeadLetterDrainItem
	Skipped []telemetryDeadLetterDrainSkip
}

func buildTelemetryDeadLetterDrainPlan(rows []TelemetryDeadLetter, now time.Time, limit int) telemetryDeadLetterDrainPlan {
	plan := telemetryDeadLetterDrainPlan{}
	for _, row := range rows {
		if limit > 0 && len(plan.Ready) >= limit {
			plan.Skipped = append(plan.Skipped, telemetryDeadLetterDrainSkip{ID: row.ID, Reason: "limit_reached"})
			continue
		}

		if !telemetryDeadLetterCanRetry(row.Status) {
			plan.Skipped = append(plan.Skipped, telemetryDeadLetterDrainSkip{ID: row.ID, Reason: "terminal_status"})
			continue
		}
		if row.Attempts >= telemetryDeadLetterMaxAttempts {
			plan.Skipped = append(plan.Skipped, telemetryDeadLetterDrainSkip{ID: row.ID, Reason: "retry_exhausted"})
			continue
		}
		if row.NextRetryAt != nil && row.NextRetryAt.After(now) {
			plan.Skipped = append(plan.Skipped, telemetryDeadLetterDrainSkip{ID: row.ID, Reason: "retry_waiting"})
			continue
		}

		history, err := telemetryDataFromDeadLetter(row)
		if err != nil {
			plan.Skipped = append(plan.Skipped, telemetryDeadLetterDrainSkip{ID: row.ID, Reason: "invalid_payload"})
			continue
		}
		plan.Ready = append(plan.Ready, telemetryDeadLetterDrainItem{
			DeadLetter: row,
			History:    history,
		})
	}
	return plan
}

func telemetryDeadLetterCanRetry(status string) bool {
	return status == TelemetryDeadLetterStatusPending || status == TelemetryDeadLetterStatusRetrying
}

func telemetryDataFromDeadLetter(row TelemetryDeadLetter) (TelemetryData, error) {
	if len(row.RawPayload) > 0 {
		var history TelemetryData
		if err := json.Unmarshal(row.RawPayload, &history); err == nil {
			history = fillTelemetryDataFromDeadLetter(history, row)
			if telemetryDataReplayable(history) {
				return history, nil
			}
		}
	}

	history := fillTelemetryDataFromDeadLetter(TelemetryData{}, row)
	if telemetryDataReplayable(history) {
		return history, nil
	}
	return TelemetryData{}, fmt.Errorf("telemetry dead letter %q is missing replay identity", row.ID)
}

func TelemetryDataFromDeadLetter(row TelemetryDeadLetter) (TelemetryData, error) {
	return telemetryDataFromDeadLetter(row)
}

func fillTelemetryDataFromDeadLetter(history TelemetryData, row TelemetryDeadLetter) TelemetryData {
	if history.DeviceID == "" {
		history.DeviceID = row.DeviceID
	}
	if history.TenantID == "" {
		history.TenantID = row.TenantID
	}
	if history.Key == "" {
		history.Key = row.Key
	}
	if history.TS == 0 {
		history.TS = row.TS
	}
	if history.BoolV == nil {
		history.BoolV = row.BoolV
	}
	if history.NumberV == nil {
		history.NumberV = row.NumberV
	}
	if history.StringV == nil {
		history.StringV = row.StringV
	}
	return history
}

func telemetryDataReplayable(history TelemetryData) bool {
	return history.DeviceID != "" && history.TenantID != "" && history.Key != "" && history.TS > 0
}

func nextTelemetryDeadLetterRetryAt(attempts int, now time.Time) *time.Time {
	if attempts < 1 {
		attempts = 1
	}

	delay := telemetryDeadLetterBaseBackoff
	for i := 1; i < attempts; i++ {
		delay *= 2
		if delay >= telemetryDeadLetterMaxBackoff {
			delay = telemetryDeadLetterMaxBackoff
			break
		}
	}

	next := now.Add(delay)
	return &next
}

func NextTelemetryDeadLetterRetryAt(attempts int, now time.Time) *time.Time {
	return nextTelemetryDeadLetterRetryAt(attempts, now)
}

func TelemetryDeadLetterMaxAttempts() int {
	return telemetryDeadLetterMaxAttempts
}
