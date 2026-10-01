/**
 * Command Job 任务级视图：计数派生、进度健康、审计、治理、状态/时间线与交接文本。
 */
import type {
  FleetCommandJobGovernanceSummary,
  FleetCommandJobProgressHealth,
  FleetCommandJobSubmitResult,
  FleetCommandJobSupportBundle
} from '@/service/api/device'
import { buildCommandJobRowFacts } from './commandCenterJobRowFacts'
import {
  type CommandJobLabelValueRow,
  type CommandJobStatusCountRow,
  type CommandJobTone,
  type Translate,
  fillCommandJobTemplate,
  formatCommandJobDateTime,
  formatCommandJobDuration,
  formatCommandJobStatus,
  formatCommandJobYesNo,
  resolveCommandJobGovernanceLevelTone,
  resolveCommandJobGovernanceStateTone,
  resolveCommandJobProgressHealthTone,
  translateCommandJobOr
} from './commandCenterJobFormat'

export interface CommandJobProgressHealthCard {
  stateLabel: string
  type: CommandJobTone
  nextAction: string
  rows: CommandJobLabelValueRow[]
}

export interface CommandJobAuditSummaryCard {
  latestLabel: string
  nextAction: string
  rows: CommandJobLabelValueRow[]
}

export interface CommandJobGovernanceSummaryItem {
  key: string
  label: string
  value: string
  detail: string
  stateLabel: string
  type: CommandJobTone
}

export interface CommandJobGovernanceSummaryCard {
  title: string
  levelLabel: string
  summary: string
  nextAction: string
  type: CommandJobTone
  items: CommandJobGovernanceSummaryItem[]
}

type JobResult = FleetCommandJobSubmitResult | null

// ---------------------------------------------------------------------------
// 计数派生：优先使用后端聚合字段，缺失时回退到明细行统计
// ---------------------------------------------------------------------------

export function commandJobRetryableRows(result: JobResult) {
  return buildCommandJobRowFacts(result).retryableRows
}

export function commandJobLogMissingRows(result: JobResult) {
  return buildCommandJobRowFacts(result).logMissingRows
}

export function commandJobRetryableCount(result: JobResult) {
  return result?.retryable_count ?? commandJobRetryableRows(result).length
}

export function commandJobRetryReadyCount(result: JobResult) {
  return result?.retry_ready_count ?? buildCommandJobRowFacts(result).retryReadyRows.length
}

export function commandJobRetryWaitingCount(result: JobResult) {
  return result?.retry_waiting_count ?? buildCommandJobRowFacts(result).retryWaitingRows.length
}

export function commandJobRetryExhaustedCount(result: JobResult) {
  return result?.retry_exhausted_count ?? buildCommandJobRowFacts(result).retryExhaustedRows.length
}

export function commandJobLogMissingCount(result: JobResult) {
  return result?.log_missing_count ?? commandJobLogMissingRows(result).length
}

export function canRetryCommandJob(result: JobResult) {
  return Boolean(result?.can_retry_failed && commandJobRetryReadyCount(result) > 0)
}

export function commandJobProgressPercent(result: JobResult) {
  if (!result || result.requested_count <= 0) return 0
  const terminalCount = (result.submitted_count || 0) + (result.failed_count || 0)
  return Math.min(100, Math.round((terminalCount / result.requested_count) * 100))
}

export function buildCommandJobProgressSummary(result: JobResult, t: Translate) {
  if (!result) return ''
  return fillCommandJobTemplate(t('custom.commandCenter.jobProgressSummary'), {
    submitted: result.submitted_count,
    failed: result.failed_count,
    total: result.requested_count
  })
}

export function buildCommandJobCapabilitySummary(result: JobResult, t: Translate) {
  if (!result) return ''
  return fillCommandJobTemplate(t('custom.commandCenter.submitCapabilitySummary'), {
    cancel: formatCommandJobYesNo(result.can_cancel, t),
    retry: formatCommandJobYesNo(result.can_retry_failed, t)
  })
}

export function buildCommandJobEvidenceSummary(result: JobResult, t: Translate) {
  if (!result) return ''
  return fillCommandJobTemplate(t('custom.commandCenter.submitEvidenceSummary'), {
    retryable: commandJobRetryableCount(result),
    missingLogs: commandJobLogMissingCount(result)
  })
}

// ---------------------------------------------------------------------------
// 进度健康
// ---------------------------------------------------------------------------

/** 旧后端缺少 progress_health 时，由任务状态推导健康状态。 */
const COMMAND_JOB_FALLBACK_HEALTH_STATE: Readonly<Record<string, string>> = {
  completed: 'complete',
  scheduled: 'scheduled',
  running: 'running',
  canceled: 'canceled'
}

export function resolveCommandJobProgressHealth(result: FleetCommandJobSubmitResult): FleetCommandJobProgressHealth {
  if (result.progress_health) return result.progress_health
  const pendingCount = Math.max(
    0,
    result.requested_count - (result.submitted_count || 0) - (result.failed_count || 0) - (result.blocked_count || 0)
  )
  return {
    state: COMMAND_JOB_FALLBACK_HEALTH_STATE[result.status] ?? 'needs_attention',
    pending_count: pendingCount,
    terminal_count: Math.max(0, result.requested_count - pendingCount),
    elapsed_seconds: 0,
    timeout_remaining_seconds: 0,
    next_action: ''
  }
}

export function buildCommandJobProgressHealthCard(
  result: JobResult,
  t: Translate
): CommandJobProgressHealthCard | null {
  if (!result) return null
  const health = resolveCommandJobProgressHealth(result)

  return {
    stateLabel: translateCommandJobOr(t, `custom.commandCenter.progressHealthState.${health.state}`, health.state),
    type: resolveCommandJobProgressHealthTone(health.state),
    nextAction: health.next_action || t(`custom.commandCenter.progressHealthNext.${health.state}`),
    rows: [
      { label: t('custom.commandCenter.progressHealthPending'), value: String(health.pending_count) },
      { label: t('custom.commandCenter.progressHealthTerminal'), value: String(health.terminal_count) },
      {
        label: t('custom.commandCenter.progressHealthElapsed'),
        value: formatCommandJobDuration(health.elapsed_seconds, t)
      },
      {
        label: t('custom.commandCenter.progressHealthRemaining'),
        value: formatCommandJobDuration(health.timeout_remaining_seconds, t)
      }
    ]
  }
}

// ---------------------------------------------------------------------------
// 审计与治理
// ---------------------------------------------------------------------------

export function buildCommandJobAuditSummaryCard(result: JobResult, t: Translate): CommandJobAuditSummaryCard | null {
  if (!result?.audit_summary) return null
  const audit = result.audit_summary
  const latestType = audit.latest_event_type

  return {
    latestLabel: latestType
      ? translateCommandJobOr(t, `custom.commandCenter.jobEvent.${latestType}`, latestType)
      : '--',
    nextAction: audit.next_action,
    rows: [
      { label: t('custom.commandCenter.auditEventCount'), value: String(audit.event_count || 0) },
      { label: t('custom.commandCenter.auditLatestEvent'), value: latestType || '--' },
      { label: t('custom.commandCenter.auditLatestAt'), value: formatCommandJobDateTime(audit.latest_event_at) },
      { label: t('custom.commandCenter.auditLatestMessage'), value: audit.latest_message || '--' }
    ]
  }
}

export function buildCommandJobGovernanceSummaryCard(
  summary: FleetCommandJobGovernanceSummary | undefined,
  t: Translate
): CommandJobGovernanceSummaryCard | null {
  if (!summary) return null

  return {
    title: summary.title || t('custom.commandCenter.governanceSummaryTitle'),
    levelLabel: translateCommandJobOr(
      t,
      `custom.commandCenter.governanceLevel.${summary.level}`,
      summary.level || 'info'
    ),
    summary: summary.summary || '-',
    nextAction: summary.next_action || '-',
    type: resolveCommandJobGovernanceLevelTone(summary.level),
    items: (summary.items ?? []).map((item) => ({
      key: item.key || item.label,
      label: item.label || item.key,
      value: item.value || '-',
      detail: item.detail || '-',
      stateLabel: translateCommandJobOr(t, `custom.commandCenter.governanceState.${item.state}`, item.state),
      type: resolveCommandJobGovernanceStateTone(item.state)
    }))
  }
}
// ---------------------------------------------------------------------------
// 状态、计数与时间线
// ---------------------------------------------------------------------------

export function buildCommandJobStatusRows(result: JobResult, t: Translate): CommandJobLabelValueRow[] {
  if (!result) return []
  const optionalDateRow = (labelKey: string, value: string | undefined) =>
    value ? [{ label: t(labelKey), value: formatCommandJobDateTime(value) }] : []

  return [
    { label: t('custom.commandCenter.jobId'), value: result.job_id },
    { label: t('common.status'), value: formatCommandJobStatus(result.status, t) },
    { label: t('custom.commandCenter.jobProgress'), value: buildCommandJobProgressSummary(result, t) },
    { label: t('custom.commandCenter.createdAt'), value: formatCommandJobDateTime(result.created_at) },
    { label: t('custom.commandCenter.updatedAt'), value: formatCommandJobDateTime(result.updated_at) },
    ...optionalDateRow('custom.commandCenter.scheduledAt', result.scheduled_at),
    ...optionalDateRow('custom.commandCenter.nextDispatchAt', result.next_dispatch_at),
    { label: t('custom.commandCenter.timeoutAt'), value: formatCommandJobDateTime(result.timeout_at) }
  ]
}

export function buildCommandJobStatusCountRows(result: JobResult, t: Translate): CommandJobStatusCountRow[] {
  return Object.entries(result?.status_counts ?? {}).map(([status, count]) => ({
    status,
    label: formatCommandJobStatus(status, t),
    count
  }))
}

export function buildCommandJobTimelineRows(result: JobResult, t: Translate): CommandJobLabelValueRow[] {
  if (!result) return []
  if (result.events?.length) {
    return result.events.map((event, index) => ({
      key: event.id || `${event.event_type}-${index}`,
      label: translateCommandJobOr(t, `custom.commandCenter.jobEvent.${event.event_type}`, event.event_type),
      value: [formatCommandJobDateTime(event.created_at), event.device_id, event.message].filter(Boolean).join(' - ')
    }))
  }

  const { submittedRows, completedRows } = buildCommandJobRowFacts(result)
  const deviceCount = (count: number) =>
    count ? fillCommandJobTemplate(t('custom.commandCenter.timelineDeviceCount'), { count }) : '--'

  return [
    {
      key: 'created',
      label: t('custom.commandCenter.timelineCreated'),
      value: formatCommandJobDateTime(result.created_at)
    },
    { key: 'submitted', label: t('custom.commandCenter.timelineSubmitted'), value: deviceCount(submittedRows.length) },
    { key: 'completed', label: t('custom.commandCenter.timelineCompleted'), value: deviceCount(completedRows.length) }
  ]
}

// ---------------------------------------------------------------------------
// 交接 / 关闭文本（面向复制粘贴，保持英文固定格式）
// ---------------------------------------------------------------------------

export function buildCommandJobHandoffSummary(result: JobResult, jobLink = '') {
  if (!result) return ''
  const execution = result.execution_summary
  const summary =
    result.handoff_summary ||
    `Command Job ${result.job_id} is ${result.status}: ${result.submitted_count}/${result.requested_count} submitted, ${result.failed_count} failed, ${result.blocked_count} blocked.`
  let closeReadiness = ''
  if (execution) {
    const blockers = execution.close_blockers?.length ? ` - ${execution.close_blockers.join(' ')}` : '.'
    closeReadiness = execution.can_close ? 'Close readiness: ready to close.' : `Close readiness: blocked${blockers}`
  }
  const nextAction = execution?.next_action ? `Next action: ${execution.next_action}` : ''
  return [summary, closeReadiness, nextAction, jobLink].filter(Boolean).join('\n')
}

function buildCommandJobCloseoutSupportLines(supportBundle?: FleetCommandJobSupportBundle | null) {
  if (!supportBundle) return ['Support bundle generated: not loaded in this browser session']
  return [
    `Support bundle generated: ${formatCommandJobDateTime(supportBundle.generated_at)}`,
    `Support next actions: ${(supportBundle.next_actions ?? []).join('; ') || '-'}`,
    `Retry ready/waiting/exhausted: ${supportBundle.retry_ready_count}/${supportBundle.retry_waiting_count}/${supportBundle.retry_exhausted_count}`,
    `Failed devices: ${supportBundle.failed_devices?.length ?? 0}`,
    `Missing log devices: ${supportBundle.missing_log_device_ids?.length ?? 0}`,
    `Support events: ${supportBundle.events?.length ?? 0}`
  ]
}

export function buildCommandJobCloseoutPacket(
  result: JobResult,
  jobLink = '',
  supportBundle?: FleetCommandJobSupportBundle | null
) {
  if (!result) return ''
  const execution = result.execution_summary
  const audit = result.audit_summary
  const closeBlockers = execution?.close_blockers?.filter(Boolean) ?? []
  const checklist = execution?.checklist?.filter((item) => item.key || item.label) ?? []

  return [
    'AetherLink Command Job closeout packet',
    `Job: ${result.job_id}`,
    `Status: ${result.status}`,
    `Progress: ${result.submitted_count}/${result.requested_count} submitted, ${result.failed_count} failed, ${result.blocked_count} blocked`,
    `Close readiness: ${execution?.can_close ? 'ready' : 'blocked'}`,
    `Close blockers: ${closeBlockers.length ? closeBlockers.join('; ') : 'none'}`,
    execution?.next_action ? `Next action: ${execution.next_action}` : '',
    checklist.length ? 'Checklist:' : '',
    ...checklist.map((item) => `- [${item.state || 'unknown'}] ${item.label || item.key}: ${item.detail || '-'}`),
    audit
      ? `Audit: ${audit.event_count || 0} events, latest=${audit.latest_event_type || '-'}, at=${formatCommandJobDateTime(audit.latest_event_at)}, message=${audit.latest_message || '-'}`
      : 'Audit: no audit summary loaded',
    ...buildCommandJobCloseoutSupportLines(supportBundle),
    jobLink ? `Job link: ${jobLink}` : ''
  ]
    .filter(Boolean)
    .join('\n')
}
