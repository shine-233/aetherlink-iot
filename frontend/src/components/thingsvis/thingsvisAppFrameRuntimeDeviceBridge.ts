import { createThingsVisDeviceWsBridge, type PlatformDeviceField } from './thingsvisDeviceWsBridge'
import { createThingsVisPlatformRuntimeBridge } from './thingsvisPlatformRuntimeBridge'

type CurrentDashboardConfig = Parameters<typeof createThingsVisPlatformRuntimeBridge>[0]['getCurrentDashboardConfig']

function resolvePlatformBufferSize(dataSources: unknown): number {
  if (!Array.isArray(dataSources)) return 0
  return Math.max(
    0,
    ...dataSources.map((dataSource: any) => {
      const normalizedType = typeof dataSource?.type === 'string' ? dataSource.type.toUpperCase() : ''
      if (normalizedType !== 'PLATFORM_FIELD' && normalizedType !== 'PLATFORM') return 0
      const bufferSize = dataSource?.config?.bufferSize
      return typeof bufferSize === 'number' && Number.isFinite(bufferSize) ? Math.max(0, Math.trunc(bufferSize)) : 0
    })
  )
}

export function createThingsVisAppFrameRuntimeDeviceBridge(options: {
  getCurrentDashboardConfig: CurrentDashboardConfig
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
}) {
  const deviceWsBridge = createThingsVisDeviceWsBridge({
    postPlatformData: options.postPlatformData
  })
  const platformRuntimeBridge = createThingsVisPlatformRuntimeBridge({
    getCurrentDashboardConfig: options.getCurrentDashboardConfig
  })

  function ensureDeviceWs(deviceId?: string) {
    if (!deviceId) return
    const device = platformRuntimeBridge.getDevice(deviceId)
    if (!device) return
    deviceWsBridge.ensureTelemetry(device)
  }

  function ensureDeviceStatusWs(deviceId?: string) {
    deviceWsBridge.ensureStatus(deviceId)
  }

  function updateDeviceFields(deviceId: string, fields: PlatformDeviceField[]) {
    const normalizedFields = platformRuntimeBridge.updateDeviceFields(deviceId, fields)
    deviceWsBridge.updateDeviceFields(deviceId, normalizedFields)
  }

  return {
    registerDevices: platformRuntimeBridge.registerDevices,
    ensureDevice: platformRuntimeBridge.ensureDevice,
    ensureDeviceWs,
    ensureDeviceStatusWs,
    updateDeviceFields,
    resolveBindingFromPayload: platformRuntimeBridge.resolveBindingFromPayload,
    getDeviceFields: platformRuntimeBridge.getDeviceFields,
    collectConfiguredDescriptors: platformRuntimeBridge.collectConfiguredDescriptors,
    resolveRuntimeDeviceId: platformRuntimeBridge.resolveRuntimeDeviceId,
    clearDevices: platformRuntimeBridge.clearDevices,
    clearDataSourceBindings: platformRuntimeBridge.clearDataSourceBindings,
    resolvePlatformBufferSize,
    disconnectAllDeviceWs: deviceWsBridge.disconnectAll
  }
}
