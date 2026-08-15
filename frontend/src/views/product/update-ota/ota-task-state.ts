import type { SelectOption } from 'naive-ui'
import { getDeviceFilterLabel, getDeviceFilterValueLabel } from '@/views/device/shared/device-filter-summary-labels'

export interface OtaTaskFormState {
  name: string
  description: string
  device_id_list: string[]
}

export interface OtaTaskDeviceFilterPayload {
  device_filter?: OtaDeviceFilter
  exclude_device_id_list?: string[]
  expected_total?: number
  max_devices?: number
}

export type OtaDeviceFilter = Record<string, string | number | boolean>

export interface OtaDeviceCandidate {
  id?: string
  device_id?: string
  name?: string
  device_name?: string
  device_number?: string
  current_version?: string
  current_firmware_version?: string
  firmware_version?: string
  sw_version?: string
  version?: string
  is_online?: number | boolean
  online?: number | boolean
  status?: string | number
  activate_flag?: string
}

export interface OtaTaskPreflightSummary {
  eligible: number
  selected: number
  offline: number
  sameVersion: number
  missingVersion: number
  riskCount: number
}

export interface OtaTaskRiskDevice {
  id: string
  label: string
  currentVersion: string
  reasonKeys: string[]
}

export interface OtaFilterSummaryItem {
  key: string
  label: string
  value: string
}

export interface OtaPreviewDeviceRow {
  id: string
  label: string
  deviceNumber: string
  currentVersion: string
  online: string
}

export function extractList(payload: any): any[] {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.data?.list)) return payload.data.list
  if (Array.isArray(payload?.records)) return payload.records
  return []
}

export function extractTotal(payload: any) {
  return Number(payload?.total || payload?.data?.total || 0)
}

export function buildOtaDeviceOptions(rows: any[]): SelectOption[] {
  return rows
    .map((item: any) => ({
      label: item.name || item.device_name || item.device_number || item.id || item.device_id,
      value: item.id || item.device_id
    }))
    .filter((item: SelectOption) => Boolean(item.value))
}

export function mergeOtaDeviceCandidates(
  currentRows: OtaDeviceCandidate[],
  nextRows: OtaDeviceCandidate[]
): OtaDeviceCandidate[] {
  const merged = new Map<string, OtaDeviceCandidate>()

  ;[...currentRows, ...nextRows].forEach((item) => {
    const id = getOtaDeviceCandidateId(item)
    if (id) merged.set(id, item)
  })

  return Array.from(merged.values())
}

export function getOtaDeviceCandidateId(item: OtaDeviceCandidate) {
  return item.id || item.device_id || ''
}

export function getOtaDeviceCandidateLabel(item: OtaDeviceCandidate) {
  return item.name || item.device_name || item.device_number || item.id || item.device_id || ''
}

export function getOtaDeviceNumber(item: OtaDeviceCandidate) {
  return item.device_number || item.id || item.device_id || ''
}

export function getOtaDeviceCurrentVersion(item: OtaDeviceCandidate) {
  return (
    item.current_version ||
    item.current_firmware_version ||
    item.firmware_version ||
    item.sw_version ||
    item.version ||
    ''
  )
}

export function buildOtaTaskRiskDevices(
  rows: OtaDeviceCandidate[],
  selectedIds: string[],
  targetVersion?: string
): OtaTaskRiskDevice[] {
  const selected = new Set(selectedIds)
  const target = (targetVersion || '').trim()

  return rows
    .filter((item) => selected.has(getOtaDeviceCandidateId(item)))
    .map((item) => {
      const currentVersion = getOtaDeviceCurrentVersion(item).trim()
      const reasonKeys: string[] = []
      if (isOtaDeviceKnownOffline(item)) reasonKeys.push('page.product.update-ota.preflightReasonOffline')
      if (target && currentVersion === target) reasonKeys.push('page.product.update-ota.preflightReasonSameVersion')
      if (!currentVersion) reasonKeys.push('page.product.update-ota.preflightReasonMissingVersion')

      return {
        id: getOtaDeviceCandidateId(item),
        label: getOtaDeviceCandidateLabel(item),
        currentVersion,
        reasonKeys
      }
    })
    .filter((item) => item.reasonKeys.length > 0)
}

export function isOtaDeviceKnownOffline(item: OtaDeviceCandidate) {
  if (item.is_online === false || item.is_online === 0) return true
  if (item.online === false || item.online === 0) return true
  if (typeof item.status === 'string' && item.status.toLowerCase() === 'offline') return true
  if (typeof item.activate_flag === 'string' && item.activate_flag.toLowerCase().includes('offline')) return true
  return false
}

export function formatOtaOnlineState(item: OtaDeviceCandidate) {
  if (isOtaDeviceKnownOffline(item)) return '离线'
  if (item.is_online === true || item.is_online === 1 || item.online === true || item.online === 1) return '在线'
  return '未知'
}

export function buildOtaFilterSummaryItems(
  deviceFilter?: OtaDeviceFilter | null
): OtaFilterSummaryItem[] {
  if (!deviceFilter) return []

  return Object.entries(deviceFilter)
    .filter(([, value]) => value !== '' && value !== null && value !== undefined)
    .map(([key, value]) => ({
      key,
      label: getDeviceFilterLabel(key),
      value: getDeviceFilterValueLabel(key, value)
    }))
}

export function buildOtaPreviewDeviceRows(rows: OtaDeviceCandidate[] = [], limit = 8): OtaPreviewDeviceRow[] {
  return rows.slice(0, limit).map((item) => ({
    id: getOtaDeviceCandidateId(item),
    label: getOtaDeviceCandidateLabel(item),
    deviceNumber: getOtaDeviceNumber(item),
    currentVersion: getOtaDeviceCurrentVersion(item) || '--',
    online: formatOtaOnlineState(item)
  }))
}

export function buildOtaTaskPreflightSummary(
  rows: OtaDeviceCandidate[],
  selectedIds: string[],
  targetVersion?: string
): OtaTaskPreflightSummary {
  const rowById = new Map<string, OtaDeviceCandidate>()
  rows.forEach((item) => {
    const id = getOtaDeviceCandidateId(item)
    if (id) rowById.set(id, item)
  })
  const selectedRows = selectedIds.map((id) => rowById.get(id)).filter(Boolean) as OtaDeviceCandidate[]
  const target = (targetVersion || '').trim()
  const sameVersion = target
    ? selectedRows.filter((item) => getOtaDeviceCurrentVersion(item).trim() === target).length
    : 0
  const missingVersion = selectedRows.filter((item) => !getOtaDeviceCurrentVersion(item).trim()).length
  const offline = selectedRows.filter(isOtaDeviceKnownOffline).length

  return {
    eligible: rows.length,
    selected: selectedIds.length,
    offline,
    sameVersion,
    missingVersion,
    riskCount: offline + sameVersion + missingVersion
  }
}

export function hasOtaTaskDeviceFilter(deviceFilter?: OtaDeviceFilter | null) {
  return Boolean(deviceFilter && Object.keys(deviceFilter).length > 0)
}

export function canSaveOtaTask(
  selectedPackageId: string | null,
  form: OtaTaskFormState,
  deviceFilter?: OtaDeviceFilter | null
) {
  return Boolean(selectedPackageId && form.name.trim() && (form.device_id_list.length > 0 || hasOtaTaskDeviceFilter(deviceFilter)))
}

export function otaTaskSaveValidationKey(
  selectedPackageId: string | null,
  form: OtaTaskFormState,
  deviceFilter?: OtaDeviceFilter | null
) {
  if (!selectedPackageId) return 'page.product.update-package.packagePlaceholder'
  if (!form.name.trim()) return 'page.product.update-ota.taskNameRequired'
  if (form.device_id_list.length === 0 && !hasOtaTaskDeviceFilter(deviceFilter)) {
    return 'page.product.update-ota.selectDeviceRequired'
  }
  return ''
}

export function buildOtaTaskSavePayload(
  selectedPackageId: string,
  form: OtaTaskFormState,
  filterPayload: OtaTaskDeviceFilterPayload = {}
) {
  const basePayload = {
    name: form.name.trim(),
    ota_upgrade_package_id: selectedPackageId,
    description: form.description.trim() || undefined
  }

  if (hasOtaTaskDeviceFilter(filterPayload.device_filter)) {
    return {
      ...basePayload,
      device_filter: filterPayload.device_filter,
      exclude_device_id_list: filterPayload.exclude_device_id_list || [],
      expected_total: filterPayload.expected_total,
      max_devices: filterPayload.max_devices
    }
  }

  return {
    ...basePayload,
    device_id_list: form.device_id_list
  }
}

export function buildOtaTaskPreviewPayload(selectedPackageId: string, filterPayload: OtaTaskDeviceFilterPayload) {
  return {
    ota_upgrade_package_id: selectedPackageId,
    device_filter: filterPayload.device_filter || {},
    exclude_device_id_list: filterPayload.exclude_device_id_list || [],
    max_devices: filterPayload.max_devices
  }
}
