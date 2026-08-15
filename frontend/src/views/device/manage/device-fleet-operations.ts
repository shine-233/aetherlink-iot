import {
  buildFleetRolloutDeviceIds,
  buildFleetRolloutQuery,
  getFleetRolloutDeviceId
} from '../modules/fleet-rollout-context'

export type DeviceFleetOperationRow = Record<string, any>

export type FleetSelectionSummary = {
  total: number
  online: number
  offline: number
  alarmed: number
  missingVersion: number
}

export function getDeviceRowId(row: DeviceFleetOperationRow) {
  return getFleetRolloutDeviceId(row)
}

export function buildFleetDeviceIds(rows: DeviceFleetOperationRow[]) {
  return buildFleetRolloutDeviceIds(rows)
}

function csvCell(value: unknown) {
  const text = value === null || value === undefined ? '' : String(value)
  return `"${text.replace(/"/g, '""')}"`
}

export function buildFleetDeviceCsv(rows: DeviceFleetOperationRow[]) {
  const headers = [
    'id',
    'device_number',
    'name',
    'device_config_name',
    'is_online',
    'warn_status',
    'current_version',
    'pid_number',
    'firmware_version',
    'description'
  ]
  const csvRows = rows.map((row) => headers.map((header) => csvCell(row[header])).join(','))

  return [headers.join(','), ...csvRows].join('\r\n')
}

export function buildFleetSelectionSummary(rows: DeviceFleetOperationRow[]): FleetSelectionSummary {
  return {
    total: rows.length,
    online: rows.filter((row) => row?.is_online === 1).length,
    offline: rows.filter((row) => row?.is_online === 0).length,
    alarmed: rows.filter((row) => row?.warn_status === 'Y').length,
    missingVersion: rows.filter((row) => !String(row?.current_version || row?.firmware_version || '').trim()).length
  }
}

export function buildFleetSelectedDeviceIdentifiers(rows: DeviceFleetOperationRow[]) {
  return rows
    .map((row) => row?.device_number || row?.pid_number || row?.id)
    .filter((value): value is string => Boolean(value))
    .join('\n')
}

export function downloadFleetDeviceCsv(rows: DeviceFleetOperationRow[], filename = `fleet-devices-${Date.now()}.csv`) {
  const blob = new Blob(['\uFEFF', buildFleetDeviceCsv(rows)], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export function buildFleetContextQuery(
  rows: DeviceFleetOperationRow[],
  fallbackParams: Record<string, unknown>,
  requestedTotal?: number | null
) {
  return buildFleetRolloutQuery(rows, fallbackParams, undefined, requestedTotal)
}

export function cleanFleetFilterParams(params: Record<string, unknown>) {
  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => value !== null && value !== undefined && String(value).trim() !== '')
  )
}
