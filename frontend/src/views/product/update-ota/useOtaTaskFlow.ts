import { computed, reactive, ref, watch, type Ref } from 'vue'
import {
  buildOtaTaskPreviewPayload,
  buildOtaTaskSavePayload,
  canSaveOtaTask,
  getOtaDeviceCandidateId,
  hasOtaTaskDeviceFilter,
  otaTaskSaveValidationKey,
  type OtaDeviceFilter,
  type OtaTaskDeviceFilterPayload,
  type OtaDeviceCandidate,
  type OtaTaskFormState
} from './ota-task-state'
import type { OtaPackageRecord } from './ota-task-types'
import { buildOtaTaskPreflightView } from './ota-task-preflight-view'
import {
  compactFleetFilterParams,
  loadSavedFleetFilters,
  mergeSavedFleetFilters,
  normalizeServerFleetSavedFilters,
  saveFleetFiltersToStorage,
  type SavedFleetFilter
} from '../../device/manage/device-fleet-saved-filters'
import {
  buildFleetRolloutSelectionResult,
  FLEET_CURRENT_PAGE_SCOPE,
  FLEET_DEVICE_FILTER_SCOPE,
  FLEET_FILTER_RESULT_SCOPE,
  type FleetRolloutContext,
  type FleetRolloutSelectionResult
} from '../../device/modules/fleet-rollout-context'

type Translate = (key: string) => string

type MessageApi = {
  success?: (message: string) => void
  warning?: (message: string) => void
}

type OtaTaskFlowData = {
  selectedPackageId: Ref<string | null>
  selectedPackage: Ref<OtaPackageRecord | null>
  deviceCandidates: Ref<OtaDeviceCandidate[]>
  deviceOptions: Ref<unknown[]>
  deviceLoading: Ref<boolean>
  fetchDevices: (search?: string, pageSize?: number) => Promise<void>
  fetchTasks: () => Promise<void>
  clearDeviceCandidates: () => void
}

type OtaTaskFlowServices = {
  addTask: (payload: ReturnType<typeof buildOtaTaskSavePayload>) => Promise<{ error?: unknown }>
  previewTask?: (payload: ReturnType<typeof buildOtaTaskPreviewPayload>) => Promise<{ data?: any; error?: unknown }>
  listFleetSavedFilters?: () => Promise<any>
}

const CURRENT_PAGE_COMPATIBILITY_MIN_PAGE_SIZE = 50
const CURRENT_PAGE_COMPATIBILITY_EXTRA_ROWS = 20
const CURRENT_PAGE_COMPATIBILITY_MAX_PAGE_SIZE = 300

const defaultStorage = () => (typeof window === 'undefined' ? null : window.localStorage)

const toOtaDeviceFilter = (params: Record<string, unknown>) => {
  const filter: OtaDeviceFilter = {}
  Object.entries(compactFleetFilterParams(params)).forEach(([key, value]) => {
    if (typeof value === 'number' && Number.isFinite(value)) filter[key] = value
    else if (typeof value === 'boolean') filter[key] = value
    else if (typeof value === 'string' && value.trim()) filter[key] = value.trim()
  })
  return filter
}

const buildSavedFleetPreselectionResult = (
  filter: SavedFleetFilter,
  deviceFilter: OtaDeviceFilter
): FleetRolloutSelectionResult => ({
  source: 'saved_fleet_filter',
  scope: FLEET_DEVICE_FILTER_SCOPE,
  requestedCount: filter.previewTotal ?? 0,
  requestedTotal: filter.previewTotal,
  currentPageCount: null,
  selectedCount: 0,
  excludedCount: 0,
  selectedDeviceIds: [],
  deviceFilter
})

const currentPageCompatibilityDevicePageSize = (context: FleetRolloutContext) => {
  const requestedRows = Math.max(context.deviceIds.length, context.currentPageCount ?? 0)
  if (requestedRows <= 0) return CURRENT_PAGE_COMPATIBILITY_MIN_PAGE_SIZE

  return Math.min(
    Math.max(requestedRows + CURRENT_PAGE_COMPATIBILITY_EXTRA_ROWS, CURRENT_PAGE_COMPATIBILITY_MIN_PAGE_SIZE),
    CURRENT_PAGE_COMPATIBILITY_MAX_PAGE_SIZE
  )
}

export const useOtaTaskFlow = (options: {
  data: OtaTaskFlowData
  services: OtaTaskFlowServices
  t: Translate
  message: MessageApi
  fleetRolloutContext?: Ref<FleetRolloutContext | null>
}) => {
  const saving = ref(false)
  const taskModalVisible = ref(false)
  const fleetPreselectionResult = ref<FleetRolloutSelectionResult | null>(null)
  const filterPreviewResult = ref<any | null>(null)
  const savedFleetFilters = ref<SavedFleetFilter[]>([])
  const savedFleetFiltersLoading = ref(false)
  const savedFleetFilterLoadFailed = ref(false)
  const selectedSavedFleetFilterId = ref<string | null>(null)
  const taskForm = reactive<OtaTaskFormState>({
    name: '',
    description: '',
    device_id_list: []
  })

  const savedFleetFilterOptions = computed(() =>
    savedFleetFilters.value.map((filter) => ({
      label: `${filter.name}${typeof filter.previewTotal === 'number' ? ` (${filter.previewTotal})` : ''}`,
      value: filter.id
    }))
  )
  const selectedSavedFleetFilter = computed(
    () => savedFleetFilters.value.find((filter) => filter.id === selectedSavedFleetFilterId.value) || null
  )
  const savedFleetFilterPayload = computed<OtaTaskDeviceFilterPayload>(() => {
    if (!selectedSavedFleetFilter.value) return {}
    const deviceFilter = toOtaDeviceFilter(selectedSavedFleetFilter.value.params)
    if (!hasOtaTaskDeviceFilter(deviceFilter)) return {}

    return {
      device_filter: deviceFilter,
      expected_total: selectedSavedFleetFilter.value.previewTotal ?? undefined,
      max_devices: 5000
    }
  })
  const fleetFilterPayload = computed<OtaTaskDeviceFilterPayload>(() => {
    if (hasOtaTaskDeviceFilter(savedFleetFilterPayload.value.device_filter)) return savedFleetFilterPayload.value

    const context = options.fleetRolloutContext?.value
    const supportedScope =
      context?.scope === FLEET_FILTER_RESULT_SCOPE ||
      context?.scope === FLEET_DEVICE_FILTER_SCOPE ||
      context?.scope === FLEET_CURRENT_PAGE_SCOPE
    if (!context || context.source !== 'device_manage' || !supportedScope) return {}

    return {
      device_filter: context.deviceFilter,
      expected_total: context.requestedTotal ?? undefined,
      max_devices: 5000
    }
  })
  const isFleetFilterRollout = computed(() => hasOtaTaskDeviceFilter(fleetFilterPayload.value.device_filter))
  const canSaveTask = computed(() =>
    canSaveOtaTask(options.data.selectedPackageId.value, taskForm, fleetFilterPayload.value.device_filter)
  )
  const showNoEligibleDeviceAlert = computed(
    () => taskModalVisible.value && !options.data.deviceLoading.value && !options.data.deviceOptions.value.length
  )
  const taskPreflightView = computed(() =>
    buildOtaTaskPreflightView(
      options.data.deviceCandidates.value,
      taskForm.device_id_list,
      options.data.selectedPackage.value,
      options.t
    )
  )
  const taskPreflight = computed(() => taskPreflightView.value.summary)
  const taskRiskDevices = computed(() => taskPreflightView.value.riskDevices)
  const taskPreflightItems = computed(() => taskPreflightView.value.items)

  const resetTaskForm = () => {
    taskForm.name = ''
    taskForm.description = ''
    taskForm.device_id_list = []
    fleetPreselectionResult.value = null
    filterPreviewResult.value = null
    selectedSavedFleetFilterId.value = null
    savedFleetFilterLoadFailed.value = false
    options.data.clearDeviceCandidates()
  }

  const loadSavedFleetFiltersForTask = async () => {
    if (!options.services.listFleetSavedFilters) return

    const storage = defaultStorage()
    const localFilters = loadSavedFleetFilters(storage)
    savedFleetFilters.value = localFilters
    savedFleetFiltersLoading.value = true
    savedFleetFilterLoadFailed.value = false
    try {
      const response = await options.services.listFleetSavedFilters()
      const data = (response as any).data ?? response
      const serverFilters = normalizeServerFleetSavedFilters(data?.list ?? [])
      savedFleetFilters.value = mergeSavedFleetFilters(serverFilters, localFilters)
      saveFleetFiltersToStorage(storage, savedFleetFilters.value)
    } catch {
      savedFleetFilters.value = localFilters
      savedFleetFilterLoadFailed.value = true
    } finally {
      savedFleetFiltersLoading.value = false
    }
  }

  const openTaskModal = async () => {
    if (!options.data.selectedPackageId.value) {
      options.message.warning?.(options.t('page.product.update-package.packagePlaceholder'))
      return
    }
    resetTaskForm()
    taskModalVisible.value = true
    await loadSavedFleetFiltersForTask()
    const context = options.fleetRolloutContext?.value
    const needsCurrentPageCompatibilityLoad =
      context?.source === 'device_manage' &&
      context.scope === FLEET_CURRENT_PAGE_SCOPE &&
      context.deviceIds.length > 0 &&
      !isFleetFilterRollout.value
    await options.data.fetchDevices(
      '',
      needsCurrentPageCompatibilityLoad ? currentPageCompatibilityDevicePageSize(context) : undefined
    )
    const preselection = buildFleetRolloutSelectionResult(
      context,
      options.data.deviceCandidates.value,
      getOtaDeviceCandidateId
    )
    if (preselection) {
      fleetPreselectionResult.value = preselection
      taskForm.device_id_list = preselection.selectedDeviceIds
    }
  }

  watch(selectedSavedFleetFilter, (filter) => {
    filterPreviewResult.value = null
    if (!filter) {
      const context = options.fleetRolloutContext?.value
      const preselection = buildFleetRolloutSelectionResult(
        context,
        options.data.deviceCandidates.value,
        getOtaDeviceCandidateId
      )
      fleetPreselectionResult.value = preselection
      taskForm.device_id_list = preselection?.selectedDeviceIds || []
      return
    }

    const deviceFilter = savedFleetFilterPayload.value.device_filter || {}
    fleetPreselectionResult.value = buildSavedFleetPreselectionResult(filter, deviceFilter)
    taskForm.device_id_list = []
  })

  const saveTask = async () => {
    const validationKey = otaTaskSaveValidationKey(
      options.data.selectedPackageId.value,
      taskForm,
      fleetFilterPayload.value.device_filter
    )
    if (validationKey) {
      options.message.warning?.(options.t(validationKey))
      return
    }
    saving.value = true
    try {
      let saveFilterPayload = fleetFilterPayload.value
      if (isFleetFilterRollout.value && options.services.previewTask && !filterPreviewResult.value) {
        const { data, error } = await options.services.previewTask(
          buildOtaTaskPreviewPayload(options.data.selectedPackageId.value as string, fleetFilterPayload.value)
        )
        if (error) {
          options.message.warning?.(options.t('common.operationFailed'))
          return
        }

        const selectedCount = Number(data?.selected_count || data?.data?.selected_count || 0)
        const overLimit = Boolean(data?.over_limit ?? data?.data?.over_limit)
        const maxDevices = Number(data?.max_devices || data?.data?.max_devices || fleetFilterPayload.value.max_devices || 5000)
        const totalMatched = Number(data?.total_matched || data?.data?.total_matched || selectedCount)
        if (selectedCount <= 0) {
          options.message.warning?.(options.t('page.product.update-ota.filterPreviewNoDevices'))
          return
        }
        if (overLimit) {
          options.message.warning?.(
            options
              .t('page.product.update-ota.filterPreviewOverLimit')
              .replace('{selected}', String(selectedCount))
              .replace('{max}', String(maxDevices))
          )
          return
        }
        filterPreviewResult.value = {
          selected_count: selectedCount,
          total_matched: totalMatched,
          max_devices: maxDevices,
          preview_devices: data?.preview_devices || data?.data?.preview_devices || []
        }
        options.message.success?.(options.t('page.product.update-ota.filterPreviewReady'))
        return
      }

      if (isFleetFilterRollout.value && filterPreviewResult.value) {
        saveFilterPayload = {
          ...fleetFilterPayload.value,
          expected_total: Number(filterPreviewResult.value.total_matched || 0)
        }
      }

      const { error } = await options.services.addTask(
        buildOtaTaskSavePayload(options.data.selectedPackageId.value as string, taskForm, saveFilterPayload)
      )
      if (!error) {
        options.message.success?.(options.t('common.saveSuccess'))
        taskModalVisible.value = false
        await options.data.fetchTasks()
      }
    } finally {
      saving.value = false
    }
  }

  return {
    saving,
    taskModalVisible,
    taskForm,
    canSaveTask,
    showNoEligibleDeviceAlert,
    taskPreflight,
    taskRiskDevices,
    taskPreflightItems,
    fleetPreselectionResult,
    filterPreviewResult,
    isFleetFilterRollout,
    savedFleetFilters,
    savedFleetFiltersLoading,
    savedFleetFilterLoadFailed,
    savedFleetFilterOptions,
    selectedSavedFleetFilterId,
    selectedSavedFleetFilter,
    openTaskModal,
    saveTask
  }
}
