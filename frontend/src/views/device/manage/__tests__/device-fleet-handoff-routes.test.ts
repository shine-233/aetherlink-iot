import { describe, expect, it } from 'vitest'
import { buildSavedFilterCommandCenterRoute } from '../device-fleet-handoff-routes'

describe('device-fleet-handoff-routes', () => {
  it('passes saved filter identity to Command Center without polluting the filter payload', () => {
    const route = buildSavedFilterCommandCenterRoute(
      { group_id: 'group-1', is_online: 1, empty: '' },
      42,
      { id: 'fleet-filter-1', name: 'Online pumps' }
    )

    expect(route).toEqual({
      path: '/device/command-center',
      query: {
        fleet_source: 'device_manage',
        fleet_scope: 'device_filter',
        device_filter: JSON.stringify({ group_id: 'group-1', is_online: 1 }),
        fleet_requested_total: '42',
        saved_filter_id: 'fleet-filter-1',
        saved_filter_name: 'Online pumps'
      }
    })
  })
})
