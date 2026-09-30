<!--
  Device management list: device search, group filtering, online-state updates,
  service-access filters, fleet actions, and RDI activation entry points.

  Composition: the page drives `useListPage` (from @/components/data-table-page/useListPage)
  directly through `useDeviceManageListPage`, which keeps the old <data-table-page> bridge
  contract (dataList / selectedRows / handleSearch / handleReset / forceChangeParamsByKey /
  clearSelection) so the fleet-operations, status-subscription and service-access-filter
  composables work unchanged. The wrapper's UI surface lives in the extracted components of
  this directory (DeviceManageListShell + search form / card view), header actions in
  `device-manage-top-actions.tsx`, row actions in `useDeviceManageRowActions.ts`; the query
  contract lives in `device-search-configs.ts`.
-->
<script setup lang="tsx">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createLogger } from '@/utils/logger'
import { checkDevice, deviceGroupRelation, deviceList } from '@/service/api/device'
import { activateRdiDevice } from '@/service/api/rdi'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { usePageCache } from '../../../utils/usePageCache'
import DeviceFleetOverview from './DeviceFleetOverview.vue'
import DeviceManageListShell from './DeviceManageListShell.vue'
import DeviceManageFleetGroupModal from './DeviceManageFleetGroupModal.vue'
import DeviceManageFleetSummaryModal from './DeviceManageFleetSummaryModal.vue'
import DeviceManageFleetScopeModal from './DeviceManageFleetScopeModal.vue'
import { createDeviceManageTopActions } from './device-manage-top-actions'
import { deviceManageRowProps, useDeviceManageListPage, type DeviceManageTableBridge } from './useDeviceManageListPage'
import { useDeviceManageRowActions } from './useDeviceManageRowActions'
import { useDeviceManageServiceAccessFilters } from './useDeviceManageServiceAccessFilters'
import { useDeviceManageFleetOperations } from './useDeviceManageFleetOperations'
import { useDeviceManageActivationFlow } from './useDeviceManageActivationFlow'
import { useDeviceManageStatusSubscription } from './useDeviceManageStatusSubscription'
import { useDeviceManageQuickActions } from './useDeviceManageQuickActions'
import { useDeviceManageAddDrawer } from './useDeviceManageAddDrawer'
import { createDeviceManageSearchConfigs } from './device-search-configs'
import { loadDeviceGroupOptions } from './device-manage-options'

const logger = createLogger('DeviceManage')

const AddDeviceDrawer = defineAsyncComponent(() => import('@/views/device/manage/modules/add-device-drawer.vue'))
const DeviceManageQuickActions = defineAsyncComponent(() => import('./DeviceManageQuickActions.vue'))
const tablePageRef = ref<DeviceManageTableBridge>()
const route: any = useRoute()
const router: any = useRouter()
const isFirstDeviceOnboarding = computed(() => route.query?.onboarding === 'first-device')

const { cache: query, setCache } = usePageCache()

const { routerPushByKey } = useRouterPush()
const goDeviceDetails = (row) => {
  routerPushByKey('device_details', {
    query: {
      d_id: row.id
    }
  })
}

const {
  quickActionsRef: deviceManageQuickActionsRef,
  quickActionsVisited,
  openEditDevice,
  openShareDevice,
  openIssueClaimToken,
  openClaimDeviceDialog
} = useDeviceManageQuickActions()

const refreshDeviceTable = () => {
  tablePageRef.value?.handleSearch?.()
}

const { columnsToShow } = useDeviceManageRowActions({
  goDeviceDetails,
  openEditDevice,
  openShareDevice,
  openIssueClaimToken,
  refresh: refreshDeviceTable
})

const { scheduleDeviceStatusSubscription } = useDeviceManageStatusSubscription({
  tablePageRef,
  logger
})

const {
  activeFleetTargetPreset,
  targetPreviewTotal,
  savedFleetFilters,
  currentPageDeviceCount,
  currentPageFleetSummary,
  selectedFleetDeviceIds,
  savedFleetFilterOptions,
  canSaveCurrentFleetFilter,
  bulkGroupModalVisible,
  selectedFleetSummaryVisible,
  bulkGroupOptions,
  bulkGroupId,
  bulkGroupAssigning,
  fleetScopeConfirmVisible,
  pendingFleetScopeAction,
  selectedFleetSummary,
  selectedFleetDeviceIdentifiers,
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
  syncFleetQueryResult,
  fleetSelectionScope,
  fleetSelectionScopeMessage,
  canSelectAllMatchingDevices,
  selectAllMatchingFleetDevices,
  clearFleetSelectAllMatching,
  openFleetSelectAllCommandContext
} = useDeviceManageFleetOperations({
  tablePageRef,
  router,
  t: $t,
  message: (window as any).$message,
  getGroupOptions: loadDeviceGroupOptions,
  assignDevicesToGroup: deviceGroupRelation
})

const {
  active,
  addDrawerVisited,
  addKey,
  placement,
  current,
  currentStatus,
  isSuccess,
  configOptions,
  deviceId,
  deviceObj,
  manualDeviceNumber,
  configId,
  formData,
  setUpId,
  activate,
  loadConfigOptions,
  setIsSuccess,
  completeHandAdd
} = useDeviceManageAddDrawer({
  openServiceAccess: () => router.push('/device/service-access'),
  onCompleted: refreshDeviceTable
})

// searchConfigs is the device-management page query contract.
const searchConfigs = ref(
  createDeviceManageSearchConfigs(query, {
    getDeviceGroupOptions: loadDeviceGroupOptions,
    getDeviceConfigOptions: loadConfigOptions
  })
)

const { initializeServiceAccessFiltersInBackground, paramsUpdateHandle, primeInitialServiceAccessFilter } =
  useDeviceManageServiceAccessFilters({
    searchConfigs,
    tablePageRef,
    initialServiceIdentifier: route.query.service_identifier,
    initialServiceAccessId: route.query.service_access_id
  })
// 在 table bridge 就绪前调用（与迁移前一致：此时 forceChangeParamsByKey 是 no-op，只补插配置项）。
primeInitialServiceAccessFilter()

const topActions = createDeviceManageTopActions({
  fleetTargetPresets,
  activeFleetTargetPreset,
  targetPreviewTotal,
  currentPageDeviceCount,
  savedFleetFilterOptions,
  savedFleetFilters,
  canSaveCurrentFleetFilter,
  selectedFleetDeviceIds,
  fleetSelectionScope,
  fleetSelectionScopeMessage,
  canSelectAllMatchingDevices,
  onSelectAllMatching: selectAllMatchingFleetDevices,
  onClearSelectAllMatching: clearFleetSelectAllMatching,
  onOpenSelectAllCommandContext: openFleetSelectAllCommandContext,
  onApplyPreset: applyFleetTargetPreset,
  onSaveFilter: saveCurrentFleetFilter,
  onRefreshSavedFilters: refreshSavedFleetFilters,
  onApplySavedFilter: applySavedFleetFilter,
  onOpenSavedFilterCommandContext: openSavedFleetFilterCommandContext,
  onDeleteSavedFilter: deleteSavedFleetFilter,
  onRenameSavedFilter: renameSavedFleetFilter,
  onShareSavedFilter: shareSavedFleetFilter,
  onExportCurrentPage: exportCurrentFleetPage,
  onAddSelectedToGroup: openSelectedDeviceGroupDialog,
  onShowSelectedSummary: openSelectedFleetSummary,
  onOpenOtaContext: openFleetOtaContext,
  onOpenAlarmContext: openFleetAlarmContext,
  onOpenCommandContext: openSelectedDeviceCommandContext,
  onOpenConfigContext: openFleetConfigContext,
  onOpenAuditContext: openFleetAuditContext,
  router,
  openClaimDeviceDialog,
  onSelectAddOption: handleSelect
})

const openManualDeviceAdd = () => {
  activate('bottom', 'hands')
}

const openServiceAccess = () => {
  router.push('/device/service-access')
}

const returnToHomeGuide = () => {
  router.push('/home')
}

onMounted(() => {
  if (route.query?.onboarding === 'first-device' && route.query?.add) {
    activate('bottom', 'hands')
  }
})

const { deviceNumber, buttonDisabled, showMessage, messageStyle, completeAdd } = useDeviceManageActivationFlow({
  checkDevice: checkDevice as unknown as Parameters<typeof useDeviceManageActivationFlow>[0]['checkDevice'],
  activateDevice: activateRdiDevice,
  logger,
  onActivated: () => {
    active.value = false
    refreshDeviceTable()
  }
})

function handleSelect(key: string | number) {
  activate('bottom', key)
}

const fetchData = async (params: Record<string, any>) => {
  setCache(params)
  const result = await deviceList(params)
  syncFleetQueryResult(params, result.error ? undefined : result.data?.total, result.data?.list)

  scheduleDeviceStatusSubscription()

  return result
}

const {
  rows,
  loading,
  total,
  page,
  pageSize,
  rowKey,
  searchCriteria,
  selectedRowKeys,
  generatedColumns,
  handleCheckedRowKeysUpdate,
  handleSearch,
  handleReset,
  setPage,
  setPageSize,
  load
} = useDeviceManageListPage({
  searchConfigs,
  tablePageRef,
  fetchData,
  columnsToShow,
  initPage: query.page,
  initPageSize: query.page_size,
  selectableRows: true,
  hooks: {
    onParamsUpdate: paramsUpdateHandle,
    onSelectionUpdate: handleFleetSelectionUpdate
  }
})

const rowProps = deviceManageRowProps(goDeviceDetails)

onMounted(() => {
  void load()
  void refreshSavedFleetFilters()
  void initializeServiceAccessFiltersInBackground()
})
</script>

<template>
  <div>
    <DeviceFleetOverview
      :current-page-summary="currentPageFleetSummary"
      :target-preview-total="targetPreviewTotal"
      :selected-device-count="selectedFleetDeviceIds.length"
      :active-preset="activeFleetTargetPreset"
      @apply-preset="applyFleetTargetPreset"
      @show-selected-summary="openSelectedFleetSummary"
      @export-current-page="exportCurrentFleetPage"
    />
    <DeviceManageListShell
      :configs="searchConfigs"
      :criteria="searchCriteria"
      :top-actions="topActions"
      :rows="rows"
      :loading="loading"
      :columns="generatedColumns as any"
      :selected-row-keys="selectedRowKeys"
      :row-key="rowKey"
      :row-props="rowProps"
      :total="total"
      :page="page"
      :page-size="pageSize"
      :first-device-onboarding="isFirstDeviceOnboarding"
      :row-click="goDeviceDetails"
      @search="handleSearch"
      @reset="handleReset"
      @refresh="load"
      @update:checked-row-keys="handleCheckedRowKeysUpdate"
      @update:page="setPage"
      @update:page-size="setPageSize"
      @add-device="openManualDeviceAdd"
      @open-service-access="openServiceAccess"
      @back-home="returnToHomeGuide"
    />
    <DeviceManageQuickActions v-if="quickActionsVisited" ref="deviceManageQuickActionsRef" @updated="refreshDeviceTable" />
    <DeviceManageFleetGroupModal
      v-model:show="bulkGroupModalVisible"
      v-model:group-id="bulkGroupId"
      :selected-count="selectedFleetDeviceIds.length"
      :group-options="bulkGroupOptions"
      :assigning="bulkGroupAssigning"
      @confirm="assignSelectedDevicesToGroup"
    />
    <DeviceManageFleetSummaryModal
      v-model:show="selectedFleetSummaryVisible"
      :summary="selectedFleetSummary"
      :device-identifiers="selectedFleetDeviceIdentifiers"
      @copy="copySelectedFleetDeviceIdentifiers"
    />
    <DeviceManageFleetScopeModal
      v-model:show="fleetScopeConfirmVisible"
      :action-label-key="pendingFleetScopeAction?.labelKey"
      :current-page-count="currentPageDeviceCount"
      :target-preview-total="targetPreviewTotal"
      :selected-count="selectedFleetDeviceIds.length"
      @confirm="confirmFleetCurrentPageAction"
      @cancel="cancelFleetCurrentPageAction"
    />
    <AddDeviceDrawer
      v-if="addDrawerVisited"
      v-model:show="active"
      v-model:manual-step="current"
      v-model:device-number="deviceNumber"
      :add-key="addKey"
      :placement="placement"
      :manual-status="currentStatus"
      :config-options="configOptions"
      :device-id="deviceId"
      :device-config-id="configId"
      :manual-device-number="manualDeviceNumber"
      :device-form-data="deviceObj"
      :form-elements="formData"
      :is-success="isSuccess"
      :button-disabled="buttonDisabled"
      :show-message="showMessage"
      :message-style="messageStyle"
      :first-device-onboarding="isFirstDeviceOnboarding"
      @after-leave="completeHandAdd"
      @set-up-id="setUpId"
      @set-is-success="setIsSuccess"
      @complete-number-add="completeAdd"
    />
  </div>
</template>
