// dead_letter_policy.go is the single retry/operator policy shared by the
// telemetry dead-letter stack (telemetry_dead_letters table, service layer)
// and the attribute/event dead-letter stack (uplink_storage_dead_letters,
// storage layer). Tables, claim fencing and replay stay stack-specific; the
// status vocabulary, attempt budget, backoff curve, manual action transitions
// and paging bounds live here so the two operator surfaces cannot drift.
package storage

import (
	"fmt"
	"strings"
	"time"
)

// Dead-letter status values persisted in both dead-letter tables.
const (
	DeadLetterStatusPending    = "pending"
	DeadLetterStatusRetrying   = "retrying"
	DeadLetterStatusProcessing = "processing"
	DeadLetterStatusResolved   = "resolved"
	DeadLetterStatusDead       = "dead"
)

// DeadLetterAction is a manual operator action accepted by both stacks.
type DeadLetterAction string

const (
	DeadLetterActionRetry   DeadLetterAction = "retry"
	DeadLetterActionReplay  DeadLetterAction = "replay"
	DeadLetterActionResolve DeadLetterAction = "resolve"
	DeadLetterActionIgnore  DeadLetterAction = "ignore"
)

const (
	deadLetterMaxAttempts = 3
	deadLetterBaseBackoff = time.Minute
	deadLetterMaxBackoff  = 15 * time.Minute

	deadLetterDefaultDrainLimit = 20
	deadLetterMaxDrainLimit     = 100
	deadLetterDefaultPageSize   = 20
	deadLetterMaxPageSize       = 1000
)

// DeadLetterMaxAttempts is the automatic replay budget of one dead letter.
func DeadLetterMaxAttempts() int { return deadLetterMaxAttempts }

// deadLetterRetryableStatuses are the statuses automatic replay may claim
// (processing rows are additionally claimable once their lease expired).
var deadLetterRetryableStatuses = []string{DeadLetterStatusPending, DeadLetterStatusRetrying}

// deadLetterManualMutableStatuses are the statuses a manual retry/resolve/
// ignore may transition from. Processing rows are owned by a claimant.
var deadLetterManualMutableStatuses = []string{
	DeadLetterStatusPending,
	DeadLetterStatusRetrying,
	DeadLetterStatusResolved,
	DeadLetterStatusDead,
}

// DeadLetterRetryableStatuses returns a copy of the auto-claimable statuses.
func DeadLetterRetryableStatuses() []string {
	return append([]string(nil), deadLetterRetryableStatuses...)
}

// DeadLetterManualMutableStatuses returns a copy of the manually mutable statuses.
func DeadLetterManualMutableStatuses() []string {
	return append([]string(nil), deadLetterManualMutableStatuses...)
}

// DeadLetterCanRetry reports whether automatic replay may pick status.
func DeadLetterCanRetry(status string) bool {
	return status == DeadLetterStatusPending || status == DeadLetterStatusRetrying
}

// DeadLetterManualStatusMutable reports whether a manual non-replay action may
// change a row currently in status.
func DeadLetterManualStatusMutable(status string) bool {
	for _, candidate := range deadLetterManualMutableStatuses {
		if status == candidate {
			return true
		}
	}
	return false
}

// DeadLetterKnownStatus reports whether status is part of the vocabulary.
func DeadLetterKnownStatus(status string) bool {
	return status == DeadLetterStatusProcessing || DeadLetterManualStatusMutable(status)
}

// DeadLetterRetryBackoff is the exponential backoff after the given attempt
// count: 1m, 2m, 4m, 8m, then capped at 15m. attempts < 1 is treated as 1.
func DeadLetterRetryBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := deadLetterBaseBackoff
	for count := 1; count < attempts; count++ {
		delay *= 2
		if delay >= deadLetterMaxBackoff {
			return deadLetterMaxBackoff
		}
	}
	return delay
}

// NextDeadLetterRetryAt is now plus DeadLetterRetryBackoff(attempts).
func NextDeadLetterRetryAt(attempts int, now time.Time) *time.Time {
	next := now.Add(DeadLetterRetryBackoff(attempts))
	return &next
}

// ErrDeadLetterReplayIsNotAStatusUpdate is returned by
// DeadLetterManualStatusUpdates for the replay action, which must execute the
// dead letter instead of rewriting its status.
var ErrDeadLetterReplayIsNotAStatusUpdate = fmt.Errorf("replay action must execute the dead letter")

// DeadLetterManualStatusUpdates returns the column updates shared by both
// stacks for a manual non-replay action. Stacks add their own columns (claim
// fencing, last_error) on top.
func DeadLetterManualStatusUpdates(action DeadLetterAction, now time.Time) (map[string]interface{}, error) {
	switch DeadLetterAction(strings.TrimSpace(string(action))) {
	case DeadLetterActionRetry:
		return map[string]interface{}{
			"status":        DeadLetterStatusPending,
			"attempts":      0,
			"next_retry_at": nil,
			"updated_at":    now,
		}, nil
	case DeadLetterActionResolve:
		return map[string]interface{}{
			"status":        DeadLetterStatusResolved,
			"next_retry_at": nil,
			"updated_at":    now,
		}, nil
	case DeadLetterActionIgnore:
		return map[string]interface{}{
			"status":        DeadLetterStatusDead,
			"next_retry_at": nil,
			"updated_at":    now,
		}, nil
	case DeadLetterActionReplay:
		return nil, ErrDeadLetterReplayIsNotAStatusUpdate
	default:
		return nil, fmt.Errorf("unsupported dead letter action %q", action)
	}
}

// NormalizeDeadLetterDrainLimit bounds one drain request to 1..100 (default 20).
func NormalizeDeadLetterDrainLimit(limit int) int {
	if limit < 1 {
		return deadLetterDefaultDrainLimit
	}
	if limit > deadLetterMaxDrainLimit {
		return deadLetterMaxDrainLimit
	}
	return limit
}

// NormalizeDeadLetterPage bounds list paging (page >= 1, size 1..1000, default 20).
func NormalizeDeadLetterPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = deadLetterDefaultPageSize
	}
	if pageSize > deadLetterMaxPageSize {
		pageSize = deadLetterMaxPageSize
	}
	return page, pageSize
}
