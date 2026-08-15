import type { CommandJobRowsStatusFilter, FleetCommandJobSubmitResult } from '@/service/api/device'
import { buildCommandJobRowFacts } from './commandCenterJobRowFacts'

type Translate = (key: string) => string

export interface CommandJobTroubleshootingRow {
  key: string
  label: string
  value: string
  reviewRowsStatusFilter?: CommandJobRowsStatusFilter
  type: 'success' | 'info' | 'warning' | 'error'
}

export interface CommandJobOperatorNextAction {
  key:
    | 'retry'
    | 'retry-waiting'
    | 'retry-exhausted'
    | 'refresh'
    | 'logs'
    | 'blocked'
    | 'device-response'
    | 'support'
    | 'done'
  title: string
  description: string
  evidence: string
  primaryActionLabel: string
  primaryAction:
    | 'refresh'
    | 'retry'
    | 'copy-retryable'
    | 'preview-support'
    | 'copy-link'
    | 'none'
  reviewRowsStatusFilter?: CommandJobRowsStatusFilter
  type: 'success' | 'info' | 'warning' | 'error'
}

export interface CommandJobActionConsequenceRow {
  key: 'cancel' | 'retry' | 'waiting' | 'exhausted' | 'device-response' | 'logs'
  label: string
  value: string
  reviewRowsStatusFilter?: CommandJobRowsStatusFilter
  type: 'success' | 'info' | 'warning' | 'error'
}

type CommandJobOperatorDecisionFacts = {
  retryableCount: number
  retryReadyCount: number
  retryWaitingCount: number
  retryExhaustedCount: number
  missingLogCount: number
  pendingCount: number
  deviceFailedCount: number
  blockedCount: number
}

const buildCommandJobOperatorDecisionFacts = (
  result: FleetCommandJobSubmitResult
): CommandJobOperatorDecisionFacts => {
  const rowFacts = buildCommandJobRowFacts(result)
  return {
    retryableCount: result.retryable_count ?? rowFacts.retryableRows.length,
    retryReadyCount: result.retry_ready_count ?? rowFacts.retryReadyRows.length,
    retryWaitingCount: result.retry_waiting_count ?? rowFacts.retryWaitingRows.length,
    retryExhaustedCount: result.retry_exhausted_count ?? rowFacts.retryExhaustedRows.length,
    missingLogCount: result.log_missing_count ?? rowFacts.logMissingRows.length,
    pendingCount:
      result.progress_health?.pending_count ??
      Math.max(0, result.requested_count - (result.submitted_count || 0) - (result.failed_count || 0)),
    deviceFailedCount: rowFacts.deviceFailedRows.length,
    blockedCount: result.blocked_count || 0
  }
}

export function buildCommandJobActionConsequenceRows(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobActionConsequenceRow[] {
  if (!result) return []

  const facts = buildCommandJobOperatorDecisionFacts(result)
  const rows: CommandJobActionConsequenceRow[] = [
    {
      key: 'cancel',
      label: t('custom.commandCenter.cancelJob'),
      value: result.can_cancel
        ? t('custom.commandCenter.actionConsequenceCancel')
            .replace('{pending}', String(facts.pendingCount))
            .replace('{status}', result.status)
        : t('custom.commandCenter.actionConsequenceCancelUnavailable').replace('{status}', result.status),
      reviewRowsStatusFilter: result.can_cancel && facts.pendingCount > 0 ? 'in_progress' : undefined,
      type: result.can_cancel ? 'warning' : 'info'
    },
    {
      key: 'retry',
      label: t('custom.commandCenter.retryFailedJob'),
      value: result.can_retry_failed
        ? t('custom.commandCenter.actionConsequenceRetry')
            .replace('{ready}', String(facts.retryReadyCount))
            .replace('{waiting}', String(facts.retryWaitingCount))
            .replace('{exhausted}', String(facts.retryExhaustedCount))
        : t('custom.commandCenter.actionConsequenceRetryUnavailable'),
      reviewRowsStatusFilter: result.can_retry_failed && facts.retryReadyCount > 0 ? 'retry_ready' : undefined,
      type: result.can_retry_failed && facts.retryReadyCount > 0 ? 'error' : 'info'
    }
  ]

  if (facts.retryWaitingCount > 0) {
    rows.push({
      key: 'waiting',
      label: t('custom.commandCenter.supportBundleRetryWaitingDevices'),
      value: t('custom.commandCenter.actionConsequenceWaiting').replace('{count}', String(facts.retryWaitingCount)),
      reviewRowsStatusFilter: 'retry_waiting',
      type: 'warning'
    })
  }

  if (facts.retryExhaustedCount > 0) {
    rows.push({
      key: 'exhausted',
      label: t('custom.commandCenter.supportBundleRetryExhaustedDevices'),
      value: t('custom.commandCenter.actionConsequenceExhausted').replace('{count}', String(facts.retryExhaustedCount)),
      reviewRowsStatusFilter: 'retry_exhausted',
      type: 'error'
    })
  }

  if (facts.deviceFailedCount > 0) {
    rows.push({
      key: 'device-response',
      label: t('custom.commandCenter.jobAttentionDeviceFailed'),
      value: t('custom.commandCenter.actionConsequenceDeviceFailed').replace('{count}', String(facts.deviceFailedCount)),
      reviewRowsStatusFilter: 'device_failed',
      type: 'error'
    })
  }

  if (facts.missingLogCount > 0) {
    rows.push({
      key: 'logs',
      label: t('custom.commandCenter.jobAttentionMissingLog'),
      value: t('custom.commandCenter.actionConsequenceLogs').replace('{count}', String(facts.missingLogCount)),
      reviewRowsStatusFilter: 'missing_log',
      type: 'warning'
    })
  }

  return rows
}

export function buildCommandJobTroubleshootingRows(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobTroubleshootingRow[] {
  if (!result) return []

  const rows: CommandJobTroubleshootingRow[] = []
  const facts = buildCommandJobOperatorDecisionFacts(result)

  if (result.can_retry_failed && facts.retryReadyCount > 0) {
    rows.push({
      key: 'retry',
      label: t('custom.commandCenter.troubleshootingRetryTitle'),
      value: t('custom.commandCenter.troubleshootingRetryDesc').replace('{count}', String(facts.retryReadyCount)),
      reviewRowsStatusFilter: 'retry_ready',
      type: 'error'
    })
  }

  if (facts.retryWaitingCount > 0) {
    rows.push({
      key: 'retry-waiting',
      label: t('custom.commandCenter.troubleshootingRetryWaitingTitle'),
      value: t('custom.commandCenter.troubleshootingRetryWaitingDesc').replace('{count}', String(facts.retryWaitingCount)),
      reviewRowsStatusFilter: 'retry_waiting',
      type: 'warning'
    })
  }

  if (facts.retryExhaustedCount > 0) {
    rows.push({
      key: 'retry-exhausted',
      label: t('custom.commandCenter.troubleshootingRetryExhaustedTitle'),
      value: t('custom.commandCenter.troubleshootingRetryExhaustedDesc').replace('{count}', String(facts.retryExhaustedCount)),
      reviewRowsStatusFilter: 'retry_exhausted',
      type: 'error'
    })
  }

  if (result.can_cancel && facts.pendingCount > 0) {
    rows.push({
      key: 'cancel',
      label: t('custom.commandCenter.troubleshootingCancelTitle'),
      value: t('custom.commandCenter.troubleshootingCancelDesc').replace('{count}', String(facts.pendingCount)),
      reviewRowsStatusFilter: 'in_progress',
      type: 'warning'
    })
  }

  if (facts.missingLogCount > 0) {
    rows.push({
      key: 'logs',
      label: t('custom.commandCenter.troubleshootingLogsTitle'),
      value: t('custom.commandCenter.troubleshootingLogsDesc').replace('{count}', String(facts.missingLogCount)),
      reviewRowsStatusFilter: 'missing_log',
      type: 'warning'
    })
  }

  if (facts.blockedCount > 0) {
    rows.push({
      key: 'blocked',
      label: t('custom.commandCenter.troubleshootingBlockedTitle'),
      value: t('custom.commandCenter.troubleshootingBlockedDesc').replace('{count}', String(facts.blockedCount)),
      reviewRowsStatusFilter: 'needs_attention',
      type: 'error'
    })
  }

  if (rows.length === 0) {
    rows.push({
      key: 'support',
      label: t('custom.commandCenter.troubleshootingSupportTitle'),
      value: t('custom.commandCenter.troubleshootingSupportDesc'),
      type: result.status === 'completed' ? 'success' : 'info'
    })
  }

  return rows
}

export function buildCommandJobOperatorNextAction(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobOperatorNextAction | null {
  if (!result) return null

  const facts = buildCommandJobOperatorDecisionFacts(result)
  const evidence = t('custom.commandCenter.operatorDecisionEvidence')
    .replace('{retryable}', String(facts.retryableCount))
    .replace('{pending}', String(facts.pendingCount))
    .replace('{missingLogs}', String(facts.missingLogCount))
    .replace('{blocked}', String(facts.blockedCount))
    .replace('{deviceFailed}', String(facts.deviceFailedCount))

  if (facts.deviceFailedCount > 0) {
    return {
      key: 'device-response',
      title: t('custom.commandCenter.operatorDecisionDeviceFailedTitle'),
      description: t('custom.commandCenter.operatorDecisionDeviceFailedDesc').replace('{count}', String(facts.deviceFailedCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.previewSupportBundle'),
      primaryAction: 'preview-support',
      reviewRowsStatusFilter: 'device_failed',
      type: 'error'
    }
  }

  if (result.can_retry_failed && facts.retryReadyCount > 0) {
    return {
      key: 'retry',
      title: t('custom.commandCenter.operatorDecisionRetryTitle'),
      description: t('custom.commandCenter.operatorDecisionRetryDesc').replace('{count}', String(facts.retryReadyCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.retryFailedJob'),
      primaryAction: 'retry',
      reviewRowsStatusFilter: 'retry_ready',
      type: 'error'
    }
  }

  if (facts.retryWaitingCount > 0) {
    return {
      key: 'retry-waiting',
      title: t('custom.commandCenter.operatorDecisionRetryWaitingTitle'),
      description: t('custom.commandCenter.operatorDecisionRetryWaitingDesc').replace('{count}', String(facts.retryWaitingCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.refreshJob'),
      primaryAction: 'refresh',
      reviewRowsStatusFilter: 'retry_waiting',
      type: 'warning'
    }
  }

  if (facts.retryExhaustedCount > 0) {
    return {
      key: 'retry-exhausted',
      title: t('custom.commandCenter.operatorDecisionRetryExhaustedTitle'),
      description: t('custom.commandCenter.operatorDecisionRetryExhaustedDesc').replace('{count}', String(facts.retryExhaustedCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.previewSupportBundle'),
      primaryAction: 'preview-support',
      reviewRowsStatusFilter: 'retry_exhausted',
      type: 'error'
    }
  }

  if (result.status === 'running' || facts.pendingCount > 0) {
    return {
      key: 'refresh',
      title: t('custom.commandCenter.operatorDecisionRefreshTitle'),
      description: t('custom.commandCenter.operatorDecisionRefreshDesc').replace('{count}', String(facts.pendingCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.refreshJob'),
      primaryAction: 'refresh',
      reviewRowsStatusFilter: 'in_progress',
      type: 'info'
    }
  }

  if (facts.missingLogCount > 0) {
    return {
      key: 'logs',
      title: t('custom.commandCenter.operatorDecisionLogsTitle'),
      description: t('custom.commandCenter.operatorDecisionLogsDesc').replace('{count}', String(facts.missingLogCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.refreshJob'),
      primaryAction: 'refresh',
      reviewRowsStatusFilter: 'missing_log',
      type: 'warning'
    }
  }

  if (facts.blockedCount > 0) {
    return {
      key: 'blocked',
      title: t('custom.commandCenter.operatorDecisionBlockedTitle'),
      description: t('custom.commandCenter.operatorDecisionBlockedDesc').replace('{count}', String(facts.blockedCount)),
      evidence,
      primaryActionLabel: t('custom.commandCenter.copyJobLink'),
      primaryAction: 'copy-link',
      reviewRowsStatusFilter: 'needs_attention',
      type: 'error'
    }
  }

  if (result.status === 'completed') {
    return {
      key: 'done',
      title: t('custom.commandCenter.operatorDecisionDoneTitle'),
      description: t('custom.commandCenter.operatorDecisionDoneDesc'),
      evidence,
      primaryActionLabel: t('custom.commandCenter.copyJobLink'),
      primaryAction: 'copy-link',
      type: 'success'
    }
  }

  return {
    key: 'support',
    title: t('custom.commandCenter.operatorDecisionSupportTitle'),
    description: t('custom.commandCenter.operatorDecisionSupportDesc'),
    evidence,
    primaryActionLabel: t('custom.commandCenter.previewSupportBundle'),
    primaryAction: 'preview-support',
    type: 'info'
  }
}
