import { firstString } from '@/components/thingsvis/thingsvisDeviceCatalogBridge'
import {
  collectPlatformSourceDescriptors,
  type PlatformSourceDescriptor
} from '@/components/thingsvis/thingsvisFieldHydrationBridge'
import type { PlatformDeviceEntry } from '@/components/thingsvis/thingsvisPlatformDeviceCatalogOrchestrator'
import type { PlatformDeviceField } from '@/components/thingsvis/thingsvisDeviceWsBridge'

const PLATFORM_DEVICE_DATA_SOURCE_ID_RE = /^__platform_(.+)__$/
const AETHERLINK_DATA_SOURCE_ID_RE = /^aetherlink_.+$/

type CurrentDashboardConfig = {
  id?: string
  name?: string
  canvas?: unknown
  nodes?: unknown[]
  dataSources?: unknown[]
  variables?: unknown[]
}

type PlatformSourceDeviceBinding = {
  dataSourceId?: string
  deviceId?: string
}

type PlatformSourceBindingInput = {
  dataSourceId?: unknown
  deviceId?: unknown
  configuredDeviceId?: unknown
  syncCurrentSchema?: boolean
}

function parsePlatformDeviceIdFromDataSourceId(dataSourceId: unknown): string | undefined {
  if (typeof dataSourceId !== 'string') return undefined
  const matched = PLATFORM_DEVICE_DATA_SOURCE_ID_RE.exec(dataSourceId)
  return firstString(matched?.[1])
}

export function buildPlatformSourceBindingOptions(source: PlatformSourceBindingInput) {
  return {
    dataSourceId: typeof source.dataSourceId === 'string' ? source.dataSourceId : undefined,
    deviceId: typeof source.deviceId === 'string' ? source.deviceId : undefined,
    configuredDeviceId: typeof source.configuredDeviceId === 'string' ? source.configuredDeviceId : undefined,
    syncCurrentSchema: source.syncCurrentSchema
  }
}

export function createThingsVisPlatformRuntimeBridge(options: {
  getCurrentDashboardConfig: () => CurrentDashboardConfig | null
}) {
  const activePlatformDevices = new Map<string, { deviceId: string; fields: PlatformDeviceField[] }>()
  const activePlatformDataSourceDeviceIds = new Map<string, string>()

  function registerDevices(devices: PlatformDeviceEntry[]) {
    devices.forEach((device) => {
      if (!device?.deviceId) return
      const existing = activePlatformDevices.get(device.deviceId)
      activePlatformDevices.set(device.deviceId, {
        deviceId: device.deviceId,
        fields: Array.isArray(existing?.fields) && existing.fields.length > 0 ? existing.fields : device.fields || []
      })
    })
  }

  function updateDeviceFields(deviceId: string, fields: PlatformDeviceField[]) {
    const normalizedFields = Array.isArray(fields) ? fields : []
    activePlatformDevices.set(deviceId, { deviceId, fields: normalizedFields })
    return normalizedFields
  }

  function ensureDevice(deviceId?: string) {
    if (!deviceId || activePlatformDevices.has(deviceId)) return
    activePlatformDevices.set(deviceId, { deviceId, fields: [] })
  }

  function getDevice(deviceId?: string) {
    return deviceId ? activePlatformDevices.get(deviceId) : undefined
  }

  function getDeviceFields(deviceId?: string): PlatformDeviceField[] {
    return getDevice(deviceId)?.fields || []
  }

  function findConfiguredDeviceId(dataSourceId: unknown): string | undefined {
    return typeof dataSourceId === 'string' ? activePlatformDataSourceDeviceIds.get(dataSourceId) : undefined
  }

  function syncDevicesFromCurrentSchema() {
    const dashboardData = options.getCurrentDashboardConfig()
    if (!dashboardData) return
    syncDevicesFromConfig(dashboardData)
  }

  function resolveDeviceIdFromDataSourceId(dataSourceId: unknown, syncCurrentSchema = true): string | undefined {
    if (syncCurrentSchema && typeof dataSourceId === 'string' && !activePlatformDataSourceDeviceIds.has(dataSourceId)) {
      syncDevicesFromCurrentSchema()
    }

    const configuredDeviceId = findConfiguredDeviceId(dataSourceId)
    if (configuredDeviceId) return configuredDeviceId
    if (typeof dataSourceId === 'string' && AETHERLINK_DATA_SOURCE_ID_RE.test(dataSourceId)) return undefined
    return parsePlatformDeviceIdFromDataSourceId(dataSourceId)
  }

  function resolveBinding(options: ReturnType<typeof buildPlatformSourceBindingOptions>): PlatformSourceDeviceBinding {
    const deviceId =
      options.deviceId ||
      options.configuredDeviceId ||
      resolveDeviceIdFromDataSourceId(options.dataSourceId, options.syncCurrentSchema)

    return {
      dataSourceId: options.dataSourceId,
      deviceId
    }
  }

  function resolveBindingFromPayload(source: PlatformSourceBindingInput): PlatformSourceDeviceBinding {
    return resolveBinding(buildPlatformSourceBindingOptions(source))
  }

  function collectConfiguredDescriptors(config: any): PlatformSourceDescriptor[] {
    return collectPlatformSourceDescriptors(config, {
      resolveDeviceId: (dataSourceId, configuredDeviceId) =>
        resolveBinding(
          buildPlatformSourceBindingOptions({
            dataSourceId,
            configuredDeviceId,
            syncCurrentSchema: false
          })
        ).deviceId
    })
  }

  function syncDevicesFromConfig(config: any) {
    activePlatformDevices.clear()
    activePlatformDataSourceDeviceIds.clear()

    collectConfiguredDescriptors(config).forEach((descriptor) => {
      if (descriptor.deviceId) {
        activePlatformDataSourceDeviceIds.set(descriptor.id, descriptor.deviceId)
      }
      if (!descriptor.deviceId || activePlatformDevices.has(descriptor.deviceId)) return

      activePlatformDevices.set(descriptor.deviceId, {
        deviceId: descriptor.deviceId,
        fields: []
      })
    })
  }

  function resolveRuntimeDeviceId(dashboardPayload: Record<string, unknown>, mode?: string) {
    syncDevicesFromConfig(dashboardPayload)
    if (mode !== 'viewer') return undefined
    return activePlatformDevices.size === 1 ? Array.from(activePlatformDevices.keys())[0] : undefined
  }

  function clearDevices() {
    activePlatformDevices.clear()
  }

  function clearDataSourceBindings() {
    activePlatformDataSourceDeviceIds.clear()
  }

  return {
    registerDevices,
    updateDeviceFields,
    ensureDevice,
    getDevice,
    getDeviceFields,
    resolveBindingFromPayload,
    collectConfiguredDescriptors,
    syncDevicesFromConfig,
    resolveRuntimeDeviceId,
    clearDevices,
    clearDataSourceBindings
  }
}
