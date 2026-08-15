import { normalizeRequestedFieldIds, splitRequestedFieldIds as splitRequestedFieldIdsByKind } from './fieldReadBridge'
import { pickRequestedPlatformFields } from './thingsvisFieldHydrationBridge'

function collectFieldRows(kvMap: Record<string, unknown>, rows: unknown) {
  if (!Array.isArray(rows)) return
  rows.forEach((item: any) => {
    if (item?.key !== undefined) kvMap[item.key] = item.value
    if (item?.label) kvMap[item.label] = item.value
  })
}

function collectRdiDeviceMeta(kvMap: Record<string, unknown>, source: any) {
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

function alarmLevelRank(row: any) {
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

function normalizeAlarmTime(row: any) {
  return row?.last_trigger_time ?? row?.create_time ?? row?.created_at ?? row?.time ?? ''
}

function isActiveAlarm(row: any) {
  const raw = String(row?.alarm_status ?? row?.status ?? row?.is_active ?? '').toLowerCase()
  return row?.is_active === true || raw === '1' || raw === 'active' || raw === 'triggered' || raw === 'a'
}

function normalizeAlarmStatusResponse(response: any) {
  const payload = response?.data ?? response
  const rows = Array.isArray(payload?.list)
    ? payload.list
    : Array.isArray(payload?.data)
      ? payload.data
      : Array.isArray(payload)
        ? payload
        : []
  return { payload, rows }
}

function pickHighestAlarmRow(activeRows: any[]) {
  return activeRows.reduce<any | null>(
    (current, row) => (!current || alarmLevelRank(row) > alarmLevelRank(current) ? row : current),
    null
  )
}

function buildAlarmStatusFieldMap(payload: any, activeRows: any[], latest: any): Record<string, unknown> {
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
  loadTelemetryCurrent: (deviceId: string, requestConfig: unknown) => Promise<any>
  loadAttributeDataSet: (params: { device_id: string }, requestConfig: unknown) => Promise<any>
  loadRdiDeviceConfig: (deviceId: string, requestConfig: unknown) => Promise<any>
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
  loadDeviceAlarmStatus: (params: { device_id: string; page: number; page_size: number }) => Promise<any>
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
  loadTelemetryCurrent: (deviceId: string, requestConfig: unknown) => Promise<any>
  loadAttributeDataSet: (params: { device_id: string }, requestConfig: unknown) => Promise<any>
  loadRdiDeviceConfig: (deviceId: string, requestConfig: unknown) => Promise<any>
  loadDeviceAlarmStatus: (params: { device_id: string; page: number; page_size: number }) => Promise<any>
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
