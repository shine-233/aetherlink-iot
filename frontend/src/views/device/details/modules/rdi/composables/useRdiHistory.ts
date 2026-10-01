/**
 * 文件用途: RDI 操作视图历史数据 composable(薄壳)。
 * 核心逻辑: 只持有响应式状态、请求竞态守卫、设备切换 watch 与 DOM 副作用(下载/打开导出文件);
 *   查询参数、分页累加、响应 normalize、缺口检测、统计与图表配置全部委托给 rdi-history-pure.ts。
 * 关键注意事项: 返回值形状是 RdiDeviceHistoryView / RdiDevicePowerConsumptionView / OperationsContext 的契约,
 *   修改前需同步调用方;设备切换必须同步(flush: 'sync')清空旧设备的证据,防止串台。
 */
import { computed, reactive, ref, watch } from 'vue'
import type { EChartsCoreOption } from 'echarts/core'
import { rdiDeviceHistory } from '@/service/api'
import type { RDIHistoryParams } from '@/service/api/rdi'
import { message } from '@/utils/common/discrete'
import { getBaseServerUrl } from '@/utils/common/tool'
import type { LabelKey } from '../constants/rdi-labels'
import { RDI_DURATION_MAX_SECONDS } from '../constants/rdi-ranges'
import {
  DEFAULT_ENERGY_RANGE,
  DEFAULT_HISTORY_CHART_SERIES_KEYS,
  buildCsvContent,
  buildEnergyRangeOptions,
  buildExportFileUrl,
  buildHistoryChartOptions,
  buildHistoryChartSeriesOptions,
  buildHistoryExportCsvRows,
  buildHistoryExportFilename,
  buildHistoryExportFormatOptions,
  buildHistoryExportKeyOptions,
  buildHistoryExportQueryParams,
  calculateEnergyStats,
  createEmptyEnergyStats,
  fetchHistoryChartData,
  formatDurationLabel,
  formatEnergyValue,
  formatHistoryChartValueForUnit,
  getExportedHistoryFilePath,
  hasRenderableHistoryChartData,
  normalizeHistoryChartSeriesKeys,
  normalizeHistoryExportRows,
  resolveHistoryRange,
  summarizeHistorySeriesResults,
  type HistoryChartData,
  type HistoryExportFormat,
  type HistoryPoint,
  type HistoryRange,
  type HistorySeriesResult,
  type RDIHistorySeriesKey,
  type TemperatureUnit
} from './rdi-history-pure'

/** Excel 需要 BOM 才能按 UTF-8 打开中文 CSV。 */
const CSV_UTF8_BOM = String.fromCharCode(0xfeff)

function downloadCsv(filename: string, rows: unknown[][]) {
  const blob = new Blob([CSV_UTF8_BOM, buildCsvContent(rows)], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

/**
 * 设备上下文 + 请求序号守卫: 只有"发起时的设备上下文仍有效且是最新一次请求"的结果才允许落地。
 * 设备切换会使所有在途请求失效。
 */
function createRequestGuard(deviceId: () => string) {
  let contextRevision = 0
  return {
    invalidateContext() {
      contextRevision += 1
    },
    channel() {
      let sequence = 0
      return {
        begin(id: string) {
          const ticket = { revision: contextRevision, sequence: ++sequence, id }
          return {
            /** 结果是否仍可写入状态(设备未变、上下文未失效、未被更新请求取代)。 */
            isCurrent: () =>
              ticket.revision === contextRevision && ticket.sequence === sequence && ticket.id === deviceId(),
            /** 是否仍持有 loading 标志(设备切换已经重置过 loading 时不再改写)。 */
            ownsLoading: () => ticket.revision === contextRevision && ticket.sequence === sequence
          }
        }
      }
    }
  }
}

export function useRdiHistory(
  deviceId: () => string,
  temperatureUnit: () => TemperatureUnit,
  t: (key: LabelKey) => string
) {
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

  const guard = createRequestGuard(deviceId)
  const loadChannel = guard.channel()
  const exportChannel = guard.channel()
  const fetchPage = (id: string) => (params: RDIHistoryParams) => rdiDeviceHistory(id, params)

  function formatHistoryChartValue(key: RDIHistorySeriesKey, value: number) {
    return formatHistoryChartValueForUnit(key, value, temperatureUnit())
  }

  function updateEnergyStats(points: readonly HistoryPoint[] = []) {
    Object.assign(energyStats, calculateEnergyStats(points))
  }

  function resolveEnergyHistoryRange() {
    return resolveHistoryRange(energyRange.value, energyCustomRange.value)
  }

  const energyRangeOptions = computed(() => buildEnergyRangeOptions(t))
  const historyExportKeyOptions = computed(() => buildHistoryExportKeyOptions())
  const historyChartSeriesOptions = computed(() => buildHistoryChartSeriesOptions(t))
  const historyExportFormatOptions = computed(() => buildHistoryExportFormatOptions(t))
  const historyChartOptions = computed<EChartsCoreOption>(() =>
    buildHistoryChartOptions(t, historyChartData.value, formatHistoryChartValue)
  )

  const historySummary = computed(() => summarizeHistorySeriesResults(historySeriesResults.value))
  const failedHistorySeriesLabels = computed(() => historySummary.value.failedLabels)
  const partialHistorySeriesLabels = computed(() => historySummary.value.partialLabels)
  const gappedHistorySeriesLabels = computed(() => historySummary.value.gappedLabels)
  const hasHistoryFailures = computed(() => historySummary.value.hasFailures)
  const hasSuccessfulHistoryData = computed(() => historySummary.value.hasSuccessfulData)
  const energyStatisticsAvailable = computed(() => historySummary.value.energyStatisticsAvailable)
  const hasHistoryChartData = computed(() => hasRenderableHistoryChartData(historyChartData.value))

  /** 公共前置: 设备与时间范围都有效时返回二者,否则提示并返回 null。 */
  function resolveRequestContext() {
    const id = deviceId()
    if (!id) return null
    const range = resolveEnergyHistoryRange()
    if (!range) {
      message.error(t('customRange'))
      return null
    }
    return { id, range }
  }

  async function loadEnergyStatistics() {
    const context = resolveRequestContext()
    if (!context) return
    const ticket = loadChannel.begin(context.id)
    energyLoading.value = true
    try {
      const selectedSeriesKeys = normalizeHistoryChartSeriesKeys(historyChartSeriesKeys.value)
      historyChartSeriesKeys.value = selectedSeriesKeys
      const nextHistory = await fetchHistoryChartData(fetchPage(context.id), context.range, selectedSeriesKeys)
      if (!ticket.isCurrent()) return
      historySeriesResults.value = nextHistory.seriesResults
      historyChartData.value = nextHistory.chartData
      updateEnergyStats(nextHistory.chartData.electricity_consumption)
    } finally {
      if (ticket.ownsLoading()) energyLoading.value = false
    }
  }

  async function exportHistoryData() {
    const context = resolveRequestContext()
    if (!context) return
    const { id, range } = context
    const ticket = exportChannel.begin(id)
    // 导出参数在发起时快照,避免等待期间用户改选导致文件内容与文件名不一致。
    const exportKey = historyExportKey.value
    const exportFormat = historyExportFormat.value
    const exportTemperatureUnit = temperatureUnit()
    historyExportLoading.value = true
    try {
      const { error, data } = await rdiDeviceHistory(id, buildHistoryExportQueryParams(exportKey, range, exportFormat))
      if (!ticket.isCurrent() || error) return
      const exportedFilePath = getExportedHistoryFilePath(data)
      if (exportedFilePath) {
        window.open(buildExportFileUrl(getBaseServerUrl(), exportedFilePath))
        return
      }
      const rows = normalizeHistoryExportRows(data)
      if (!rows.length) {
        message.error(t('empty'))
        return
      }
      downloadCsv(
        buildHistoryExportFilename(id, exportKey),
        buildHistoryExportCsvRows(exportKey, rows, exportTemperatureUnit)
      )
    } finally {
      if (ticket.ownsLoading()) historyExportLoading.value = false
    }
  }

  watch(
    deviceId,
    (nextId, previousId) => {
      if (nextId === previousId) return
      guard.invalidateContext()
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
