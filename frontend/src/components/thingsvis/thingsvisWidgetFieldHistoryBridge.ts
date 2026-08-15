/**
 * 文件说明：
 * - 承接 ThingsVisWidget 中历史字段补水相关的配置扫描与时序读取逻辑。
 * - 负责从 widget 配置推导 `__history` 字段需求、判断是否需要 buffer 预填，并统一遥测历史接口返回。
 * 维护提示：
 * - 这里直接影响 widget 首屏时序完整性；调整字段表达式、时间范围或接口归一化规则前要同步核对 guest 绑定协议。
 * - 该模块只负责“历史需求识别 + 历史数据读取”，不负责 `tv:platform-data` 回推。
 */
import type { HistoryRequestConfig } from '@/components/thingsvis/widgetFieldDataBridge'

const FIELD_BINDING_EXPR_GLOBAL_RE = /\{\{\s*ds\.([^.\s]+)\.data(?:\.(.+?))?\s*\}\}/g

const HISTORY_TIME_RANGE_BY_PRESET: Record<string, string> = {
  '1h': 'last_1h',
  '6h': 'last_6h',
  '24h': 'last_24h',
  '7d': 'last_7d',
  '30d': 'last_30d',
  all: 'last_30d'
}
const TELEMETRY_HISTORY_CACHE_TTL_MS = 1000
const TELEMETRY_HISTORY_CACHE_MAX_ENTRIES = 200

type ParsedFieldBinding = {
  dataSourceId: string
  fieldPath: string
}

type TelemetryHistoryRow = {
  value: unknown
  ts: number
}

type TelemetryHistoryCacheEntry = {
  rows: TelemetryHistoryRow[]
  cachedAt: number
}

type ThingsVisWidgetFieldHistoryBridgeOptions = {
  getConfig: () => any
  historyFieldSuffix: string
  templateDeviceId: string
  runtimeStatusFieldIds: Set<string>
  getFieldDataTypeMap: () => Record<string, string>
  getFieldRoot: (fieldPath: string) => string
  parseFieldBindingExpression: (input: unknown) => ParsedFieldBinding | null
  loadTelemetryHistory: (
    params: Record<string, unknown>,
    options?: Record<string, unknown>
  ) => Promise<any>
}

const normalizeTelemetryHistoryRows = (payload: any) => {
  // 历史接口存在多种返回壳结构，这里统一压成 widget 能直接消费的时序点。
  const source = payload?.data !== undefined ? payload.data : payload
  const rows = Array.isArray(source)
    ? source
    : Array.isArray(source?.list)
      ? source.list
      : Array.isArray(source?.data)
        ? source.data
        : []

  return rows
    .map((item: any) => {
      const timestamp = item?.timestamp ?? item?.time ?? item?.ts ?? item?.x
      const value = item?.value ?? item?.y ?? item?.avg
      const ts =
        typeof timestamp === 'number'
          ? timestamp
          : typeof timestamp === 'string'
            ? Date.parse(timestamp)
            : Number(timestamp)
      return { timestamp, ts, value: Number(value) }
    })
    .filter((item: any) => Number.isFinite(item.ts) && Number.isFinite(item.value))
}

const normalizeHistoryTimeRange = (value: unknown) => {
  if (typeof value !== 'string') return 'last_30d'
  if (value.startsWith('last_')) return value
  return HISTORY_TIME_RANGE_BY_PRESET[value] || 'last_30d'
}

const normalizeHistoryConfig = (config?: HistoryRequestConfig): Required<HistoryRequestConfig> => ({
  timeRange: normalizeHistoryTimeRange(config?.timeRange),
  aggFunction: config?.aggFunction || 'NONE_RAW',
  aggWindow: config?.aggWindow || 'no_aggregate'
})

const buildTelemetryHistoryCacheKey = (
  deviceId: string,
  fieldId: string,
  historyConfig: Required<HistoryRequestConfig>
) =>
  [
    deviceId,
    fieldId,
    historyConfig.timeRange,
    historyConfig.aggWindow,
    historyConfig.aggFunction
  ].join('|')

const visitStringLeaves = (value: unknown, visitor: (input: string) => void) => {
  if (typeof value === 'string') {
    visitor(value)
    return
  }

  if (Array.isArray(value)) {
    value.forEach((item) => visitStringLeaves(item, visitor))
    return
  }

  if (value && typeof value === 'object') {
    Object.values(value).forEach((item) => visitStringLeaves(item, visitor))
  }
}

const resolveNodeHistoryTimeRange = (node: any) => {
  return normalizeHistoryTimeRange(node?.props?.timeRangePreset)
}

const mergeHistoryTimeRange = (current: string | undefined, next: string) => {
  const historyTimeRangeWeight: Record<string, number> = {
    last_1h: 1,
    last_6h: 2,
    last_24h: 3,
    last_7d: 4,
    last_30d: 5
  }
  if (!current) return next
  const currentWeight = historyTimeRangeWeight[current] ?? 0
  const nextWeight = historyTimeRangeWeight[next] ?? 0
  return nextWeight > currentWeight ? next : current
}

const registerParsedHistoryBinding = (
  requests: Map<string, string>,
  parsed: ParsedFieldBinding | null,
  dataSourceId: string | undefined,
  timeRange: string,
  getFieldRoot: (fieldPath: string) => string,
  historyFieldSuffix: string
) => {
  if (!parsed || (dataSourceId && parsed.dataSourceId !== dataSourceId)) return

  const fieldRoot = getFieldRoot(parsed.fieldPath)
  if (!fieldRoot.endsWith(historyFieldSuffix)) return

  const sourceFieldId = fieldRoot.slice(0, -historyFieldSuffix.length)
  if (!sourceFieldId) return

  requests.set(sourceFieldId, mergeHistoryTimeRange(requests.get(sourceFieldId), timeRange))
}

const registerNodeHistoryBindings = (
  requests: Map<string, string>,
  node: any,
  dataSourceId: string | undefined,
  options: Pick<
    ThingsVisWidgetFieldHistoryBridgeOptions,
    'parseFieldBindingExpression' | 'getFieldRoot' | 'historyFieldSuffix'
  >
) => {
  const bindings = Array.isArray(node?.data) ? node.data : []

  bindings.forEach((binding: any) => {
    registerParsedHistoryBinding(
      requests,
      options.parseFieldBindingExpression(binding?.expression),
      dataSourceId,
      normalizeHistoryTimeRange(binding?.historyConfig?.timeRange),
      options.getFieldRoot,
      options.historyFieldSuffix
    )
  })
}

const registerNodeHistoryStringLeaves = (
  requests: Map<string, string>,
  node: any,
  dataSourceId: string | undefined,
  options: Pick<ThingsVisWidgetFieldHistoryBridgeOptions, 'getFieldRoot' | 'historyFieldSuffix'>
) => {
  const historyTimeRange = resolveNodeHistoryTimeRange(node)

  visitStringLeaves(node, (input) => {
    FIELD_BINDING_EXPR_GLOBAL_RE.lastIndex = 0

    let match: RegExpExecArray | null
    while ((match = FIELD_BINDING_EXPR_GLOBAL_RE.exec(input)) !== null) {
      if (!match[1] || !match[2]) continue

      registerParsedHistoryBinding(
        requests,
        {
          dataSourceId: match[1],
          fieldPath: match[2]
        },
        dataSourceId,
        historyTimeRange,
        options.getFieldRoot,
        options.historyFieldSuffix
      )
    }
  })
}

const getConfiguredPlatformDataSource = (config: any, dataSourceId?: string) => {
  if (!dataSourceId) return null
  const dataSources = Array.isArray(config?.dataSources) ? config.dataSources : []
  return dataSources.find((dataSource: any) => dataSource?.id === dataSourceId) || null
}

export const createThingsVisWidgetFieldHistoryBridge = (options: ThingsVisWidgetFieldHistoryBridgeOptions) => {
  const telemetryHistoryCache = new Map<string, TelemetryHistoryCacheEntry>()
  const telemetryHistoryInFlight = new Map<string, Promise<TelemetryHistoryRow[]>>()

  const pruneTelemetryHistoryCache = () => {
    if (telemetryHistoryCache.size <= TELEMETRY_HISTORY_CACHE_MAX_ENTRIES) return
    const overflow = telemetryHistoryCache.size - TELEMETRY_HISTORY_CACHE_MAX_ENTRIES
    Array.from(telemetryHistoryCache.keys())
      .slice(0, overflow)
      .forEach((key) => telemetryHistoryCache.delete(key))
  }

  const collectConfiguredHistoryFields = (dataSourceId?: string) => {
    // 同时扫描 data 绑定和节点字符串属性里的历史字段引用。
    const requests = new Map<string, string>()
    const config = options.getConfig()
    const nodes = Array.isArray(config?.nodes) ? config.nodes : []

    nodes.forEach((node: any) => {
      registerNodeHistoryBindings(requests, node, dataSourceId, options)
      registerNodeHistoryStringLeaves(requests, node, dataSourceId, options)
    })

    return requests
  }

  const shouldPrefillHistoryForDataSource = (dataSourceId?: string) => {
    const dataSource = getConfiguredPlatformDataSource(options.getConfig(), dataSourceId)
    const bufferSize = dataSource?.config?.bufferSize
    return typeof bufferSize === 'number' && Number.isFinite(bufferSize) && bufferSize > 0
  }

  const fetchTelemetryHistoryField = async (deviceId: string, fieldId: string, config?: HistoryRequestConfig) => {
    if (!deviceId || deviceId === options.templateDeviceId || !fieldId) return [] as TelemetryHistoryRow[]
    if (options.runtimeStatusFieldIds.has(fieldId)) return [] as TelemetryHistoryRow[]

    const fieldDataTypeMap = options.getFieldDataTypeMap()
    // 非 telemetry 字段没有时序历史，不要误打历史查询接口。
    if (fieldDataTypeMap[fieldId] && fieldDataTypeMap[fieldId] !== 'telemetry') return [] as TelemetryHistoryRow[]

    const historyConfig = normalizeHistoryConfig(config)
    const cacheKey = buildTelemetryHistoryCacheKey(deviceId, fieldId, historyConfig)
    const cached = telemetryHistoryCache.get(cacheKey)
    if (cached && Date.now() - cached.cachedAt <= TELEMETRY_HISTORY_CACHE_TTL_MS) return cached.rows
    const inFlight = telemetryHistoryInFlight.get(cacheKey)
    if (inFlight) return inFlight

    const request = (async () => {
      try {
        const response = await options.loadTelemetryHistory(
          {
            device_id: deviceId,
            key: fieldId,
            time_range: historyConfig.timeRange,
            aggregate_window: historyConfig.aggWindow,
            aggregate_function: historyConfig.aggFunction
          },
          { silentError: true }
        )
        const rows = response?.error ? ([] as TelemetryHistoryRow[]) : normalizeTelemetryHistoryRows(response)
        telemetryHistoryCache.set(cacheKey, { rows, cachedAt: Date.now() })
        pruneTelemetryHistoryCache()
        return rows
      } catch (error) {
        console.warn('[ThingsVisWidget] telemetry history request failed:', fieldId, error)
        return [] as TelemetryHistoryRow[]
      } finally {
        telemetryHistoryInFlight.delete(cacheKey)
      }
    })()

    telemetryHistoryInFlight.set(cacheKey, request)
    return request
  }

  return {
    collectConfiguredHistoryFields,
    shouldPrefillHistoryForDataSource,
    fetchTelemetryHistoryField
  }
}
