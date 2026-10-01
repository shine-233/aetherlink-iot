/**
 * Command Job 设备级视图：单设备下一步建议、结果分组与进度轨道。
 *
 * 行分类以有序规则表表达（首个命中生效），分组与语气都由表驱动，
 * 便于对每种状态组合单独做回归测试。
 */
import type { FleetCommandJobSubmitResult, FleetCommandJobSubmitRow } from '@/service/api/device'
import { isFutureCommandJobDate, resolveCommandJobRowRetryState } from './commandCenterJobRowFacts'
import {
  type CommandJobRule,
  type CommandJobTone,
  type CommandJobToneTable,
  type Translate,
  commandJobToneSeverity,
  createCommandJobToneResolver,
  fillCommandJobTemplate,
  formatCommandJobDateTime,
  formatCommandJobReadiness,
  formatCommandJobResponseStatus,
  formatCommandJobStatus,
  matchCommandJobRule,
  resolveCommandJobDeviceStepTone
} from './commandCenterJobFormat'

export interface CommandJobOutcomeDeviceRow {
  key: string
  deviceId: string
  device: string
  status: string
  readiness: string
  reason: string
  action: string
}

export type CommandJobOutcomeGroupKey =
  'retryable' | 'device_failed' | 'missing_logs' | 'blocked' | 'in_progress' | 'completed'

export interface CommandJobOutcomeGroup {
  key: CommandJobOutcomeGroupKey
  title: string
  description: string
  count: number
  type: CommandJobTone
  rows: CommandJobOutcomeDeviceRow[]
}

export interface CommandJobDeviceProgressStep {
  key: 'preview' | 'dispatch' | 'ack' | 'evidence'
  label: string
  state: string
  detail: string
  type: CommandJobTone
}

export interface CommandJobDeviceProgressTrack {
  key: string
  deviceId: string
  device: string
  summary: string
  nextAction: string
  type: CommandJobTone
  steps: CommandJobDeviceProgressStep[]
}

type Row = FleetCommandJobSubmitRow

// ---------------------------------------------------------------------------
// 行谓词
// ---------------------------------------------------------------------------

const isAckSuccess = (row: Row) => row.response_status_label === 'device_ack_success'
const isAckFailed = (row: Row) => row.response_status_label === 'device_ack_failed'
const isPreviewBlocked = (row: Row) => !row.eligible || row.recommended_path === 'blocked'
const isLogMissing = (row: Row) => Boolean(row.eligible && !row.log_recorded)
const isRetryExhausted = (row: Row) => resolveCommandJobRowRetryState(row) === 'max_attempts_reached'
const isRetryWaiting = (row: Row) => resolveCommandJobRowRetryState(row) === 'waiting_backoff'

function commandJobRowIdentity(row: Row) {
  return {
    key: row.detail_id || row.device_id || row.message_id || `${row.status}-${row.device_number || row.name}`,
    deviceId: row.device_id,
    device: row.device_number || row.name || row.device_id
  }
}

// ---------------------------------------------------------------------------
// 下一步建议
// ---------------------------------------------------------------------------

export function buildCommandJobSubmitNextAction(row: Row, t: Translate) {
  if (isAckSuccess(row)) return t('custom.commandCenter.nextActionDeviceAckSuccess')
  if (isAckFailed(row)) return row.response_error || t('custom.commandCenter.nextActionDeviceAckFailed')
  if (isRetryWaiting(row) && isFutureCommandJobDate(row.next_retry_after)) {
    return fillCommandJobTemplate(t('custom.commandCenter.nextActionRetryAfter'), {
      time: formatCommandJobDateTime(row.next_retry_after)
    })
  }
  if (isRetryExhausted(row)) return t('custom.commandCenter.nextActionRetryLimitReached')
  if (row.advice) return row.advice
  if (row.can_retry) return t('custom.commandCenter.nextActionRetry')
  if (isLogMissing(row)) return t('custom.commandCenter.nextActionRefreshLogs')
  if (isPreviewBlocked(row)) return t('custom.commandCenter.nextActionFixBlocker')
  if (row.status === 'completed') return t('custom.commandCenter.nextActionCompleted')
  if (row.message_id) return t('custom.commandCenter.nextActionTrackMessage')
  return t('custom.commandCenter.nextActionSupportBundle')
}

// ---------------------------------------------------------------------------
// 结果分组
// ---------------------------------------------------------------------------

/** 分组展示顺序（同时是 i18n key 片段）。 */
const COMMAND_JOB_OUTCOME_GROUP_DEFS: ReadonlyArray<{ key: CommandJobOutcomeGroupKey; i18n: string }> = [
  { key: 'retryable', i18n: 'Retry' },
  { key: 'device_failed', i18n: 'DeviceFailed' },
  { key: 'missing_logs', i18n: 'MissingLogs' },
  { key: 'blocked', i18n: 'Blocked' },
  { key: 'in_progress', i18n: 'InProgress' },
  { key: 'completed', i18n: 'Completed' }
]

export const COMMAND_JOB_OUTCOME_GROUP_TONES: CommandJobToneTable = {
  retryable: 'error',
  device_failed: 'error',
  missing_logs: 'warning',
  blocked: 'error',
  in_progress: 'info',
  completed: 'success'
}
export const resolveCommandJobOutcomeGroupTone = createCommandJobToneResolver(COMMAND_JOB_OUTCOME_GROUP_TONES, 'info')

/** 设备 ACK 结论优先于本地重试/日志推断。 */
const COMMAND_JOB_OUTCOME_GROUP_RULES: ReadonlyArray<CommandJobRule<Row, CommandJobOutcomeGroupKey>> = [
  { when: isAckSuccess, then: 'completed' },
  { when: isAckFailed, then: 'device_failed' },
  { when: (row) => Boolean(row.can_retry), then: 'retryable' },
  { when: isLogMissing, then: 'missing_logs' },
  { when: isPreviewBlocked, then: 'blocked' },
  { when: (row) => row.status === 'completed', then: 'completed' }
]

export function resolveCommandJobOutcomeGroupKey(row: Row): CommandJobOutcomeGroupKey {
  return matchCommandJobRule(COMMAND_JOB_OUTCOME_GROUP_RULES, row, 'in_progress')
}

const COMMAND_JOB_OUTCOME_GROUP_ROW_LIMIT = 5

function buildCommandJobOutcomeRow(row: Row, t: Translate): CommandJobOutcomeDeviceRow {
  return {
    ...commandJobRowIdentity(row),
    status: formatCommandJobStatus(row.status, t),
    readiness: formatCommandJobReadiness(row.readiness),
    reason: row.reason || '-',
    action: buildCommandJobSubmitNextAction(row, t)
  }
}

export function buildCommandJobOutcomeGroups(
  result: FleetCommandJobSubmitResult | null,
  t: Translate
): CommandJobOutcomeGroup[] {
  if (!result) return []

  const groups = new Map<CommandJobOutcomeGroupKey, CommandJobOutcomeGroup>(
    COMMAND_JOB_OUTCOME_GROUP_DEFS.map(({ key, i18n }) => [
      key,
      {
        key,
        title: t(`custom.commandCenter.outcome${i18n}Title`),
        description: t(`custom.commandCenter.outcome${i18n}Desc`),
        count: 0,
        type: resolveCommandJobOutcomeGroupTone(key),
        rows: []
      }
    ])
  )

  for (const row of result.rows ?? []) {
    const group = groups.get(resolveCommandJobOutcomeGroupKey(row))!
    group.count += 1
    if (group.rows.length < COMMAND_JOB_OUTCOME_GROUP_ROW_LIMIT) group.rows.push(buildCommandJobOutcomeRow(row, t))
  }

  return [...groups.values()].filter((group) => group.count > 0)
}

// ---------------------------------------------------------------------------
// 单设备进度轨道
// ---------------------------------------------------------------------------

/**
 * 轨道整体语气。与结果分组不同：任何失败/阻断信号都优先于 ACK 成功，
 * 让运维先看到需要处理的设备。
 */
const COMMAND_JOB_DEVICE_TRACK_TONE_RULES: ReadonlyArray<CommandJobRule<Row, CommandJobTone>> = [
  { when: isAckFailed, then: 'error' },
  { when: (row) => Boolean(row.can_retry) || isRetryExhausted(row), then: 'error' },
  { when: isPreviewBlocked, then: 'error' },
  { when: (row) => isAckSuccess(row) || Boolean(row.completed_at) || row.status === 'completed', then: 'success' },
  { when: isLogMissing, then: 'warning' },
  { when: isRetryWaiting, then: 'warning' }
]

export function resolveCommandJobDeviceTrackTone(row: Row): CommandJobTone {
  return matchCommandJobRule(COMMAND_JOB_DEVICE_TRACK_TONE_RULES, row, 'info')
}

function buildDeviceProgressStep(
  key: CommandJobDeviceProgressStep['key'],
  labelKey: string,
  state: string,
  detail: string,
  t: Translate,
  stateSuffix = ''
): CommandJobDeviceProgressStep {
  return {
    key,
    label: t(labelKey),
    state: `${t(`custom.commandCenter.deviceProgressState.${state}`)}${stateSuffix}`,
    detail,
    type: resolveCommandJobDeviceStepTone(state)
  }
}

function previewStep(row: Row, t: Translate) {
  const blocked = isPreviewBlocked(row)
  return buildDeviceProgressStep(
    'preview',
    'custom.commandCenter.deviceProgressPreview',
    blocked ? 'blocked' : 'done',
    blocked
      ? row.reason || t('custom.commandCenter.deviceProgressPreviewBlocked')
      : formatCommandJobReadiness(row.readiness),
    t
  )
}

function dispatchStep(row: Row, t: Translate) {
  const failed = row.status === 'failed' || Boolean(row.can_retry) || isRetryExhausted(row)
  const submitted = Boolean(row.submitted_at || row.message_id || row.last_dispatch_started_at)
  let state = 'waiting'
  if (failed) state = 'failed'
  else if (submitted) state = 'done'

  const attempts =
    row.dispatch_attempts || row.max_dispatch_attempts
      ? ` ${row.dispatch_attempts ?? 0}/${row.max_dispatch_attempts ?? '-'}`
      : ''
  const detail = failed
    ? row.reason || row.advice || formatCommandJobStatus(row.status, t)
    : row.message_id ||
      row.last_dispatch_started_at ||
      row.submitted_at ||
      t('custom.commandCenter.deviceProgressDispatchWaiting')

  return buildDeviceProgressStep('dispatch', 'custom.commandCenter.deviceProgressDispatch', state, detail, t, attempts)
}

function ackStep(row: Row, t: Translate) {
  let state = 'waiting'
  if (isAckSuccess(row)) state = 'done'
  else if (isAckFailed(row)) state = 'failed'
  else if (row.completed_at || row.response_recorded) state = 'done'

  const detail =
    row.response_error ||
    formatCommandJobResponseStatus(row.response_status_label, t) ||
    row.completed_at ||
    t('custom.commandCenter.deviceProgressAckWaiting')

  return buildDeviceProgressStep('ack', 'custom.commandCenter.deviceProgressAck', state, detail, t)
}

function evidenceStep(row: Row, t: Translate) {
  return buildDeviceProgressStep(
    'evidence',
    'custom.commandCenter.deviceProgressEvidence',
    row.log_recorded || row.command_log_created_at ? 'done' : 'missing',
    row.command_log_created_at || row.next_retry_after || t('custom.commandCenter.deviceProgressEvidenceMissing'),
    t
  )
}

export function buildCommandJobDeviceProgressTracks(
  result: FleetCommandJobSubmitResult | null,
  t: Translate,
  limit = 8
): CommandJobDeviceProgressTrack[] {
  // 先算一次语气再排序（稳定排序），避免比较器里重复分类。
  return (result?.rows ?? [])
    .map((row) => ({ row, tone: resolveCommandJobDeviceTrackTone(row) }))
    .sort((left, right) => commandJobToneSeverity(left.tone) - commandJobToneSeverity(right.tone))
    .slice(0, Math.max(0, limit))
    .map(({ row, tone }) => ({
      ...commandJobRowIdentity(row),
      summary: formatCommandJobStatus(row.status, t),
      nextAction: buildCommandJobSubmitNextAction(row, t),
      type: tone,
      steps: [previewStep(row, t), dispatchStep(row, t), ackStep(row, t), evidenceStep(row, t)]
    }))
}
