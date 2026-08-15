import { computed, ref } from 'vue'
import type { SelectOption } from 'naive-ui'
import { deleteFleetSavedFilter, listFleetSavedFilters, updateFleetSavedFilter } from '@/service/api/device'
import {
  buildFleetSavedFilterPayload,
  loadSavedFleetFilters,
  mergeSavedFleetFilters,
  normalizeServerFleetSavedFilter,
  normalizeServerFleetSavedFilters,
  saveFleetFiltersToStorage,
  type SavedFleetFilter
} from '../manage/device-fleet-saved-filters'

type UseCommandCenterSavedFleetFiltersOptions = {
  getRouteSavedFilterId: () => string
}

type SavedFleetFilterStatus = 'idle' | 'server' | 'local' | 'local-fallback' | 'empty'

const defaultStorage = () => (typeof window === 'undefined' ? null : window.localStorage)

const isLocalFleetFilter = (id: string) => id.startsWith('fleet-filter-')

const getApiErrorMessage = (error: unknown) => {
  const maybeError = error as {
    message?: string
    response?: { data?: { message?: string; msg?: string } }
    data?: { message?: string; msg?: string }
  }

  return (
    maybeError?.response?.data?.message ||
    maybeError?.response?.data?.msg ||
    maybeError?.data?.message ||
    maybeError?.data?.msg ||
    maybeError?.message ||
    ''
  )
}

export function useCommandCenterSavedFleetFilters(options: UseCommandCenterSavedFleetFiltersOptions) {
  const savedFleetFilters = ref<SavedFleetFilter[]>([])
  const savedFleetFilterLoading = ref(false)
  const savedFleetFilterLoaded = ref(false)
  const savedFleetFilterStatus = ref<SavedFleetFilterStatus>('idle')
  const savedFleetFilterActionError = ref('')
  const selectedSavedFleetFilterId = ref<string | null>(null)

  const syncSelectedSavedFleetFilterFromRoute = () => {
    const routeFilterId = options.getRouteSavedFilterId()
    if (!routeFilterId) {
      selectedSavedFleetFilterId.value = null
      return
    }
    const exists = savedFleetFilters.value.some((filter) => filter.id === routeFilterId)
    selectedSavedFleetFilterId.value = exists ? routeFilterId : null
  }

  const refreshCommandCenterSavedFilters = async () => {
    const storage = defaultStorage()
    const localFilters = loadSavedFleetFilters(storage)
    savedFleetFilterLoading.value = true
    savedFleetFilterActionError.value = ''
    savedFleetFilters.value = localFilters
    savedFleetFilterStatus.value = localFilters.length ? 'local' : 'empty'
    syncSelectedSavedFleetFilterFromRoute()

    try {
      const response = await listFleetSavedFilters()
      const data = (response as any).data ?? response
      const serverFilters = normalizeServerFleetSavedFilters(data?.list ?? [])
      savedFleetFilters.value = mergeSavedFleetFilters(serverFilters, localFilters)
      savedFleetFilterStatus.value = serverFilters.length ? 'server' : savedFleetFilters.value.length ? 'local' : 'empty'
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
      syncSelectedSavedFleetFilterFromRoute()
    } catch {
      savedFleetFilters.value = localFilters
      savedFleetFilterStatus.value = localFilters.length ? 'local-fallback' : 'empty'
      syncSelectedSavedFleetFilterFromRoute()
    } finally {
      savedFleetFilterLoaded.value = true
      savedFleetFilterLoading.value = false
    }
  }

  const savedFleetFilterOptions = computed<SelectOption[]>(() =>
    savedFleetFilters.value.map((filter) => ({
      label: `${filter.name}${typeof filter.previewTotal === 'number' ? ` (${filter.previewTotal})` : ''}`,
      value: filter.id
    }))
  )

  const activeSavedFleetFilter = computed(
    () =>
      savedFleetFilters.value.find((filter) => filter.id === selectedSavedFleetFilterId.value) ||
      savedFleetFilters.value.find((filter) => filter.id === options.getRouteSavedFilterId()) ||
      null
  )

  const staleRouteSavedFilter = computed(
    () => savedFleetFilterLoaded.value && Boolean(options.getRouteSavedFilterId()) && !activeSavedFleetFilter.value
  )

  const savedFleetFilterNoticeKey = computed(() => {
    if (staleRouteSavedFilter.value) return 'custom.commandCenter.savedFilterRouteMissing'
    if (savedFleetFilterStatus.value === 'local-fallback') return 'custom.commandCenter.savedFilterLocalFallback'
    if (savedFleetFilterStatus.value === 'local') return 'custom.commandCenter.savedFilterLocalOnly'
    return ''
  })

  const clearCommandCenterSavedFilterSelection = () => {
    selectedSavedFleetFilterId.value = null
  }

  const renameCommandCenterSavedFilter = async (filterId: string | number, name: string) => {
    const id = String(filterId)
    const nextName = name.trim()
    savedFleetFilterActionError.value = ''
    if (!nextName) return false

    const filter = savedFleetFilters.value.find((item) => item.id === id)
    if (!filter) return false

    const storage = defaultStorage()
    if (isLocalFleetFilter(id)) {
      savedFleetFilters.value = savedFleetFilters.value.map((item) =>
        item.id === id ? { ...item, name: nextName } : item
      )
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
      return true
    }

    try {
      const response = await updateFleetSavedFilter(
        id,
        buildFleetSavedFilterPayload(filter.params, filter.previewTotal, nextName)
      )
      const updated = normalizeServerFleetSavedFilter(((response as any).data ?? response) as any)
      savedFleetFilters.value = savedFleetFilters.value.map((item) =>
        item.id === id ? updated || { ...item, name: nextName } : item
      )
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
      return true
    } catch (error) {
      savedFleetFilterActionError.value = getApiErrorMessage(error)
      return false
    }
  }

  const deleteCommandCenterSavedFilter = async (filterId: string | number) => {
    const id = String(filterId)
    savedFleetFilterActionError.value = ''
    const filter = savedFleetFilters.value.find((item) => item.id === id)
    if (!filter) return false

    if (!isLocalFleetFilter(id)) {
      try {
        await deleteFleetSavedFilter(id)
      } catch (error) {
        savedFleetFilterActionError.value = getApiErrorMessage(error)
        return false
      }
    }

    const storage = defaultStorage()
    savedFleetFilters.value = savedFleetFilters.value.filter((item) => item.id !== id)
    if (selectedSavedFleetFilterId.value === id) selectedSavedFleetFilterId.value = null
    saveFleetFiltersToStorage(storage, savedFleetFilters.value)
    return true
  }

  return {
    activeSavedFleetFilter,
    clearCommandCenterSavedFilterSelection,
    deleteCommandCenterSavedFilter,
    renameCommandCenterSavedFilter,
    refreshCommandCenterSavedFilters,
    savedFleetFilterActionError,
    savedFleetFilterLoaded,
    savedFleetFilterLoading,
    savedFleetFilterNoticeKey,
    savedFleetFilterOptions,
    savedFleetFilterStatus,
    savedFleetFilters,
    selectedSavedFleetFilterId,
    staleRouteSavedFilter,
    syncSelectedSavedFleetFilterFromRoute
  }
}
