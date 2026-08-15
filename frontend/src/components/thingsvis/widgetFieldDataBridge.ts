/**
 * 文件说明：
 * - ThingsVis 字段读取桥接里的“响应拼装层”。
 * - 负责把 guest 请求拆成实时值、告警派生字段、历史序列三部分并合并返回。
 * - 这里的策略会直接影响 widget 首屏数据完整性，是嵌入体验的关键稳定点。
 */
import { classifyRequestedFieldIds, type RequestedFieldGroups } from '@/components/thingsvis/fieldReadBridge'

const HISTORY_TIME_RANGE_WEIGHT: Record<string, number> = {
  last_1h: 1,
  last_6h: 2,
  last_24h: 3,
  last_7d: 4,
  last_30d: 5
}
const DEFAULT_HISTORY_FETCH_CONCURRENCY = 4
const DEFAULT_PREFILL_HISTORY_FIELD_LIMIT = 8

export type HistoryRequestConfig = {
  timeRange?: string
  aggFunction?: string
  aggWindow?: string
}

export type FieldDataRequestPayload = {
  dataSourceId?: string
  deviceId?: string
  fieldIds?: string[]
  historyConfig?: HistoryRequestConfig
}

export type ResolvedFieldDataRequest = {
  payload: FieldDataRequestPayload | undefined
  fieldIds: string[]
  targetDeviceId: string | undefined
}

type TelemetryHistoryRow = {
  value: unknown
  ts: number
}

type FieldDataBridgeOptions = {
  historyFieldSuffix: string
  templateDeviceId: string
  alarmStatusFieldIds: Set<string>
  currentData?: Record<string, unknown>
  collectConfiguredHistoryFields: (dataSourceId?: string) => Map<string, string>
  shouldPrefillHistoryForDataSource: (dataSourceId?: string) => boolean
  fetchTelemetryHistoryField: (
    deviceId: string,
    fieldId: string,
    config?: HistoryRequestConfig
  ) => Promise<TelemetryHistoryRow[]>
  pushPlatformFieldHistory: (fieldId: string, history: TelemetryHistoryRow[], deviceId?: string) => void
  loadAlarmStatus: (deviceId: string) => Promise<any>
}

const mergeHistoryTimeRange = (current: string | undefined, next: string) => {
  // 同一字段被多个节点请求时，选择覆盖范围更大的时间窗口，避免数据不足。
  if (!current) return next
  const currentWeight = HISTORY_TIME_RANGE_WEIGHT[current] ?? 0
  const nextWeight = HISTORY_TIME_RANGE_WEIGHT[next] ?? 0
  return nextWeight > currentWeight ? next : current
}

const registerHistoryTimeRange = (requests: Map<string, string | undefined>, fieldId: string, timeRange?: string) => {
  const current = requests.get(fieldId)
  if (!timeRange) {
    if (!requests.has(fieldId)) requests.set(fieldId, current)
    return
  }

  requests.set(fieldId, mergeHistoryTimeRange(current, timeRange))
}

const buildRequestedFieldData = (
  fieldIds: string[],
  currentData: Record<string, unknown> | undefined,
  historyFieldSuffix: string
) => {
  if (!fieldIds.length) return {}

  const result: Record<string, unknown> = {}
  fieldIds.forEach((fieldId) => {
    // __history 字段由历史查询通道处理，这里只负责当前值。
    if (fieldId.endsWith(historyFieldSuffix)) return
    if (currentData && Object.prototype.hasOwnProperty.call(currentData, fieldId)) {
      result[fieldId] = currentData[fieldId]
    }
  })

  return result
}

const normalizeAlarmLevel = (raw: unknown) => {
  const value = String(raw ?? '')
    .toLowerCase()
    .trim()
  if (value === '1' || value === 'critical' || value === 'high' || value === 'serious') return 'critical'
  if (value === '2' || value === 'warning' || value === 'medium' || value === 'warn') return 'warning'
  if (value === '3' || value === 'info' || value === 'low') return 'info'
  return value || ''
}

const alarmLevelRank = (row: any) => {
  switch (normalizeAlarmLevel(row?.alarm_level ?? row?.level)) {
    case 'critical':
      return 3
    case 'warning':
      return 2
    case 'info':
      return 1
    default:
      return 0
  }
}

const normalizeAlarmTime = (row: any) =>
  row?.last_trigger_time ?? row?.create_time ?? row?.created_at ?? row?.time ?? ''

const getRequestedAlarmStatusFields = (fieldIds: string[], alarmStatusFieldIds: Set<string>) => {
  return fieldIds.filter((fieldId) => alarmStatusFieldIds.has(fieldId))
}

const normalizeAlarmStatusPayload = (response: any) => response?.data ?? response

const getAlarmStatusRows = (payload: any) => {
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.data)) return payload.data
  if (Array.isArray(payload)) return payload
  return []
}

const pickHighestAlarmRow = (activeRows: any[], fallback: any) => {
  return (
    activeRows.reduce<any | null>(
      (current, row) => (!current || alarmLevelRank(row) > alarmLevelRank(current) ? row : current),
      null
    ) ?? fallback
  )
}

const buildAlarmStatusFields = (payload: any, rows: any[]) => {
  const activeRows = rows.filter((row) => {
    const raw = String(row?.alarm_status ?? row?.status ?? row?.is_active ?? '').toLowerCase()
    return row?.is_active === true || raw === '1' || raw === 'active' || raw === 'triggered' || raw === 'a'
  })
  const latest = rows[0] ?? null
  const highest = pickHighestAlarmRow(activeRows, latest)

  return {
    device_alarm_active: activeRows.length > 0 ? 1 : 0,
    device_alarm_count: Number(payload?.total ?? activeRows.length ?? 0),
    device_alarm_highest_level: normalizeAlarmLevel(highest?.alarm_level ?? highest?.level),
    latest_device_alarm_title: String(latest?.alarm_name ?? latest?.name ?? latest?.title ?? ''),
    latest_device_alarm_level: normalizeAlarmLevel(latest?.alarm_level ?? latest?.level),
    latest_device_alarm_time: latest ? normalizeAlarmTime(latest) : ''
  } as Record<string, unknown>
}

const pickRequestedFieldValues = (fieldValues: Record<string, unknown>, requestedFields: string[]) => {
  return requestedFields.reduce<Record<string, unknown>>((acc, fieldId) => {
    acc[fieldId] = fieldValues[fieldId]
    return acc
  }, {})
}

const buildRequestedAlarmStatusData = async (
  fieldIds: string[],
  deviceId: string | undefined,
  options: Pick<FieldDataBridgeOptions, 'alarmStatusFieldIds' | 'templateDeviceId' | 'loadAlarmStatus'>
) => {
  const requestedAlarmFields = getRequestedAlarmStatusFields(fieldIds, options.alarmStatusFieldIds)
  // 模板设备与空设备上下文没有真实告警数据，直接跳过。
  if (!deviceId || deviceId === options.templateDeviceId || requestedAlarmFields.length === 0) return {}

  try {
    const response = await options.loadAlarmStatus(deviceId)
    const payload = normalizeAlarmStatusPayload(response)
    const rows = getAlarmStatusRows(payload)
    return pickRequestedFieldValues(buildAlarmStatusFields(payload, rows), requestedAlarmFields)
  } catch (error) {
    console.warn('[ThingsVisWidget] device alarm status request failed:', deviceId, error)
    return {}
  }
}

const registerExplicitHistoryRequests = (historyRequests: Map<string, string | undefined>, fieldIds: Set<string>) => {
  fieldIds.forEach((fieldId) => registerHistoryTimeRange(historyRequests, fieldId))
}

const registerPrefillHistoryRequests = (
  historyRequests: Map<string, string | undefined>,
  payload: FieldDataRequestPayload | undefined,
  fieldGroups: RequestedFieldGroups,
  configuredHistoryFields: Map<string, string>,
  shouldPrefillHistoryForDataSource: (dataSourceId?: string) => boolean
) => {
  if (!shouldPrefillHistoryForDataSource(payload?.dataSourceId)) return

  // 开启 buffer 预填时，即使 guest 没显式请求 __history，也会为相关当前值补一份历史序列。
  const historyBoundFieldIds = new Set<string>([
    ...fieldGroups.explicitHistorySourceFieldIds,
    ...Array.from(configuredHistoryFields.keys())
  ])
  const prefillFieldIds =
    historyBoundFieldIds.size > 0
      ? fieldGroups.currentFieldIds.filter((fieldId) => historyBoundFieldIds.has(fieldId))
      : fieldGroups.currentFieldIds

  prefillFieldIds.slice(0, DEFAULT_PREFILL_HISTORY_FIELD_LIMIT).forEach((fieldId) => {
    registerHistoryTimeRange(historyRequests, fieldId)
  })
}

const buildHistoryRequests = (
  payload: FieldDataRequestPayload | undefined,
  fieldGroups: RequestedFieldGroups,
  options: Pick<FieldDataBridgeOptions, 'collectConfiguredHistoryFields' | 'shouldPrefillHistoryForDataSource'>
) => {
  const historyRequests = new Map<string, string | undefined>()
  const configuredHistoryFields = options.collectConfiguredHistoryFields(payload?.dataSourceId)

  registerExplicitHistoryRequests(historyRequests, fieldGroups.explicitHistorySourceFieldIds)
  registerPrefillHistoryRequests(
    historyRequests,
    payload,
    fieldGroups,
    configuredHistoryFields,
    options.shouldPrefillHistoryForDataSource
  )
  configuredHistoryFields.forEach((timeRange, fieldId) => {
    registerHistoryTimeRange(historyRequests, fieldId, timeRange)
  })

  return historyRequests
}

const buildRequestedCurrentFieldPayload = async (
  fieldIds: string[],
  deviceId: string | undefined,
  options: FieldDataBridgeOptions
) => ({
  ...buildRequestedFieldData(fieldIds, options.currentData, options.historyFieldSuffix),
  ...(await buildRequestedAlarmStatusData(fieldIds, deviceId, options))
})

const mapWithConcurrency = async <T, R>(items: T[], concurrency: number, mapper: (item: T) => Promise<R>) => {
  const results: R[] = new Array(items.length)
  let nextIndex = 0
  const workerCount = Math.min(Math.max(concurrency, 1), items.length)

  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (nextIndex < items.length) {
        const currentIndex = nextIndex
        nextIndex += 1
        results[currentIndex] = await mapper(items[currentIndex])
      }
    })
  )

  return results
}

const pushRequestedHistoryFields = async (
  request: ResolvedFieldDataRequest,
  historyRequests: Map<string, string | undefined>,
  fields: Record<string, unknown>,
  explicitHistoryFieldIds: string[],
  options: Pick<FieldDataBridgeOptions, 'historyFieldSuffix' | 'fetchTelemetryHistoryField' | 'pushPlatformFieldHistory'>
) => {
  if (historyRequests.size === 0 || !request.targetDeviceId) return

  // 历史数据除了用于即时响应，也会主动推回 widget 的平台历史缓存。
  const historyEntries = await mapWithConcurrency(
    Array.from(historyRequests.entries()),
    DEFAULT_HISTORY_FETCH_CONCURRENCY,
    async ([fieldId, timeRange]) => {
      const rows = await options.fetchTelemetryHistoryField(request.targetDeviceId!, fieldId, {
        ...(request.payload?.historyConfig || {}),
        timeRange: request.payload?.historyConfig?.timeRange || timeRange || 'last_30d'
      })
      return [fieldId, rows] as const
    }
  )

  historyEntries.forEach(([fieldId, rows]) => {
    if (rows.length > 0) {
      options.pushPlatformFieldHistory(fieldId, rows, request.targetDeviceId)
    }
    if (explicitHistoryFieldIds.includes(`${fieldId}${options.historyFieldSuffix}`)) {
      fields[`${fieldId}${options.historyFieldSuffix}`] = rows
    }
  })
}

export async function buildWidgetFieldDataResponseFields(
  request: ResolvedFieldDataRequest,
  options: FieldDataBridgeOptions
) {
  const fieldGroups = classifyRequestedFieldIds(request.fieldIds, options.historyFieldSuffix)
  const historyRequests = buildHistoryRequests(request.payload, fieldGroups, options)
  const fields = await buildRequestedCurrentFieldPayload(fieldGroups.currentFieldIds, request.targetDeviceId, options)

  await pushRequestedHistoryFields(
    request,
    historyRequests,
    fields,
    fieldGroups.explicitHistoryFieldIds,
    options
  )

  return fields
}
