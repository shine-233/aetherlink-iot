/**
 * 文件用途：设备诊断概览（上下行/存储成功率 + 最近失败记录）的状态与纯函数层。
 * 核心逻辑：
 * 1. `useDeviceDiagnosticsStats` 负责拉取 `deviceDiagnostics`，把结果收敛为显式的
 *    `idle | loading | ready | empty | error` 状态，而不是在组件里静默吞错；
 * 2. 诊断时间线、下一步建议、支持摘要等都是纯函数，便于单测，也供调试控制台复用；
 * 3. 敏感字段脱敏（sanitize/mask）集中在这里，摘要复制与时间线展示共用同一套规则。
 * 关键注意事项：
 * 1. `request` 是 flat 实例，HTTP 失败通过 `{ error }` 返回而不是抛出，两条路径都要视为失败；
 * 2. empty/error 态保留上一次成功数据，避免接口抖动时整块面板闪空；切换设备时才重置；
 * 3. 并发刷新以请求序号判定，只采纳最后一次发起的请求结果，防止旧设备数据回写。
 */
import { h, ref, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import dayjs from 'dayjs'
import type { DataTableColumns } from 'naive-ui'

import { $t } from '@/locales'
import { deviceDiagnostics } from '@/service/api'

// ---------------------------------------------------------------------------
// 类型
// ---------------------------------------------------------------------------

// 诊断统计卡片使用统一的数据结构，避免模板层区分接口返回中的可选字段。
export interface StatisticsItem {
  success: number
  total: number
  rate: number
}

export interface Statistics {
  uplink: StatisticsItem
  downlink: StatisticsItem
  storage: StatisticsItem
}

export interface FailureRecord {
  timestamp: string
  direction: 'uplink' | 'downlink'
  stage: string
  error: string
}

export interface DebugLogEntry {
  ts?: string | number
  event?: string
  stage?: string
  diagnostic_code?: string
  error?: string
  message?: string
  meta?: Record<string, unknown>
  [key: string]: unknown
}

export interface DiagnosticTimelineItem {
  time: string
  title: string
  detail: string
  nextAction: string
  type: 'error' | 'info'
}

interface DiagnosticsStatsItem {
  success?: number
  total?: number
  success_rate?: number
}

interface DiagnosticsStats {
  uplink?: DiagnosticsStatsItem
  downlink?: DiagnosticsStatsItem
  storage?: DiagnosticsStatsItem
}

export interface DiagnosticsData {
  stats?: DiagnosticsStats
  recent_failures?: Array<{
    timestamp?: string | number
    direction?: 'uplink' | 'downlink'
    stage?: string
    error?: string
  }>
}

/** 兼容 `{ data }` 包装、直接主体、以及 flat request 的 `{ data, error }` 三种返回形态。 */
type DiagnosticsApiResponse = { data?: DiagnosticsData | null; error?: unknown } | DiagnosticsData

export type DiagnosticsLoadStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

// ---------------------------------------------------------------------------
// 常量与脱敏
// ---------------------------------------------------------------------------

export const DIAGNOSIS_SUMMARY_LOG_LIMIT = 8

const sensitiveDiagnosticKeyPattern = /(password|passwd|secret|token|credential|private|key|cert|authorization|cookie)/i
const sensitiveJsonFieldPattern =
  /("(?:password|passwd|pwd|token|secret|voucher|authorization|cookie|key|cert)"\s*:\s*)"[^"]*"/gi
const sensitiveQueryFieldPattern =
  /\b(password|passwd|pwd|token|secret|voucher|authorization|cookie|key|cert)=([^\s,;]+)/gi

/** 递归把敏感 key 的值替换为 `***`，用于对象形态的日志。 */
export const sanitizeDiagnosticValue = (value: unknown): unknown => {
  if (Array.isArray(value)) {
    return value.map((item) => sanitizeDiagnosticValue(item))
  }
  if (!value || typeof value !== 'object') {
    return value
  }
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).map(([key, item]) => [
      key,
      sensitiveDiagnosticKeyPattern.test(key) ? '***' : sanitizeDiagnosticValue(item)
    ])
  )
}

/** 对已经是字符串的报文做 JSON 字段 / query 参数两种形态的脱敏。 */
export const maskSensitiveDiagnosticText = (value: unknown) => {
  return String(value ?? '')
    .replace(sensitiveJsonFieldPattern, '$1"***"')
    .replace(sensitiveQueryFieldPattern, '$1=***')
}

export const formatTimelineTime = (time: string | number | undefined) => {
  if (!time) return '--'
  const parsed = dayjs(time)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : String(time)
}

// ---------------------------------------------------------------------------
// 接口归一化
// ---------------------------------------------------------------------------

export const createEmptyStatistics = (): Statistics => ({
  uplink: { success: 0, total: 0, rate: 0 },
  downlink: { success: 0, total: 0, rate: 0 },
  storage: { success: 0, total: 0, rate: 0 }
})

/**
 * 归一化诊断接口返回。
 * - `{ error }`（flat request 的 HTTP 失败）→ 抛出，由调用方转为 error 态；
 * - 无 `stats` → `null`（empty 态）；
 * - 其余 → 诊断主体。
 */
export const normalizeDiagnosticsResponse = (response: DiagnosticsApiResponse | null | undefined) => {
  if (response && typeof response === 'object' && 'error' in response && response.error) {
    throw response.error
  }
  const wrapped = (response as { data?: DiagnosticsData | null } | undefined)?.data
  const data = wrapped || (response as DiagnosticsData | null | undefined)
  return data?.stats ? data : null
}

const mapProgress = (item?: DiagnosticsStatsItem): StatisticsItem => ({
  success: item?.success ?? 0,
  total: item?.total ?? 0,
  rate: item?.success_rate ?? 0
})

export const mapDiagnosticsStats = (stats: DiagnosticsStats = {}): Statistics => ({
  uplink: mapProgress(stats.uplink),
  downlink: mapProgress(stats.downlink),
  storage: mapProgress(stats.storage)
})

export const mapDiagnosticsFailureRecords = (failures?: DiagnosticsData['recent_failures']): FailureRecord[] => {
  if (!Array.isArray(failures)) return []
  return failures.map((failure) => ({
    timestamp: String(failure.timestamp ?? ''),
    direction: failure.direction ?? 'uplink',
    stage: failure.stage ?? '',
    error: failure.error ?? ''
  }))
}

// ---------------------------------------------------------------------------
// 时间线 / 下一步 / 支持摘要（纯函数，概览与调试控制台共用）
// ---------------------------------------------------------------------------

const getFailureNextAction = (item: FailureRecord) =>
  item.direction === 'uplink'
    ? $t('custom.device_details.nextActionUplink')
    : $t('custom.device_details.nextActionDownlink')

const getDebugLogCode = (item: DebugLogEntry) => String(item.diagnostic_code || item.event || item.stage || '')

const getDebugLogNextAction = (item: DebugLogEntry) => {
  const code = getDebugLogCode(item).toLowerCase()
  if (code.includes('auth')) return $t('custom.device_details.nextActionAuth')
  if (code.includes('topic')) return $t('custom.device_details.nextActionTopic')
  if (code.includes('disconnect')) return $t('custom.device_details.nextActionDisconnect')
  if (code.includes('payload') || code.includes('parse')) return $t('custom.device_details.nextActionPayload')
  return $t('custom.device_details.nextActionDefault')
}

export const buildDiagnosticTimeline = (
  failures: FailureRecord[],
  logs: DebugLogEntry[],
  limit = DIAGNOSIS_SUMMARY_LOG_LIMIT
): DiagnosticTimelineItem[] => [
  ...failures.slice(0, limit).map((item) => ({
    time: formatTimelineTime(item.timestamp),
    title: `${item.direction || 'unknown'} / ${item.stage || 'unknown stage'}`,
    detail: maskSensitiveDiagnosticText(item.error || 'No error detail returned'),
    nextAction: getFailureNextAction(item),
    type: 'error' as const
  })),
  ...logs.slice(0, limit).map((item) => ({
    time: formatTimelineTime(item.ts),
    title: getDebugLogCode(item) || 'debug_log',
    detail: maskSensitiveDiagnosticText(
      item.error || item.message || item.stage || JSON.stringify(sanitizeDiagnosticValue(item))
    ),
    nextAction: getDebugLogNextAction(item),
    type: 'info' as const
  }))
]

export interface DiagnosticNextStepsInput {
  diagnosticsStatus: DiagnosticsLoadStatus
  logEnabled: boolean
  logCount: number
}

export const resolveDiagnosticNextSteps = ({ diagnosticsStatus, logEnabled, logCount }: DiagnosticNextStepsInput) => {
  if (diagnosticsStatus === 'error') {
    return [$t('custom.device_details.nextStepErrorRefresh'), $t('custom.device_details.nextStepErrorReadyCheck')]
  }
  if (!logEnabled) {
    return [$t('custom.device_details.nextStepEnableDebug'), $t('custom.device_details.nextStepDisableDebugAfter')]
  }
  if (logCount === 0) {
    return [$t('custom.device_details.nextStepDebugNoLogs'), $t('custom.device_details.nextStepCheckEndpoint')]
  }
  return [$t('custom.device_details.nextStepReviewLatest'), $t('custom.device_details.nextStepCopySummary')]
}

export interface DiagnosticSupportSummaryInput {
  deviceId: string
  logEnabled: boolean
  statistics: Statistics
  failureRecords: FailureRecord[]
  timeline: DiagnosticTimelineItem[]
  nextSteps: string[]
  generatedAt?: dayjs.ConfigType
}

const formatSupportStat = (label: string, item: StatisticsItem) =>
  `${label}: ${item.success}/${item.total} (${item.rate.toFixed(1)}%)`

/** 生成可直接贴进工单的纯文本摘要；所有错误文本都会经过脱敏。 */
export const buildDiagnosticSupportSummary = (input: DiagnosticSupportSummaryInput) => {
  const { statistics, failureRecords, timeline } = input
  const lines = [
    $t('custom.device_details.summaryTitle'),
    `${$t('custom.device_details.summaryGeneratedAt')}: ${dayjs(input.generatedAt).format('YYYY-MM-DD HH:mm:ss')}`,
    `${$t('custom.device_details.summaryDeviceId')}: ${input.deviceId || '--'}`,
    `${$t('custom.device_details.summaryDebugMode')}: ${input.logEnabled ? 'enabled' : 'disabled'}`,
    $t('custom.device_details.summarySensitiveNote'),
    '',
    $t('custom.device_details.summaryStatsSection'),
    formatSupportStat('uplink', statistics.uplink),
    formatSupportStat('downlink', statistics.downlink),
    formatSupportStat('storage', statistics.storage),
    '',
    $t('custom.device_details.summaryFailuresSection'),
    `${$t('custom.device_details.summaryCount')}: ${failureRecords.length}`
  ]

  if (failureRecords.length === 0) {
    lines.push(`- ${$t('custom.device_details.summaryNoFailures')}`)
  } else {
    failureRecords.slice(0, DIAGNOSIS_SUMMARY_LOG_LIMIT).forEach((item) => {
      lines.push(
        `- ${formatTimelineTime(item.timestamp)} ${item.direction || 'unknown'} ${item.stage || 'unknown'} ${maskSensitiveDiagnosticText(item.error)}`
      )
    })
  }

  lines.push('', $t('custom.device_details.summaryTimelineSection'))
  if (timeline.length === 0) {
    lines.push(`- ${$t('custom.device_details.diagnosticEvidenceEmpty')}`)
  } else {
    timeline.forEach((item) => {
      lines.push(
        `- ${item.time} [${item.type}] ${item.title}; ${item.detail}; ${$t('custom.device_details.nextStepLabel')}: ${item.nextAction}`
      )
    })
  }

  lines.push('', $t('custom.device_details.summaryNextStepsSection'))
  input.nextSteps.forEach((step) => lines.push(`- ${step}`))
  return lines.join('\n')
}

// ---------------------------------------------------------------------------
// 表格列：聚焦“何时失败、哪一段失败、为什么失败”
// ---------------------------------------------------------------------------

export const createFailureRecordColumns = (): DataTableColumns<FailureRecord> => [
  {
    title: $t('custom.device_details.time'),
    key: 'timestamp',
    width: 200,
    render: (row: FailureRecord) => (row.timestamp ? dayjs(row.timestamp).format('YYYY-MM-DD HH:mm:ss') : '--')
  },
  {
    title: $t('custom.device_details.direction'),
    key: 'direction',
    width: 150,
    render: (row: FailureRecord) => {
      const direction =
        row.direction === 'uplink' ? $t('custom.device_details.uplink') : $t('custom.device_details.downlink')
      return h('span', {}, { default: () => direction })
    }
  },
  {
    title: $t('custom.device_details.phase'),
    key: 'stage',
    width: 200
  },
  {
    title: $t('custom.device_details.errorDescription'),
    key: 'error',
    ellipsis: {
      tooltip: true
    }
  }
]

// ---------------------------------------------------------------------------
// Composable
// ---------------------------------------------------------------------------

export interface UseDeviceDiagnosticsStatsOptions {
  /** 默认 true：设备 ID 可用时（含切换）立即拉取一次。 */
  immediate?: boolean
}

export const useDeviceDiagnosticsStats = (
  deviceId: MaybeRefOrGetter<string>,
  options: UseDeviceDiagnosticsStatsOptions = {}
) => {
  const statistics = ref<Statistics>(createEmptyStatistics())
  const failureRecords = ref<FailureRecord[]>([])
  const status = ref<DiagnosticsLoadStatus>('idle')
  const error = shallowRef<unknown>(null)
  /** 是否至少成功拿到过一次 ready 数据；用于区分“首屏空”和“刷新失败但有旧数据”。 */
  const hasData = ref(false)

  let requestSeq = 0

  const reset = () => {
    requestSeq += 1
    statistics.value = createEmptyStatistics()
    failureRecords.value = []
    status.value = 'idle'
    error.value = null
    hasData.value = false
  }

  /** 刷新诊断数据。不会抛出：失败收敛为 `status = 'error'`，保持详情页其他 tab 不受影响。 */
  const refresh = async () => {
    const id = toValue(deviceId)
    if (!id) return
    const seq = ++requestSeq
    status.value = 'loading'
    try {
      const data = normalizeDiagnosticsResponse((await deviceDiagnostics(id)) as DiagnosticsApiResponse)
      if (seq !== requestSeq) return
      error.value = null
      if (!data) {
        status.value = 'empty'
        return
      }
      statistics.value = mapDiagnosticsStats(data.stats)
      failureRecords.value = mapDiagnosticsFailureRecords(data.recent_failures)
      hasData.value = true
      status.value = 'ready'
    } catch (err) {
      if (seq !== requestSeq) return
      error.value = err
      status.value = 'error'
    }
  }

  watch(
    () => toValue(deviceId),
    (next, prev) => {
      if (prev !== undefined && next !== prev) reset()
      if (options.immediate !== false) void refresh()
    },
    { immediate: true }
  )

  return { statistics, failureRecords, status, error, hasData, refresh, reset }
}
