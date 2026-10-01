/**
 * 文件用途: RDI 历史数据的纯函数层(零 Vue / DOM / API 依赖)。
 * 核心逻辑: 时间范围解析、查询参数构造、分页响应 normalize、分页累加状态机、缺口检测、
 *   能耗统计、序列结果汇总、CSV 导出行与 ECharts 配置构造。
 * 关键注意事项: 字段读取优先级、分页上限与缺口阈值需与 RDI history API 及既有图表行为保持一致;
 *   IO 只通过注入的 fetchPage 回调发生,便于在单元测试中模拟空数据/分页失败/截断。
 */
import type { EChartsCoreOption } from 'echarts/core'
import type { RDIHistoryParams } from '@/service/api/rdi'
import type { LabelKey } from '../constants/rdi-labels'
import { RDI_DURATION_MAX_SECONDS } from '../constants/rdi-ranges'

// ---------------------------------------------------------------------------
// 类型
// ---------------------------------------------------------------------------

export type HistoryExportFormat = 'excel' | 'csv'
export type EnergyRange = 'last_1h' | 'last_24h' | 'last_7d' | 'last_30d' | 'custom'
export type PresetEnergyRange = Exclude<EnergyRange, 'custom'>
export type HistoryRange = [number, number]
export type TemperatureUnit = 'C' | 'F'
export type RDIHistoryTranslate = (key: LabelKey) => string

export type RDIHistorySeriesKey =
  'temperature_1' | 'temperature_2' | 'switch_1' | 'switch_2' | 'dry_contact_output' | 'electricity_consumption'

export type HistoryPoint = {
  ts: number
  value: number | null
}

export type HistoryExportRow = {
  ts: number | string
  value: unknown
}

export type HistorySeriesStatus = 'loaded' | 'empty' | 'partial' | 'failed'

export type HistorySeriesDefinition = {
  key: RDIHistorySeriesKey
  label: string
  color: string
  required?: boolean
  switchSeries?: boolean
}

export type EnergyStatsSnapshot = {
  sample_count: number
  latest: number | null
  min: number | null
  max: number | null
  delta: number | null
}
export type HistoryChartData = Partial<Record<RDIHistorySeriesKey, HistoryPoint[]>>

export type HistorySeriesResult = {
  key: RDIHistorySeriesKey
  status: HistorySeriesStatus
  points: HistoryPoint[]
  loadedCount: number
  expectedCount: number | null
  missingCount: number
  invalidCount: number
  failedPage?: number
  truncated: boolean
  detectedGapCount: number
}

export type HistoryChartLoadResult = {
  chartData: HistoryChartData
  seriesResults: HistorySeriesResult[]
}

export type NormalizedHistoryPage = {
  points: HistoryPoint[]
  invalidCount: number
}

export type FormatHistoryChartValue = (key: RDIHistorySeriesKey, value: number) => number

/** 分页拉取上限;默认值即生产配置,测试可注入更小的值来覆盖截断分支。 */
export type HistoryPaginationLimits = {
  pageSize: number
  maxPoints: number
  maxPages: number
}

/** 单序列分页累加状态(可变,仅在一次 fetchHistorySeriesPages 内部使用)。 */
export type HistoryPaginationState = {
  points: HistoryPoint[]
  seenPointIdentities: Set<string>
  expectedCount: number | null
  invalidCount: number
  failedPage?: number
  truncated: boolean
}

/** fetchPage 返回值与 rdiDeviceHistory 的 `{ error, data }` 形状对齐。 */
export type HistoryPageResponse = { error: unknown; data: unknown }
export type HistoryPageFetcher = (params: RDIHistoryParams) => Promise<HistoryPageResponse>

export type HistorySeriesSummary = {
  failedLabels: string[]
  partialLabels: string[]
  gappedLabels: string[]
  hasFailures: boolean
  hasSuccessfulData: boolean
  energyStatisticsAvailable: boolean
}

// ---------------------------------------------------------------------------
// 常量
// ---------------------------------------------------------------------------

export const HISTORY_QUERY_PAGE = 1
export const HISTORY_CHART_PAGE_SIZE = 5000
export const HISTORY_CHART_MAX_POINTS_PER_SERIES = 100000
export const HISTORY_CHART_MAX_PAGES_PER_SERIES = Math.ceil(
  HISTORY_CHART_MAX_POINTS_PER_SERIES / HISTORY_CHART_PAGE_SIZE
)
export const HISTORY_GAP_THRESHOLD_MS = 90 * 1000
export const HISTORY_EXPORT_PAGE_SIZE = 10000
export const DEFAULT_ENERGY_RANGE: PresetEnergyRange = 'last_1h'
export const ENERGY_SERIES_KEY: RDIHistorySeriesKey = 'electricity_consumption'

export const DEFAULT_HISTORY_PAGINATION_LIMITS: Readonly<HistoryPaginationLimits> = Object.freeze({
  pageSize: HISTORY_CHART_PAGE_SIZE,
  maxPoints: HISTORY_CHART_MAX_POINTS_PER_SERIES,
  maxPages: HISTORY_CHART_MAX_PAGES_PER_SERIES
})

export const ENERGY_RANGE_DURATIONS: Readonly<Record<PresetEnergyRange, number>> = Object.freeze({
  last_1h: 60 * 60 * 1000,
  last_24h: 24 * 60 * 60 * 1000,
  last_7d: 7 * 24 * 60 * 60 * 1000,
  last_30d: 30 * 24 * 60 * 60 * 1000
})

export const HISTORY_SERIES_DEFINITIONS: readonly HistorySeriesDefinition[] = Object.freeze([
  { key: 'temperature_1', label: 'T1', color: '#f43f5e' },
  { key: 'temperature_2', label: 'T2', color: '#2563eb' },
  { key: 'switch_1', label: 'SW1', color: '#7c3aed', switchSeries: true },
  { key: 'switch_2', label: 'SW2', color: '#f97316', switchSeries: true },
  { key: 'dry_contact_output', label: 'DO', color: '#16a34a', switchSeries: true },
  { key: ENERGY_SERIES_KEY, label: 'kWh', color: '#0891b2', required: true }
])

export const DEFAULT_HISTORY_CHART_SERIES_KEYS: readonly RDIHistorySeriesKey[] = Object.freeze(
  HISTORY_SERIES_DEFINITIONS.map((item) => item.key)
)

const HISTORY_SERIES_LABELS = new Map(HISTORY_SERIES_DEFINITIONS.map((item) => [item.key, item.label]))
const MILLISECOND_TIMESTAMP_FLOOR = 1_000_000_000_000
const TRUTHY_HISTORY_STRINGS = new Set(['true', 'high', 'open', 'on'])
const FALSY_HISTORY_STRINGS = new Set(['false', 'low', 'close', 'closed', 'off'])

// ---------------------------------------------------------------------------
// 响应 normalize
// ---------------------------------------------------------------------------

type HistoryRecord = Record<string, unknown>

function asRecord(value: unknown): HistoryRecord | undefined {
  return value !== null && typeof value === 'object' ? (value as HistoryRecord) : undefined
}

export function isTemperatureHistoryKey(key: string) {
  return key === 'temperature_1' || key === 'temperature_2'
}

/** 秒级数字/数字字符串转毫秒;ISO 字符串与 Date 按原值解析;其余返回 null。 */
export function normalizeHistoryTimestamp(value: unknown): number | null {
  if (value instanceof Date) return value.getTime()
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value < MILLISECOND_TIMESTAMP_FLOOR ? value * 1000 : value
  }
  if (typeof value === 'string') {
    const numeric = Number(value)
    if (Number.isFinite(numeric)) return numeric < MILLISECOND_TIMESTAMP_FLOOR ? numeric * 1000 : numeric
    const parsed = Date.parse(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

/** 布尔/开关语义字符串转 1/0,数字(含数字字符串)原样返回,无法解析时为 null。 */
export function normalizeHistoryValue(value: unknown): number | null {
  if (typeof value === 'boolean') return value ? 1 : 0
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (TRUTHY_HISTORY_STRINGS.has(normalized)) return 1
    if (FALSY_HISTORY_STRINGS.has(normalized)) return 0
    const numeric = Number(value)
    if (Number.isFinite(numeric)) return numeric
  }
  return null
}

/** 兼容裸数组与 `{ list }` 两种响应形状。 */
export function normalizeHistoryList(payload: unknown): unknown[] {
  if (Array.isArray(payload)) return payload
  const list = asRecord(payload)?.list
  return Array.isArray(list) ? list : []
}

export function normalizeHistoryTotal(payload: unknown): number | null {
  const rawTotal = asRecord(payload)?.total
  if (rawTotal === null || rawTotal === undefined || rawTotal === '') return null
  const total = Number(rawTotal)
  return Number.isFinite(total) && total >= 0 ? Math.floor(total) : null
}

function readHistoryPointTimestamp(item: HistoryRecord | undefined) {
  return item?.ts ?? item?.x ?? item?.time ?? item?.UpdateAt ?? item?.created_at
}

function readHistoryPointValue(item: HistoryRecord | undefined, key: RDIHistorySeriesKey) {
  return item?.value ?? item?.y ?? item?.number_v ?? item?.bool_v ?? item?.string_v ?? item?.[key]
}

/** 单页响应 -> 按时间升序的点列;时间戳无法解析的条目计入 invalidCount 而非静默丢弃。 */
export function normalizeHistoryPoints(payload: unknown, key: RDIHistorySeriesKey): NormalizedHistoryPage {
  const points: HistoryPoint[] = []
  let invalidCount = 0

  for (const rawItem of normalizeHistoryList(payload)) {
    const item = asRecord(rawItem)
    const ts = normalizeHistoryTimestamp(readHistoryPointTimestamp(item))
    if (ts === null) {
      invalidCount += 1
      continue
    }
    points.push({ ts, value: normalizeHistoryValue(readHistoryPointValue(item, key)) })
  }

  return {
    points: points.sort((a, b) => a.ts - b.ts),
    invalidCount
  }
}

/** 导出行保留原始 ts/value(服务端格式),只过滤缺失时间戳的条目。 */
export function normalizeHistoryExportRows(payload: unknown): HistoryExportRow[] {
  return normalizeHistoryList(payload)
    .map((rawItem) => {
      const item = asRecord(rawItem)
      return {
        ts: (item?.ts ?? item?.x ?? item?.time ?? '') as number | string,
        value: item?.value ?? item?.y ?? item?.number_v ?? item?.string_v ?? item?.bool_v ?? ''
      }
    })
    .filter((item) => item.ts !== '')
}

export function getExportedHistoryFilePath(payload: unknown): unknown {
  const record = asRecord(payload)
  return record?.filePath || record?.file_path
}

// ---------------------------------------------------------------------------
// 缺口检测
// ---------------------------------------------------------------------------

/**
 * 相邻两个有效采样间隔超过阈值时插入一个 null 标记,让图表(connectNulls=false)断线;
 * 服务端原生的 null 值同样计为一个缺口。输入不要求有序。
 */
export function insertHistoryGapMarkers(points: readonly HistoryPoint[], thresholdMs = HISTORY_GAP_THRESHOLD_MS) {
  const sortedPoints = [...points].sort((a, b) => a.ts - b.ts)
  if (!sortedPoints.length) return { points: sortedPoints, detectedGapCount: 0 }

  const markedPoints: HistoryPoint[] = [sortedPoints[0]]
  let detectedGapCount = sortedPoints[0].value === null ? 1 : 0

  for (let index = 1; index < sortedPoints.length; index += 1) {
    const previousPoint = sortedPoints[index - 1]
    const currentPoint = sortedPoints[index]
    const gapDuration = currentPoint.ts - previousPoint.ts

    if (gapDuration > thresholdMs && previousPoint.value !== null && currentPoint.value !== null) {
      markedPoints.push({ ts: previousPoint.ts + Math.floor(gapDuration / 2), value: null })
      detectedGapCount += 1
    }

    markedPoints.push(currentPoint)
    if (currentPoint.value === null) detectedGapCount += 1
  }

  return { points: markedPoints, detectedGapCount }
}

// ---------------------------------------------------------------------------
// 分页累加状态机
// ---------------------------------------------------------------------------

function historyPointIdentity(point: HistoryPoint) {
  return `${point.ts}\u0000${point.value}`
}

export function createHistoryPaginationState(): HistoryPaginationState {
  return {
    points: [],
    seenPointIdentities: new Set<string>(),
    expectedCount: null,
    invalidCount: 0,
    failedPage: undefined,
    truncated: false
  }
}

/** 追加去重后的点(分页边界重复点),达到 maxPoints 即停止;返回实际新增数。 */
function appendUniqueHistoryPoints(state: HistoryPaginationState, nextPoints: HistoryPoint[], maxPoints: number) {
  let addedCount = 0
  for (const point of nextPoints) {
    if (state.points.length >= maxPoints) break
    const identity = historyPointIdentity(point)
    if (state.seenPointIdentities.has(identity)) continue
    state.seenPointIdentities.add(identity)
    state.points.push(point)
    addedCount += 1
  }
  return addedCount
}

/**
 * 把第 `page` 页的成功响应并入 state,返回是否需要继续请求下一页。
 * 停止条件(按优先级): 达到点数上限 / 已满足 total / 空页 / 整页重复 / 无 total 的短页 / 达到页数上限。
 * 任何"客户端主动停止但服务端尚未证明数据已尽"的情况都会置 truncated。
 */
export function applyHistoryPage(
  state: HistoryPaginationState,
  page: number,
  data: unknown,
  key: RDIHistorySeriesKey,
  limits: HistoryPaginationLimits = DEFAULT_HISTORY_PAGINATION_LIMITS
): boolean {
  const rawPageItems = normalizeHistoryList(data)
  const responseTotal = normalizeHistoryTotal(data)
  if (responseTotal !== null) state.expectedCount = Math.max(state.expectedCount ?? 0, responseTotal)

  const normalizedPage = normalizeHistoryPoints(rawPageItems, key)
  state.invalidCount += normalizedPage.invalidCount
  const addedCount = appendUniqueHistoryPoints(state, normalizedPage.points, limits.maxPoints)
  const { expectedCount } = state
  const loadedCount = state.points.length

  if (loadedCount >= limits.maxPoints) {
    state.truncated = expectedCount === null || expectedCount > loadedCount
    return false
  }
  if (expectedCount !== null && loadedCount >= expectedCount) return false
  if (rawPageItems.length === 0) return false
  if (addedCount === 0) {
    // 无 total 时重复出现的整页无法证明序列已结束。
    if (expectedCount === null && rawPageItems.length >= limits.pageSize) state.truncated = true
    return false
  }
  if (expectedCount === null && rawPageItems.length < limits.pageSize) return false
  if (page >= limits.maxPages) {
    // 最后一页仍是满页: 客户端上限先于服务端耗尽。
    state.truncated = expectedCount === null || loadedCount < expectedCount
    return false
  }
  return true
}

export function markHistoryPageFailed(state: HistoryPaginationState, page: number) {
  state.failedPage = page
}

/** 状态 -> 序列结果: 计算缺失数、插入缺口标记并判定 loaded/empty/partial/failed。 */
export function finalizeHistorySeriesResult(
  key: RDIHistorySeriesKey,
  state: Pick<HistoryPaginationState, 'points' | 'expectedCount' | 'invalidCount' | 'failedPage' | 'truncated'>,
  gapThresholdMs = HISTORY_GAP_THRESHOLD_MS
): HistorySeriesResult {
  const { points, expectedCount, invalidCount, failedPage, truncated } = state
  const loadedCount = points.length
  const missingCount = expectedCount === null ? 0 : Math.max(0, expectedCount - loadedCount)
  const chartEvidence = insertHistoryGapMarkers(points, gapThresholdMs)
  const failed = failedPage !== undefined && loadedCount === 0
  const partial = !failed && (failedPage !== undefined || truncated || invalidCount > 0 || missingCount > 0)

  return {
    key,
    status: failed ? 'failed' : partial ? 'partial' : loadedCount === 0 ? 'empty' : 'loaded',
    points: chartEvidence.points,
    loadedCount,
    expectedCount,
    missingCount,
    invalidCount,
    failedPage,
    truncated,
    detectedGapCount: chartEvidence.detectedGapCount
  }
}

/** 顺序拉取一个序列的全部分页;IO 通过 fetchPage 注入,任何错误/异常都记录为失败页而不是抛出。 */
export async function fetchHistorySeriesPages(
  fetchPage: HistoryPageFetcher,
  key: RDIHistorySeriesKey,
  range: HistoryRange,
  limits: HistoryPaginationLimits = DEFAULT_HISTORY_PAGINATION_LIMITS
): Promise<HistorySeriesResult> {
  const state = createHistoryPaginationState()

  for (let page = HISTORY_QUERY_PAGE; page <= limits.maxPages; page += 1) {
    try {
      const { error, data } = await fetchPage(buildHistoryQueryParams(key, range, limits.pageSize, page))
      if (error) {
        markHistoryPageFailed(state, page)
        break
      }
      if (!applyHistoryPage(state, page, data, key, limits)) break
    } catch {
      markHistoryPageFailed(state, page)
      break
    }
  }

  return finalizeHistorySeriesResult(key, state)
}

/** 并发拉取所选序列(按定义顺序),汇总为图表数据与逐序列结果。 */
export async function fetchHistoryChartData(
  fetchPage: HistoryPageFetcher,
  range: HistoryRange,
  seriesKeys: readonly RDIHistorySeriesKey[],
  limits: HistoryPaginationLimits = DEFAULT_HISTORY_PAGINATION_LIMITS
): Promise<HistoryChartLoadResult> {
  const definitions = selectHistorySeriesDefinitions(seriesKeys)
  const seriesResults = await Promise.all(
    definitions.map((item) => fetchHistorySeriesPages(fetchPage, item.key, range, limits))
  )
  return { chartData: buildHistoryChartData(seriesResults), seriesResults }
}

// ---------------------------------------------------------------------------
// 序列选择与结果汇总
// ---------------------------------------------------------------------------

/** 空选择恢复默认全集;非空选择只保留合法且去重的 key,不把其余默认曲线隐式加回。 */
export function normalizeHistoryChartSeriesKeys(keys: readonly RDIHistorySeriesKey[]): RDIHistorySeriesKey[] {
  if (keys.length === 0) return [...DEFAULT_HISTORY_CHART_SERIES_KEYS]
  return Array.from(new Set(keys.filter((key) => HISTORY_SERIES_LABELS.has(key))))
}

/** 按定义顺序(而非用户选择顺序)返回所选序列定义,保证颜色/图例顺序稳定。 */
export function selectHistorySeriesDefinitions(keys: readonly RDIHistorySeriesKey[]): HistorySeriesDefinition[] {
  const selected = new Set(keys)
  return HISTORY_SERIES_DEFINITIONS.filter((item) => selected.has(item.key))
}

export function buildHistoryChartData(seriesResults: readonly HistorySeriesResult[]): HistoryChartData {
  const chartData: HistoryChartData = {}
  for (const result of seriesResults) chartData[result.key] = result.points
  return chartData
}

export function historySeriesLabel(key: RDIHistorySeriesKey) {
  return HISTORY_SERIES_LABELS.get(key) || key
}

export function summarizeHistorySeriesResults(results: readonly HistorySeriesResult[]): HistorySeriesSummary {
  const labelsWhere = (predicate: (result: HistorySeriesResult) => boolean) =>
    results.filter(predicate).map((result) => historySeriesLabel(result.key))
  const energyResult = results.find((item) => item.key === ENERGY_SERIES_KEY)

  return {
    failedLabels: labelsWhere((result) => result.status === 'failed'),
    partialLabels: labelsWhere((result) => result.status === 'partial'),
    gappedLabels: labelsWhere((result) => result.detectedGapCount > 0),
    hasFailures: results.some((result) => result.status === 'failed' || result.status === 'partial'),
    hasSuccessfulData: results.some((result) => result.status !== 'failed'),
    // 成功的空能耗序列仍可展示"0 样本"统计;失败或只有缺口标记时不可用。
    energyStatisticsAvailable:
      energyResult !== undefined &&
      energyResult.status !== 'failed' &&
      (energyResult.status === 'empty' || energyResult.points.some((point) => point.value !== null))
  }
}

export function hasRenderableHistoryChartData(chartData: HistoryChartData) {
  return Object.values(chartData).some((points) => points?.some((point) => point.value !== null))
}

// ---------------------------------------------------------------------------
// 时间范围与查询参数
// ---------------------------------------------------------------------------

export function isPresetEnergyRange(range: string): range is PresetEnergyRange {
  return Object.prototype.hasOwnProperty.call(ENERGY_RANGE_DURATIONS, range)
}

/** 预设范围以 now 为终点回推;custom 原样返回(未选择时为 null);未知值回退到默认预设。 */
export function resolveHistoryRange(
  range: string,
  customRange: HistoryRange | null,
  now = Date.now()
): HistoryRange | null {
  if (range === 'custom') return customRange
  const duration = ENERGY_RANGE_DURATIONS[isPresetEnergyRange(range) ? range : DEFAULT_ENERGY_RANGE]
  return [now - duration, now]
}

export function buildHistoryQueryParams(
  key: RDIHistorySeriesKey,
  range: HistoryRange,
  pageSize: number,
  page = HISTORY_QUERY_PAGE,
  options: Pick<RDIHistoryParams, 'export_excel' | 'export_format'> = {}
): RDIHistoryParams {
  return {
    key,
    start_time: range[0],
    end_time: range[1],
    page,
    page_size: pageSize,
    ...options
  }
}

export function buildHistoryChartQueryParams(key: RDIHistorySeriesKey, range: HistoryRange, page = HISTORY_QUERY_PAGE) {
  return buildHistoryQueryParams(key, range, HISTORY_CHART_PAGE_SIZE, page)
}

export function buildHistoryExportQueryParams(
  key: RDIHistorySeriesKey,
  range: HistoryRange,
  format: HistoryExportFormat
) {
  return buildHistoryQueryParams(key, range, HISTORY_EXPORT_PAGE_SIZE, HISTORY_QUERY_PAGE, {
    export_excel: format === 'excel',
    export_format: format
  })
}

// ---------------------------------------------------------------------------
// 能耗统计
// ---------------------------------------------------------------------------

export function createEmptyEnergyStats(): EnergyStatsSnapshot {
  return { sample_count: 0, latest: null, min: null, max: null, delta: null }
}

/**
 * 单次遍历计算能耗统计,忽略 null(缺口标记/无效值)。
 * 不排序、不展开参数,10 万点也不会触发 Math.min(...values) 的调用栈上限。
 * 时间戳相同时: 最早点取第一个出现者,最新点取最后一个出现者(与稳定排序后取首尾一致)。
 */
export function calculateEnergyStats(points: readonly HistoryPoint[] = []): EnergyStatsSnapshot {
  let sampleCount = 0
  let min = Infinity
  let max = -Infinity
  let earliest: HistoryPoint | undefined
  let latest: HistoryPoint | undefined

  for (const point of points) {
    if (point.value === null) continue
    sampleCount += 1
    if (point.value < min) min = point.value
    if (point.value > max) max = point.value
    if (!earliest || point.ts < earliest.ts) earliest = point
    if (!latest || point.ts >= latest.ts) latest = point
  }

  if (!earliest || !latest) return createEmptyEnergyStats()
  const latestValue = latest.value as number
  return {
    sample_count: sampleCount,
    latest: latestValue,
    min,
    max,
    // 累计电量计数器只增不减;回绕/重置时不报告负增量。
    delta: sampleCount > 1 ? Math.max(0, latestValue - (earliest.value as number)) : 0
  }
}

// ---------------------------------------------------------------------------
// 值格式化
// ---------------------------------------------------------------------------

export function formatEnergyValue(value: number | null) {
  return value === null ? '--' : `${value.toFixed(2)} kWh`
}

export function formatDurationLabel(value: number | null | undefined) {
  const seconds = Math.max(0, Math.min(RDI_DURATION_MAX_SECONDS, Number(value) || 0))
  if (seconds === 0) return '0H'
  if (seconds % 3600 === 0) return `${seconds / 3600}H`
  if (seconds >= 3600) {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.round((seconds % 3600) / 60)
    return minutes ? `${hours}H ${minutes}M` : `${hours}H`
  }
  if (seconds % 60 === 0) return `${seconds / 60}M`
  return `${seconds}S`
}

export function formatHistoryChartValueForUnit(key: RDIHistorySeriesKey, value: number, unit: TemperatureUnit) {
  return isTemperatureHistoryKey(key) && unit === 'F' ? (value * 9) / 5 + 32 : value
}

// ---------------------------------------------------------------------------
// 导出
// ---------------------------------------------------------------------------

export function historyExportValueLabelForUnit(key: string, unit: TemperatureUnit) {
  return isTemperatureHistoryKey(key) ? `value (${unit})` : 'value'
}

export function formatHistoryExportValueForUnit(key: string, value: unknown, unit: TemperatureUnit) {
  if (!isTemperatureHistoryKey(key) || unit !== 'F') return value
  const numeric = normalizeHistoryValue(value)
  return numeric === null ? value : formatHistoryChartValueForUnit(key as RDIHistorySeriesKey, numeric, unit).toFixed(2)
}

export function formatHistoryExportTimestamp(ts: number | string) {
  return typeof ts === 'number' ? new Date(ts).toISOString() : ts
}

export function buildHistoryExportCsvRows(
  key: RDIHistorySeriesKey,
  rows: readonly HistoryExportRow[],
  unit: TemperatureUnit
): unknown[][] {
  return [
    ['time', 'key', historyExportValueLabelForUnit(key, unit)],
    ...rows.map((row) => [
      formatHistoryExportTimestamp(row.ts),
      key,
      formatHistoryExportValueForUnit(key, row.value, unit)
    ])
  ]
}

export function csvEscape(value: unknown) {
  const text = value === undefined || value === null ? '' : String(value)
  return `"${text.replace(/"/g, '""')}"`
}

export function buildCsvContent(rows: readonly (readonly unknown[])[]) {
  return rows.map((row) => row.map(csvEscape).join(',')).join('\n')
}

export function buildHistoryExportFilename(deviceId: string, key: RDIHistorySeriesKey) {
  return `rdi_${deviceId}_${key}.csv`
}

/** 服务端返回的是相对文件路径,需挂到去掉 `/api/v1` 的服务根地址下。 */
export function buildExportFileUrl(baseServerUrl: string, filePath: unknown) {
  return `${baseServerUrl.replace('/api/v1', '/')}${filePath}`
}

// ---------------------------------------------------------------------------
// 选项列表与图表配置
// ---------------------------------------------------------------------------

export function buildEnergyRangeOptions(t: RDIHistoryTranslate) {
  return [
    { label: t('last1Hour'), value: 'last_1h' },
    { label: t('last1Day'), value: 'last_24h' },
    { label: t('last1Week'), value: 'last_7d' },
    { label: t('last30Days'), value: 'last_30d' },
    { label: t('customRange'), value: 'custom' }
  ]
}

export function buildHistoryExportKeyOptions() {
  return HISTORY_SERIES_DEFINITIONS.map((item) => ({ label: item.label, value: item.key }))
}

export function buildHistoryChartSeriesOptions(t: RDIHistoryTranslate) {
  return HISTORY_SERIES_DEFINITIONS.map((item) => ({
    label: item.required ? `${item.label} (${t('energy')})` : item.label,
    value: item.key,
    disabled: item.required
  }))
}

export function buildHistoryExportFormatOptions(t: RDIHistoryTranslate) {
  return [
    { label: t('excelFormat'), value: 'excel' },
    { label: t('csvFormat'), value: 'csv' }
  ]
}

function buildHistoryChartSeries(
  item: HistorySeriesDefinition,
  points: readonly HistoryPoint[],
  formatValue: FormatHistoryChartValue
) {
  return {
    name: item.label,
    type: 'line',
    animation: false,
    smooth: !item.switchSeries,
    step: item.switchSeries ? 'middle' : false,
    showSymbol: false,
    sampling: item.switchSeries ? undefined : 'lttb',
    // 缺口标记依赖 connectNulls=false 才能在图上断线。
    connectNulls: false,
    emphasis: { disabled: true },
    yAxisIndex: item.switchSeries ? 1 : 0,
    lineStyle: { width: 2, color: item.color },
    itemStyle: { color: item.color },
    data: points.map((point) => [point.ts, point.value === null ? null : formatValue(item.key, point.value)])
  }
}

/** 只为 chartData 中存在的序列生成 series(含空数组),颜色与定义一一对应。 */
export function buildHistoryChartOptions(
  t: RDIHistoryTranslate,
  chartData: HistoryChartData,
  formatValue: FormatHistoryChartValue
): EChartsCoreOption {
  const activeDefinitions = HISTORY_SERIES_DEFINITIONS.filter((item) =>
    Object.prototype.hasOwnProperty.call(chartData, item.key)
  )
  return {
    color: activeDefinitions.map((item) => item.color),
    title: {
      text: t('historyChart'),
      left: 8,
      top: 0,
      textStyle: { fontSize: 13, fontWeight: 600 }
    },
    tooltip: { trigger: 'axis' },
    legend: { top: 28, type: 'scroll' },
    grid: { top: 72, left: 48, right: 48, bottom: 56 },
    xAxis: { type: 'time' },
    yAxis: [
      { type: 'value', name: t('valueAxis'), scale: true },
      {
        type: 'value',
        name: t('switchAxis'),
        min: -0.1,
        max: 1.1,
        interval: 1,
        axisLabel: {
          formatter: (value: number | string) => {
            const numeric = Number(value)
            if (numeric === 1) return t('high')
            if (numeric === 0) return t('low')
            return ''
          }
        }
      }
    ],
    dataZoom: [
      { type: 'inside', throttle: 80 },
      { type: 'slider', height: 22, bottom: 18 }
    ],
    series: activeDefinitions.map((item) => buildHistoryChartSeries(item, chartData[item.key] || [], formatValue))
  }
}
