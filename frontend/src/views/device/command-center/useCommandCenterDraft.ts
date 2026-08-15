import { computed, ref } from 'vue'
import {
  DEFAULT_FILTER_JOB_MAX_DEVICES,
  DEFAULT_FILTER_JOB_SUBSET_LIMIT,
  buildFleetCommandPayload,
  getFleetCommandPayloadValidationKey,
  serializeFleetCommandPayload,
  type DeviceFilterPayload,
  type FleetCommandScopeType
} from './commandCenterState'
import { buildCommandPayloadInsight } from './commandCenterPayloadAssistant'

type UseCommandCenterDraftOptions = {
  selectedDeviceIds: () => string[]
  scopeType: () => FleetCommandScopeType
  deviceFilter: () => DeviceFilterPayload
  requestedTotal: () => number | null
  currentPageCount: () => number | null
  source: () => string
  hasSelectedDevices: () => boolean
  hasDeviceFilter: () => boolean
  setError: (message: string) => void
  t: (key: string) => string
}

export function useCommandCenterDraft(options: UseCommandCenterDraftOptions) {
  const commandIdentify = ref('')
  const commandValue = ref('')
  const timeoutSeconds = ref<number | null>(60)
  const scheduledAt = ref<number | null>(null)
  const maxDevices = ref<number | null>(DEFAULT_FILTER_JOB_MAX_DEVICES)
  const subsetLimit = ref<number | null>(DEFAULT_FILTER_JOB_SUBSET_LIMIT)

  const buildCurrentFleetCommandPayload = () =>
    buildFleetCommandPayload({
      deviceIds: options.selectedDeviceIds(),
      scopeType: options.scopeType(),
      deviceFilter: options.deviceFilter(),
      expectedTotal: options.requestedTotal(),
      currentPageCount: options.currentPageCount(),
      source: options.source(),
      identify: commandIdentify.value,
      value: commandValue.value,
      timeoutSeconds: timeoutSeconds.value,
      scheduledAt: scheduledAt.value,
      maxDevices: maxDevices.value,
      subsetLimit: subsetLimit.value
    })

  const validateFleetCommandPayload = () => {
    options.setError('')
    const validationKey = getFleetCommandPayloadValidationKey({
      hasSelectedDevices: options.hasSelectedDevices(),
      hasDeviceFilter: options.hasDeviceFilter(),
      scopeType: options.scopeType(),
      identify: commandIdentify.value,
      scheduledAt: scheduledAt.value
    })
    if (validationKey) {
      options.setError(options.t(validationKey))
      return false
    }
    if (buildCommandPayloadInsight(commandValue.value).type === 'error') {
      options.setError(options.t('custom.commandCenter.commandValueInvalidJson'))
      return false
    }
    return true
  }

  const currentPayloadFingerprint = computed(() =>
    serializeFleetCommandPayload(buildCurrentFleetCommandPayload())
  )

  return {
    buildCurrentFleetCommandPayload,
    commandIdentify,
    commandValue,
    currentPayloadFingerprint,
    maxDevices,
    scheduledAt,
    subsetLimit,
    timeoutSeconds,
    validateFleetCommandPayload
  }
}
