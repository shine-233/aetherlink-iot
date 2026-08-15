export type FleetTargetPresetKey =
  | 'all'
  | 'online'
  | 'offline'
  | 'never_reported'
  | 'alarmed'
  | 'unshared'
  | 'direct'
  | 'gateway'

export type FleetTargetPreset = {
  key: FleetTargetPresetKey
  labelKey: string
  params: Record<string, string | number | boolean | null>
}

const FLEET_TARGET_FILTER_KEYS = [
  'is_online',
  'warn_status',
  'shared_status',
  'device_type',
  'last_reported_after',
  'last_reported_before',
  'never_reported'
] as const

export const fleetTargetPresets: FleetTargetPreset[] = [
  {
    key: 'all',
    labelKey: 'custom.devicePage.fleetTargetAll',
    params: {}
  },
  {
    key: 'online',
    labelKey: 'custom.devicePage.fleetTargetOnline',
    params: { is_online: 1 }
  },
  {
    key: 'offline',
    labelKey: 'custom.devicePage.fleetTargetOffline',
    params: { is_online: 0 }
  },
  {
    key: 'never_reported',
    labelKey: 'custom.devicePage.neverReported',
    params: { never_reported: true }
  },
  {
    key: 'alarmed',
    labelKey: 'custom.devicePage.fleetTargetAlarmed',
    params: { warn_status: 'Y' }
  },
  {
    key: 'unshared',
    labelKey: 'custom.devicePage.fleetTargetUnshared',
    params: { shared_status: 'unshared' }
  },
  {
    key: 'direct',
    labelKey: 'custom.devicePage.fleetTargetDirect',
    params: { device_type: '1' }
  },
  {
    key: 'gateway',
    labelKey: 'custom.devicePage.fleetTargetGateway',
    params: { device_type: '2' }
  }
]

export function buildFleetTargetPresetParams(presetKey: FleetTargetPresetKey) {
  const preset = fleetTargetPresets.find((item) => item.key === presetKey) || fleetTargetPresets[0]
  const clearedParams = Object.fromEntries(FLEET_TARGET_FILTER_KEYS.map((key) => [key, null]))

  return {
    ...clearedParams,
    ...preset.params
  }
}
