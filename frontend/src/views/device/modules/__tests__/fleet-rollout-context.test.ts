import { describe, expect, it } from 'vitest'
import {
  buildFleetRolloutQuery,
  buildFleetRolloutSelectionResult,
  FLEET_DEVICE_FILTER_SCOPE,
  FLEET_FILTER_RESULT_SCOPE,
  parseFleetRolloutContext
} from '../fleet-rollout-context'

describe('fleet-rollout-context', () => {
  it('carries a backend-safe device filter for full-filter OTA rollouts', () => {
    const context = parseFleetRolloutContext({
      device_ids: 'dev-1,dev-2',
      fleet_source: 'device_manage',
      fleet_scope: 'current_page',
      fleet_requested_total: '42',
      fleet_current_page_count: '2',
      page: '3',
      page_size: '20',
      group_id: 'group-1',
      is_online: '1',
      search: 'pump'
    })

    expect(context?.deviceFilter).toEqual({
      group_id: 'group-1',
      is_online: 1,
      search: 'pump'
    })

    expect(buildFleetRolloutSelectionResult(context, [{ id: 'dev-1' }])?.deviceFilter).toEqual({
      group_id: 'group-1',
      is_online: 1,
      search: 'pump'
    })
  })

  it('marks filter-based OTA rollouts as a filter result rather than current-page-only scope', () => {
    expect(
      buildFleetRolloutQuery([{ id: 'dev-1' }], { group_id: 'group-1', page: 3, page_size: 20 }, 'device_manage', 42)
    ).toEqual(
      expect.objectContaining({
        fleet_scope: FLEET_FILTER_RESULT_SCOPE,
        fleet_current_page_count: 1,
        fleet_requested_total: 42,
        device_ids: 'dev-1',
        group_id: 'group-1'
      })
    )
  })

  it('accepts command-center device_filter scope and preserves preview sample ids for OTA', () => {
    const context = parseFleetRolloutContext({
      fleet_source: 'device_manage',
      fleet_scope: FLEET_DEVICE_FILTER_SCOPE,
      fleet_requested_total: '42',
      fleet_current_page_count: '2',
      preview_sample_device_ids: 'dev-1,dev-2',
      device_filter:
        '{"group_id":"group-1","is_online":1,"last_reported_after":1752883200000,"never_reported":false}'
    })

    expect(context).toMatchObject({
      scope: FLEET_DEVICE_FILTER_SCOPE,
      deviceIds: ['dev-1', 'dev-2'],
      requestedTotal: 42,
      currentPageCount: 2,
      deviceFilter: {
        group_id: 'group-1',
        is_online: 1,
        last_reported_after: 1752883200000,
        never_reported: false
      }
    })
    expect(buildFleetRolloutSelectionResult(context, [{ id: 'dev-1' }, { id: 'dev-2' }])?.selectedDeviceIds).toEqual([
      'dev-1',
      'dev-2'
    ])
  })
})
