import type { FleetSavedFilterItem, FleetSavedFilterPayload } from '@/service/api/device'

export type SavedFleetFilter = {
  id: string
  name: string
  params: Record<string, unknown>
  previewTotal: number | null
  createdAt: string
  shared: boolean
  owned: boolean
  ownerUserId: string
}

export type FleetFilterStorageLike = Pick<Storage, 'getItem' | 'setItem'>

export const FLEET_SAVED_FILTER_STORAGE_KEY = 'aetherlink.deviceManage.savedFleetFilters'

const MAX_SAVED_FLEET_FILTERS = 8

const SAVED_FLEET_FILTER_KEYS = [
  'access_way',
  'batch_number',
  'current_version',
  'group_id',
  'product_id',
  'device_template_id',
  'device_config_id',
  'is_enabled',
  'is_online',
  'last_reported_after',
  'last_reported_before',
  'never_reported',
  'lifecycle_status',
  'warn_status',
  'device_type',
  'service_identifier',
  'service_access_id',
  'search',
  'name',
  'device_number',
  'pid_number',
  'firmware_version',
  'description',
  'shared_status',
  'label'
] as const

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

export function normalizeFleetFilterParams(params: Record<string, unknown>) {
  return Object.fromEntries(SAVED_FLEET_FILTER_KEYS.map((key) => [key, params[key] ?? null]))
}

export function compactFleetFilterParams(params: Record<string, unknown>) {
  return Object.fromEntries(
    Object.entries(normalizeFleetFilterParams(params)).filter(([, value]) => {
      if (typeof value === 'string') return value.trim() !== ''
      return typeof value === 'number' || typeof value === 'boolean'
    })
  )
}

export function hasUsableFleetFilterParams(params: Record<string, unknown>) {
  return Object.keys(compactFleetFilterParams(params)).length > 0
}

function normalizeSavedFleetFilter(value: unknown): SavedFleetFilter | null {
  if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string') return null
  if (!isRecord(value.params)) return null

  return {
    id: value.id,
    name: value.name,
    params: normalizeFleetFilterParams(value.params),
    previewTotal: typeof value.previewTotal === 'number' ? value.previewTotal : null,
    createdAt: typeof value.createdAt === 'string' ? value.createdAt : '',
    shared: value.shared === true,
    // Local-only filters are always owned by the current user.
    owned: value.owned !== false,
    ownerUserId: typeof value.ownerUserId === 'string' ? value.ownerUserId : ''
  }
}

export function normalizeServerFleetSavedFilter(value: FleetSavedFilterItem): SavedFleetFilter | null {
  if (!value?.id || !value.name || !isRecord(value.device_filter)) return null

  return {
    id: value.id,
    name: value.name,
    params: normalizeFleetFilterParams(value.device_filter),
    previewTotal: typeof value.preview_total === 'number' ? value.preview_total : null,
    createdAt: value.created_at || value.updated_at || '',
    shared: value.shared === true,
    // The backend always sends `owned` (false only for another member's shared
    // filter). Treat a missing value defensively as owned so local-origin rows
    // stay editable.
    owned: value.owned !== false,
    ownerUserId: typeof value.owner_user_id === 'string' ? value.owner_user_id : ''
  }
}

export function normalizeServerFleetSavedFilters(values: FleetSavedFilterItem[] = []): SavedFleetFilter[] {
  return values.map(normalizeServerFleetSavedFilter).filter(Boolean) as SavedFleetFilter[]
}

export function buildFleetSavedFilterPayload(
  params: Record<string, unknown>,
  previewTotal: number | null,
  name?: string,
  shared?: boolean
): FleetSavedFilterPayload {
  const local = createSavedFleetFilter(params, previewTotal)
  const payload: FleetSavedFilterPayload = {
    name: name || local.name,
    device_filter: compactFleetFilterParams(params),
    preview_total: previewTotal
  }
  if (typeof shared === 'boolean') {
    payload.shared = shared
  }
  return payload
}

export function loadSavedFleetFilters(storage: FleetFilterStorageLike | null | undefined): SavedFleetFilter[] {
  if (!storage) return []

  try {
    const raw = storage.getItem(FLEET_SAVED_FILTER_STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    if (!Array.isArray(parsed)) return []
    return parsed.map(normalizeSavedFleetFilter).filter(Boolean).slice(0, MAX_SAVED_FLEET_FILTERS) as SavedFleetFilter[]
  } catch {
    return []
  }
}

export function saveFleetFiltersToStorage(
  storage: FleetFilterStorageLike | null | undefined,
  filters: SavedFleetFilter[]
) {
  storage?.setItem(FLEET_SAVED_FILTER_STORAGE_KEY, JSON.stringify(filters.slice(0, MAX_SAVED_FLEET_FILTERS)))
}

export function createSavedFleetFilter(
  params: Record<string, unknown>,
  previewTotal: number | null,
  now = new Date()
): SavedFleetFilter {
  const createdAt = now.toISOString()
  const displayTime = createdAt.slice(0, 19).replace('T', ' ')
  const suffix = Math.random().toString(36).slice(2, 6)

  return {
    id: `fleet-filter-${createdAt}-${suffix}`,
    name: `Fleet ${displayTime}-${suffix}`,
    params: normalizeFleetFilterParams(params),
    previewTotal,
    createdAt,
    // Local-only filters live in this browser and are always owned, never shared.
    shared: false,
    owned: true,
    ownerUserId: ''
  }
}

export function saveFleetFilter(
  storage: FleetFilterStorageLike | null | undefined,
  existingFilters: SavedFleetFilter[],
  params: Record<string, unknown>,
  previewTotal: number | null,
  now = new Date()
) {
  const nextFilters = [createSavedFleetFilter(params, previewTotal, now), ...existingFilters].slice(
    0,
    MAX_SAVED_FLEET_FILTERS
  )

  storage?.setItem(FLEET_SAVED_FILTER_STORAGE_KEY, JSON.stringify(nextFilters))
  return nextFilters
}

export function mergeSavedFleetFilters(primary: SavedFleetFilter[], fallback: SavedFleetFilter[]) {
  const seen = new Set<string>()
  const result: SavedFleetFilter[] = []
  for (const filter of [...primary, ...fallback]) {
    const key = `${filter.name}:${JSON.stringify(normalizeFleetFilterParams(filter.params))}`
    if (seen.has(key)) continue
    seen.add(key)
    result.push(filter)
  }
  return result.slice(0, MAX_SAVED_FLEET_FILTERS)
}
