/**
 * File purpose: pure state helpers for the RDI Overview device-card snapshot.
 * Inputs and outputs stay framework-agnostic so pagination, alarm state, and
 * system-info summary normalization can be reviewed and tested without page
 * lifecycle side effects. Keep network requests and reactive mutations in the
 * owning view rather than adding them here.
 */
export const RDI_SNAPSHOT_LIMIT = 12

export const RDI_SNAPSHOT_KEYS = [
  'temperature_1',
  'temperature_2',
  'switch_1',
  'switch_2',
  'dry_contact_output',
  'electricity_consumption'
]

export type TemperatureUnit = 'C' | 'F'

export interface AlarmRecord {
  id: string
  tenant_id?: string
  TenantID?: string
  name?: string
  content?: string
  description?: string
  alarm_status?: string
  alarm_level?: string
  create_at?: string
  alarm_device_list?: Array<{ id: string; name?: string }>
  remark?: string | Record<string, unknown>
}

export interface DeviceSnapshot {
  id: string
  name: string
  pid: string
  firmware: string
  online: boolean
  alarm: boolean | null
  // alarmLevel mirrors the backend warn_status (H/M/L/N). Empty when the
  // backend row does not carry a severity, so callers should still consult
  // `alarm` for the boolean intent.
  alarmLevel: string
  // groupId is the first device-group id the backend returned for the row.
  // It powers the client-side group filter without introducing an extra
  // relation query per snapshot; downstream code should treat it as an
  // opaque string and never assume a specific format.
  groupId: string
  serialNumber: string
  installLocation: string
  installAddress: string
  installDate: string
  installerName: string
  installerContact: string
  adminName: string
  tenantId: string
  telemetry: Record<string, unknown>
}

export interface SnapshotSystemInfo {
  present: boolean
  value: Record<string, unknown>
}

export interface RdiOverviewStats {
  totalDevices: number
  onlineDevices: number
  offlineDevices: number
  activeAlarms: number
}

export interface AlarmTrendPoint {
  month: number
  count: number
}

export type Translate = (key: string) => string

export function parseAlarmRemark(raw: unknown) {
  if (!raw) return {} as Record<string, unknown>
  if (typeof raw === 'object') return raw as Record<string, unknown>
  if (typeof raw !== 'string') return {} as Record<string, unknown>
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : {}
  } catch {
    return {} as Record<string, unknown>
  }
}

export function isAcknowledgedAlarm(row: AlarmRecord) {
  return parseAlarmRemark(row.remark).acknowledged === true
}

export function alarmStatusLabel(status: string | undefined, t: Translate) {
  const labels: Record<string, string> = {
    H: t('rdi.overview.high'),
    M: t('rdi.overview.medium'),
    L: t('rdi.overview.low'),
    N: t('rdi.overview.normal')
  }
  return labels[status || ''] || status || '-'
}

export function alarmTagType(status?: string) {
  if (status === 'H') return 'error'
  if (status === 'M') return 'warning'
  if (status === 'L') return 'info'
  return 'success'
}

export function alarmTypeLabel(row: AlarmRecord, t: Translate) {
  const remark = parseAlarmRemark(row.remark)
  const eventType = String(remark.event_type || '')
  const labels: Record<string, string> = {
    temperature_alarm: t('rdi.overview.temperatureAlarm'),
    switch_alarm: t('rdi.overview.switchAlarm'),
    warranty_alarm: t('rdi.overview.warrantyAlarm'),
    pressure_alarm: t('rdi.overview.pressureAlarm'),
    sw2_long_press: t('rdi.overview.sw2LongPress'),
    sw3_short_press: t('rdi.overview.sw3ShortPress'),
    sw3_long_press: t('rdi.overview.sw3LongPress'),
    PT: t('rdi.overview.pressureAlarm')
  }
  return labels[eventType] || row.name || row.content || '-'
}

export function normalizeDeviceRows(payload: any): Record<string, unknown>[] {
  const data = payload?.data ?? payload
  if (Array.isArray(data)) return data
  if (Array.isArray(data?.list)) return data.list
  if (Array.isArray(data?.data?.list)) return data.data.list
  return []
}

export function normalizeTelemetry(payload: unknown) {
  const data = (payload as any)?.data ?? payload
  const list = Array.isArray(data) ? data : Array.isArray((data as any)?.list) ? (data as any).list : null
  const result: Record<string, unknown> = {}
  if (list) {
    list.forEach((item: any) => {
      const key = String(item?.key || '')
      if (key) result[key] = item?.value ?? item?.number_v ?? item?.string_v ?? item?.bool_v
    })
    return result
  }
  return data && typeof data === 'object' ? (data as Record<string, unknown>) : result
}

export function rowText(row: Record<string, unknown>, keys: string[], fallback = '--') {
  for (const key of keys) {
    const value = row[key]
    if (value !== undefined && value !== null && value !== '') return String(value)
  }
  return fallback
}

function recordValue(value: unknown): Record<string, unknown> {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>
  if (typeof value !== 'string') return {}
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

export function snapshotSystemInfo(row: Record<string, unknown>): SnapshotSystemInfo {
  const additionalInfo = recordValue(row.additional_info)
  const hasListSummary = Object.prototype.hasOwnProperty.call(row, 'rdi_system_info_summary')
  const sources = [
    additionalInfo.system_info,
    additionalInfo.rdi_system_info,
    row.system_info,
    row.rdi_system_info,
    row.rdi_system_info_summary
  ]
  const present = hasListSummary || sources.some((source) => Object.keys(recordValue(source)).length > 0)
  const value = sources.reduce<Record<string, unknown>>((result, source) => {
    return { ...result, ...recordValue(source) }
  }, {})
  return {
    present,
    value: { ...recordValue(value.extra_fields), ...value }
  }
}

export function isRowOnline(row: Record<string, unknown>) {
  const value = row.is_online ?? row.online ?? row.status
  return value === true || value === 1 || value === '1' || value === 'online'
}

export function formatTemperature(value: unknown, unit: TemperatureUnit = 'C') {
  if (value === null || value === undefined || value === '') return '--'
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '--'
  const converted = unit === 'F' ? (numeric * 9) / 5 + 32 : numeric
  return `${converted.toFixed(2)} ${unit}`
}

export function formatSwitch(value: unknown, t: Translate) {
  if (value === true || value === 1 || value === '1' || value === 'true' || value === 'high') {
    return t('rdi.overview.high')
  }
  if (value === false || value === 0 || value === '0' || value === 'false' || value === 'low') {
    return t('rdi.overview.low')
  }
  return value === undefined || value === null || value === '' ? '--' : String(value)
}

export function buildOperationsFocus(stats: RdiOverviewStats, alarmDeviceTotal: number) {
  if (stats.activeAlarms > 0) {
    return {
      type: 'error',
      titleKey: 'rdi.overview.focusAlarmTitle',
      descKey: 'rdi.overview.focusAlarmDesc',
      tags: [
        { labelKey: 'rdi.overview.activeAlarms', value: stats.activeAlarms, type: 'error' },
        { labelKey: 'rdi.overview.alarmDevices', value: alarmDeviceTotal, type: 'warning' }
      ]
    }
  }

  if (stats.offlineDevices > 0) {
    return {
      type: 'warning',
      titleKey: 'rdi.overview.focusOfflineTitle',
      descKey: 'rdi.overview.focusOfflineDesc',
      tags: [
        { labelKey: 'rdi.overview.offline', value: stats.offlineDevices, type: 'warning' },
        { labelKey: 'rdi.overview.online', value: stats.onlineDevices, type: 'success' }
      ]
    }
  }

  return {
    type: 'success',
    titleKey: 'rdi.overview.focusNormalTitle',
    descKey: 'rdi.overview.focusNormalDesc',
    tags: [
      { labelKey: 'rdi.overview.online', value: stats.onlineDevices, type: 'success' },
      { labelKey: 'rdi.overview.devices', value: stats.totalDevices, type: 'info' }
    ]
  }
}

export function normalizeAlarmMonthlyTrendPoints(rows: Array<Partial<AlarmTrendPoint>>): AlarmTrendPoint[] {
  const counts = new Map<number, number>()
  rows.forEach((row) => {
    const month = Number(row.month)
    const count = Number(row.count)
    if (!Number.isInteger(month) || month < 1 || month > 12 || counts.has(month)) return
    counts.set(month, Number.isFinite(count) && count > 0 ? count : 0)
  })
  return Array.from({ length: 12 }, (_, index) => ({
    month: index + 1,
    count: counts.get(index + 1) || 0
  }))
}
