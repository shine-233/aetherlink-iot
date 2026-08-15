import { computed, ref, type Ref } from 'vue'
import type { Router } from 'vue-router'
import {
  createFleetSavedFilter,
  deleteFleetSavedFilter,
  listFleetSavedFilters,
  updateFleetSavedFilter
} from '@/service/api/device'
import type { FleetTargetPresetKey } from './device-fleet-target-presets'
import { buildSavedFilterCommandCenterRoute } from './device-fleet-handoff-routes'
import {
  buildFleetSavedFilterPayload,
  hasUsableFleetFilterParams,
  loadSavedFleetFilters,
  mergeSavedFleetFilters,
  normalizeServerFleetSavedFilter,
  normalizeServerFleetSavedFilters,
  saveFleetFilter,
  saveFleetFiltersToStorage,
  type SavedFleetFilter
} from './device-fleet-saved-filters'

type TablePageRef = Ref<
  | {
      forceChangeParamsByKey?: (params: Record<string, unknown>) => void
    }
  | undefined
>

type MessageApi = {
  success?: (message: string) => void
  warning?: (message: string) => void
}

type UseDeviceManageFleetSavedFiltersOptions = {
  tablePageRef: TablePageRef
  router: Router
  t: (key: string) => string
  message?: MessageApi
  activeFleetTargetPreset: Ref<FleetTargetPresetKey>
  lastDeviceQueryParams: Ref<Record<string, unknown>>
  targetPreviewTotal: Ref<number | null>
  getStorage?: () => Pick<Storage, 'getItem' | 'setItem'> | null
}

const defaultStorage = () => (typeof window === 'undefined' ? null : window.localStorage)

const getApiErrorMessage = (error: unknown) => {
  const maybeError = error as {
    message?: string
    response?: {
      data?: {
        message?: string
        msg?: string
      }
    }
    data?: {
      message?: string
      msg?: string
    }
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

export function useDeviceManageFleetSavedFilters(options: UseDeviceManageFleetSavedFiltersOptions) {
  const savedFleetFilters = ref<SavedFleetFilter[]>([])
  const getStorage = options.getStorage || defaultStorage

  const savedFleetFilterOptions = computed(() =>
    savedFleetFilters.value.map((filter) => ({
      key: filter.id,
      rawName: filter.name,
      label: typeof filter.previewTotal === 'number' ? `${filter.name} (${filter.previewTotal})` : filter.name,
      shared: filter.shared,
      owned: filter.owned,
      ownerUserId: filter.ownerUserId
    }))
  )

  const canSaveCurrentFleetFilter = computed(() =>
    hasUsableFleetFilterParams(options.lastDeviceQueryParams.value)
  )

  const refreshSavedFleetFilters = async () => {
    const storage = getStorage()
    const localFilters = loadSavedFleetFilters(storage)
    savedFleetFilters.value = localFilters

    try {
      const response = await listFleetSavedFilters()
      const data = (response as any).data ?? response
      const serverFilters = normalizeServerFleetSavedFilters(data?.list ?? [])
      const localOnly = mergeSavedFleetFilters(localFilters, serverFilters).filter(
        (filter) =>
          !serverFilters.some(
            (item) => item.name === filter.name && JSON.stringify(item.params) === JSON.stringify(filter.params)
          )
      )
      const importedFilters: SavedFleetFilter[] = []
      for (const filter of localOnly) {
        const response = await createFleetSavedFilter(
          buildFleetSavedFilterPayload(filter.params, filter.previewTotal, filter.name)
        )
        const created = normalizeServerFleetSavedFilter(((response as any).data ?? response) as any)
        if (created) importedFilters.push(created)
      }
      savedFleetFilters.value = mergeSavedFleetFilters([...importedFilters, ...serverFilters], localFilters)
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
    } catch {
      savedFleetFilters.value = localFilters
    }
  }

  const saveCurrentFleetFilter = async () => {
    const storage = getStorage()
    try {
      const response = await createFleetSavedFilter(
        buildFleetSavedFilterPayload(
          options.lastDeviceQueryParams.value,
          options.targetPreviewTotal.value
        )
      )
      const created = normalizeServerFleetSavedFilter(((response as any).data ?? response) as any)
      if (created) {
        savedFleetFilters.value = mergeSavedFleetFilters([created], savedFleetFilters.value)
        saveFleetFiltersToStorage(storage, savedFleetFilters.value)
        options.message?.success?.(options.t('custom.devicePage.fleetFilterSaved'))
        return
      }
    } catch {
      // Keep the operator flow usable when the backend saved-filter API is unavailable.
    }

    savedFleetFilters.value = saveFleetFilter(
      storage,
      savedFleetFilters.value,
      options.lastDeviceQueryParams.value,
      options.targetPreviewTotal.value
    )
    options.message?.success?.(options.t('custom.devicePage.fleetFilterSaved'))
  }

  const applySavedFleetFilter = (filterID: string | number) => {
    const filter = savedFleetFilters.value.find((item) => item.id === String(filterID))
    if (!filter) return

    options.activeFleetTargetPreset.value = 'all'
    options.tablePageRef.value?.forceChangeParamsByKey?.(filter.params)
  }

  const openSavedFleetFilterCommandContext = (filterID: string | number) => {
    const filter = savedFleetFilters.value.find((item) => item.id === String(filterID))
    if (!filter) return

    const route = buildSavedFilterCommandCenterRoute(filter.params, filter.previewTotal, filter)
    if (!route) {
      options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterEmpty'))
      return
    }

    options.router.push(route)
  }

  const deleteSavedFleetFilter = async (filterID: string | number) => {
    const id = String(filterID)
    const storage = getStorage()
    const target = savedFleetFilters.value.find((item) => item.id === id)
    if (target && target.owned === false) {
      options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterNotOwned'))
      return
    }
    const nextFilters = savedFleetFilters.value.filter((item) => item.id !== id)

    if (!id.startsWith('fleet-filter-')) {
      try {
        await deleteFleetSavedFilter(id)
      } catch {
        options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterDeleteFailed'))
        return
      }
    }

    savedFleetFilters.value = nextFilters
    saveFleetFiltersToStorage(storage, savedFleetFilters.value)
    options.message?.success?.(options.t('custom.devicePage.savedFleetFilterDeleted'))
  }

  const renameSavedFleetFilter = async (filterID: string | number, name: string) => {
    const id = String(filterID)
    const nextName = name.trim()
    if (!nextName) {
      options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterNameRequired'))
      return false
    }

    const filter = savedFleetFilters.value.find((item) => item.id === id)
    if (!filter) return false
    if (filter.owned === false) {
      options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterNotOwned'))
      return false
    }

    const storage = getStorage()
    if (id.startsWith('fleet-filter-')) {
      savedFleetFilters.value = savedFleetFilters.value.map((item) =>
        item.id === id ? { ...item, name: nextName } : item
      )
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
      options.message?.success?.(options.t('custom.devicePage.savedFleetFilterRenamed'))
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
      options.message?.success?.(options.t('custom.devicePage.savedFleetFilterRenamed'))
      return true
    } catch (error) {
      options.message?.warning?.(
        getApiErrorMessage(error) || options.t('custom.devicePage.savedFleetFilterRenameFailed')
      )
      return false
    }
  }

  const shareSavedFleetFilter = async (filterID: string | number, shared: boolean) => {
    const id = String(filterID)
    const filter = savedFleetFilters.value.find((item) => item.id === id)
    if (!filter) return false
    if (filter.owned === false) {
      options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterNotOwned'))
      return false
    }
    // Sharing lives on the backend row; a local-only filter must be persisted first.
    if (id.startsWith('fleet-filter-')) {
      options.message?.warning?.(options.t('custom.devicePage.savedFleetFilterShareNeedsSync'))
      return false
    }

    const storage = getStorage()
    try {
      const response = await updateFleetSavedFilter(
        id,
        buildFleetSavedFilterPayload(filter.params, filter.previewTotal, filter.name, shared)
      )
      const updated = normalizeServerFleetSavedFilter(((response as any).data ?? response) as any)
      savedFleetFilters.value = savedFleetFilters.value.map((item) =>
        item.id === id ? updated || { ...item, shared } : item
      )
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
      options.message?.success?.(
        options.t(shared ? 'custom.devicePage.savedFleetFilterShared' : 'custom.devicePage.savedFleetFilterUnshared')
      )
      return true
    } catch (error) {
      options.message?.warning?.(
        getApiErrorMessage(error) || options.t('custom.devicePage.savedFleetFilterShareFailed')
      )
      return false
    }
  }

  return {
    savedFleetFilters,
    savedFleetFilterOptions,
    canSaveCurrentFleetFilter,
    refreshSavedFleetFilters,
    saveCurrentFleetFilter,
    applySavedFleetFilter,
    openSavedFleetFilterCommandContext,
    deleteSavedFleetFilter,
    renameSavedFleetFilter,
    shareSavedFleetFilter
  }
}
