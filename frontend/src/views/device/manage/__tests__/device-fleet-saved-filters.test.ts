import { describe, expect, it, vi } from 'vitest'
import {
  FLEET_SAVED_FILTER_STORAGE_KEY,
  buildFleetSavedFilterPayload,
  compactFleetFilterParams,
  createSavedFleetFilter,
  hasUsableFleetFilterParams,
  loadSavedFleetFilters,
  mergeSavedFleetFilters,
  normalizeFleetFilterParams,
  normalizeServerFleetSavedFilter,
  saveFleetFilter
} from '../device-fleet-saved-filters'

const createStorage = (initialValue: string | null = null) => {
  let value = initialValue
  return {
    getItem: vi.fn(() => value),
    setItem: vi.fn((_: string, nextValue: string) => {
      value = nextValue
    })
  }
}

describe('device-fleet-saved-filters', () => {
  it('normalizes filter params to the supported device query dimensions', () => {
    const result = normalizeFleetFilterParams({
      page: 9,
      is_online: 0,
      last_reported_after: 1752883200000,
      never_reported: false,
      warn_status: 'Y',
      unsafe: 'ignored'
    })

    expect(result).toMatchObject({
      group_id: null,
      is_online: 0,
      last_reported_after: 1752883200000,
      never_reported: false,
      warn_status: 'Y'
    })
    expect(result).not.toHaveProperty('unsafe')
  })

  it('compacts fleet filters before sending them to the backend', () => {
    const compacted = compactFleetFilterParams({
      group_id: null,
      is_online: 0,
      never_reported: true,
      warn_status: '',
      product_id: 'product-1',
      unsafe: 'ignored'
    })

    expect(compacted).toEqual({
      is_online: 0,
      never_reported: true,
      product_id: 'product-1'
    })
    expect(hasUsableFleetFilterParams({ group_id: null, warn_status: '' })).toBe(false)
    expect(hasUsableFleetFilterParams({ is_online: 0 })).toBe(true)
  })

  it('loads only valid saved filters from storage', () => {
    const saved = createSavedFleetFilter({ is_online: 1 }, 3, new Date('2026-07-05T02:00:00.000Z'))
    const storage = createStorage(JSON.stringify([saved, { id: 'broken' }]))

    expect(loadSavedFleetFilters(storage)).toEqual([saved])
  })

  it('saves newest filters first and caps the list length', () => {
    const storage = createStorage()
    let filters = []

    for (let index = 0; index < 9; index += 1) {
      filters = saveFleetFilter(
        storage,
        filters,
        { is_online: index % 2 },
        index,
        new Date(`2026-07-05T02:0${index}:00.000Z`)
      )
    }

    expect(filters).toHaveLength(8)
    expect(filters[0].previewTotal).toBe(8)
    expect(storage.setItem).toHaveBeenLastCalledWith(FLEET_SAVED_FILTER_STORAGE_KEY, JSON.stringify(filters))
  })

  it('normalizes backend saved filters into the same local selector shape', () => {
    const result = normalizeServerFleetSavedFilter({
      id: 'server-filter-1',
      name: 'Online devices',
      device_filter: {
        is_online: 1,
        page: 99,
        unsafe: 'ignored'
      },
      preview_total: 12,
      created_at: '2026-07-05T11:00:00Z',
      updated_at: '2026-07-05T12:00:00Z'
    })

    expect(result).toMatchObject({
      id: 'server-filter-1',
      name: 'Online devices',
      previewTotal: 12,
      createdAt: '2026-07-05T11:00:00Z'
    })
    expect(result?.params).toMatchObject({
      is_online: 1,
      group_id: null
    })
    expect(result?.params).not.toHaveProperty('unsafe')
  })

  it('carries backend sharing metadata and treats a missing owned flag as owned', () => {
    const shared = normalizeServerFleetSavedFilter({
      id: 'server-shared-1',
      name: 'Team scope',
      device_filter: { is_online: 1 },
      shared: true,
      owned: false,
      owner_user_id: 'user-42'
    })
    expect(shared).toMatchObject({ shared: true, owned: false, ownerUserId: 'user-42' })

    const ownedByDefault = normalizeServerFleetSavedFilter({
      id: 'server-shared-2',
      name: 'My scope',
      device_filter: { is_online: 1 }
    })
    // Backend omits owner metadata for the caller's own filters, so absence means owned + private.
    expect(ownedByDefault).toMatchObject({ shared: false, owned: true, ownerUserId: '' })
  })

  it('marks locally created filters as owned and private', () => {
    const local = createSavedFleetFilter({ is_online: 1 }, 3, new Date('2026-07-05T02:00:00.000Z'))
    expect(local).toMatchObject({ shared: false, owned: true, ownerUserId: '' })
  })

  it('builds backend payloads from the same supported fleet filter fields', () => {
    expect(
      buildFleetSavedFilterPayload(
        { is_online: 1, lifecycle_status: 'transmitted', unsafe: 'ignored' },
        4,
        'Ready devices'
      )
    ).toMatchObject({
      name: 'Ready devices',
      preview_total: 4,
      device_filter: {
        is_online: 1,
        lifecycle_status: 'transmitted'
      }
    })
  })

  it('only includes the shared flag in the payload when it is explicitly provided', () => {
    expect(buildFleetSavedFilterPayload({ is_online: 1 }, 4, 'Scope')).not.toHaveProperty('shared')
    expect(buildFleetSavedFilterPayload({ is_online: 1 }, 4, 'Scope', true)).toMatchObject({ shared: true })
    expect(buildFleetSavedFilterPayload({ is_online: 1 }, 4, 'Scope', false)).toMatchObject({ shared: false })
  })

  it('merges backend filters before local fallback without duplicating the same saved query', () => {
    const local = createSavedFleetFilter({ is_online: 1 }, 3, new Date('2026-07-05T02:00:00.000Z'))
    const server = {
      ...local,
      id: 'server-filter-1'
    }

    expect(mergeSavedFleetFilters([server], [local])).toEqual([server])
  })
})
