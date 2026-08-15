import type {
  FleetCommandJobGovernanceSummary,
  FleetCommandJobListAttentionCounts,
  FleetCommandJobListItem,
  FleetCommandJobSubmitResult,
  FleetCommandJobSubmitRow,
  FleetCommandJobSupportBundle
} from '@/service/api/device'
import {
  buildCommandJobRowFacts,
  isFutureCommandJobDate,
  resolveCommandJobRowRetryState
} from './commandCenterJobRowFacts'

export {
  buildCommandJobActionConsequenceRows,
  buildCommandJobOperatorNextAction,
  buildCommandJobTroubleshootingRows,
  type CommandJobActionConsequenceRow,
  type CommandJobOperatorNextAction,
  type CommandJobTroubleshootingRow
} from './commandCenterJobOperatorDecisionView'
export {
  buildCommandJobExecutionSummaryCard,
  buildCommandJobSupportBundlePreview,
  type CommandJobExecutionChecklistItem,
  type CommandJobExecutionSummaryCard,
  type CommandJobSupportBundlePreview,
  type CommandJobSupportFailedDeviceEvidence,
  type CommandJobSupportDiagnosticPreview
} from './commandCenterJobSupportBundlePreviewView'

type Translate = (key: string) => string

export interface CommandJobLabelValueRow {
  key?: string
  label: string
  value: string
}

export interface CommandJobStatusCountRow {
  status: string
  label: string
  count: number
}

export interface CommandJobHistoryAttentionAggregateRow {
  key: string
  label: string
  count: number
  filter?: string
  type: 'success' | 'info' | 'warning' | 'error'
}

export interface CommandJobOutcomeDeviceRow {
  key: string
  deviceId: string
  device: string
  status: string
  readiness: string
  reason: string
  action: string
}

export interface CommandJobOutcomeGroup {
  key: string
  title: string
  description: string
  count: number
  type: 'success' | 'info' | 'warning' | 'error'
  rows: CommandJobOutcomeDeviceRow[]
}

export interface CommandJobDeviceProgressStep {
  key: 'preview' | 'dispatch' | 'ack' | 'evidence'
  label: string
  state: string
  detail: string
  type: 'success' | 'info' | 'warning' | 'error'
}

export interface CommandJobDeviceProgressTrack {
  key: string
  deviceId: string
  device: string
  summary: string
  nextAction: string
  type: 'success' | 'info' | 'warning' | 'error'
  steps: CommandJobDeviceProgressStep[]
}

export interface CommandJobProgressHealthCard {
  stateLabel: string
  type: 'success' | 'info' | 'warning' | 'error'
  nextAction: string
  rows: CommandJobLabelValueRow[]
}

export interface CommandJobAuditSummaryCard {
  latestLabel: string
  nextAction: string
  rows: CommandJobLabelValueRow[]
}

export interface CommandJobGovernanceSummaryCard {
  title: string
  levelLabel: string
  summary: string
  nextAction: string
  type: 'success' | 'info' | 'warning' | 'error'
  items: CommandJobGovernanceSummaryItem[]
}

export interface CommandJobGovernanceSummaryItem {
  key: string
  label: string
  value: string
  detail: string
  stateLabel: string
  type: 'success' | 'info' | 'warning' | 'error'
}

export function formatCommandJobStatus(status: string | undefined, t: Translate) {
  if (!status) return '-'
  const key = `custom.commandCenter.jobStatus.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

export function formatCommandJobResponseStatus(statusLabel: string | undefined, t: Translate) {
  if (!statusLabel) return t('custom.commandCenter.responseStatus.awaiting')
  const key = `custom.commandCenter.responseStatus.${statusLabel}`
  const translated = t(key)
  return translated === key ? statusLabel : translated
}

export function formatCommandJobReadiness(readiness: string[] | undefined) {
  const items = readiness?.filter(Boolean) ?? []
  return items.length ? items.join(', ') : '-'
}

export function formatCommandJobDateTime(value?: string) {
  if (!value) return '--'
  return value
    .replace('T', ' ')
    .replace(/\.\d+Z$/, ' UTC')
    .replace(/Z$/, ' UTC')
}

export function commandJobRetryableRows(result: FleetCommandJobSubmitResult | null) {
  return buildCommandJobRowFacts(result).retryableRows
}

export function commandJobLogMissingRows(result: FleetCommandJobSubmitResult | null) {
  return buildCommandJobRowFacts(result).logMissingRows
}

export function commandJobRetryableCount(result: FleetCommandJobSubmitResult | null) {
  return result?.retryable_count ?? commandJobRetryableRows(result).length
}

export function commandJobRetryReadyCount(result: FleetCommandJobSubmitResult | null) {
  return result?.retry_ready_count ?? buildCommandJobRowFacts(result).retryReadyRows.length
}

export function commandJobRetryWaitingCount(result: FleetCommandJobSubmitResult | null) {
  return result?.retry_waiting_count ?? buildCommandJobRowFacts(result).retryWaitingRows.length
}

export function commandJobRetryExhaustedCount(result: FleetCommandJobSubmitResult | null) {
  return result?.retry_exhausted_count ?? buildCommandJobRowFacts(result).retryExhaustedRows.length
}

export function commandJobLogMissingCount(result: FleetCommandJobSubmitResult | null) {
  return result?.log_missing_count ?? commandJobLogMissingRows(result).length
}

export function canRetryCommandJob(result: FleetCommandJobSubmitResult | null) {
  return Boolean(result?.can_retry_failed && commandJobRetryReadyCount(result) > 0)
}

export function commandJobProgressPercent(result: FleetCommandJobSubmitResult | null) {
  if (!result || result.requested_count <= 0) return 0
  const terminalCount = (result.submitted_count || 0) + (result.failed_count || 0)
  return Math.min(100, Math.round((terminalCount / result.requested_count) * 100))
}

export function buildCommandJobProgressSummary(result: FleetCommandJobSubmitResult | null, t: Translate) {
  if (!result) return ''
  return t('custom.commandCenter.jobProgressSummary')
    .replace('{submitted}', String(result.submitted_count))
    .replace('{failed}', String(result.failed_count))
    .replace('{total}', String(result.requested_count))
}

const commandJobProgressHealthType = (state: string): CommandJobProgressHealthCard['type'] => {
  if (state === 'complete') return 'success'
  if (state === 'scheduled' || state === 'running') return 'info'
  if (state === 'timeout_risk') return 'warning'
  return 'error'
}

const formatCommandJobDuration = (seconds: number | undefined, t: Translate) => {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds)) return '--'
  if (seconds < 0) return t('custom.commandCenter.progressHealthExpired')
  if (seconds < 60) return t('custom.commandCenter.progressHealthSeconds').replace('{seconds}', String(Math.round(seconds)))
  return t('custom.commandCenter.progressHealthMinutes').replace('{minutes}', String(Math.ceil(seconds / 60)))
}

export function buildCommandJobProgressHealthCard(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobProgressHealthCard | null {
  if (!result) return null

  const pendingFallback = Math.max(
    0,
    result.requested_count - (result.submitted_count || 0) - (result.failed_count || 0) - (result.blocked_count || 0)
  )
  const fallbackState =
    result.status === 'completed'
      ? 'complete'
      : result.status === 'scheduled'
        ? 'scheduled'
        : result.status === 'running'
          ? 'running'
          : result.status === 'canceled'
            ? 'canceled'
            : 'needs_attention'
  const health = result.progress_health || {
    state: fallbackState,
    pending_count: pendingFallback,
    terminal_count: Math.max(0, result.requested_count - pendingFallback),
    elapsed_seconds: 0,
    timeout_remaining_seconds: 0,
    next_action: ''
  }
  const stateKey = `custom.commandCenter.progressHealthState.${health.state}`
  const translatedState = t(stateKey)
  const nextAction = health.next_action || t(`custom.commandCenter.progressHealthNext.${health.state}`)

  return {
    stateLabel: translatedState === stateKey ? health.state : translatedState,
    type: commandJobProgressHealthType(health.state),
    nextAction,
    rows: [
      { label: t('custom.commandCenter.progressHealthPending'), value: String(health.pending_count) },
      { label: t('custom.commandCenter.progressHealthTerminal'), value: String(health.terminal_count) },
      { label: t('custom.commandCenter.progressHealthElapsed'), value: formatCommandJobDuration(health.elapsed_seconds, t) },
      {
        label: t('custom.commandCenter.progressHealthRemaining'),
        value: formatCommandJobDuration(health.timeout_remaining_seconds, t)
      }
    ]
  }
}

export function buildCommandJobHandoffSummary(result: FleetCommandJobSubmitResult | null, jobLink = '') {
  if (!result) return ''
  const execution = result.execution_summary
  const summary =
    result.handoff_summary ||
    `Command Job ${result.job_id} is ${result.status}: ${result.submitted_count}/${result.requested_count} submitted, ${result.failed_count} failed, ${result.blocked_count} blocked.`
  const closeReadiness = execution
    ? execution.can_close
      ? 'Close readiness: ready to close.'
      : `Close readiness: blocked${execution.close_blockers?.length ? ` - ${execution.close_blockers.join(' ')}` : '.'}`
    : ''
  const nextAction = execution?.next_action ? `Next action: ${execution.next_action}` : ''
  return [summary, closeReadiness, nextAction, jobLink].filter(Boolean).join('\n')
}

export function buildCommandJobCloseoutPacket(
  result: FleetCommandJobSubmitResult | null,
  jobLink = '',
  supportBundle?: FleetCommandJobSupportBundle | null
) {
  if (!result) return ''
  const execution = result.execution_summary
  const audit = result.audit_summary
  const closeReadiness = execution?.can_close ? 'ready' : 'blocked'
  const closeBlockers = execution?.close_blockers?.filter(Boolean) ?? []
  const checklist = execution?.checklist?.filter(item => item.key || item.label) ?? []
  const supportRows = supportBundle
    ? [
        `Support bundle generated: ${formatCommandJobDateTime(supportBundle.generated_at)}`,
        `Support next actions: ${(supportBundle.next_actions ?? []).join('; ') || '-'}`,
        `Retry ready/waiting/exhausted: ${supportBundle.retry_ready_count}/${supportBundle.retry_waiting_count}/${supportBundle.retry_exhausted_count}`,
        `Failed devices: ${supportBundle.failed_devices?.length ?? 0}`,
        `Missing log devices: ${supportBundle.missing_log_device_ids?.length ?? 0}`,
        `Support events: ${supportBundle.events?.length ?? 0}`
      ]
    : ['Support bundle generated: not loaded in this browser session']

  return [
    'AetherLink Command Job closeout packet',
    `Job: ${result.job_id}`,
    `Status: ${result.status}`,
    `Progress: ${result.submitted_count}/${result.requested_count} submitted, ${result.failed_count} failed, ${result.blocked_count} blocked`,
    `Close readiness: ${closeReadiness}`,
    closeBlockers.length ? `Close blockers: ${closeBlockers.join('; ')}` : 'Close blockers: none',
    execution?.next_action ? `Next action: ${execution.next_action}` : '',
    checklist.length ? 'Checklist:' : '',
    ...checklist.map(item => `- [${item.state || 'unknown'}] ${item.label || item.key}: ${item.detail || '-'}`),
    audit
      ? `Audit: ${audit.event_count || 0} events, latest=${audit.latest_event_type || '-'}, at=${formatCommandJobDateTime(audit.latest_event_at)}, message=${audit.latest_message || '-'}`
      : 'Audit: no audit summary loaded',
    ...supportRows,
    jobLink ? `Job link: ${jobLink}` : ''
  ].filter(Boolean).join('\n')
}

export function buildCommandJobAuditSummaryCard(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobAuditSummaryCard | null {
  if (!result?.audit_summary) return null
  const audit = result.audit_summary
  const eventKey = audit.latest_event_type ? `custom.commandCenter.jobEvent.${audit.latest_event_type}` : ''
  const translatedEvent = eventKey ? t(eventKey) : ''

  return {
    latestLabel: translatedEvent && translatedEvent !== eventKey ? translatedEvent : audit.latest_event_type || '--',
    nextAction: audit.next_action,
    rows: [
      { label: t('custom.commandCenter.auditEventCount'), value: String(audit.event_count || 0) },
      { label: t('custom.commandCenter.auditLatestEvent'), value: audit.latest_event_type || '--' },
      { label: t('custom.commandCenter.auditLatestAt'), value: formatCommandJobDateTime(audit.latest_event_at) },
      { label: t('custom.commandCenter.auditLatestMessage'), value: audit.latest_message || '--' }
    ]
  }
}

const commandJobGovernanceLevelType = (level: string): CommandJobGovernanceSummaryCard['type'] => {
  if (level === 'success') return 'success'
  if (level === 'warning') return 'warning'
  if (level === 'error' || level === 'blocked') return 'error'
  return 'info'
}

const commandJobGovernanceStateType = (state: string): CommandJobGovernanceSummaryItem['type'] => {
  if (state === 'done') return 'success'
  if (state === 'blocked') return 'error'
  if (state === 'watch' || state === 'todo') return 'warning'
  return 'info'
}

export function buildCommandJobGovernanceSummaryCard(
  summary: FleetCommandJobGovernanceSummary | undefined,
  t: Translate
): CommandJobGovernanceSummaryCard | null {
  if (!summary) return null

  return {
    title: summary.title || t('custom.commandCenter.governanceSummaryTitle'),
    levelLabel: (() => {
      const levelKey = `custom.commandCenter.governanceLevel.${summary.level}`
      const translatedLevel = t(levelKey)
      return translatedLevel === levelKey ? summary.level || 'info' : translatedLevel
    })(),
    summary: summary.summary || '-',
    nextAction: summary.next_action || '-',
    type: commandJobGovernanceLevelType(summary.level),
    items: (summary.items ?? []).map((item) => {
      const stateKey = `custom.commandCenter.governanceState.${item.state}`
      const translatedState = t(stateKey)
      return {
        key: item.key || item.label,
        label: item.label || item.key,
        value: item.value || '-',
        detail: item.detail || '-',
        stateLabel: translatedState === stateKey ? item.state : translatedState,
        type: commandJobGovernanceStateType(item.state)
      }
    })
  }
}

export function buildCommandJobCapabilitySummary(result: FleetCommandJobSubmitResult | null, t: Translate) {
  if (!result) return ''
  return t('custom.commandCenter.submitCapabilitySummary')
    .replace('{cancel}', result.can_cancel ? t('common.yesOrNo.yes') : t('common.yesOrNo.no'))
    .replace('{retry}', result.can_retry_failed ? t('common.yesOrNo.yes') : t('common.yesOrNo.no'))
}

export function buildCommandJobEvidenceSummary(result: FleetCommandJobSubmitResult | null, t: Translate) {
  if (!result) return ''
  return t('custom.commandCenter.submitEvidenceSummary')
    .replace('{retryable}', String(commandJobRetryableCount(result)))
    .replace('{missingLogs}', String(commandJobLogMissingCount(result)))
}

export function buildCommandJobSubmitNextAction(row: FleetCommandJobSubmitRow, t: Translate) {
  if (row.response_status_label === 'device_ack_success') return t('custom.commandCenter.nextActionDeviceAckSuccess')
  if (row.response_status_label === 'device_ack_failed') return row.response_error || t('custom.commandCenter.nextActionDeviceAckFailed')
  const retryState = resolveCommandJobRowRetryState(row)
  if (retryState === 'waiting_backoff' && isFutureCommandJobDate(row.next_retry_after)) {
    return t('custom.commandCenter.nextActionRetryAfter').replace('{time}', formatCommandJobDateTime(row.next_retry_after))
  }
  if (retryState === 'max_attempts_reached') return t('custom.commandCenter.nextActionRetryLimitReached')
  if (row.advice) return row.advice
  if (row.can_retry) return t('custom.commandCenter.nextActionRetry')
  if (row.eligible && !row.log_recorded) return t('custom.commandCenter.nextActionRefreshLogs')
  if (!row.eligible || row.recommended_path === 'blocked') return t('custom.commandCenter.nextActionFixBlocker')
  if (row.status === 'completed') return t('custom.commandCenter.nextActionCompleted')
  if (row.message_id) return t('custom.commandCenter.nextActionTrackMessage')
  return t('custom.commandCenter.nextActionSupportBundle')
}

function buildCommandJobOutcomeRow(row: FleetCommandJobSubmitRow, t: Translate): CommandJobOutcomeDeviceRow {
  return {
    key: row.detail_id || row.device_id || row.message_id || `${row.status}-${row.device_number || row.name}`,
    deviceId: row.device_id,
    device: row.device_number || row.name || row.device_id,
    status: formatCommandJobStatus(row.status, t),
    readiness: formatCommandJobReadiness(row.readiness),
    reason: row.reason || '-',
    action: buildCommandJobSubmitNextAction(row, t)
  }
}

function commandJobDeviceProgressType(row: FleetCommandJobSubmitRow): 'success' | 'info' | 'warning' | 'error' {
  if (row.response_status_label === 'device_ack_failed') return 'error'
  if (row.can_retry || resolveCommandJobRowRetryState(row) === 'max_attempts_reached') return 'error'
  if (!row.eligible || row.recommended_path === 'blocked') return 'error'
  if (row.response_status_label === 'device_ack_success' || row.completed_at || row.status === 'completed') return 'success'
  if (row.eligible && !row.log_recorded) return 'warning'
  if (resolveCommandJobRowRetryState(row) === 'waiting_backoff') return 'warning'
  return 'info'
}

function commandJobDeviceProgressPriority(row: FleetCommandJobSubmitRow) {
  const type = commandJobDeviceProgressType(row)
  if (type === 'error') return 0
  if (type === 'warning') return 1
  if (type === 'info') return 2
  return 3
}

function commandJobDeviceProgressStepType(state: string): 'success' | 'info' | 'warning' | 'error' {
  if (state === 'done') return 'success'
  if (state === 'blocked' || state === 'failed') return 'error'
  if (state === 'waiting' || state === 'missing') return 'warning'
  return 'info'
}

function commandJobDeviceProgressPreviewStep(row: FleetCommandJobSubmitRow, t: Translate): CommandJobDeviceProgressStep {
  const blocked = !row.eligible || row.recommended_path === 'blocked'
  const state = blocked ? 'blocked' : 'done'
  return {
    key: 'preview',
    label: t('custom.commandCenter.deviceProgressPreview'),
    state: t(`custom.commandCenter.deviceProgressState.${state}`),
    detail: blocked ? row.reason || t('custom.commandCenter.deviceProgressPreviewBlocked') : formatCommandJobReadiness(row.readiness),
    type: commandJobDeviceProgressStepType(state)
  }
}

function commandJobDeviceProgressDispatchStep(row: FleetCommandJobSubmitRow, t: Translate): CommandJobDeviceProgressStep {
  const retryState = resolveCommandJobRowRetryState(row)
  const failed = row.status === 'failed' || row.can_retry || retryState === 'max_attempts_reached'
  const submitted = Boolean(row.submitted_at || row.message_id || row.last_dispatch_started_at)
  const state = failed ? 'failed' : submitted ? 'done' : 'waiting'
  const attempts =
    row.dispatch_attempts || row.max_dispatch_attempts
      ? ` ${row.dispatch_attempts ?? 0}/${row.max_dispatch_attempts ?? '-'}`
      : ''
  const detail = failed
    ? row.reason || row.advice || formatCommandJobStatus(row.status, t)
    : row.message_id || row.last_dispatch_started_at || row.submitted_at || t('custom.commandCenter.deviceProgressDispatchWaiting')

  return {
    key: 'dispatch',
    label: t('custom.commandCenter.deviceProgressDispatch'),
    state: `${t(`custom.commandCenter.deviceProgressState.${state}`)}${attempts}`,
    detail,
    type: commandJobDeviceProgressStepType(state)
  }
}

function commandJobDeviceProgressAckStep(row: FleetCommandJobSubmitRow, t: Translate): CommandJobDeviceProgressStep {
  let state = 'waiting'
  if (row.response_status_label === 'device_ack_success') state = 'done'
  else if (row.response_status_label === 'device_ack_failed') state = 'failed'
  else if (row.completed_at || row.response_recorded) state = 'done'

  return {
    key: 'ack',
    label: t('custom.commandCenter.deviceProgressAck'),
    state: t(`custom.commandCenter.deviceProgressState.${state}`),
    detail:
      row.response_error ||
      formatCommandJobResponseStatus(row.response_status_label, t) ||
      row.completed_at ||
      t('custom.commandCenter.deviceProgressAckWaiting'),
    type: commandJobDeviceProgressStepType(state)
  }
}

function commandJobDeviceProgressEvidenceStep(row: FleetCommandJobSubmitRow, t: Translate): CommandJobDeviceProgressStep {
  const state = row.log_recorded || row.command_log_created_at ? 'done' : 'missing'
  return {
    key: 'evidence',
    label: t('custom.commandCenter.deviceProgressEvidence'),
    state: t(`custom.commandCenter.deviceProgressState.${state}`),
    detail: row.command_log_created_at || row.next_retry_after || t('custom.commandCenter.deviceProgressEvidenceMissing'),
    type: commandJobDeviceProgressStepType(state)
  }
}

export function buildCommandJobDeviceProgressTracks(
  result: FleetCommandJobSubmitResult | null,
  t: Translate,
  limit = 8
): CommandJobDeviceProgressTrack[] {
  const rows = result?.rows ?? []
  return [...rows]
    .sort((left, right) => commandJobDeviceProgressPriority(left) - commandJobDeviceProgressPriority(right))
    .slice(0, Math.max(0, limit))
    .map(row => ({
      key: row.detail_id || row.device_id || row.message_id || `${row.status}-${row.device_number || row.name}`,
      deviceId: row.device_id,
      device: row.device_number || row.name || row.device_id,
      summary: formatCommandJobStatus(row.status, t),
      nextAction: buildCommandJobSubmitNextAction(row, t),
      type: commandJobDeviceProgressType(row),
      steps: [
        commandJobDeviceProgressPreviewStep(row, t),
        commandJobDeviceProgressDispatchStep(row, t),
        commandJobDeviceProgressAckStep(row, t),
        commandJobDeviceProgressEvidenceStep(row, t)
      ]
    }))
}

export function buildCommandJobOutcomeGroups(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobOutcomeGroup[] {
  if (!result) return []

  const groups: CommandJobOutcomeGroup[] = [
    {
      key: 'retryable',
      title: t('custom.commandCenter.outcomeRetryTitle'),
      description: t('custom.commandCenter.outcomeRetryDesc'),
      count: 0,
      type: 'error',
      rows: []
    },
    {
      key: 'device_failed',
      title: t('custom.commandCenter.outcomeDeviceFailedTitle'),
      description: t('custom.commandCenter.outcomeDeviceFailedDesc'),
      count: 0,
      type: 'error',
      rows: []
    },
    {
      key: 'missing_logs',
      title: t('custom.commandCenter.outcomeMissingLogsTitle'),
      description: t('custom.commandCenter.outcomeMissingLogsDesc'),
      count: 0,
      type: 'warning',
      rows: []
    },
    {
      key: 'blocked',
      title: t('custom.commandCenter.outcomeBlockedTitle'),
      description: t('custom.commandCenter.outcomeBlockedDesc'),
      count: 0,
      type: 'error',
      rows: []
    },
    {
      key: 'in_progress',
      title: t('custom.commandCenter.outcomeInProgressTitle'),
      description: t('custom.commandCenter.outcomeInProgressDesc'),
      count: 0,
      type: 'info',
      rows: []
    },
    {
      key: 'completed',
      title: t('custom.commandCenter.outcomeCompletedTitle'),
      description: t('custom.commandCenter.outcomeCompletedDesc'),
      count: 0,
      type: 'success',
      rows: []
    }
  ]
  const byKey = Object.fromEntries(groups.map((group) => [group.key, group])) as Record<string, CommandJobOutcomeGroup>

  result.rows.forEach((row) => {
    let group = byKey.in_progress
    if (row.response_status_label === 'device_ack_success') group = byKey.completed
    else if (row.response_status_label === 'device_ack_failed') group = byKey.device_failed
    else if (row.can_retry) group = byKey.retryable
    else if (row.eligible && !row.log_recorded) group = byKey.missing_logs
    else if (!row.eligible || row.recommended_path === 'blocked') group = byKey.blocked
    else if (row.status === 'completed') group = byKey.completed

    group.count += 1
    if (group.rows.length < 5) group.rows.push(buildCommandJobOutcomeRow(row, t))
  })

  return groups.filter((group) => group.count > 0)
}

export function buildCommandJobStatusRows(result: FleetCommandJobSubmitResult | null, t: Translate): CommandJobLabelValueRow[] {
  if (!result) return []
  return [
    { label: t('custom.commandCenter.jobId'), value: result.job_id },
    { label: t('common.status'), value: formatCommandJobStatus(result.status, t) },
    { label: t('custom.commandCenter.jobProgress'), value: buildCommandJobProgressSummary(result, t) },
    { label: t('custom.commandCenter.createdAt'), value: formatCommandJobDateTime(result.created_at) },
    { label: t('custom.commandCenter.updatedAt'), value: formatCommandJobDateTime(result.updated_at) },
    ...(result.scheduled_at
      ? [{ label: t('custom.commandCenter.scheduledAt'), value: formatCommandJobDateTime(result.scheduled_at) }]
      : []),
    ...(result.next_dispatch_at
      ? [{ label: t('custom.commandCenter.nextDispatchAt'), value: formatCommandJobDateTime(result.next_dispatch_at) }]
      : []),
    { label: t('custom.commandCenter.timeoutAt'), value: formatCommandJobDateTime(result.timeout_at) }
  ]
}

export function buildCommandJobStatusCountRows(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobStatusCountRow[] {
  return Object.entries(result?.status_counts ?? {}).map(([status, count]) => ({
    status,
    label: formatCommandJobStatus(status, t),
    count
  }))
}

export function buildCommandJobTimelineRows(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobLabelValueRow[] {
  if (!result) return []
  if (result.events?.length) {
    return result.events.map((event, index) => {
      const key = `custom.commandCenter.jobEvent.${event.event_type}`
      const translated = t(key)
      const label = translated === key ? event.event_type : translated
      const details = [formatCommandJobDateTime(event.created_at), event.device_id, event.message]
        .filter(Boolean)
        .join(' - ')
      return {
        key: event.id || `${event.event_type}-${index}`,
        label,
        value: details
      }
    })
  }
  const { submittedRows, completedRows } = buildCommandJobRowFacts(result)
  return [
    {
      key: 'created',
      label: t('custom.commandCenter.timelineCreated'),
      value: formatCommandJobDateTime(result.created_at)
    },
    {
      key: 'submitted',
      label: t('custom.commandCenter.timelineSubmitted'),
      value: submittedRows.length
        ? t('custom.commandCenter.timelineDeviceCount').replace('{count}', String(submittedRows.length))
        : '--'
    },
    {
      key: 'completed',
      label: t('custom.commandCenter.timelineCompleted'),
      value: completedRows.length
        ? t('custom.commandCenter.timelineDeviceCount').replace('{count}', String(completedRows.length))
        : '--'
    }
  ]
}

export function buildCommandJobHistoryStatusOptions(t: Translate) {
  return [
    { label: t('custom.commandCenter.jobStatusAll'), value: '' },
    { label: formatCommandJobStatus('scheduled', t), value: 'scheduled' },
    { label: formatCommandJobStatus('running', t), value: 'running' },
    { label: formatCommandJobStatus('completed', t), value: 'completed' },
    { label: formatCommandJobStatus('partially_failed', t), value: 'partially_failed' },
    { label: formatCommandJobStatus('failed', t), value: 'failed' },
    { label: formatCommandJobStatus('canceled', t), value: 'canceled' }
  ]
}

const formatCommandJobAttentionOptionLabel = (
  label: string,
  count: number | undefined
) => (typeof count === 'number' && count > 0 ? `${label} (${count})` : label)

export function buildCommandJobHistoryAttentionOptions(
  t: Translate,
  counts?: FleetCommandJobListAttentionCounts
) {
  return [
    { label: t('custom.commandCenter.jobAttentionAll'), value: '' },
    {
      label: formatCommandJobAttentionOptionLabel(
        t('custom.commandCenter.jobAttentionNeedsOperatorAction'),
        counts?.needs_operator_action_count
      ),
      value: 'needs_operator_action'
    },
    {
      label: formatCommandJobAttentionOptionLabel(t('custom.commandCenter.jobAttentionRetryable'), counts?.retryable_count),
      value: 'retryable'
    },
    {
      label: formatCommandJobAttentionOptionLabel(
        t('custom.commandCenter.supportBundleRetryReadyDevices'),
        counts?.retry_ready_count
      ),
      value: 'retry_ready'
    },
    {
      label: formatCommandJobAttentionOptionLabel(
        t('custom.commandCenter.supportBundleRetryWaitingDevices'),
        counts?.retry_waiting_count
      ),
      value: 'retry_waiting'
    },
    {
      label: formatCommandJobAttentionOptionLabel(
        t('custom.commandCenter.supportBundleRetryExhaustedDevices'),
        counts?.retry_exhausted_count
      ),
      value: 'retry_exhausted'
    },
    {
      label: formatCommandJobAttentionOptionLabel(
        t('custom.commandCenter.jobAttentionDeviceFailed'),
        counts?.device_ack_failed_count
      ),
      value: 'device_failed'
    },
    {
      label: formatCommandJobAttentionOptionLabel(t('custom.commandCenter.jobAttentionMissingLog'), counts?.log_missing_count),
      value: 'missing_log'
    },
    {
      label: formatCommandJobAttentionOptionLabel(t('custom.commandCenter.jobAttentionBlocked'), counts?.blocked_count),
      value: 'blocked'
    }
  ]
}

export function buildCommandJobHistoryProgress(row: FleetCommandJobListItem, t: Translate) {
  return `${row.submitted_count}/${row.requested_count} ${t('custom.commandCenter.submittedShort')}, ${row.failed_count} ${t('custom.commandCenter.failedShort')}`
}

export function buildCommandJobHistoryAttentionSummary(row: FleetCommandJobListItem, t: Translate) {
  const needsAction = row.needs_operator_action_count ?? 0
  if (needsAction <= 0) return t('custom.commandCenter.jobAttentionNone')
  return t('custom.commandCenter.jobAttentionSummary')
    .replace('{count}', String(needsAction))
    .replace('{retryable}', String(row.retryable_count ?? 0))
    .replace('{deviceFailed}', String(row.device_ack_failed_count ?? 0))
    .replace('{missingLogs}', String(row.log_missing_count ?? 0))
}

export function buildCommandJobHistoryAttentionTotalSummary(
  counts: FleetCommandJobListAttentionCounts | undefined,
  t: Translate
) {
  const needsAction = counts?.needs_operator_action_count ?? 0
  if (needsAction <= 0) return t('custom.commandCenter.jobAttentionNone')
  return t('custom.commandCenter.jobAttentionSummary')
    .replace('{count}', String(needsAction))
    .replace('{retryable}', String(counts?.retryable_count ?? 0))
    .replace('{deviceFailed}', String(counts?.device_ack_failed_count ?? 0))
    .replace('{missingLogs}', String(counts?.log_missing_count ?? 0))
}

export function buildCommandJobHistoryAttentionAggregateRows(
  counts: FleetCommandJobListAttentionCounts | undefined,
  t: Translate
): CommandJobHistoryAttentionAggregateRow[] {
  const value = counts ?? {}
  return [
    {
      key: 'needs_operator_action',
      label: t('custom.commandCenter.jobAttentionNeedsOperatorAction'),
      count: value.needs_operator_action_count ?? 0,
      filter: 'needs_operator_action',
      type: (value.needs_operator_action_count ?? 0) > 0 ? 'warning' : 'success'
    },
    {
      key: 'retry_ready',
      label: t('custom.commandCenter.supportBundleRetryReadyDevices'),
      count: value.retry_ready_count ?? 0,
      filter: 'retry_ready',
      type: (value.retry_ready_count ?? 0) > 0 ? 'warning' : 'success'
    },
    {
      key: 'retry_waiting',
      label: t('custom.commandCenter.supportBundleRetryWaitingDevices'),
      count: value.retry_waiting_count ?? 0,
      filter: 'retry_waiting',
      type: (value.retry_waiting_count ?? 0) > 0 ? 'info' : 'success'
    },
    {
      key: 'retry_exhausted',
      label: t('custom.commandCenter.supportBundleRetryExhaustedDevices'),
      count: value.retry_exhausted_count ?? 0,
      filter: 'retry_exhausted',
      type: (value.retry_exhausted_count ?? 0) > 0 ? 'error' : 'success'
    },
    {
      key: 'device_ack_failed',
      label: t('custom.commandCenter.jobAttentionDeviceFailed'),
      count: value.device_ack_failed_count ?? 0,
      filter: 'device_failed',
      type: (value.device_ack_failed_count ?? 0) > 0 ? 'error' : 'success'
    },
    {
      key: 'blocked',
      label: t('custom.commandCenter.jobAttentionBlocked'),
      count: value.blocked_count ?? 0,
      filter: 'blocked',
      type: (value.blocked_count ?? 0) > 0 ? 'error' : 'success'
    },
    {
      key: 'missing_log',
      label: t('custom.commandCenter.jobAttentionMissingLog'),
      count: value.log_missing_count ?? 0,
      filter: 'missing_log',
      type: (value.log_missing_count ?? 0) > 0 ? 'warning' : 'success'
    }
  ]
}
