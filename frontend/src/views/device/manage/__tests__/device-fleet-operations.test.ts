import { describe, expect, it } from 'vitest'
import { buildFleetSelectionSummary } from '../device-fleet-operations'

describe('device-fleet-operations', () => {
  it('builds current-page fleet health summary for the overview strip', () => {
    expect(
      buildFleetSelectionSummary([
        { id: 'online-ready', is_online: 1, warn_status: 'N', current_version: '1.0.0' },
        { id: 'online-alarmed', is_online: 1, warn_status: 'Y', firmware_version: '2.0.0' },
        { id: 'offline-missing-version', is_online: 0, warn_status: 'N', current_version: '' },
        { id: 'unknown-missing-version', is_online: null, warn_status: 'Y' }
      ])
    ).toEqual({
      total: 4,
      online: 2,
      offline: 1,
      alarmed: 2,
      missingVersion: 2
    })
  })
})
