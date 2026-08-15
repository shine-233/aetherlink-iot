import { ref, toValue, type MaybeRefOrGetter } from 'vue'
import { deviceAlarmStatus, deviceList, telemetryDataCurrentKeys } from '@/service/api/device'
import { rdiDeviceConfig } from '@/service/api/rdi'
import {
  RDI_SNAPSHOT_KEYS,
  RDI_SNAPSHOT_LIMIT,
  isRowOnline,
  normalizeDeviceRows,
  normalizeTelemetry,
  rowText,
  snapshotSystemInfo,
  type DeviceSnapshot
} from './rdiOverviewState'

const RDI_SNAPSHOT_TELEMETRY_CONCURRENCY = 3
const RDI_SNAPSHOT_IDLE_DELAY_MS = 350

export async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  mapper: (item: T, index: number) => Promise<R>
): Promise<R[]> {
  const results = new Array<R>(items.length)
  let nextIndex = 0
  const workerCount = Math.min(Math.max(concurrency, 1), items.length)

  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (nextIndex < items.length) {
        const currentIndex = nextIndex
        nextIndex += 1
        results[currentIndex] = await mapper(items[currentIndex], currentIndex)
      }
    })
  )
  return results
}

export function useRdiDeviceSnapshots(options: {
  activeSystemsOnly: MaybeRefOrGetter<boolean>
  isMasterAccount: MaybeRefOrGetter<boolean>
}) {
  const snapshotLoading = ref(false)
  const deviceSnapshots = ref<DeviceSnapshot[]>([])
  const snapshotPage = ref(1)
  const snapshotTotal = ref(0)
  let snapshotRequestSeq = 0
  let snapshotRefreshTimer: number | undefined
  let snapshotIdleCallbackId: number | undefined

  async function buildDeviceSnapshot(row: Record<string, unknown>): Promise<DeviceSnapshot> {
    const id = rowText(row, ['id', 'device_id'], '')
    const listSystemInfo = snapshotSystemInfo(row)
    const isRdiDevice = rowText(row, ['pid_number', 'PIDNumber'], '') !== ''
    let telemetry: Record<string, unknown> = {}
    let alarm: boolean | null = null
    let systemInfo = listSystemInfo.value
    if (id) {
      const [telemetryRes, alarmRes, configRes] = await Promise.all([
        telemetryDataCurrentKeys({ device_id: id, keys: RDI_SNAPSHOT_KEYS }).catch(() => null),
        deviceAlarmStatus({ device_id: id }).catch(() => null),
        // Older APIs may omit the opt-in list summary. Hydrate only in that
        // compatibility case; an explicitly returned empty summary is authoritative.
        isRdiDevice && !listSystemInfo.present ? rdiDeviceConfig(id).catch(() => null) : Promise.resolve(null)
      ])
      if (telemetryRes) {
        telemetry = normalizeTelemetry(telemetryRes)
      }
      const alarmValue = alarmRes?.data?.alarm ?? alarmRes?.data?.data?.alarm
      if (typeof alarmValue === 'boolean') alarm = alarmValue
      if (configRes && !configRes.error && configRes.data?.system_info) {
        systemInfo = snapshotSystemInfo({ system_info: configRes.data.system_info }).value
      }
    }
    const rowSerialNumber = rowText(row, ['serial_number', 'device_serial_number', 'rdi_serial_number', 'SerialNumber'])
    const rowInstallLocation = rowText(row, ['install_location', 'installation_location', 'location', 'address'])
    const rowInstallAddress = rowText(row, ['install_address', 'installation_address', 'device_address', 'address'])
    const rowInstallDate = rowText(row, ['install_date', 'installation_date', 'installed_at', 'InstallDate'])
    const rowInstallerName = rowText(row, ['installer_name', 'service_technician', 'technician_name', 'maintainer_name'])
    const rowInstallerContact = rowText(row, [
      'installer_contact',
      'installer_phone',
      'technician_phone',
      'technician_email'
    ])
    const installerContact = [rowText(systemInfo, ['installer_phone'], ''), rowText(systemInfo, ['installer_email'], '')]
      .filter(Boolean)
      .join(' · ')
    const alarmLevelRaw = rowText(row, ['alarm_level', 'AlarmLevel', 'warn_status', 'WarnStatus'], '')
    const alarmLevel = alarmLevelRaw && alarmLevelRaw !== '--' ? alarmLevelRaw : ''
    const groupIdRaw = rowText(row, ['group_id', 'GroupId', 'group_ids', 'GroupIds'], '')
    const groupId = groupIdRaw && groupIdRaw !== '--' ? groupIdRaw.split(',')[0].trim() : ''
    return {
      id,
      name: rowText(row, ['name', 'device_name', 'DeviceName'], id || '--'),
      pid: rowText(row, ['pid_number', 'device_number', 'DeviceNumber']),
      firmware: rowText(row, ['firmware_version', 'current_version', 'CurrentVersion']),
      online: isRowOnline(row),
      alarm,
      alarmLevel,
      groupId,
      serialNumber: rowText(systemInfo, ['controller_serial_number', 'rdi_serial_number'], rowSerialNumber),
      installLocation: rowText(systemInfo, ['installation_location'], rowInstallLocation),
      installAddress: rowText(systemInfo, ['address', 'installation_address'], rowInstallAddress),
      installDate: rowText(systemInfo, ['installation_date'], rowInstallDate),
      installerName: rowText(
        systemInfo,
        ['installer_name', 'installer_company', 'maintenance_technician', 'installer_contact'],
        rowInstallerName
      ),
      installerContact: installerContact || rowText(systemInfo, ['installer_contact'], rowInstallerContact),
      adminName: rowText(
        systemInfo,
        ['customer_name', 'admin_name', 'administrator'],
        rowText(row, ['admin_name', 'owner_name', 'user_name', 'UserName'])
      ),
      tenantId: rowText(row, ['scope_tenant_id', 'tenant_id', 'TenantID']),
      telemetry
    }
  }

  async function fetchDeviceSnapshots(requestSeq = ++snapshotRequestSeq) {
    snapshotLoading.value = true
    try {
      const res = await deviceList({
        page: snapshotPage.value,
        page_size: RDI_SNAPSHOT_LIMIT,
        include_rdi_system_info_summary: true,
        ...(toValue(options.activeSystemsOnly) ? { warn_status: 'Y' } : {}),
        ...(toValue(options.isMasterAccount) ? { all_tenants: true } : {})
      })
      const rows = normalizeDeviceRows(res).slice(0, RDI_SNAPSHOT_LIMIT)
      const snapshots = await mapWithConcurrency(rows, RDI_SNAPSHOT_TELEMETRY_CONCURRENCY, buildDeviceSnapshot)
      if (requestSeq === snapshotRequestSeq) {
        deviceSnapshots.value = snapshots.filter((item) => item.id)
        const payload = (res as any)?.data ?? res
        snapshotTotal.value = Number(payload?.total ?? payload?.data?.total ?? rows.length)
      }
    } finally {
      if (requestSeq === snapshotRequestSeq) {
        snapshotLoading.value = false
      }
    }
  }

  function changeSnapshotPage(page: number) {
    snapshotPage.value = page
    void fetchDeviceSnapshots()
  }

  function cancelScheduledDeviceSnapshots() {
    if (snapshotRefreshTimer !== undefined) {
      window.clearTimeout(snapshotRefreshTimer)
      snapshotRefreshTimer = undefined
    }
    if (snapshotIdleCallbackId !== undefined) {
      const cancelIdleCallback = (window as any).cancelIdleCallback
      if (typeof cancelIdleCallback === 'function') {
        cancelIdleCallback(snapshotIdleCallbackId)
      }
      snapshotIdleCallbackId = undefined
    }
  }

  function scheduleDeviceSnapshotsRefresh() {
    const requestSeq = ++snapshotRequestSeq
    snapshotLoading.value = true
    cancelScheduledDeviceSnapshots()
    const run = () => {
      snapshotRefreshTimer = undefined
      snapshotIdleCallbackId = undefined
      void fetchDeviceSnapshots(requestSeq).catch(() => {
        if (requestSeq === snapshotRequestSeq) {
          snapshotLoading.value = false
        }
      })
    }
    const requestIdleCallback = (window as any).requestIdleCallback
    if (typeof requestIdleCallback === 'function') {
      snapshotIdleCallbackId = requestIdleCallback(run, { timeout: 1200 })
      return
    }
    snapshotRefreshTimer = window.setTimeout(run, RDI_SNAPSHOT_IDLE_DELAY_MS)
  }

  function dispose() {
    cancelScheduledDeviceSnapshots()
    snapshotRequestSeq += 1
  }

  return {
    snapshotLoading,
    deviceSnapshots,
    snapshotPage,
    snapshotTotal,
    mapWithConcurrency,
    buildDeviceSnapshot,
    fetchDeviceSnapshots,
    changeSnapshotPage,
    scheduleDeviceSnapshotsRefresh,
    cancelScheduledDeviceSnapshots,
    dispose
  }
}
