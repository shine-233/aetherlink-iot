import {
  buildSearchDevicesPagedResultPayload,
  type SearchDevicesPagedRequest
} from '@/components/thingsvis/searchDevicesPagedBridge'
import type {
  PlatformDeviceCatalogOrchestrator,
  PlatformDeviceEntry
} from '@/components/thingsvis/thingsvisPlatformDeviceCatalogOrchestrator'
import type { PlatformDeviceField } from '@/components/thingsvis/thingsvisDeviceWsBridge'
import {
  buildThingsVisHostDiagnostic,
  buildThingsVisHostErrorPayload,
  type ThingsVisHostDiagnostic
} from '@/components/thingsvis/thingsvisHostErrorPayload'

type DeviceHostLogger = {
  warn: (...args: any[]) => void
}

type DeviceHostBridgeOptions = {
  catalog: PlatformDeviceCatalogOrchestrator
  isFrameReady: () => boolean
  registerDevices: (devices: PlatformDeviceEntry[]) => void
  updateDeviceFields: (deviceId: string, fields: PlatformDeviceField[]) => void
  postToThingsVis: (type: string, payload: Record<string, unknown>) => void
  onDiagnostic?: (diagnostic: ThingsVisHostDiagnostic) => void
  onRecovered?: () => void
  logger: DeviceHostLogger
}

type SearchDevicesPagedResponse = {
  devices: PlatformDeviceEntry[]
  total: unknown
}

function getStringPayloadValue(payload: Record<string, unknown>, key: string, fallback = ''): string {
  const value = payload[key]
  return typeof value === 'string' ? value : fallback
}

export function createThingsVisPlatformDeviceHostBridge(options: DeviceHostBridgeOptions) {
  const {
    catalog,
    isFrameReady,
    registerDevices,
    updateDeviceFields,
    postToThingsVis,
    onDiagnostic,
    onRecovered,
    logger
  } = options

  function reportDiagnostic(scope: string, error: unknown) {
    onDiagnostic?.(buildThingsVisHostDiagnostic('device_bridge', scope, error))
  }

  async function requestDeviceGroups() {
    try {
      const groups = await catalog.loadGroups()
      onRecovered?.()
      postToThingsVis('tv:device-groups', { groups })
    } catch (error) {
      logger.warn('[AppFrame] Failed to load requested device groups:', error)
      reportDiagnostic('device_groups', error)
      postToThingsVis('tv:device-groups', {
        groups: [],
        ...buildThingsVisHostErrorPayload('device_groups', error)
      })
    }
  }

  async function requestDeviceFilterOptions(payload: Record<string, unknown>) {
    const reqId = getStringPayloadValue(payload, 'reqId')

    try {
      const options = await catalog.loadFilterOptions()
      postToThingsVis('tv:device-filter-options', {
        reqId,
        ...options
      })
      onRecovered?.()
    } catch (error) {
      logger.warn('[AppFrame] Failed to load requested device filter options:', error)
      reportDiagnostic('device_filter_options', error)
      postToThingsVis('tv:device-filter-options', {
        reqId,
        deviceConfigs: [],
        serviceOptions: [],
        ...buildThingsVisHostErrorPayload('device_filter_options', error)
      })
    }
  }

  async function requestDeviceById(payload: Record<string, unknown>) {
    const reqId = getStringPayloadValue(payload, 'reqId')
    const deviceId = getStringPayloadValue(payload, 'deviceId')
    if (!deviceId) return

    try {
      const device = await catalog.loadDeviceById(deviceId)
      if (device) registerDevices([device])
      postToThingsVis('tv:device-by-id', {
        reqId,
        deviceId,
        device
      })
      onRecovered?.()
    } catch (error) {
      logger.warn('[AppFrame] Failed to load requested device by id:', deviceId, error)
      reportDiagnostic('device_by_id', error)
      postToThingsVis('tv:device-by-id', {
        reqId,
        deviceId,
        device: null,
        ...buildThingsVisHostErrorPayload('device_by_id', error)
      })
    }
  }

  async function requestDevicesByGroup(payload: Record<string, unknown>) {
    const groupId = getStringPayloadValue(payload, 'groupId') || undefined
    if (!groupId) return

    try {
      const devices = await catalog.loadDevicesByGroup(groupId)
      registerDevices(devices)
      postToThingsVis('tv:devices-by-group', { groupId, devices })
      onRecovered?.()
    } catch (error) {
      logger.warn('[AppFrame] Failed to load requested device group:', groupId, error)
      reportDiagnostic('devices_by_group', error)
      postToThingsVis('tv:devices-by-group', {
        groupId,
        devices: [],
        ...buildThingsVisHostErrorPayload('devices_by_group', error)
      })
    }
  }

  function postSearchDevicesPagedResult(
    request: Pick<SearchDevicesPagedRequest, 'reqId' | 'page' | 'pageSize'>,
    devices: PlatformDeviceEntry[],
    total: unknown,
    extraPayload: Record<string, unknown> = {}
  ) {
    postToThingsVis('tv:search-devices-paged-result', {
      ...buildSearchDevicesPagedResultPayload(request, devices, total),
      ...extraPayload
    })
  }

  async function searchDevicesPaged(request: SearchDevicesPagedRequest): Promise<SearchDevicesPagedResponse> {
    const result = await catalog.searchDevicesPaged(request)
    const devices = result.devices
    registerDevices(devices)
    return result
  }

  async function requestSearchDevicesPaged(payload: Record<string, unknown>) {
    const request = catalog.normalizeSearchPayload(payload)

    try {
      const result = await searchDevicesPaged(request)
      postSearchDevicesPagedResult(request, result.devices, result.total)
      onRecovered?.()
    } catch (error) {
      logger.warn('[AppFrame] Failed to search devices:', error)
      reportDiagnostic('search_devices', error)
      postSearchDevicesPagedResult(request, [], 0, buildThingsVisHostErrorPayload('search_devices', error))
    }
  }

  async function requestDeviceFields(payload: Record<string, unknown>) {
    const deviceId = getStringPayloadValue(payload, 'deviceId') || undefined
    const directTemplateId = getStringPayloadValue(payload, 'templateId') || undefined
    const deviceConfigId = getStringPayloadValue(payload, 'deviceConfigId') || undefined
    if (!isFrameReady() || !deviceId) return

    try {
      const result = await catalog.loadDeviceFields({
        deviceId,
        templateId: directTemplateId,
        deviceConfigId
      })
      updateDeviceFields(deviceId, result.fields)
      postToThingsVis('tv:device-fields', result)
      onRecovered?.()
    } catch (error) {
      logger.warn('[AppFrame] Failed to load requested device fields:', deviceId, directTemplateId, error)
      reportDiagnostic('device_fields', error)
      postToThingsVis('tv:device-fields', {
        deviceId,
        templateId: directTemplateId,
        deviceConfigId,
        fields: [],
        ...buildThingsVisHostErrorPayload('device_fields', error)
      })
    }
  }

  return {
    requestDeviceGroups,
    requestDeviceFilterOptions,
    requestDeviceById,
    requestDevicesByGroup,
    searchDevicesPaged: requestSearchDevicesPaged,
    requestDeviceFields
  }
}
