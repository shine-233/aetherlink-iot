import { describe, expect, it } from 'vitest'
import { buildFleetTargetPresetParams, fleetTargetPresets } from '../device-fleet-target-presets'

describe('device-fleet-target-presets', () => {
  it('keeps common fleet target presets available', () => {
    expect(fleetTargetPresets.map((item) => item.key)).toEqual([
      'all',
      'online',
      'offline',
      'never_reported',
      'alarmed',
      'unshared',
      'direct',
      'gateway'
    ])
  })

  it('clears other target dimensions before applying a preset', () => {
    expect(buildFleetTargetPresetParams('offline')).toEqual({
      is_online: 0,
      warn_status: null,
      shared_status: null,
      device_type: null,
      last_reported_after: null,
      last_reported_before: null,
      never_reported: null
    })
  })

  it('returns a full clear payload for all devices', () => {
    expect(buildFleetTargetPresetParams('all')).toEqual({
      is_online: null,
      warn_status: null,
      shared_status: null,
      device_type: null,
      last_reported_after: null,
      last_reported_before: null,
      never_reported: null
    })
  })
})
