/**
 * Command Job 历史列表视图：状态/关注筛选项、行进度与关注聚合。
 *
 * 关注维度只在 COMMAND_JOB_ATTENTION_METRICS 定义一次，筛选项与聚合行都由它派生。
 */
import type { FleetCommandJobListAttentionCounts, FleetCommandJobListItem } from '@/service/api/device'
import {
  type CommandJobTone,
  type Translate,
  fillCommandJobTemplate,
  formatCommandJobStatus,
  resolveCommandJobCountTone
} from './commandCenterJobFormat'

export interface CommandJobHistoryAttentionAggregateRow {
  key: string
  label: string
  count: number
  filter?: string
  type: CommandJobTone
}

type AttentionCountField = keyof FleetCommandJobListAttentionCounts

interface CommandJobAttentionMetric {
  /** 聚合行 key。 */
  key: string
  /** 历史列表 attention 查询参数。 */
  filter: string
  labelKey: string
  countField: AttentionCountField
  /** 计数大于 0 时的语气。 */
  activeTone: CommandJobTone
}

const metric = (
  key: string,
  filter: string,
  labelKey: string,
  countField: AttentionCountField,
  activeTone: CommandJobTone
): CommandJobAttentionMetric => ({ key, filter, labelKey, countField, activeTone })

export const COMMAND_JOB_ATTENTION_METRICS = {
  needsOperatorAction: metric(
    'needs_operator_action',
    'needs_operator_action',
    'custom.commandCenter.jobAttentionNeedsOperatorAction',
    'needs_operator_action_count',
    'warning'
  ),
  retryable: metric(
    'retryable',
    'retryable',
    'custom.commandCenter.jobAttentionRetryable',
    'retryable_count',
    'warning'
  ),
  retryReady: metric(
    'retry_ready',
    'retry_ready',
    'custom.commandCenter.supportBundleRetryReadyDevices',
    'retry_ready_count',
    'warning'
  ),
  retryWaiting: metric(
    'retry_waiting',
    'retry_waiting',
    'custom.commandCenter.supportBundleRetryWaitingDevices',
    'retry_waiting_count',
    'info'
  ),
  retryExhausted: metric(
    'retry_exhausted',
    'retry_exhausted',
    'custom.commandCenter.supportBundleRetryExhaustedDevices',
    'retry_exhausted_count',
    'error'
  ),
  deviceFailed: metric(
    'device_ack_failed',
    'device_failed',
    'custom.commandCenter.jobAttentionDeviceFailed',
    'device_ack_failed_count',
    'error'
  ),
  missingLog: metric(
    'missing_log',
    'missing_log',
    'custom.commandCenter.jobAttentionMissingLog',
    'log_missing_count',
    'warning'
  ),
  blocked: metric('blocked', 'blocked', 'custom.commandCenter.jobAttentionBlocked', 'blocked_count', 'error')
} as const satisfies Record<string, CommandJobAttentionMetric>

const M = COMMAND_JOB_ATTENTION_METRICS

/** 下拉筛选项顺序。 */
const COMMAND_JOB_ATTENTION_OPTION_ORDER: readonly CommandJobAttentionMetric[] = [
  M.needsOperatorAction,
  M.retryable,
  M.retryReady,
  M.retryWaiting,
  M.retryExhausted,
  M.deviceFailed,
  M.missingLog,
  M.blocked
]

/** 聚合卡片顺序（不含 retryable：它是 ready+waiting+exhausted 的合计）。 */
const COMMAND_JOB_ATTENTION_AGGREGATE_ORDER: readonly CommandJobAttentionMetric[] = [
  M.needsOperatorAction,
  M.retryReady,
  M.retryWaiting,
  M.retryExhausted,
  M.deviceFailed,
  M.blocked,
  M.missingLog
]

export const COMMAND_JOB_HISTORY_STATUSES = [
  'scheduled',
  'running',
  'completed',
  'partially_failed',
  'failed',
  'canceled'
] as const

export function buildCommandJobHistoryStatusOptions(t: Translate) {
  return [
    { label: t('custom.commandCenter.jobStatusAll'), value: '' },
    ...COMMAND_JOB_HISTORY_STATUSES.map((status) => ({ label: formatCommandJobStatus(status, t), value: status }))
  ]
}

const formatCommandJobAttentionOptionLabel = (label: string, count: number | undefined) =>
  typeof count === 'number' && count > 0 ? `${label} (${count})` : label

export function buildCommandJobHistoryAttentionOptions(t: Translate, counts?: FleetCommandJobListAttentionCounts) {
  return [
    { label: t('custom.commandCenter.jobAttentionAll'), value: '' },
    ...COMMAND_JOB_ATTENTION_OPTION_ORDER.map((item) => ({
      label: formatCommandJobAttentionOptionLabel(t(item.labelKey), counts?.[item.countField]),
      value: item.filter
    }))
  ]
}

export function buildCommandJobHistoryProgress(row: FleetCommandJobListItem, t: Translate) {
  return `${row.submitted_count}/${row.requested_count} ${t('custom.commandCenter.submittedShort')}, ${row.failed_count} ${t('custom.commandCenter.failedShort')}`
}

/** 单行与全量汇总共用：列表项本身就携带同名的关注计数字段。 */
function buildCommandJobAttentionSummaryText(counts: FleetCommandJobListAttentionCounts | undefined, t: Translate) {
  const needsAction = counts?.needs_operator_action_count ?? 0
  if (needsAction <= 0) return t('custom.commandCenter.jobAttentionNone')
  return fillCommandJobTemplate(t('custom.commandCenter.jobAttentionSummary'), {
    count: needsAction,
    retryable: counts?.retryable_count ?? 0,
    deviceFailed: counts?.device_ack_failed_count ?? 0,
    missingLogs: counts?.log_missing_count ?? 0
  })
}

export function buildCommandJobHistoryAttentionSummary(row: FleetCommandJobListItem, t: Translate) {
  return buildCommandJobAttentionSummaryText(row, t)
}

export function buildCommandJobHistoryAttentionTotalSummary(
  counts: FleetCommandJobListAttentionCounts | undefined,
  t: Translate
) {
  return buildCommandJobAttentionSummaryText(counts, t)
}

export function buildCommandJobHistoryAttentionAggregateRows(
  counts: FleetCommandJobListAttentionCounts | undefined,
  t: Translate
): CommandJobHistoryAttentionAggregateRow[] {
  return COMMAND_JOB_ATTENTION_AGGREGATE_ORDER.map((item) => {
    const count = counts?.[item.countField] ?? 0
    return {
      key: item.key,
      label: t(item.labelKey),
      count,
      filter: item.filter,
      type: resolveCommandJobCountTone(count, item.activeTone)
    }
  })
}
