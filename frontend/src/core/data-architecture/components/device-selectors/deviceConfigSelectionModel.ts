import type { DeviceInfo, DeviceMetric } from '@/core/data-architecture/types/device-parameter-group'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'

export interface DeviceConfig {
  selectedDevice: DeviceInfo | null
  includeDeviceId: boolean
  includeMetric: boolean
  selectedMetric: DeviceMetric | null
  includeLocation: boolean
  includeStatus: boolean
}

export type DeviceParameterKey = 'deviceId' | 'metric' | 'deviceLocation' | 'deviceStatus'
type IncludeFlagKey = 'includeDeviceId' | 'includeMetric' | 'includeLocation' | 'includeStatus'

export interface PreviewParameter {
  key: DeviceParameterKey
  value: string
  type: string
}

interface DeviceParameterDefinition {
  key: DeviceParameterKey
  includeKey: IncludeFlagKey
  previewType: string
  idPrefix: string
  isReady: (state: DeviceConfig) => boolean
  resolveValue: (state: DeviceConfig) => string
  resolveDescription: (state: DeviceConfig) => string
  resolveDeviceContext?: (state: DeviceConfig) => EnhancedParameter['deviceContext']
}

export interface RestoredDeviceConfig {
  deviceId: string
  metricKey: string
  existingDevice?: DeviceInfo
  existingMetric?: DeviceMetric
  includeDeviceId: boolean
  includeMetric: boolean
  includeLocation: boolean
  includeStatus: boolean
}

interface ExistingSelectionConfig {
  selectedDevice?: DeviceInfo
  selectedMetric?: DeviceMetric
}

export const createDefaultDeviceConfig = (): DeviceConfig => ({
  selectedDevice: null,
  includeDeviceId: false,
  includeMetric: false,
  selectedMetric: null,
  includeLocation: false,
  includeStatus: false
})

const createDeviceContext = (
  state: DeviceConfig,
  selectedMetric?: DeviceMetric | null
): EnhancedParameter['deviceContext'] => {
  if (!state.selectedDevice) return undefined

  return {
    sourceType: 'device-selection',
    selectionConfig: {
      selectedDevice: state.selectedDevice,
      ...(selectedMetric ? { selectedMetric } : {})
    },
    timestamp: Date.now()
  }
}

const deviceParameterDefinitions: DeviceParameterDefinition[] = [
  {
    key: 'deviceId',
    includeKey: 'includeDeviceId',
    previewType: '设备ID',
    idPrefix: 'param_device_id',
    isReady: (state) => Boolean(state.selectedDevice),
    resolveValue: (state) => state.selectedDevice?.deviceId ?? '',
    resolveDescription: (state) => `设备ID: ${state.selectedDevice?.deviceName ?? ''}`,
    resolveDeviceContext: (state) => createDeviceContext(state)
  },
  {
    key: 'metric',
    includeKey: 'includeMetric',
    previewType: '指标键',
    idPrefix: 'param_metric',
    isReady: (state) => Boolean(state.selectedMetric),
    resolveValue: (state) => state.selectedMetric?.metricKey ?? '',
    resolveDescription: (state) => `指标: ${state.selectedMetric?.metricLabel ?? ''}`,
    resolveDeviceContext: (state) => createDeviceContext(state, state.selectedMetric)
  }
]

const getReadyParameterDefinitions = (state: DeviceConfig) => {
  if (!state.selectedDevice) return []

  return deviceParameterDefinitions.filter((definition) => state[definition.includeKey] && definition.isReady(state))
}

const createParameterId = (prefix: string) => `${prefix}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`

const createEnhancedParameter = (definition: DeviceParameterDefinition, state: DeviceConfig): EnhancedParameter => {
  const deviceContext = definition.resolveDeviceContext?.(state)

  return {
    key: definition.key,
    value: definition.resolveValue(state),
    enabled: true,
    valueMode: 'manual',
    dataType: 'string',
    _id: createParameterId(definition.idPrefix),
    description: definition.resolveDescription(state),
    ...(deviceContext ? { deviceContext } : {})
  }
}

export const buildPreviewParameters = (state: DeviceConfig): PreviewParameter[] =>
  getReadyParameterDefinitions(state).map((definition) => ({
    key: definition.key,
    value: definition.resolveValue(state),
    type: definition.previewType
  }))

export const canGenerateDeviceParameters = (state: DeviceConfig) => getReadyParameterDefinitions(state).length > 0

export const generateDeviceParameters = (state: DeviceConfig): EnhancedParameter[] =>
  getReadyParameterDefinitions(state).map((definition) => createEnhancedParameter(definition, state))

const getExistingSelectionConfig = (param: EnhancedParameter): ExistingSelectionConfig | undefined => {
  return param.deviceContext?.selectionConfig as ExistingSelectionConfig | undefined
}

const createEmptyRestoredConfig = (): RestoredDeviceConfig => ({
  deviceId: '',
  metricKey: '',
  includeDeviceId: false,
  includeMetric: false,
  includeLocation: false,
  includeStatus: false
})

export const readExistingDeviceConfig = (parameters: EnhancedParameter[]): RestoredDeviceConfig => {
  const restoredConfig = createEmptyRestoredConfig()

  for (const param of parameters) {
    const selectionConfig = getExistingSelectionConfig(param)

    if (param.key === 'deviceId') {
      restoredConfig.deviceId = param.value
      restoredConfig.includeDeviceId = true
      restoredConfig.existingDevice = selectionConfig?.selectedDevice
    }
    if (param.key === 'metric') {
      restoredConfig.metricKey = param.value
      restoredConfig.includeMetric = true
      restoredConfig.existingMetric = selectionConfig?.selectedMetric
    }
    if (param.key === 'deviceLocation') {
      restoredConfig.includeLocation = true
    }
    if (param.key === 'deviceStatus') {
      restoredConfig.includeStatus = true
    }
  }

  return restoredConfig
}

const preservedDeviceFromId = (deviceId: string): DeviceInfo => ({
  deviceId,
  deviceName: deviceId,
  deviceType: ''
})

const preservedMetricFromKey = (metricKey: string): DeviceMetric => ({
  metricKey,
  metricLabel: metricKey,
  metricType: 'string'
})

export interface RestoredDeviceOptions {
  device?: DeviceInfo
  metric?: DeviceMetric
}

export interface DeviceSelectionChange {
  shouldClearMetricOptions: boolean
  deviceIdToLoad?: string
}

export const mergeSelectionOptions = <T>(
  currentOptions: T[],
  incomingOptions: T[],
  selectedOption: T | null,
  getKey: (item: T) => string
) => {
  const byKey = new Map<string, T>()

  for (const option of currentOptions) byKey.set(getKey(option), option)
  for (const option of incomingOptions) byKey.set(getKey(option), option)
  if (selectedOption) {
    const selectedKey = getKey(selectedOption)
    byKey.set(selectedKey, byKey.get(selectedKey) ?? selectedOption)
  }

  return Array.from(byKey.values())
}

export const selectDeviceById = (state: DeviceConfig, devices: DeviceInfo[], deviceId: string | null) => {
  state.selectedDevice = devices.find((device) => device.deviceId === deviceId) ?? null
}

export const selectMetricByKey = (state: DeviceConfig, metrics: DeviceMetric[], metricKey: string | null) => {
  state.selectedMetric = metrics.find((metric) => metric.metricKey === metricKey) ?? null
}

export const clearSelectedMetric = (state: DeviceConfig) => {
  state.selectedMetric = null
}

export const reconcileDeviceSelectionChange = (
  state: DeviceConfig,
  newDeviceId: string | undefined,
  oldDeviceId: string | undefined
): DeviceSelectionChange => {
  const shouldClearMetricOptions = newDeviceId !== oldDeviceId && oldDeviceId !== undefined

  if (shouldClearMetricOptions) {
    clearSelectedMetric(state)
  }

  return {
    shouldClearMetricOptions,
    ...(newDeviceId ? { deviceIdToLoad: newDeviceId } : {})
  }
}

export const reconcileMetricInclusion = (state: DeviceConfig, includeMetric: boolean) => {
  if (!includeMetric) {
    clearSelectedMetric(state)
  }
}

export const applyRestoredDeviceConfig = (
  state: DeviceConfig,
  restoredConfig: RestoredDeviceConfig
): RestoredDeviceOptions => {
  state.includeDeviceId = restoredConfig.includeDeviceId
  state.includeMetric = restoredConfig.includeMetric
  state.includeLocation = restoredConfig.includeLocation
  state.includeStatus = restoredConfig.includeStatus

  if (!restoredConfig.deviceId) return {}

  const device =
    restoredConfig.existingDevice?.deviceId === restoredConfig.deviceId
      ? restoredConfig.existingDevice
      : preservedDeviceFromId(restoredConfig.deviceId)
  state.selectedDevice = device

  if (!restoredConfig.metricKey) return { device }

  const metric =
    restoredConfig.existingMetric?.metricKey === restoredConfig.metricKey
      ? restoredConfig.existingMetric
      : preservedMetricFromKey(restoredConfig.metricKey)
  state.selectedMetric = metric

  return { device, metric }
}
