<!--
  Device management list: device search, group filtering, online-state updates,
  service-access filters, fleet actions, and RDI activation entry points.

  Composition: the page wires per-feature composables (fleet ops, service-access filters,
  quick actions, add-device drawer) into the shared `data-table-page`; the query contract lives
  in `device-search-configs.ts`.
-->
<script setup lang="tsx">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DeviceManage')
import { checkDevice, deleteDevice as deleteDeviceApi, deviceGroupRelation, deviceList } from '@/service/api/device'
import { activateRdiDevice } from '@/service/api/rdi'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { usePageCache } from '../../../utils/usePageCache'
import { createDeviceManageColumns } from './device-table-columns'
import DeviceFleetTargetToolbar from './DeviceFleetTargetToolbar.vue'
import DeviceFleetOverview from './DeviceFleetOverview.vue'
import DeviceManageEmptyState from './DeviceManageEmptyState.vue'
import DeviceManageFleetGroupModal from './DeviceManageFleetGroupModal.vue'
import DeviceManageFleetSummaryModal from './DeviceManageFleetSummaryModal.vue'
import DeviceManageFleetScopeModal from './DeviceManageFleetScopeModal.vue'
import { useDeviceManageServiceAccessFilters } from './useDeviceManageServiceAccessFilters'
import { useDeviceManageFleetOperations } from './useDeviceManageFleetOperations'
import { useDeviceManageActivationFlow } from './useDeviceManageActivationFlow'
import { useDeviceManageStatusSubscription } from './useDeviceManageStatusSubscription'
import { useDeviceManageQuickActions } from './useDeviceManageQuickActions'
import { useDeviceManageAddDrawer } from './useDeviceManageAddDrawer'
import { createDeviceManageSearchConfigs } from './device-search-configs'
import { loadDeviceGroupOptions } from './device-manage-options'

const AddDeviceDrawer = defineAsyncComponent(() => import('@/views/device/manage/modules/add-device-drawer.vue'))
const DeviceManageQuickActions = defineAsyncComponent(() => import('./DeviceManageQuickActions.vue'))
const tablePageRef = ref()
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

const confirmDeleteDevice = (row: any) => {
  const id = String(row?.id || '')
  if (!id) return
  window.$dialog?.warning({
    title: $t('common.delete'),
    content: $t('common.confirmDelete'),
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const { error } = await deleteDeviceApi({ id })
      if (!error) {
        window.$message?.success($t('common.deleteSuccess'))
        refreshDeviceTable()
      }
    }
  })
}

const columns_to_show = ref(
  createDeviceManageColumns(goDeviceDetails, openEditDevice, confirmDeleteDevice, openShareDevice, openIssueClaimToken)
)
const actions = []

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
primeInitialServiceAccessFilter()

const dropOption = [
  {
    label: () => $t('custom.devicePage.manualAdd'),
    key: 'hands'
  },
  {
    label: () => $t('custom.devicePage.addByNumber'),
    key: 'number',
    disabled: false
  }
]

const topActions = [
  {
    element: () => (
      <DeviceFleetTargetToolbar
        presets={fleetTargetPresets}
        activePreset={activeFleetTargetPreset.value}
        targetPreviewTotal={targetPreviewTotal.value}
        currentPageDeviceCount={currentPageDeviceCount.value}
        savedFilterOptions={savedFleetFilterOptions.value}
        savedFilterCount={savedFleetFilters.value.length}
        canSaveCurrentFleetFilter={canSaveCurrentFleetFilter.value}
        selectedDeviceCount={selectedFleetDeviceIds.value.length}
        selectionScope={fleetSelectionScope.value}
        selectionScopeMessage={fleetSelectionScopeMessage.value}
        canSelectAllMatching={canSelectAllMatchingDevices.value}
        onSelectAllMatching={selectAllMatchingFleetDevices}
        onClearSelectAllMatching={clearFleetSelectAllMatching}
        onOpenSelectAllCommandContext={openFleetSelectAllCommandContext}
        onApplyPreset={applyFleetTargetPreset}
        onSaveFilter={saveCurrentFleetFilter}
        onRefreshSavedFilters={refreshSavedFleetFilters}
        onApplySavedFilter={applySavedFleetFilter}
        onOpenSavedFilterCommandContext={openSavedFleetFilterCommandContext}
        onDeleteSavedFilter={deleteSavedFleetFilter}
        onRenameSavedFilter={renameSavedFleetFilter}
        onShareSavedFilter={shareSavedFleetFilter}
        onExportCurrentPage={exportCurrentFleetPage}
        onAddSelectedToGroup={openSelectedDeviceGroupDialog}
        onShowSelectedSummary={openSelectedFleetSummary}
        onOpenOtaContext={openFleetOtaContext}
        onOpenAlarmContext={openFleetAlarmContext}
        onOpenCommandContext={openSelectedDeviceCommandContext}
        onOpenConfigContext={openFleetConfigContext}
        onOpenAuditContext={openFleetAuditContext}
      />
    )
  },
  {
    element: () => (
      <n-button onClick={() => router.push('/device/shared-with-me')}>{$t('route.device_shared-with-me')}</n-button>
    )
  },
  {
    element: () => <n-button onClick={openClaimDeviceDialog}>{$t('custom.devicePage.claimDevice')}</n-button>
  },
  {
    element: () => (
      <n-dropdown options={dropOption} trigger="click" onSelect={handleSelect}>
        <n-button type="primary">+{$t('custom.devicePage.addDevice')}</n-button>
      </n-dropdown>
    )
  }
]

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

onMounted(() => {
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
    <data-table-page
      ref="tablePageRef"
      :fetch-data="fetchData"
      :columns-to-show="columns_to_show as any"
      :table-actions="actions"
      :search-configs="searchConfigs"
      :top-actions="topActions"
      :init-page="query.page"
      :init-page-size="query.page_size"
      :row-click="goDeviceDetails"
      selectable-rows
      @params-update="paramsUpdateHandle"
      @selection-update="handleFleetSelectionUpdate"
    >
      <template #empty="{ reset, searchCriteria }">
        <DeviceManageEmptyState
          :search-criteria="searchCriteria"
          :first-device-onboarding="isFirstDeviceOnboarding"
          @add-device="openManualDeviceAdd"
          @open-service-access="openServiceAccess"
          @clear-filters="reset"
          @back-home="returnToHomeGuide"
        />
      </template>
    </data-table-page>
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
