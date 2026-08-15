/**
 * 文件用途: RDI 操作视图历史数据 composable。
 * 核心逻辑: 管理历史查询参数、加载状态、图表选项和后端历史数据响应。
 * 关键注意事项: 时间范围、聚合粒度、导出 URL 和字段名需与 RDI history API 及图表展示保持一致。
 * 重构建议: 将查询参数构造、响应 normalize 和图表配置拆成纯函数，并补空数据/失败/时间范围测试。
 */
import { computed, reactive, ref, watch } from 'vue'
import type { EChartsCoreOption } from 'echarts/core'
import { rdiDeviceHistory } from '@/service/api'
import type { RDIHistoryParams } from '@/service/api/rdi'
import { message } from '@/utils/common/discrete'
import { getBaseServerUrl } from '@/utils/common/tool'
import type { LabelKey } from '../constants/rdi-labels'
import { RDI_DURATION_MAX_SECONDS } from '../constants/rdi-ranges'

type EnergyPoint = {
  x: number
  y: number
}

type HistoryExportRow = {
  ts: number | string
  value: unknown
}

type HistoryExportFormat = 'excel' | 'csv'
type EnergyRange = 'last_1h' | 'last_24h' | 'last_7d' | 'last_30d' | 'custom'
type PresetEnergyRange = Exclude<EnergyRange, 'custom'>
type HistoryRange = [number, number]

type RDIHistorySeriesKey =
  | 'temperature_1'
  | 'temperature_2'
  | 'switch_1'
  | 'switch_2'
  | 'dry_contact_output'
  | 'electricity_consumption'

type HistoryPoint = {
  ts: number
  value: number | null
}

type HistorySeriesStatus = 'loaded' | 'empty' | 'partial' | 'failed'

type HistorySeriesDefinition = {
  key: RDIHistorySeriesKey
  label: string
  color: string
  required?: boolean
  switchSeries?: boolean
}

type EnergyStatsSnapshot = {
  sample_count: number
  latest: number | null
  min: number | null
  max: number | null
  delta: number | null
}

type HistoryChartData = Partial<Record<RDIHistorySeriesKey, HistoryPoint[]>>

type HistorySeriesResult = {
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

type HistoryChartLoadResult = {
  chartData: HistoryChartData
  seriesResults: HistorySeriesResult[]
}

type NormalizedHistoryPage = {
  points: HistoryPoint[]
  invalidCount: number
}

type TemperatureUnit = 'C' | 'F'
type RDIHistoryTranslate = (key: LabelKey) => string
type FormatHistoryChartValue = (key: RDIHistorySeriesKey, value: number) => number

const HISTORY_QUERY_PAGE = 1
const HISTORY_CHART_PAGE_SIZE = 5000
const HISTORY_CHART_MAX_POINTS_PER_SERIES = 100000
const HISTORY_CHART_MAX_PAGES_PER_SERIES = Math.ceil(
  HISTORY_CHART_MAX_POINTS_PER_SERIES / HISTORY_CHART_PAGE_SIZE
)
const HISTORY_GAP_THRESHOLD_MS = 90 * 1000
const HISTORY_EXPORT_PAGE_SIZE = 10000
const DEFAULT_ENERGY_RANGE: PresetEnergyRange = 'last_1h'

const energyRangeDurations: Record<PresetEnergyRange, number> = {
  last_1h: 60 * 60 * 1000,
  last_24h: 24 * 60 * 60 * 1000,
  last_7d: 7 * 24 * 60 * 60 * 1000,
  last_30d: 30 * 24 * 60 * 60 * 1000
}

const historySeriesDefinitions: HistorySeriesDefinition[] = [
  { key: 'temperature_1', label: 'T1', color: '#f43f5e' },
  { key: 'temperature_2', label: 'T2', color: '#2563eb' },
  { key: 'switch_1', label: 'SW1', color: '#7c3aed', switchSeries: true },
  { key: 'switch_2', label: 'SW2', color: '#f97316', switchSeries: true },
  { key: 'dry_contact_output', label: 'DO', color: '#16a34a', switchSeries: true },
  { key: 'electricity_consumption', label: 'kWh', color: '#0891b2', required: true }
]

const DEFAULT_HISTORY_CHART_SERIES_KEYS: RDIHistorySeriesKey[] = [
  'temperature_1',
  'temperature_2',
  'switch_1',
  'switch_2',
  'dry_contact_output',
  'electricity_consumption'
]

function createEmptyEnergyStats(): EnergyStatsSnapshot {
  return {
    sample_count: 0,
    latest: null,
    min: null,
    max: null,
    delta: null
  }
}

function isTemperatureHistoryKey(key: string) {
  return key === 'temperature_1' || key === 'temperature_2'
}

function normalizeHistoryTimestamp(value: unknown): number | null {
  if (value instanceof Date) return value.getTime()
  if (typeof value === 'number' && Number.isFinite(value)) return value < 1_000_000_000_000 ? value * 1000 : value
  if (typeof value === 'string') {
    const numeric = Number(value)
    if (Number.isFinite(numeric)) return numeric < 1_000_000_000_000 ? numeric * 1000 : numeric
    const parsed = Date.parse(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function normalizeHistoryValue(value: unknown): number | null {
  if (typeof value === 'boolean') return value ? 1 : 0
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === 'high' || normalized === 'open' || normalized === 'on') return 1
    if (
      normalized === 'false' ||
      normalized === 'low' ||
      normalized === 'close' ||
      normalized === 'closed' ||
      normalized === 'off'
    ) {
      return 0
    }
    const numeric = Number(value)
    if (Number.isFinite(numeric)) return numeric
  }
  return null
}

function normalizeHistoryList(payload: any): any[] {
  return Array.isArray(payload) ? payload : Array.isArray(payload?.list) ? payload.list : []
}

function normalizeHistoryTotal(payload: any): number | null {
  const rawTotal = payload?.total
  if (rawTotal === null || rawTotal === undefined || rawTotal === '') return null
  const total = Number(rawTotal)
  return Number.isFinite(total) && total >= 0 ? Math.floor(total) : null
}

function readHistoryPointTimestamp(item: any) {
  return item?.ts ?? item?.x ?? item?.time ?? item?.UpdateAt ?? item?.created_at
}

function readHistoryPointValue(item: any, key: RDIHistorySeriesKey) {
  return item?.value ?? item?.y ?? item?.number_v ?? item?.bool_v ?? item?.string_v ?? item?.[key]
}

function normalizeHistoryPoints(payload: any, key: RDIHistorySeriesKey): NormalizedHistoryPage {
  const points: HistoryPoint[] = []
  let invalidCount = 0

  for (const item of normalizeHistoryList(payload)) {
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

function historyPointIdentity(point: HistoryPoint) {
  return `${point.ts}\u0000${point.value}`
}

function appendUniqueHistoryPoints(
  target: HistoryPoint[],
  seenPointIdentities: Set<string>,
  nextPoints: HistoryPoint[],
  maxPoints: number
) {
  let addedCount = 0
  for (const point of nextPoints) {
    if (target.length >= maxPoints) break
    const identity = historyPointIdentity(point)
    if (seenPointIdentities.has(identity)) continue
    seenPointIdentities.add(identity)
    target.push(point)
    addedCount += 1
  }
  return addedCount
}

function insertHistoryGapMarkers(points: HistoryPoint[]) {
  const sortedPoints = [...points].sort((a, b) => a.ts - b.ts)
  if (!sortedPoints.length) return { points: sortedPoints, detectedGapCount: 0 }

  const markedPoints: HistoryPoint[] = [sortedPoints[0]]
  let detectedGapCount = sortedPoints[0].value === null ? 1 : 0

  for (let index = 1; index < sortedPoints.length; index += 1) {
    const previousPoint = sortedPoints[index - 1]
    const currentPoint = sortedPoints[index]
    const gapDuration = currentPoint.ts - previousPoint.ts

    if (
      gapDuration > HISTORY_GAP_THRESHOLD_MS &&
      previousPoint.value !== null &&
      currentPoint.value !== null
    ) {
      markedPoints.push({
        ts: previousPoint.ts + Math.floor(gapDuration / 2),
        value: null
      })
      detectedGapCount += 1
    }

    markedPoints.push(currentPoint)
    if (currentPoint.value === null) detectedGapCount += 1
  }

  return { points: markedPoints, detectedGapCount }
}

function finalizeHistorySeriesResult(
  key: RDIHistorySeriesKey,
  points: HistoryPoint[],
  expectedCount: number | null,
  invalidCount: number,
  failedPage: number | undefined,
  truncated: boolean
): HistorySeriesResult {
  const loadedCount = points.length
  const missingCount = expectedCount === null ? 0 : Math.max(0, expectedCount - loadedCount)
  const chartEvidence = insertHistoryGapMarkers(points)
  const failed = failedPage !== undefined && loadedCount === 0
  const partial =
    !failed &&
    (failedPage !== undefined ||
      truncated ||
      invalidCount > 0 ||
      missingCount > 0)

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

function normalizeHistoryExportRows(payload: any): HistoryExportRow[] {
  return normalizeHistoryList(payload)
    .map((item: any) => ({
      ts: item?.ts ?? item?.x ?? item?.time ?? '',
      value: item?.value ?? item?.y ?? item?.number_v ?? item?.string_v ?? item?.bool_v ?? ''
    }))
    .filter((item: HistoryExportRow) => item.ts !== '')
}

function csvEscape(value: unknown) {
  const text = value === undefined || value === null ? '' : String(value)
  return `"${text.replace(/"/g, '""')}"`
}

function downloadCsv(filename: string, rows: unknown[][]) {
  const csvContent = rows.map((row) => row.map(csvEscape).join(',')).join('\n')
  const blob = new Blob(['\uFEFF', csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

function resolveHistoryRange(range: string, customRange: HistoryRange | null, now = Date.now()): HistoryRange | null {
  if (range === 'custom') return customRange
  const duration = energyRangeDurations[range as PresetEnergyRange] ?? energyRangeDurations[DEFAULT_ENERGY_RANGE]
  return [now - duration, now]
}

function buildHistoryQueryParams(
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

function buildHistoryChartQueryParams(key: RDIHistorySeriesKey, range: HistoryRange, page = HISTORY_QUERY_PAGE) {
  return buildHistoryQueryParams(key, range, HISTORY_CHART_PAGE_SIZE, page)
}

function buildHistoryExportQueryParams(key: RDIHistorySeriesKey, range: HistoryRange, format: HistoryExportFormat) {
  return buildHistoryQueryParams(key, range, HISTORY_EXPORT_PAGE_SIZE, HISTORY_QUERY_PAGE, {
    export_excel: format === 'excel',
    export_format: format
  })
}

async function fetchHistorySeries(
  id: string,
  definition: HistorySeriesDefinition,
  range: HistoryRange
): Promise<HistorySeriesResult> {
  const points: HistoryPoint[] = []
  const seenPointIdentities = new Set<string>()
  let expectedPointCount: number | null = null
  let invalidCount = 0
  let failedPage: number | undefined
  let truncated = false

  for (let page = HISTORY_QUERY_PAGE; page <= HISTORY_CHART_MAX_PAGES_PER_SERIES; page += 1) {
    try {
      const { error, data } = await rdiDeviceHistory(id, buildHistoryChartQueryParams(definition.key, range, page))
      if (error) {
        failedPage = page
        break
      }

      const rawPageItems = normalizeHistoryList(data)
      const responseTotal = normalizeHistoryTotal(data)
      if (responseTotal !== null) {
        expectedPointCount = Math.max(expectedPointCount ?? 0, responseTotal)
      }

      const normalizedPage = normalizeHistoryPoints(rawPageItems, definition.key)
      invalidCount += normalizedPage.invalidCount

      const addedCount = appendUniqueHistoryPoints(
        points,
        seenPointIdentities,
        normalizedPage.points,
        HISTORY_CHART_MAX_POINTS_PER_SERIES
      )

      if (points.length >= HISTORY_CHART_MAX_POINTS_PER_SERIES) {
        truncated = expectedPointCount === null || expectedPointCount > points.length
        break
      }
      if (expectedPointCount !== null && points.length >= expectedPointCount) break
      if (rawPageItems.length === 0) break
      if (addedCount === 0) {
        // A repeated full page without a total cannot prove that the series ended.
        if (expectedPointCount === null && rawPageItems.length >= HISTORY_CHART_PAGE_SIZE) truncated = true
        break
      }
      if (expectedPointCount === null && rawPageItems.length < HISTORY_CHART_PAGE_SIZE) break
      if (page === HISTORY_CHART_MAX_PAGES_PER_SERIES) {
        // A full final page means the bounded client stopped before the server proved exhaustion.
        truncated = expectedPointCount === null || points.length < expectedPointCount
      }
    } catch {
      failedPage = page
      break
    }
  }

  return finalizeHistorySeriesResult(
    definition.key,
    points,
    expectedPointCount,
    invalidCount,
    failedPage,
    truncated
  )
}

function normalizeHistoryChartSeriesKeys(keys: RDIHistorySeriesKey[]): RDIHistorySeriesKey[] {
  const allowedKeys = new Set(historySeriesDefinitions.map((item) => item.key))
  // 空选择恢复初始默认；只要用户已有选择，就不能再把其余默认曲线隐式加回。
  if (keys.length === 0) return [...DEFAULT_HISTORY_CHART_SERIES_KEYS]

  const selectedKeys = keys.filter((key) => allowedKeys.has(key))
  return Array.from(new Set(selectedKeys))
}

async function fetchHistoryChartData(
  id: string,
  range: HistoryRange,
  seriesKeys: RDIHistorySeriesKey[]
): Promise<HistoryChartLoadResult> {
  const selectedKeySet = new Set(seriesKeys)
  const selectedDefinitions = historySeriesDefinitions.filter((item) => selectedKeySet.has(item.key))
  const seriesResults = await Promise.all(selectedDefinitions.map((item) => fetchHistorySeries(id, item, range)))
  const chartData = seriesResults.reduce<HistoryChartData>((nextChartData, item) => {
    nextChartData[item.key] = item.points
    return nextChartData
  }, {})
  return { chartData, seriesResults }
}

function calculateEnergyStats(points: EnergyPoint[]): EnergyStatsSnapshot {
  const sorted = [...points].sort((a, b) => a.x - b.x)
  if (!sorted.length) return createEmptyEnergyStats()

  const values = sorted.map((item) => item.y)
  return {
    sample_count: sorted.length,
    latest: sorted[sorted.length - 1].y,
    min: Math.min(...values),
    max: Math.max(...values),
    delta: sorted.length > 1 ? Math.max(0, sorted[sorted.length - 1].y - sorted[0].y) : 0
  }
}

function toEnergyPoints(points: HistoryPoint[] = []): EnergyPoint[] {
  return points.flatMap((item) =>
    item.value === null
      ? []
      : [
          {
            x: item.ts,
            y: item.value
          }
        ]
  )
}

function formatHistoryChartValueForUnit(key: RDIHistorySeriesKey, value: number, unit: TemperatureUnit) {
  if (isTemperatureHistoryKey(key) && unit === 'F') {
    return (value * 9) / 5 + 32
  }
  return value
}

function historyExportValueLabelForUnit(key: string, unit: TemperatureUnit) {
  return isTemperatureHistoryKey(key) ? `value (${unit})` : 'value'
}

function formatHistoryExportValueForUnit(key: string, value: unknown, unit: TemperatureUnit) {
  if (isTemperatureHistoryKey(key) && unit === 'F') {
    const numeric = normalizeHistoryValue(value)
    return numeric === null
      ? value
      : formatHistoryChartValueForUnit(key as RDIHistorySeriesKey, numeric, unit).toFixed(2)
  }
  return value
}

function formatHistoryExportTimestamp(ts: number | string) {
  return typeof ts === 'number' ? new Date(ts).toISOString() : ts
}

function buildHistoryExportCsvRows(
  key: RDIHistorySeriesKey,
  rows: HistoryExportRow[],
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

function getExportedHistoryFilePath(payload: any) {
  return payload?.filePath || payload?.file_path
}

function buildExportFileUrl(filePath: unknown) {
  const baseUrlWithoutApi = getBaseServerUrl().replace('/api/v1', '/')
  return `${baseUrlWithoutApi}${filePath}`
}

function buildHistoryChartOptions(
  t: RDIHistoryTranslate,
  chartData: HistoryChartData,
  formatValue: FormatHistoryChartValue
): EChartsCoreOption {
  const activeDefinitions = historySeriesDefinitions.filter((item) =>
    Object.prototype.hasOwnProperty.call(chartData, item.key)
  )
  return {
    color: activeDefinitions.map((item) => item.color),
    title: {
      text: t('historyChart'),
      left: 8,
      top: 0,
      textStyle: {
        fontSize: 13,
        fontWeight: 600
      }
    },
    tooltip: {
      trigger: 'axis'
    },
    legend: {
      top: 28,
      type: 'scroll'
    },
    grid: {
      top: 72,
      left: 48,
      right: 48,
      bottom: 56
    },
    xAxis: {
      type: 'time'
    },
    yAxis: [
      {
        type: 'value',
        name: t('valueAxis'),
        scale: true
      },
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
    series: activeDefinitions.map((item) => ({
      name: item.label,
      type: 'line',
      animation: false,
      smooth: !item.switchSeries,
      step: item.switchSeries ? 'middle' : false,
      showSymbol: false,
      sampling: item.switchSeries ? undefined : 'lttb',
      connectNulls: false,
      emphasis: {
        disabled: true
      },
      yAxisIndex: item.switchSeries ? 1 : 0,
      lineStyle: {
        width: 2,
        color: item.color
      },
      itemStyle: {
        color: item.color
      },
      data: (chartData[item.key] || []).map((point) => [
        point.ts,
        point.value === null ? null : formatValue(item.key, point.value)
      ])
    }))
  }
}

export function useRdiHistory(deviceId: () => string, temperatureUnit: () => 'C' | 'F', t: (key: LabelKey) => string) {
  const energyLoading = ref(false)
  const historyExportLoading = ref(false)
  const energyRange = ref<string>(DEFAULT_ENERGY_RANGE)
  const energyCustomRange = ref<HistoryRange | null>(null)
  const historyExportKey = ref<RDIHistorySeriesKey>('electricity_consumption')
  const historyChartSeriesKeys = ref<RDIHistorySeriesKey[]>([...DEFAULT_HISTORY_CHART_SERIES_KEYS])
  const historyExportFormat = ref<HistoryExportFormat>('excel')

  const energyStats = reactive(createEmptyEnergyStats())

  const historyChartData = ref<HistoryChartData>({})
  const historySeriesResults = ref<HistorySeriesResult[]>([])
  let historyContextRevision = 0
  let historyLoadSequence = 0
  let historyExportSequence = 0

  function formatHistoryChartValue(key: RDIHistorySeriesKey, value: number) {
    return formatHistoryChartValueForUnit(key, value, temperatureUnit())
  }

  function formatEnergyValue(value: number | null) {
    return value === null ? '--' : `${value.toFixed(2)} kWh`
  }

  function formatDurationLabel(value: number | null | undefined) {
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

  function updateEnergyStats(points: EnergyPoint[]) {
    Object.assign(energyStats, calculateEnergyStats(points))
  }

  function resolveEnergyHistoryRange() {
    return resolveHistoryRange(energyRange.value, energyCustomRange.value)
  }

  const energyRangeOptions = computed(() => [
    { label: t('last1Hour'), value: 'last_1h' },
    { label: t('last1Day'), value: 'last_24h' },
    { label: t('last1Week'), value: 'last_7d' },
    { label: t('last30Days'), value: 'last_30d' },
    { label: t('customRange'), value: 'custom' }
  ])

  const historyExportKeyOptions = computed(() => [
    ...historySeriesDefinitions.map((item) => ({ label: item.label, value: item.key }))
  ])

  const historyChartSeriesOptions = computed(() =>
    historySeriesDefinitions.map((item) => ({
      label: item.required ? `${item.label} (${t('energy')})` : item.label,
      value: item.key,
      disabled: item.required
    }))
  )

  const historyExportFormatOptions = computed(() => [
    { label: t('excelFormat'), value: 'excel' },
    { label: t('csvFormat'), value: 'csv' }
  ])

  const historyChartOptions = computed<EChartsCoreOption>(() =>
    buildHistoryChartOptions(t, historyChartData.value, formatHistoryChartValue)
  )

  function labelsForHistoryResults(predicate: (result: HistorySeriesResult) => boolean) {
    const labelsByKey = new Map(historySeriesDefinitions.map((definition) => [definition.key, definition.label]))
    return historySeriesResults.value
      .filter(predicate)
      .map((result) => labelsByKey.get(result.key) || result.key)
  }

  const failedHistorySeriesLabels = computed(() =>
    labelsForHistoryResults((result) => result.status === 'failed')
  )
  const partialHistorySeriesLabels = computed(() =>
    labelsForHistoryResults((result) => result.status === 'partial')
  )
  const gappedHistorySeriesLabels = computed(() =>
    labelsForHistoryResults((result) => result.detectedGapCount > 0)
  )
  const hasHistoryFailures = computed(() =>
    historySeriesResults.value.some((result) => result.status === 'failed' || result.status === 'partial')
  )
  const hasSuccessfulHistoryData = computed(() =>
    historySeriesResults.value.some((result) => result.status !== 'failed')
  )
  const hasHistoryChartData = computed(() =>
    Object.values(historyChartData.value).some((points) => points?.some((point) => point.value !== null))
  )
  const energyStatisticsAvailable = computed(() => {
    const result = historySeriesResults.value.find((item) => item.key === 'electricity_consumption')
    if (!result || result.status === 'failed') return false
    return result.status === 'empty' || result.points.some((point) => point.value !== null)
  })

  async function loadEnergyStatistics() {
    const id = deviceId()
    if (!id) return
    const range = resolveEnergyHistoryRange()
    if (!range) {
      message.error(t('customRange'))
      return
    }
    const contextRevision = historyContextRevision
    const requestSequence = ++historyLoadSequence
    energyLoading.value = true
    try {
      const selectedSeriesKeys = normalizeHistoryChartSeriesKeys(historyChartSeriesKeys.value)
      historyChartSeriesKeys.value = selectedSeriesKeys
      const nextHistory = await fetchHistoryChartData(id, range, selectedSeriesKeys)
      if (
        contextRevision !== historyContextRevision ||
        requestSequence !== historyLoadSequence ||
        id !== deviceId()
      ) {
        return
      }
      historySeriesResults.value = nextHistory.seriesResults
      historyChartData.value = nextHistory.chartData
      updateEnergyStats(toEnergyPoints(nextHistory.chartData.electricity_consumption))
    } finally {
      if (contextRevision === historyContextRevision && requestSequence === historyLoadSequence) {
        energyLoading.value = false
      }
    }
  }

  async function exportHistoryData() {
    const id = deviceId()
    if (!id) return
    const range = resolveEnergyHistoryRange()
    if (!range) {
      message.error(t('customRange'))
      return
    }
    const contextRevision = historyContextRevision
    const requestSequence = ++historyExportSequence
    const exportKey = historyExportKey.value
    const exportFormat = historyExportFormat.value
    const exportTemperatureUnit = temperatureUnit()
    historyExportLoading.value = true
    try {
      const { error, data } = await rdiDeviceHistory(
        id,
        buildHistoryExportQueryParams(exportKey, range, exportFormat)
      )
      if (
        contextRevision !== historyContextRevision ||
        requestSequence !== historyExportSequence ||
        id !== deviceId()
      ) {
        return
      }
      if (error) return
      const exportedFilePath = getExportedHistoryFilePath(data)
      if (exportedFilePath) {
        window.open(buildExportFileUrl(exportedFilePath))
        return
      }
      const rows = normalizeHistoryExportRows(data)
      if (!rows.length) {
        message.error(t('empty'))
        return
      }
      downloadCsv(
        `rdi_${id}_${exportKey}.csv`,
        buildHistoryExportCsvRows(exportKey, rows, exportTemperatureUnit)
      )
    } finally {
      if (contextRevision === historyContextRevision && requestSequence === historyExportSequence) {
        historyExportLoading.value = false
      }
    }
  }

  watch(
    deviceId,
    (nextId, previousId) => {
      if (nextId === previousId) return
      historyContextRevision += 1
      historyChartData.value = {}
      historySeriesResults.value = []
      updateEnergyStats([])
      energyLoading.value = false
      historyExportLoading.value = false
    },
    { flush: 'sync' }
  )

  watch(energyRange, (value) => {
    if (value !== 'custom') energyCustomRange.value = null
  })

  return {
    RDI_DURATION_MAX_SECONDS,
    energyLoading,
    historyExportLoading,
    energyRange,
    energyCustomRange,
    historyExportKey,
    historyChartSeriesKeys,
    historyExportFormat,
    energyStats,
    historyChartData,
    historySeriesResults,
    historyChartOptions,
    failedHistorySeriesLabels,
    partialHistorySeriesLabels,
    gappedHistorySeriesLabels,
    hasHistoryFailures,
    hasSuccessfulHistoryData,
    hasHistoryChartData,
    energyStatisticsAvailable,
    energyRangeOptions,
    historyChartSeriesOptions,
    historyExportKeyOptions,
    historyExportFormatOptions,
    formatDurationLabel,
    formatEnergyValue,
    loadEnergyStatistics,
    exportHistoryData
  }
}
