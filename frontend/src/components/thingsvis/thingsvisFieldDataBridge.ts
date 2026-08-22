import { normalizeRequestedFieldIds, splitRequestedFieldIds as splitRequestedFieldIdsByKind } from './fieldReadBridge'
import { pickRequestedPlatformFields } from './thingsvisFieldHydrationBridge'

/** 告警历史行（后端返回，字段宽松，snake_case/camelCase 双写法兼容） */
type AlarmRowLike = {
  alarm_level?: unknown
  level?: unknown
  alarm_status?: unknown
  status?: unknown
  is_active?: unknown
  last_trigger_time?: unknown
  create_time?: unknown
  created_at?: unknown
  time?: unknown
  alarm_name?: unknown
  name?: unknown
  title?: unknown
  [key: string]: unknown
}

/** 接口响应的局部视图（分页列表 / 设备元信息两种形态的字段并集） */
type ListResponseLike = {
  data?:
    | ({
        list?: unknown[]
        total?: unknown
        data?: unknown[]
        device?: Record<string, unknown> | null
      } & Record<string, unknown>)
    | null
  device?: Record<string, unknown> | null
  pid_number?: unknown
  firmware_version?: unknown
  current_version?: unknown
  description?: unknown
  shared_status?: unknown
  total?: unknown
  [key: string]: unknown
}

function collectFieldRows(kvMap: Record<string, unknown>, rows: unknown) {
  if (!Array.isArray(rows)) return
  rows.forEach(item => {
    if (item?.key !== undefined) kvMap[item.key] = item.value
    if (item?.label) kvMap[item.label] = item.value
  })
}

function collectRdiDeviceMeta(kvMap: Record<string, unknown>, source: ListResponseLike | null) {
  const data = source?.data?.device || source?.data || source?.device || source
  if (!data || typeof data !== 'object') return
  if (data.pid_number !== undefined) kvMap.pid_number = data.pid_number
  if (data.firmware_version !== undefined || data.current_version !== undefined) {
    kvMap.firmware_version = data.firmware_version ?? data.current_version
  }
  if (data.description !== undefined) kvMap.description = data.description
  if (data.shared_status !== undefined) kvMap.shared_status = data.shared_status
}

function normalizeAlarmLevel(raw: unknown) {
  const value = String(raw ?? '')
    .toLowerCase()
    .trim()
  if (value === '1' || value === 'critical' || value === 'high' || value === 'serious') return 'critical'
  if (value === '2' || value === 'warning' || value === 'medium' || value === 'warn') return 'warning'
  if (value === '3' || value === 'info' || value === 'low') return 'info'
  return value || ''
}

function alarmLevelRank(row: AlarmRowLike | null) {
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

function normalizeAlarmTime(row: AlarmRowLike | null) {
  return row?.last_trigger_time ?? row?.create_time ?? row?.created_at ?? row?.time ?? ''
}

function isActiveAlarm(row: AlarmRowLike): boolean {
  const raw = String(row?.alarm_status ?? row?.status ?? row?.is_active ?? '').toLowerCase()
  return row?.is_active === true || raw === '1' || raw === 'active' || raw === 'triggered' || raw === 'a'
}

function normalizeAlarmStatusResponse(response: ListResponseLike | null): {
  payload: (ListResponseLike & { total?: unknown }) | null
  rows: AlarmRowLike[]
} {
  const payload = ((response?.data ?? response) ?? null) as (ListResponseLike & { total?: unknown }) | null
  const rows = Array.isArray(payload?.list)
    ? (payload.list as AlarmRowLike[])
    : Array.isArray(payload?.data)
      ? (payload.data as AlarmRowLike[])
      : Array.isArray(payload)
        ? (payload as AlarmRowLike[])
        : []
  return { payload, rows }
}

function pickHighestAlarmRow(activeRows: AlarmRowLike[]) {
  return activeRows.reduce<AlarmRowLike | null>(
    (current, row) => (!current || alarmLevelRank(row) > alarmLevelRank(current) ? row : current),
    null
  )
}

function buildAlarmStatusFieldMap(
  payload: (ListResponseLike & { total?: unknown }) | null,
  activeRows: AlarmRowLike[],
  latest: AlarmRowLike | null
): Record<string, unknown> {
  const highest = pickHighestAlarmRow(activeRows) ?? latest
  return {
    device_alarm_active: activeRows.length > 0 ? 1 : 0,
    device_alarm_count: Number(payload?.total ?? activeRows.length ?? 0),
    device_alarm_highest_level: normalizeAlarmLevel(highest?.alarm_level ?? highest?.level),
    latest_device_alarm_title: String(latest?.alarm_name ?? latest?.name ?? latest?.title ?? ''),
    latest_device_alarm_level: normalizeAlarmLevel(latest?.alarm_level ?? latest?.level),
    latest_device_alarm_time: latest ? normalizeAlarmTime(latest) : ''
  }
}

export async function loadCurrentFieldValueMap(options: {
  deviceId: string
  currentFieldIds: string[]
  rdiMetaFieldIds: Set<string>
  silentRequestConfig: unknown
  loadTelemetryCurrent: (deviceId: string, requestConfig: unknown) => Promise<ListResponseLike | null>
  loadAttributeDataSet: (params: { device_id: string }, requestConfig: unknown) => Promise<ListResponseLike | null>
  loadRdiDeviceConfig: (deviceId: string, requestConfig: unknown) => Promise<ListResponseLike | null>
}): Promise<Record<string, unknown>> {
  const shouldLoadRdiDeviceMeta = options.currentFieldIds.some((fieldId) => options.rdiMetaFieldIds.has(fieldId))
  const [telemetryResult, attributeResult, rdiDeviceResult] = await Promise.allSettled([
    options.loadTelemetryCurrent(options.deviceId, options.silentRequestConfig),
    options.loadAttributeDataSet({ device_id: options.deviceId }, options.silentRequestConfig),
    shouldLoadRdiDeviceMeta
      ? options.loadRdiDeviceConfig(options.deviceId, options.silentRequestConfig)
      : Promise.resolve(null)
  ])

  const telemetryRes = telemetryResult.status === 'fulfilled' ? telemetryResult.value : null
  const attributeRes = attributeResult.status === 'fulfilled' ? attributeResult.value : null
  const rdiDeviceRes = rdiDeviceResult.status === 'fulfilled' ? rdiDeviceResult.value : null
  const firstLoadError =
    telemetryResult.status === 'rejected'
      ? telemetryResult.reason
      : attributeResult.status === 'rejected'
        ? attributeResult.reason
        : rdiDeviceResult.status === 'rejected'
          ? rdiDeviceResult.reason
          : undefined

  if (firstLoadError) {
    throw firstLoadError
  }

  const kvMap: Record<string, unknown> = {}
  collectFieldRows(kvMap, telemetryRes?.data)
  collectFieldRows(kvMap, attributeRes?.data)
  collectRdiDeviceMeta(kvMap, rdiDeviceRes)
  return kvMap
}

export async function buildRequestedAlarmStatusData(options: {
  fieldIds: string[]
  deviceId?: string
  loadDeviceAlarmStatus: (params: { device_id: string; page: number; page_size: number }) => Promise<ListResponseLike | null>
  onError?: (deviceId: string, error: unknown) => void
}): Promise<Record<string, unknown>> {
  if (!options.deviceId || options.fieldIds.length === 0) return {}

  try {
    const response = await options.loadDeviceAlarmStatus({ device_id: options.deviceId, page: 1, page_size: 20 })
    const { payload, rows } = normalizeAlarmStatusResponse(response)
    const activeRows = rows.filter(isActiveAlarm)
    const latest = rows[0] ?? null
    const allFields = buildAlarmStatusFieldMap(payload, activeRows, latest)
    return pickRequestedPlatformFields(allFields, options.fieldIds)
  } catch (error) {
    options.onError?.(options.deviceId, error)
    return {}
  }
}

export async function buildRequestedFieldData(options: {
  fieldIds: unknown[]
  deviceId?: string
  alarmStatusFieldIds: Set<string>
  rdiMetaFieldIds: Set<string>
  historyFieldSuffix: string
  silentRequestConfig: unknown
  loadTelemetryCurrent: (deviceId: string, requestConfig: unknown) => Promise<ListResponseLike | null>
  loadAttributeDataSet: (params: { device_id: string }, requestConfig: unknown) => Promise<ListResponseLike | null>
  loadRdiDeviceConfig: (deviceId: string, requestConfig: unknown) => Promise<ListResponseLike | null>
  loadDeviceAlarmStatus: (params: { device_id: string; page: number; page_size: number }) => Promise<ListResponseLike | null>
  onAlarmError?: (deviceId: string, error: unknown) => void
}): Promise<Record<string, unknown>> {
  const requestedFields = normalizeRequestedFieldIds(options.fieldIds)
  if (!options.deviceId || requestedFields.length === 0) return {}

  const { alarmFieldIds, currentFieldIds } = splitRequestedFieldIdsByKind(requestedFields, {
    alarmStatusFieldIds: options.alarmStatusFieldIds,
    historyFieldSuffix: options.historyFieldSuffix
  })
  const alarmDataPromise =
    alarmFieldIds.length > 0
      ? buildRequestedAlarmStatusData({
        fieldIds: alarmFieldIds,
        deviceId: options.deviceId,
        loadDeviceAlarmStatus: options.loadDeviceAlarmStatus,
        onError: options.onAlarmError
      })
      : Promise.resolve({})
  const currentDataPromise =
    currentFieldIds.length > 0
      ? loadCurrentFieldValueMap({
          deviceId: options.deviceId,
          currentFieldIds,
          rdiMetaFieldIds: options.rdiMetaFieldIds,
          silentRequestConfig: options.silentRequestConfig,
          loadTelemetryCurrent: options.loadTelemetryCurrent,
          loadAttributeDataSet: options.loadAttributeDataSet,
          loadRdiDeviceConfig: options.loadRdiDeviceConfig
        }).then(kvMap => pickRequestedPlatformFields(kvMap, currentFieldIds))
      : Promise.resolve({})

  const [alarmData, currentData] = await Promise.all([alarmDataPromise, currentDataPromise])
  return Object.assign({}, alarmData, currentData)
}
