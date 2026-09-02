import { computed, type Ref } from 'vue'
import type {
  FleetCommandJobSubmitResult,
  FleetCommandJobSubmitRow,
  FleetCommandJobSupportBundle
} from '@/service/api/device'
import {
  buildCommandJobActionConsequenceRows,
  buildCommandJobCapabilitySummary,
  buildCommandJobAuditSummaryCard,
  buildCommandJobEvidenceSummary,
  buildCommandJobExecutionSummaryCard,
  buildCommandJobGovernanceSummaryCard,
  buildCommandJobDeviceProgressTracks,
  buildCommandJobHandoffSummary,
  buildCommandJobOperatorNextAction,
  buildCommandJobOutcomeGroups,
  buildCommandJobProgressHealthCard,
  buildCommandJobProgressSummary,
  buildCommandJobStatusCountRows,
  buildCommandJobStatusRows,
  buildCommandJobSupportBundlePreview,
  buildCommandJobTimelineRows,
  buildCommandJobTroubleshootingRows,
  canRetryCommandJob,
  commandJobRetryReadyCount,
  commandJobRetryWaitingCount,
  commandJobRetryExhaustedCount,
  commandJobProgressPercent,
  formatCommandJobStatus
} from './commandCenterJobView'

type Translate = (key: string) => string
type CommandJobEvidenceAlertType = 'success' | 'info' | 'warning' | 'error'
const COMMAND_JOB_RESULT_DISPLAY_LIMIT = 200

const commandJobSubmitRowPriority = (row: FleetCommandJobSubmitRow) => {
  if (row.can_retry) return 0
  if (row.status === 'failed') return 1
  if (row.eligible && !row.log_recorded) return 2
  if (!row.eligible || row.recommended_path === 'blocked') return 3
  if (row.status && row.status !== 'completed') return 4
  return 10
}

export const commandJobSubmitRowsForCustomer = (
  rows: FleetCommandJobSubmitRow[],
  limit = COMMAND_JOB_RESULT_DISPLAY_LIMIT
) => {
  if (limit <= 0 || rows.length <= limit) {
    return [...rows].sort((left, right) => commandJobSubmitRowPriority(left) - commandJobSubmitRowPriority(right))
  }

  const buckets: FleetCommandJobSubmitRow[][] = Array.from({ length: 11 }, () => [])
  for (const row of rows) {
    const priority = commandJobSubmitRowPriority(row)
    buckets[Math.min(priority, buckets.length - 1)].push(row)
  }

  const selected: FleetCommandJobSubmitRow[] = []
  for (const bucket of buckets) {
    for (const row of bucket) {
      selected.push(row)
      if (selected.length >= limit) return selected
    }
  }
  return selected
}

export const commandJobSubmitRowsHiddenCount = (result: FleetCommandJobSubmitResult | null, displayedCount: number) => {
  if (!result) return 0
  const total = Math.max(result.rows_total ?? result.rows.length, result.rows.length)
  return Math.max(0, total - displayedCount)
}

export function useCommandCenterSubmitEvidenceView(options: {
  submitResult: Ref<FleetCommandJobSubmitResult | null>
  supportBundle: Ref<FleetCommandJobSupportBundle | null>
  t: Translate
}) {
  const submitCapabilitySummary = computed(() =>
    buildCommandJobCapabilitySummary(options.submitResult.value, options.t)
  )
  const jobAuditSummaryCard = computed(() => buildCommandJobAuditSummaryCard(options.submitResult.value, options.t))
  const jobExecutionSummaryCard = computed(() =>
    buildCommandJobExecutionSummaryCard(options.submitResult.value, options.t)
  )
  const jobGovernanceSummaryCard = computed(() =>
    buildCommandJobGovernanceSummaryCard(options.submitResult.value?.governance_summary, options.t)
  )
  const submitEvidenceSummary = computed(() => buildCommandJobEvidenceSummary(options.submitResult.value, options.t))
  const jobHandoffSummary = computed(() => buildCommandJobHandoffSummary(options.submitResult.value))
  const submitEvidenceAlertType = computed<CommandJobEvidenceAlertType>(() => {
    const result = options.submitResult.value
    if (!result) return 'info'
    if (
      (result.blocked_count || 0) > 0 ||
      commandJobRetryReadyCount(result) > 0 ||
      commandJobRetryExhaustedCount(result) > 0
    )
      return 'error'
    if (
      (result.failed_count || 0) > 0 ||
      commandJobRetryWaitingCount(result) > 0 ||
      (result.log_missing_count || 0) > 0
    )
      return 'warning'
    return 'success'
  })
  const submitRowsForCustomer = computed(() => commandJobSubmitRowsForCustomer(options.submitResult.value?.rows ?? []))
  const submitRowsHiddenCount = computed(() =>
    commandJobSubmitRowsHiddenCount(options.submitResult.value, submitRowsForCustomer.value.length)
  )
  const jobProgressPercent = computed(() => commandJobProgressPercent(options.submitResult.value))
  const jobProgressHealthCard = computed(() => buildCommandJobProgressHealthCard(options.submitResult.value, options.t))
  const jobProgressSummary = computed(() => buildCommandJobProgressSummary(options.submitResult.value, options.t))
  const jobStatusLabel = computed(() => formatCommandJobStatus(options.submitResult.value?.status, options.t))
  const jobActionConsequenceRows = computed(() =>
    buildCommandJobActionConsequenceRows(options.submitResult.value, options.t)
  )
  const jobDeviceProgressTracks = computed(() =>
    buildCommandJobDeviceProgressTracks(options.submitResult.value, options.t)
  )
  const jobStatusRows = computed(() => buildCommandJobStatusRows(options.submitResult.value, options.t))
  const jobStatusCountRows = computed(() => buildCommandJobStatusCountRows(options.submitResult.value, options.t))
  const jobTimelineRows = computed(() => buildCommandJobTimelineRows(options.submitResult.value, options.t))
  const jobTroubleshootingRows = computed(() =>
    buildCommandJobTroubleshootingRows(options.submitResult.value, options.t)
  )
  const jobOperatorNextAction = computed(() => buildCommandJobOperatorNextAction(options.submitResult.value, options.t))
  const jobOutcomeGroups = computed(() => buildCommandJobOutcomeGroups(options.submitResult.value, options.t))
  const supportBundlePreview = computed(() =>
    buildCommandJobSupportBundlePreview(options.supportBundle.value, options.t)
  )
  const canRetryCurrentCommandJob = computed(() => canRetryCommandJob(options.submitResult.value))

  return {
    canRetryCurrentCommandJob,
    jobAuditSummaryCard,
    jobExecutionSummaryCard,
    jobGovernanceSummaryCard,
    jobActionConsequenceRows,
    jobOutcomeGroups,
    jobOperatorNextAction,
    jobProgressHealthCard,
    jobProgressPercent,
    jobProgressSummary,
    jobHandoffSummary,
    jobDeviceProgressTracks,
    jobStatusCountRows,
    jobStatusLabel,
    jobStatusRows,
    jobTimelineRows,
    jobTroubleshootingRows,
    submitCapabilitySummary,
    submitEvidenceAlertType,
    submitEvidenceSummary,
    submitRowsHiddenCount,
    submitRowsForCustomer,
    supportBundlePreview
  }
}
