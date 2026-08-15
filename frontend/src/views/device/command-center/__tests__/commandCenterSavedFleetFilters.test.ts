import { afterEach, describe, expect, it, vi } from 'vitest'
import { useCommandCenterSavedFleetFilters } from '../useCommandCenterSavedFleetFilters'
import { createSavedFleetFilter, FLEET_SAVED_FILTER_STORAGE_KEY } from '../../manage/device-fleet-saved-filters'
import { deleteFleetSavedFilter, listFleetSavedFilters, updateFleetSavedFilter } from '@/service/api/device'

vi.mock('@/service/api/device', () => ({
  deleteFleetSavedFilter: vi.fn(),
  listFleetSavedFilters: vi.fn(),
  updateFleetSavedFilter: vi.fn()
}))

const mockedDeleteFleetSavedFilter = vi.mocked(deleteFleetSavedFilter)
const mockedListFleetSavedFilters = vi.mocked(listFleetSavedFilters)
const mockedUpdateFleetSavedFilter = vi.mocked(updateFleetSavedFilter)

const createStorage = (initialValue: string | null = null) => {
  let value = initialValue
  return {
    getItem: vi.fn(() => value),
    setItem: vi.fn((_: string, nextValue: string) => {
      value = nextValue
    })
  }
}

describe('useCommandCenterSavedFleetFilters', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.unstubAllGlobals()
  })

  it('merges server filters into the command-center picker and syncs the route selection', async () => {
    const storage = createStorage()
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockResolvedValue({
      data: {
        list: [
          {
            id: 'server-filter-1',
            name: 'Online pumps',
            device_filter: { is_online: 1, unsafe: 'ignored' },
            preview_total: 12,
            created_at: '2026-07-05T12:00:00Z'
          }
        ]
      }
    } as any)

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => 'server-filter-1'
    })

    await state.refreshCommandCenterSavedFilters()

    expect(state.selectedSavedFleetFilterId.value).toBe('server-filter-1')
    expect(state.savedFleetFilterStatus.value).toBe('server')
    expect(state.savedFleetFilterNoticeKey.value).toBe('')
    expect(state.activeSavedFleetFilter.value).toMatchObject({
      id: 'server-filter-1',
      name: 'Online pumps',
      previewTotal: 12
    })
    expect(state.savedFleetFilterOptions.value).toEqual([{ label: 'Online pumps (12)', value: 'server-filter-1' }])
    expect(storage.setItem).toHaveBeenCalledWith(
      FLEET_SAVED_FILTER_STORAGE_KEY,
      expect.stringContaining('server-filter-1')
    )
  })

  it('keeps local saved filters when the backend list cannot be loaded', async () => {
    const localFilter = createSavedFleetFilter({ is_online: 1 }, 5, new Date('2026-07-05T02:00:00.000Z'))
    const storage = createStorage(JSON.stringify([localFilter]))
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockRejectedValue(new Error('network unavailable'))

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => localFilter.id
    })

    await state.refreshCommandCenterSavedFilters()

    expect(state.savedFleetFilters.value).toEqual([localFilter])
    expect(state.selectedSavedFleetFilterId.value).toBe(localFilter.id)
    expect(state.activeSavedFleetFilter.value?.name).toBe(localFilter.name)
    expect(state.savedFleetFilterStatus.value).toBe('local-fallback')
    expect(state.savedFleetFilterNoticeKey.value).toBe('custom.commandCenter.savedFilterLocalFallback')
  })

  it('marks a route saved-filter identity as stale when the filter no longer exists', async () => {
    const storage = createStorage(JSON.stringify([]))
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockResolvedValue({
      data: {
        list: []
      }
    } as any)

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => 'deleted-filter'
    })

    await state.refreshCommandCenterSavedFilters()

    expect(state.selectedSavedFleetFilterId.value).toBeNull()
    expect(state.activeSavedFleetFilter.value).toBeNull()
    expect(state.staleRouteSavedFilter.value).toBe(true)
    expect(state.savedFleetFilterNoticeKey.value).toBe('custom.commandCenter.savedFilterRouteMissing')
  })

  it('renames a local-only saved filter without calling the backend', async () => {
    const localFilter = createSavedFleetFilter({ is_online: 1 }, 5, new Date('2026-07-05T02:00:00.000Z'))
    const storage = createStorage(JSON.stringify([localFilter]))
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockRejectedValue(new Error('network unavailable'))

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => localFilter.id
    })

    await state.refreshCommandCenterSavedFilters()
    await expect(state.renameCommandCenterSavedFilter(localFilter.id, 'Online chillers')).resolves.toBe(true)

    expect(mockedUpdateFleetSavedFilter).not.toHaveBeenCalled()
    expect(state.activeSavedFleetFilter.value?.name).toBe('Online chillers')
    expect(storage.setItem).toHaveBeenLastCalledWith(
      FLEET_SAVED_FILTER_STORAGE_KEY,
      expect.stringContaining('Online chillers')
    )
  })

  it('keeps a backend saved filter unchanged when rename fails', async () => {
    const storage = createStorage()
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockResolvedValue({
      data: {
        list: [
          {
            id: 'server-filter-1',
            name: 'Online pumps',
            device_filter: { is_online: 1 },
            preview_total: 12
          }
        ]
      }
    } as any)
    mockedUpdateFleetSavedFilter.mockRejectedValue(new Error('duplicate name'))

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => 'server-filter-1'
    })

    await state.refreshCommandCenterSavedFilters()
    await expect(state.renameCommandCenterSavedFilter('server-filter-1', 'Duplicate')).resolves.toBe(false)

    expect(mockedUpdateFleetSavedFilter).toHaveBeenCalled()
    expect(state.activeSavedFleetFilter.value?.name).toBe('Online pumps')
    expect(state.savedFleetFilterActionError.value).toBe('duplicate name')
  })

  it('deletes a local-only saved filter without calling the backend', async () => {
    const localFilter = createSavedFleetFilter({ is_online: 1 }, 5, new Date('2026-07-05T02:00:00.000Z'))
    const storage = createStorage(JSON.stringify([localFilter]))
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockRejectedValue(new Error('network unavailable'))

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => localFilter.id
    })

    await state.refreshCommandCenterSavedFilters()
    await expect(state.deleteCommandCenterSavedFilter(localFilter.id)).resolves.toBe(true)

    expect(mockedDeleteFleetSavedFilter).not.toHaveBeenCalled()
    expect(state.savedFleetFilters.value).toEqual([])
    expect(state.selectedSavedFleetFilterId.value).toBeNull()
  })

  it('keeps a backend saved filter when delete fails', async () => {
    const storage = createStorage()
    vi.stubGlobal('window', { localStorage: storage })
    mockedListFleetSavedFilters.mockResolvedValue({
      data: {
        list: [
          {
            id: 'server-filter-1',
            name: 'Online pumps',
            device_filter: { is_online: 1 },
            preview_total: 12
          }
        ]
      }
    } as any)
    mockedDeleteFleetSavedFilter.mockRejectedValue(new Error('delete denied'))

    const state = useCommandCenterSavedFleetFilters({
      getRouteSavedFilterId: () => 'server-filter-1'
    })

    await state.refreshCommandCenterSavedFilters()
    await expect(state.deleteCommandCenterSavedFilter('server-filter-1')).resolves.toBe(false)

    expect(mockedDeleteFleetSavedFilter).toHaveBeenCalledWith('server-filter-1')
    expect(state.savedFleetFilters.value).toHaveLength(1)
    expect(state.savedFleetFilterActionError.value).toBe('delete denied')
  })
})
