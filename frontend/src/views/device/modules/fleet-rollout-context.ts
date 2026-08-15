export type FleetRolloutRow = Record<string, any>
export type FleetRolloutRouteQueryValue = string | string[] | null | undefined
export type FleetDeviceFilterValue = string | number | boolean

export interface FleetRolloutContext {
  source: string
  scope: string
  deviceIds: string[]
  requestedTotal: number | null
  currentPageCount: number | null
  deviceFilter: Record<string, FleetDeviceFilterValue>
}

export interface FleetRolloutSelectionResult {
  source: string
  scope: string
  requestedCount: number
  requestedTotal: number | null
  currentPageCount: number | null
  selectedCount: number
  excludedCount: number
  selectedDeviceIds: string[]
  deviceFilter: Record<string, FleetDeviceFilterValue>
}

const FLEET_SOURCE_QUERY_KEY = 'fleet_source'
const FLEET_SCOPE_QUERY_KEY = 'fleet_scope'
const FLEET_REQUESTED_TOTAL_QUERY_KEY = 'fleet_requested_total'
const FLEET_CURRENT_PAGE_COUNT_QUERY_KEY = 'fleet_current_page_count'
const DEVICE_IDS_QUERY_KEY = 'device_ids'
const DEVICE_FILTER_QUERY_KEY = 'device_filter'
const PREVIEW_SUBSET_DEVICE_IDS_QUERY_KEY = 'preview_sample_device_ids'
const DEFAULT_FLEET_SOURCE = 'device_manage'
export const FLEET_CURRENT_PAGE_SCOPE = 'current_page'
export const FLEET_FILTER_RESULT_SCOPE = 'filter_result'
export const FLEET_DEVICE_FILTER_SCOPE = 'device_filter'
const FLEET_QUERY_KEYS = new Set([
  FLEET_SOURCE_QUERY_KEY,
  FLEET_SCOPE_QUERY_KEY,
  FLEET_REQUESTED_TOTAL_QUERY_KEY,
  FLEET_CURRENT_PAGE_COUNT_QUERY_KEY,
  DEVICE_IDS_QUERY_KEY,
  DEVICE_FILTER_QUERY_KEY,
  PREVIEW_SUBSET_DEVICE_IDS_QUERY_KEY
])
const DEVICE_FILTER_QUERY_KEYS = new Set([
  'device_number',
  'is_enabled',
  'product_id',
  'label',
  'name',
  'current_version',
  'pid_number',
  'firmware_version',
  'description',
  'shared_status',
  'group_id',
  'device_config_id',
  'device_template_id',
  'is_online',
  'last_reported_after',
  'last_reported_before',
  'never_reported',
  'warn_status',
  'search',
  'access_way',
  'batch_number',
  'device_type',
  'service_identifier',
  'service_access_id'
])
const DEVICE_FILTER_NUMERIC_KEYS = new Set(['is_online', 'last_reported_after', 'last_reported_before'])
const DEVICE_FILTER_BOOLEAN_KEYS = new Set(['never_reported'])

export function getFleetRolloutDeviceId(row: FleetRolloutRow) {
  return String(row.id || row.device_id || '')
}

export function buildFleetRolloutDeviceIds(rows: FleetRolloutRow[]) {
  return Array.from(new Set(rows.map(getFleetRolloutDeviceId).filter(Boolean)))
}

export function buildFleetRolloutQuery(
  rows: FleetRolloutRow[],
  fallbackParams: Record<string, unknown>,
  source = DEFAULT_FLEET_SOURCE,
  requestedTotal?: number | null
) {
  const deviceIds = buildFleetRolloutDeviceIds(rows)
  const hasFilterParams = Object.entries(fallbackParams).some(([key, value]) => {
    if (!DEVICE_FILTER_QUERY_KEYS.has(key)) return false
    if (value === null || value === undefined || value === '') return false
    return !(Array.isArray(value) && value.length === 0)
  })
  const scope =
    hasFilterParams && requestedTotal !== null && requestedTotal !== undefined
      ? FLEET_FILTER_RESULT_SCOPE
      : FLEET_CURRENT_PAGE_SCOPE
  const query: Record<string, unknown> = {
    ...fallbackParams,
    [FLEET_SOURCE_QUERY_KEY]: source,
    [FLEET_SCOPE_QUERY_KEY]: scope,
    [FLEET_CURRENT_PAGE_COUNT_QUERY_KEY]: deviceIds.length,
    [FLEET_REQUESTED_TOTAL_QUERY_KEY]: requestedTotal
  }

  if (deviceIds.length > 0) {
    query[DEVICE_IDS_QUERY_KEY] = deviceIds.join(',')
  }

  return Object.fromEntries(
    Object.entries(query).filter(([, value]) => value !== '' && value !== null && value !== undefined)
  )
}

const normalizeQueryValue = (value: FleetRolloutRouteQueryValue) => {
  if (Array.isArray(value)) return value.join(',')
  return value || ''
}

const normalizeQueryNumber = (value: FleetRolloutRouteQueryValue) => {
  const rawValue = normalizeQueryValue(value)
  if (!rawValue) return null
  const parsed = Number(rawValue)

  return Number.isFinite(parsed) ? parsed : null
}

const buildDeviceFilterFromQuery = (query: Record<string, FleetRolloutRouteQueryValue>) => {
  const filter: Record<string, FleetDeviceFilterValue> = {}
  const rawDeviceFilter = normalizeQueryValue(query[DEVICE_FILTER_QUERY_KEY])
  if (rawDeviceFilter) {
    try {
      const parsed = JSON.parse(rawDeviceFilter) as Record<string, unknown>
      Object.entries(parsed).forEach(([key, value]) => {
        if (!DEVICE_FILTER_QUERY_KEYS.has(key)) return
        if (value === null || value === undefined || value === '') return
        if (DEVICE_FILTER_BOOLEAN_KEYS.has(key)) {
          if (typeof value === 'boolean') filter[key] = value
          else if (value === 0 || value === 1) filter[key] = value === 1
          return
        }
        if (DEVICE_FILTER_NUMERIC_KEYS.has(key)) {
          const numericValue = Number(value)
          if (Number.isFinite(numericValue)) filter[key] = numericValue
          return
        }
        if (typeof value === 'string' && value.trim()) filter[key] = value.trim()
      })
    } catch {
      // Keep compatible query-field parsing below as a safe fallback for older links.
    }
  }

  Object.entries(query).forEach(([key, value]) => {
    if (FLEET_QUERY_KEYS.has(key) || !DEVICE_FILTER_QUERY_KEYS.has(key)) return
    const normalized = normalizeQueryValue(value).trim()
    if (!normalized) return
    if (DEVICE_FILTER_BOOLEAN_KEYS.has(key)) {
      if (normalized === 'true' || normalized === '1') filter[key] = true
      else if (normalized === 'false' || normalized === '0') filter[key] = false
      return
    }
    if (DEVICE_FILTER_NUMERIC_KEYS.has(key)) {
      const numericValue = Number(normalized)
      if (Number.isFinite(numericValue)) filter[key] = numericValue
      return
    }
    filter[key] = normalized
  })

  return Object.fromEntries(
    Object.entries(filter).filter(([, value]) => {
      if (typeof value === 'number') return Number.isFinite(value)
      if (typeof value === 'boolean') return true
      return Boolean(value)
    })
  )
}

export function parseFleetRolloutContext(
  query: Record<string, FleetRolloutRouteQueryValue>
): FleetRolloutContext | null {
  const rawDeviceIds = normalizeQueryValue(query[DEVICE_IDS_QUERY_KEY])
  const rawPreviewSubsetDeviceIds = normalizeQueryValue(query[PREVIEW_SUBSET_DEVICE_IDS_QUERY_KEY])
  const subsetDeviceIds = rawDeviceIds || rawPreviewSubsetDeviceIds
  const scope = normalizeQueryValue(query[FLEET_SCOPE_QUERY_KEY]) || FLEET_CURRENT_PAGE_SCOPE
  const deviceFilter = buildDeviceFilterFromQuery(query)
  const deviceIds = Array.from(
    new Set(
      subsetDeviceIds
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )

  const hasFilterOnlyScope =
    (scope === FLEET_FILTER_RESULT_SCOPE || scope === FLEET_DEVICE_FILTER_SCOPE) && Object.keys(deviceFilter).length > 0
  if (deviceIds.length === 0 && !hasFilterOnlyScope) return null

  return {
    source: normalizeQueryValue(query[FLEET_SOURCE_QUERY_KEY]) || DEFAULT_FLEET_SOURCE,
    scope,
    deviceIds,
    requestedTotal: normalizeQueryNumber(query[FLEET_REQUESTED_TOTAL_QUERY_KEY]),
    currentPageCount: normalizeQueryNumber(query[FLEET_CURRENT_PAGE_COUNT_QUERY_KEY]),
    deviceFilter
  }
}

export function buildFleetRolloutSelectionResult(
  context: FleetRolloutContext | null | undefined,
  candidates: FleetRolloutRow[],
  getCandidateId = getFleetRolloutDeviceId
): FleetRolloutSelectionResult | null {
  if (!context) return null
  const hasFilterOnlyScope =
    (context.scope === FLEET_FILTER_RESULT_SCOPE || context.scope === FLEET_DEVICE_FILTER_SCOPE) &&
    Object.keys(context.deviceFilter).length > 0
  if (context.deviceIds.length === 0 && !hasFilterOnlyScope) return null

  const candidateIds = new Set(candidates.map(getCandidateId).filter(Boolean))
  const selectedDeviceIds = context.deviceIds.filter((deviceId) => candidateIds.has(deviceId))
  const requestedCount = context.deviceIds.length || context.requestedTotal || context.currentPageCount || 0

  return {
    source: context.source,
    scope: context.scope,
    requestedCount,
    requestedTotal: context.requestedTotal,
    currentPageCount: context.currentPageCount,
    selectedCount: selectedDeviceIds.length,
    excludedCount: Math.max(context.deviceIds.length - selectedDeviceIds.length, 0),
    selectedDeviceIds,
    deviceFilter: context.deviceFilter
  }
}
