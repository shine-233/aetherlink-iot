import type {
  FleetCommandJobExecutionSummary,
  FleetCommandJobGovernanceSummary,
  FleetCommandJobSupportBundle,
  FleetCommandJobSupportDiagnostic
} from '@/service/api/device'

type Translate = (key: string) => string

export interface CommandJobLabelValueRow {
  key?: string
  label: string
  value: string
}

export interface CommandJobSupportBundlePreview {
  summaryRows: CommandJobLabelValueRow[]
  executionSummary: CommandJobExecutionSummaryCard | null
  governanceSummary: CommandJobGovernanceSummaryCard | null
  nextActions: string[]
  failedDeviceEvidence: CommandJobSupportFailedDeviceEvidence[]
}

export interface CommandJobSupportFailedDeviceEvidence {
  key: string
  deviceId: string
  detailId?: string
  readyCheckUrl?: string
  jobDetailUrl?: string
  diagnosticSummary?: CommandJobSupportDiagnosticPreview
  rows: CommandJobLabelValueRow[]
}

export interface CommandJobSupportDiagnosticPreview {
  type: 'success' | 'info' | 'warning' | 'error'
  code: string
  summary: string
  evidence: string[]
  nextActions: string[]
}

export interface CommandJobExecutionSummaryCard {
  pathLabel: string
  decisionLabel: string
  canClose: boolean
  closeBlockers: string[]
  nextAction: string
  type: 'success' | 'info' | 'warning' | 'error'
  evidence: string[]
  checklist: CommandJobExecutionChecklistItem[]
}

export interface CommandJobExecutionChecklistItem {
  key: string
  label: string
  detail: string
  stateLabel: string
  type: 'success' | 'info' | 'warning' | 'error'
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

const commandJobExecutionDecisionType = (decision: string): CommandJobExecutionSummaryCard['type'] => {
  if (decision === 'close') return 'success'
  if (decision === 'monitor') return 'info'
  if (decision === 'wait' || decision === 'watch_timeout' || decision === 'collect_evidence') return 'warning'
  return 'error'
}

const commandJobChecklistStateType = (state: string): CommandJobExecutionChecklistItem['type'] => {
  if (state === 'done') return 'success'
  if (state === 'watch') return 'warning'
  if (state === 'blocked') return 'error'
  if (state === 'todo') return 'error'
  return 'info'
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

const formatCommandJobDateTime = (value?: string) => {
  if (!value) return '--'
  return value
    .replace('T', ' ')
    .replace(/\.\d+Z$/, ' UTC')
    .replace(/Z$/, ' UTC')
}

const formatCommandJobStatus = (status: string | undefined, t: Translate) => {
  if (!status) return '-'
  const key = `custom.commandCenter.jobStatus.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

const formatCommandJobReadiness = (readiness: string[] | undefined) => {
  const items = readiness?.filter(Boolean) ?? []
  return items.length ? items.join(', ') : '-'
}

const formatCommandJobResponseStatus = (statusLabel: string | undefined, t: Translate) => {
  if (!statusLabel) return t('custom.commandCenter.responseStatus.awaiting')
  const key = `custom.commandCenter.responseStatus.${statusLabel}`
  const translated = t(key)
  return translated === key ? statusLabel : translated
}

const formatCommandJobDispatchAttempts = (attempts?: number, maxAttempts?: number) => {
  const used = attempts ?? 0
  return maxAttempts ? `${used}/${maxAttempts}` : String(used)
}

const formatCommandJobRetryState = (state: string | undefined, t: Translate) => {
  if (!state) return '-'
  const key = `custom.commandCenter.retryState.${state}`
  const translated = t(key)
  return translated === key ? state : translated
}

const normalizeCommandJobSupportDiagnosticType = (
  level: string | undefined
): CommandJobSupportDiagnosticPreview['type'] => {
  if (level === 'ok' || level === 'success') return 'success'
  if (level === 'warning') return 'warning'
  if (level === 'error') return 'error'
  return 'info'
}

const formatCommandJobDiagnosticEvidence = (item: string, t: Translate) => {
  const attemptsPrefix = 'dispatch_attempts='
  if (item.startsWith(attemptsPrefix)) {
    return `${t('custom.commandCenter.dispatchAttempts')}: ${item.slice(attemptsPrefix.length)}`
  }
  const maxAttemptsPrefix = 'max_dispatch_attempts='
  if (item.startsWith(maxAttemptsPrefix)) {
    return `${t('custom.commandCenter.maxDispatchAttempts')}: ${item.slice(maxAttemptsPrefix.length)}`
  }
  const retryStatePrefix = 'retry_state='
  if (item.startsWith(retryStatePrefix)) {
    return `${t('custom.commandCenter.retryState')}: ${formatCommandJobRetryState(item.slice(retryStatePrefix.length), t)}`
  }
  const nextRetryPrefix = 'next_retry_after='
  if (item.startsWith(nextRetryPrefix)) {
    return `${t('custom.commandCenter.nextRetryAfter')}: ${formatCommandJobDateTime(item.slice(nextRetryPrefix.length))}`
  }
  return item
}

const buildCommandJobExecutionSummaryPreview = (
  summary: FleetCommandJobExecutionSummary | undefined,
  t: Translate
): CommandJobExecutionSummaryCard | null => {
  if (!summary) return null
  const decisionKey = `custom.commandCenter.executionDecision.${summary.decision}`
  const translatedDecision = t(decisionKey)

  return {
    pathLabel: summary.path_label || summary.path_type || '-',
    decisionLabel: translatedDecision === decisionKey ? summary.decision : translatedDecision,
    canClose: Boolean(summary.can_close),
    closeBlockers: summary.close_blockers?.filter(Boolean) ?? [],
    nextAction: summary.next_action,
    type: commandJobExecutionDecisionType(summary.decision),
    evidence: summary.evidence?.filter(Boolean) ?? [],
    checklist: (summary.checklist ?? []).map((item) => {
      const stateKey = `custom.commandCenter.executionChecklistState.${item.state}`
      const translatedState = t(stateKey)
      return {
        key: item.key || item.label,
        label: item.label || item.key,
        detail: item.detail || '-',
        stateLabel: translatedState === stateKey ? item.state : translatedState,
        type: commandJobChecklistStateType(item.state)
      }
    })
  }
}

export const buildCommandJobExecutionSummaryCard = (
  result: { execution_summary?: FleetCommandJobExecutionSummary } | null,
  t: Translate
): CommandJobExecutionSummaryCard | null => {
  if (!result?.execution_summary) return null
  return buildCommandJobExecutionSummaryPreview(result.execution_summary, t)
}

const buildCommandJobGovernanceSummaryPreview = (
  summary: FleetCommandJobGovernanceSummary | undefined,
  t: Translate
): CommandJobGovernanceSummaryCard | null => {
  if (!summary) return null
  const levelKey = `custom.commandCenter.governanceLevel.${summary.level}`
  const translatedLevel = t(levelKey)
  return {
    title: summary.title || t('custom.commandCenter.governanceSummaryTitle'),
    levelLabel: translatedLevel === levelKey ? summary.level || 'info' : translatedLevel,
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

const buildCommandJobSupportDiagnosticPreview = (
  diagnostic: FleetCommandJobSupportDiagnostic | undefined,
  t: Translate
): CommandJobSupportDiagnosticPreview | undefined => {
  if (!diagnostic) return undefined
  return {
    type: normalizeCommandJobSupportDiagnosticType(diagnostic.level),
    code: diagnostic.code || '-',
    summary: diagnostic.summary || diagnostic.code || '-',
    evidence: diagnostic.evidence?.filter(Boolean).map((item) => formatCommandJobDiagnosticEvidence(item, t)) ?? [],
    nextActions: diagnostic.next_actions?.filter(Boolean) ?? []
  }
}

export const buildCommandJobSupportBundlePreview = (
  bundle: FleetCommandJobSupportBundle | null,
  t: Translate
): CommandJobSupportBundlePreview | null => {
  if (!bundle) return null

  return {
    summaryRows: [
      { label: t('custom.commandCenter.supportBundleGeneratedAt'), value: formatCommandJobDateTime(bundle.generated_at) },
      ...(bundle.scheduled_at
        ? [{ label: t('custom.commandCenter.scheduledAt'), value: formatCommandJobDateTime(bundle.scheduled_at) }]
        : []),
      ...(bundle.next_dispatch_at
        ? [{ label: t('custom.commandCenter.nextDispatchAt'), value: formatCommandJobDateTime(bundle.next_dispatch_at) }]
        : []),
      {
        label: t('custom.commandCenter.supportBundleRetryableDevices'),
        value: String(bundle.retryable_device_ids?.length ?? 0)
      },
      {
        label: t('custom.commandCenter.supportBundleRetryReadyDevices'),
        value: String(bundle.retry_ready_count ?? 0)
      },
      {
        label: t('custom.commandCenter.supportBundleRetryWaitingDevices'),
        value: String(bundle.retry_waiting_count ?? 0)
      },
      {
        label: t('custom.commandCenter.supportBundleRetryExhaustedDevices'),
        value: String(bundle.retry_exhausted_count ?? 0)
      },
      {
        label: t('custom.commandCenter.supportBundleMissingLogDevices'),
        value: String(bundle.missing_log_device_ids?.length ?? 0)
      },
      { label: t('custom.commandCenter.supportBundleFailedDevices'), value: String(bundle.failed_devices?.length ?? 0) },
      { label: t('custom.commandCenter.supportBundleEvents'), value: String(bundle.events?.length ?? 0) },
      { label: t('custom.commandCenter.supportBundleShareHint'), value: bundle.share_hint || '-' }
    ],
    executionSummary: buildCommandJobExecutionSummaryPreview(bundle.execution_summary, t),
    governanceSummary: buildCommandJobGovernanceSummaryPreview(bundle.governance_summary, t),
    nextActions: bundle.next_actions ?? [],
    failedDeviceEvidence: (bundle.failed_devices ?? []).slice(0, 5).map((device, index) => {
      const diagnosticSummary = buildCommandJobSupportDiagnosticPreview(device.diagnostic_summary, t)
      const rows = [
        {
          label: t('custom.commandCenter.supportBundleDevice'),
          value: device.device_number || device.name || device.device_id
        },
        { label: t('common.status'), value: formatCommandJobStatus(device.status, t) },
        { label: t('custom.commandCenter.readinessEvidence'), value: formatCommandJobReadiness(device.readiness) },
        { label: t('custom.commandCenter.reason'), value: device.reason || '-' },
        { label: t('custom.commandCenter.advice'), value: device.advice || '-' },
        {
          label: t('custom.commandCenter.dispatchAttempts'),
          value: formatCommandJobDispatchAttempts(device.dispatch_attempts, device.max_dispatch_attempts)
        },
        {
          label: t('custom.commandCenter.retryState'),
          value: formatCommandJobRetryState(device.retry_state, t)
        },
        {
          label: t('custom.commandCenter.nextRetryAfter'),
          value: formatCommandJobDateTime(device.next_retry_after)
        },
        { label: t('custom.commandCenter.messageId'), value: device.message_id || '-' }
      ]
      if (device.response_status_label || device.response_status) {
        rows.push({
          label: t('custom.commandCenter.deviceResponseStatus'),
          value: formatCommandJobResponseStatus(device.response_status_label, t)
        })
      }
      if (device.response_error || device.response_data) {
        rows.push({
          label: t('custom.commandCenter.deviceResponseEvidence'),
          value: device.response_error || device.response_data || '-'
        })
      }
      return {
        key: device.device_id || device.message_id || `support-device-${index}`,
        deviceId: device.device_id,
        detailId: device.detail_id,
        readyCheckUrl: device.ready_check_url,
        jobDetailUrl: device.job_detail_url,
        diagnosticSummary,
        rows
      }
    })
  }
}
