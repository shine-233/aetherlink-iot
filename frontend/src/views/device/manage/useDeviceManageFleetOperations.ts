import { computed, ref, type Ref } from 'vue'
import type { Router } from 'vue-router'
import type { TreeSelectOption } from 'naive-ui/es/tree-select/src/interface'
import {
  buildFleetTargetPresetParams,
  fleetTargetPresets,
  type FleetTargetPresetKey
} from './device-fleet-target-presets'
import {
  buildFleetSelectionSummary,
  downloadFleetDeviceCsv
} from './device-fleet-operations'
import { buildSelectAllMatchingCommandCenterRoute } from './device-fleet-handoff-routes'
import {
  buildFleetSelectionScope,
  buildFleetSelectionScopeMessage,
  canSelectAllMatchingFleetDevices,
  normalizeFleetSelectAllMaxDevices,
  type FleetSelectionMode
} from './device-fleet-select-all'
import { useDeviceManageFleetCurrentPageActions } from './useDeviceManageFleetCurrentPageActions'
import { useDeviceManageFleetSavedFilters } from './useDeviceManageFleetSavedFilters'
import { useDeviceManageFleetSelectionActions } from './useDeviceManageFleetSelectionActions'

type TablePageRef = Ref<
  | {
      dataList?: any[]
      forceChangeParamsByKey?: (params: Record<string, unknown>) => void
      clearSelection?: () => void
      handleSearch?: () => void
    }
  | undefined
>

type MessageApi = {
  success?: (message: string) => void
  warning?: (message: string) => void
}

type UseDeviceManageFleetOperationsOptions = {
  tablePageRef: TablePageRef
  router: Router
  t: (key: string) => string
  message?: MessageApi
  getGroupOptions: () => Promise<TreeSelectOption[]>
  assignDevicesToGroup: (payload: {
    group_id: string | number
    device_id_list: string[]
  }) => Promise<{ error?: unknown }>
  getStorage?: () => Pick<Storage, 'getItem' | 'setItem'> | null
  /** 全量选择的安全上限，默认与 Command Center / 后端 device_filter 上限一致。 */
  selectAllMaxDevices?: number | null
}

export function useDeviceManageFleetOperations(options: UseDeviceManageFleetOperationsOptions) {
  const activeFleetTargetPreset = ref<FleetTargetPresetKey>('all')
  const targetPreviewTotal = ref<number | null>(null)
  const lastDeviceQueryParams = ref<Record<string, unknown>>({})
  const currentPageDeviceCount = ref(0)
  const currentPageFleetSummary = ref(buildFleetSelectionSummary([]))

  const fleetSelectionMode = ref<FleetSelectionMode>('current_page')
  const fleetSelectAllMaxDevices = ref(normalizeFleetSelectAllMaxDevices(options.selectAllMaxDevices))

  const getCurrentPageDeviceRows = () => options.tablePageRef.value?.dataList || []

  const applyFleetTargetPreset = (presetKey: FleetTargetPresetKey) => {
    activeFleetTargetPreset.value = presetKey
    fleetSelectionMode.value = 'current_page'
    options.tablePageRef.value?.forceChangeParamsByKey?.(buildFleetTargetPresetParams(presetKey))
  }

  const {
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
  } = useDeviceManageFleetSavedFilters({
    tablePageRef: options.tablePageRef,
    router: options.router,
    t: options.t,
    message: options.message,
    activeFleetTargetPreset,
    lastDeviceQueryParams,
    targetPreviewTotal,
    getStorage: options.getStorage
  })

  const {
    selectedFleetDeviceRows,
    selectedFleetDeviceIds,
    selectedFleetSummary,
    selectedFleetSummaryVisible,
    selectedFleetDeviceIdentifiers,
    bulkGroupModalVisible,
    bulkGroupOptions,
    bulkGroupId,
    bulkGroupAssigning,
    openSelectedDeviceGroupDialog,
    assignSelectedDevicesToGroup,
    handleFleetSelectionUpdate: applyFleetRowSelection,
    openSelectedFleetSummary,
    copySelectedFleetDeviceIdentifiers,
    openSelectedDeviceCommandContext,
    openFleetConfigContext
  } = useDeviceManageFleetSelectionActions({
    tablePageRef: options.tablePageRef,
    router: options.router,
    t: options.t,
    message: options.message,
    getGroupOptions: options.getGroupOptions,
    assignDevicesToGroup: options.assignDevicesToGroup
  })

  const {
    fleetScopeConfirmVisible,
    pendingFleetScopeAction,
    confirmFleetCurrentPageAction,
    cancelFleetCurrentPageAction,
    openFleetOtaContext,
    openFleetAlarmContext,
    openFleetAuditContext
  } = useDeviceManageFleetCurrentPageActions({
    tablePageRef: options.tablePageRef,
    router: options.router,
    t: options.t,
    message: options.message,
    lastDeviceQueryParams,
    targetPreviewTotal
  })

  const exportCurrentFleetPage = () => {
    const rows = getCurrentPageDeviceRows()
    if (rows.length === 0) {
      options.message?.warning?.(options.t('custom.devicePage.noFleetDevicesToOperate'))
      return
    }

    downloadFleetDeviceCsv(rows)
    options.message?.success?.(options.t('custom.devicePage.fleetExportStarted'))
  }

  const syncFleetQueryResult = (params: Record<string, unknown>, total?: number, rows?: any[]) => {
    const previousTotal = targetPreviewTotal.value
    lastDeviceQueryParams.value = { ...params }
    if (typeof total === 'number') {
      targetPreviewTotal.value = total
    }
    currentPageDeviceCount.value = Array.isArray(rows) ? rows.length : 0
    currentPageFleetSummary.value = buildFleetSelectionSummary(Array.isArray(rows) ? rows : [])

    // 筛选条件/结果总数变了以后，"全部匹配" 的含义也变了；退回当页语义，避免沿用过期的全量范围。
    if (fleetSelectionMode.value === 'all_matching' && typeof total === 'number' && total !== previousTotal) {
      fleetSelectionMode.value = 'current_page'
    }
  }

  // 操作员手动改动当页勾选时，"全部匹配" 语义立即失效，否则两种语义会同时显示、互相矛盾。
  const handleFleetSelectionUpdate = (rows: any[]) => {
    fleetSelectionMode.value = 'current_page'
    applyFleetRowSelection(rows)
  }

  const canSelectAllMatchingDevices = computed(() => canSelectAllMatchingFleetDevices(targetPreviewTotal.value))

  const fleetSelectionScope = computed(() =>
    buildFleetSelectionScope({
      mode: fleetSelectionMode.value,
      matchedTotal: targetPreviewTotal.value,
      currentPageCount: currentPageDeviceCount.value,
      checkedCount: selectedFleetDeviceIds.value.length,
      maxDevices: fleetSelectAllMaxDevices.value
    })
  )

  const fleetSelectionScopeMessage = computed(() => buildFleetSelectionScopeMessage(fleetSelectionScope.value))

  const selectAllMatchingFleetDevices = () => {
    if (!canSelectAllMatchingDevices.value) {
      options.message?.warning?.(options.t('custom.devicePage.fleetSelectAllUnavailable'))
      return
    }
    fleetSelectionMode.value = 'all_matching'
  }

  const clearFleetSelectAllMatching = () => {
    fleetSelectionMode.value = 'current_page'
  }

  const openFleetSelectAllCommandContext = () => {
    const scope = fleetSelectionScope.value
    if (scope.mode !== 'all_matching') {
      options.message?.warning?.(options.t('custom.devicePage.fleetSelectAllRequired'))
      return
    }

    const route = buildSelectAllMatchingCommandCenterRoute({
      params: lastDeviceQueryParams.value,
      matchedTotal: scope.matchedTotal,
      effectiveCount: scope.effectiveCount,
      maxDevices: scope.maxDevices
    })
    if (!route) {
      options.message?.warning?.(options.t('custom.devicePage.fleetSelectAllUnavailable'))
      return
    }

    options.router.push(route)
  }

  return {
    activeFleetTargetPreset,
    targetPreviewTotal,
    lastDeviceQueryParams,
    savedFleetFilters,
    currentPageDeviceCount,
    currentPageFleetSummary,
    selectedFleetDeviceRows,
    selectedFleetDeviceIds,
    selectedFleetSummary,
    selectedFleetSummaryVisible,
    selectedFleetDeviceIdentifiers,
    savedFleetFilterOptions,
    canSaveCurrentFleetFilter,
    bulkGroupModalVisible,
    bulkGroupOptions,
    bulkGroupId,
    bulkGroupAssigning,
    fleetScopeConfirmVisible,
    pendingFleetScopeAction,
    fleetSelectionMode,
    fleetSelectAllMaxDevices,
    fleetSelectionScope,
    fleetSelectionScopeMessage,
    canSelectAllMatchingDevices,
    selectAllMatchingFleetDevices,
    clearFleetSelectAllMatching,
    openFleetSelectAllCommandContext,
    fleetTargetPresets,
    applyFleetTargetPreset,
    refreshSavedFleetFilters,
    saveCurrentFleetFilter,
    applySavedFleetFilter,
    openSavedFleetFilterCommandContext,
    deleteSavedFleetFilter,
    renameSavedFleetFilter,
    shareSavedFleetFilter,
    exportCurrentFleetPage,
    openSelectedDeviceGroupDialog,
    assignSelectedDevicesToGroup,
    handleFleetSelectionUpdate,
    openSelectedFleetSummary,
    copySelectedFleetDeviceIdentifiers,
    confirmFleetCurrentPageAction,
    cancelFleetCurrentPageAction,
    openFleetOtaContext,
    openFleetAlarmContext,
    openSelectedDeviceCommandContext,
    openFleetConfigContext,
    openFleetAuditContext,
    syncFleetQueryResult
  }
}
