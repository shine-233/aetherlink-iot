package service

import "time"

// Telemetry dead letters are split by risk profile:
//
//   - telemetry_dead_letters_query.go: read side. Tenant/owner scoping, the
//     "claimable" predicate, pagination normalisation and response mapping.
//     Pure query builders with no state changes.
//   - telemetry_dead_letters_lifecycle.go: write side. The claim/lease ->
//     replay -> resolve/retry/dead state machine, claim release on error and
//     manual operator transitions. Transactional and concurrency-sensitive.
//
// Lifecycle (status column):
//
//	pending|retrying --claim--> processing --replay ok--> resolved
//	                                       --replay err-> retrying (attempts+1, backoff)
//	                                                    -> dead     (attempts >= max)
//	processing --claim older than processingTimeout--> reclaimable
//	processing --drain aborted--> retrying (next_retry_at = release time)
//	pending|retrying|resolved|dead --manual retry/resolve/ignore--> pending|resolved|dead

const (
	telemetryDeadLetterActionRetry   = "retry"
	telemetryDeadLetterActionResolve = "resolve"
	telemetryDeadLetterActionIgnore  = "ignore"
	telemetryDeadLetterActionReplay  = "replay"

	telemetryDeadLetterProcessingTimeout   = 5 * time.Minute
	telemetryDeadLetterClaimReleaseTimeout = 2 * time.Second
	telemetryDeadLetterStatusConflict      = "dead letter status conflict; refresh and retry"

	telemetryDeadLetterDefaultDrainLimit = 20
	telemetryDeadLetterMaxDrainLimit     = 100
	telemetryDeadLetterDefaultPageSize   = 20
	telemetryDeadLetterMaxPageSize       = 1000

	// telemetryDeadLetterDrainItemFailed is the per-item status reported by a
	// drain when replay of that row failed (the row itself is retrying/dead).
	telemetryDeadLetterDrainItemFailed = "failed"
)
