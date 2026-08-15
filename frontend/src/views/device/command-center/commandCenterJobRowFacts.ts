import type { FleetCommandJobSubmitResult, FleetCommandJobSubmitRow } from '@/service/api/device'

export interface CommandJobRowFacts {
  rows: FleetCommandJobSubmitRow[]
  retryableRows: FleetCommandJobSubmitRow[]
  retryReadyRows: FleetCommandJobSubmitRow[]
  retryWaitingRows: FleetCommandJobSubmitRow[]
  retryExhaustedRows: FleetCommandJobSubmitRow[]
  logMissingRows: FleetCommandJobSubmitRow[]
  deviceFailedRows: FleetCommandJobSubmitRow[]
  submittedRows: FleetCommandJobSubmitRow[]
  completedRows: FleetCommandJobSubmitRow[]
}

export function isFutureCommandJobDate(value?: string) {
  if (!value) return false
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp) && timestamp > Date.now()
}

export function resolveCommandJobRowRetryState(row: FleetCommandJobSubmitRow) {
  if (row.retry_state) return row.retry_state
  if (row.status !== 'failed') return 'not_retryable'
  if (!row.can_retry) {
    if ((row.dispatch_attempts ?? 0) >= (row.max_dispatch_attempts ?? Number.POSITIVE_INFINITY)) {
      return 'max_attempts_reached'
    }
    return 'not_retryable'
  }
  if (isFutureCommandJobDate(row.next_retry_after)) return 'waiting_backoff'
  return 'retryable'
}

export function buildCommandJobRowFacts(result: FleetCommandJobSubmitResult | null): CommandJobRowFacts {
  const rows = result?.rows ?? []
  const facts: CommandJobRowFacts = {
    rows,
    retryableRows: [],
    retryReadyRows: [],
    retryWaitingRows: [],
    retryExhaustedRows: [],
    logMissingRows: [],
    deviceFailedRows: [],
    submittedRows: [],
    completedRows: []
  }

  for (const row of rows) {
    if (row.can_retry) facts.retryableRows.push(row)
    if (row.eligible && !row.log_recorded) facts.logMissingRows.push(row)
    if (row.response_status_label === 'device_ack_failed') facts.deviceFailedRows.push(row)
    if (row.submitted_at) facts.submittedRows.push(row)
    if (row.completed_at) facts.completedRows.push(row)

    const retryState = resolveCommandJobRowRetryState(row)
    if (retryState === 'retryable') facts.retryReadyRows.push(row)
    else if (retryState === 'waiting_backoff') facts.retryWaitingRows.push(row)
    else if (retryState === 'max_attempts_reached') facts.retryExhaustedRows.push(row)
  }

  return facts
}
