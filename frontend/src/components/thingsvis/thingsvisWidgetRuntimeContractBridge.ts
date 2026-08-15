import {
  THINGSVIS_COMPAT_PROVIDER,
  getPlatformApiBase,
  getThingsVisApiBase
} from '@/utils/thingsvis/constants'
import { localStg } from '@/utils/storage'
import { THINGSVIS_WIDGET_RUNTIME_CAPABILITIES } from './thingsvisWidgetRuntimeCapabilities'

export const THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID = '__template__'

type ThingsVisWidgetMode = 'viewer' | 'editor'
type ThingsVisEmbeddedContext = 'device-template' | 'current-device' | 'dashboard'

type ThingsVisWidgetRuntimeContractOptions = {
  getDeviceId: () => string | undefined
  getMode: () => ThingsVisWidgetMode | undefined
  getBufferSize: () => number | undefined
  getPlatformFields: () => any[] | undefined
  getPlatformDevices: () => any[] | undefined
  cloneValue: <T>(value: T) => T
}

export function createThingsVisWidgetRuntimeContract(options: ThingsVisWidgetRuntimeContractOptions) {
  const getPreviewDeviceId = () => {
    const deviceId = options.getDeviceId()
    if (typeof deviceId === 'string' && deviceId.trim()) return deviceId

    const platformDevices = options.getPlatformDevices()
    if (platformDevices?.length === 1) {
      const platformDeviceId = platformDevices[0]?.deviceId
      if (typeof platformDeviceId === 'string' && platformDeviceId.trim()) return platformDeviceId
    }

    return undefined
  }

  const getEmbeddedContext = (): ThingsVisEmbeddedContext => {
    if (options.getDeviceId() === THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID) return 'device-template'
    if (getPreviewDeviceId()) return 'current-device'
    return 'dashboard'
  }

  const buildWidgetUrl = (baseUrl: string, token: string) => {
    const route = options.getMode() === 'editor' ? 'editor' : 'embed'
    const tokenParams = token ? `&token=${encodeURIComponent(token)}` : ''
    const embeddedContext = encodeURIComponent(getEmbeddedContext())
    const thingsvisApiBaseUrl = encodeURIComponent(getThingsVisApiBase())
    const platformApiBaseUrl = encodeURIComponent(getPlatformApiBase())

    // saveTarget=host and provider are fixed host/guest runtime handshake params.
    return `${baseUrl}#/${route}?mode=embedded&provider=${THINGSVIS_COMPAT_PROVIDER}&saveTarget=host${tokenParams}&context=${embeddedContext}&thingsvisApiBaseUrl=${thingsvisApiBaseUrl}&platformApiBaseUrl=${platformApiBaseUrl}`
  }

  const getPlatformDevices = () => {
    const platformDevices = options.getPlatformDevices()
    if (Array.isArray(platformDevices) && platformDevices.length > 0) return options.cloneValue(platformDevices)
    if (getEmbeddedContext() !== 'device-template') return []

    return [
      {
        deviceId: THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID,
        deviceName: 'thing-model-fields',
        groupId: THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID,
        groupName: 'thing-model-fields',
        fields: options.cloneValue(options.getPlatformFields() || [])
      }
    ]
  }

  const getPlatformFieldStringMap = (property: 'dataType' | 'type') => {
    return (options.getPlatformFields() || []).reduce<Record<string, string>>((acc, field) => {
      const fieldId = typeof field?.id === 'string' ? field.id : ''
      if (fieldId) {
        acc[fieldId] = typeof field?.[property] === 'string' ? field[property] : ''
      }
      return acc
    }, {})
  }

  const getFieldValueTypeMap = () => getPlatformFieldStringMap('type')
  const getFieldDataTypeMap = () => getPlatformFieldStringMap('dataType')

  const getLoadOptions = () => ({
    platformBufferSize: options.getBufferSize() ?? 0,
    platformDevices: getPlatformDevices(),
    deviceId: options.getDeviceId() || getPreviewDeviceId(),
    thingsvisApiBaseUrl: getThingsVisApiBase(),
    platformApiBaseUrl: getPlatformApiBase(),
    platformToken: localStg.get('token') as string | undefined,
    runtimeCapabilities: options.cloneValue(THINGSVIS_WIDGET_RUNTIME_CAPABILITIES)
  })

  return {
    getPreviewDeviceId,
    getEmbeddedContext,
    buildWidgetUrl,
    getPlatformDevices,
    getPlatformFieldStringMap,
    getFieldValueTypeMap,
    getFieldDataTypeMap,
    getLoadOptions
  }
}
