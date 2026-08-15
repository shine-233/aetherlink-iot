import type { LocationQueryRaw, RouteLocationRaw } from 'vue-router'
import { buildFleetContextQuery, cleanFleetFilterParams, type DeviceFleetOperationRow } from './device-fleet-operations'

export type FleetCurrentPageAction = {
  labelKey: string
  path: string
}

export function buildFleetCurrentPageHandoffQuery(
  rows: DeviceFleetOperationRow[],
  fallbackParams: Record<string, unknown>,
  requestedTotal?: number | null
): LocationQueryRaw | null {
  if (rows.length === 0) return null

  return buildFleetContextQuery(rows, fallbackParams, requestedTotal) as LocationQueryRaw
}

export function buildFleetCurrentPageActionRoute(
  action: FleetCurrentPageAction,
  query: LocationQueryRaw
): RouteLocationRaw {
  return {
    path: action.path,
    query
  }
}

export function buildSelectedDeviceCommandCenterRoute(deviceIds: string[]): RouteLocationRaw | null {
  const firstDeviceID = deviceIds[0]
  if (!firstDeviceID) return null

  return {
    path: '/device/command-center',
    query: {
      device_ids: deviceIds.join(','),
      fleet_source: 'device_manage',
      fleet_scope: 'selected_devices',
      fleet_selected_count: deviceIds.length,
      first_device_id: firstDeviceID
    }
  }
}

/**
 * 全量选择走 device_filter 交接，而不是把上千个 device_id 塞进 URL / 请求体。
 * effectiveCount 是上限截断后实际会被作用的台数，随查询透传，便于下游如实显示。
 */
export function buildSelectAllMatchingCommandCenterRoute(input: {
  params: Record<string, unknown>
  matchedTotal: number
  effectiveCount: number
  maxDevices: number
}): RouteLocationRaw | null {
  if (!input.matchedTotal || input.effectiveCount <= 0) return null

  const deviceFilter = cleanFleetFilterParams(input.params)

  return {
    path: '/device/command-center',
    query: {
      fleet_source: 'device_manage',
      fleet_scope: 'device_filter',
      device_filter: JSON.stringify(deviceFilter),
      fleet_requested_total: String(input.matchedTotal),
      fleet_effective_count: String(input.effectiveCount),
      max_devices: String(input.maxDevices)
    }
  }
}

export function buildSavedFilterCommandCenterRoute(
  params: Record<string, unknown>,
  requestedTotal?: number | null,
  savedFilter?: { id?: string; name?: string }
): RouteLocationRaw | null {
  const deviceFilter = cleanFleetFilterParams(params)
  if (Object.keys(deviceFilter).length === 0) return null

  const query: LocationQueryRaw = {
    fleet_source: 'device_manage',
    fleet_scope: 'device_filter',
    device_filter: JSON.stringify(deviceFilter)
  }
  if (typeof requestedTotal === 'number') {
    query.fleet_requested_total = String(requestedTotal)
  }
  if (savedFilter?.id) {
    query.saved_filter_id = savedFilter.id
  }
  if (savedFilter?.name) {
    query.saved_filter_name = savedFilter.name
  }

  return {
    path: '/device/command-center',
    query
  }
}
