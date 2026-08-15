<!--
  Device management list: device search, group filtering, online-state updates,
  service-access filters, fleet actions, and RDI activation entry points.
-->
<script setup lang="tsx">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { DrawerPlacement, StepsProps } from 'naive-ui'
import type { TreeSelectOption } from 'naive-ui/es/tree-select/src/interface'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DeviceManage')
import { checkDevice, deleteDevice as deleteDeviceApi, deviceConnectForm, deviceGroupRelation, deviceGroupTree, deviceList, getDeviceConfigList } from '@/service/api/device'
import { activateRdiDevice } from '@/service/api/rdi'
import type { SearchConfig } from '@/components/data-table-page/types'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { usePageCache } from '../../../utils/usePageCache'
import { createDeviceManageColumns } from './device-table-columns'
import DeviceFleetTargetToolbar from './DeviceFleetTargetToolbar.vue'
import DeviceFleetOverview from './DeviceFleetOverview.vue'
import DeviceManageEmptyState from './DeviceManageEmptyState.vue'
import { useDeviceManageServiceAccessFilters } from './useDeviceManageServiceAccessFilters'
import { useDeviceManageFleetOperations } from './useDeviceManageFleetOperations'
import { useDeviceManageActivationFlow } from './useDeviceManageActivationFlow'
import { useDeviceManageStatusSubscription } from './useDeviceManageStatusSubscription'
import { buildLifecycleStatusOptions } from './device-lifecycle-filter'

const AddDeviceDrawer = defineAsyncComponent(() => import('@/views/device/manage/modules/add-device-drawer.vue'))
const DeviceManageQuickActions = defineAsyncComponent(() => import('./DeviceManageQuickActions.vue'))
const addKey = ref()
const configOptions = ref()
const deviceId = ref()
const deviceObj = ref()
const manualDeviceNumber = ref('')
const configId = ref()
const formData = ref()
const tablePageRef = ref()
const route: any = useRoute()
const router: any = useRouter()
const isFirstDeviceOnboarding = computed(() => route.query?.onboarding === 'first-device')

const { cache: query, setCache } = usePageCache()

const getFormJson = async (id) => {
  const res = await deviceConnectForm({ device_id: id })

  formData.value = res.data
}

const setUpId = (dId, cId, dobj, nextDeviceNumber = '') => {
  deviceId.value = dId
  manualDeviceNumber.value = nextDeviceNumber
  configId.value = cId
  deviceObj.value = JSON.parse(dobj)
  getFormJson(dId)
}
const getDeviceGroupOptions = async () => {
  // Convert backend group nodes into tree-select options.
  function convertTreeNodeToTarget(treeNode: DeviceManagement.TreeNode): TreeSelectOption {
    const { group, children } = treeNode
    const targetNode: TreeSelectOption = {
      label: group.name,
      key: group.id
    }

    if (children && children.length > 0) {
      targetNode.children = children.map(convertTreeNodeToTarget)
    }

    return targetNode
  }

  // Convert a TreeNode array into target tree-select option data.
  function convertTreeNodesToTarget(treeNodes: DeviceManagement.TreeNode[]): TreeSelectOption[] {
    return treeNodes.map(convertTreeNodeToTarget)
  }

  const res = await deviceGroupTree({})
  let options: any[] = []
  if (res.data) {
    options = convertTreeNodesToTarget(res.data)
  }
  return options
}

const getDeviceConfigOptions = async () => {
  const res = await getDeviceConfigList({
    page: 1,
    page_size: 99
    // device_type: pattern
  })
  let options: any[] = []
  if (res.data && res.data.list) {
    options = res.data.list
  }
  configOptions.value = [{ name: $t('custom.devicePage.unlimitedDeviceConfig'), id: '' }, ...options]

  return configOptions.value
}

const { routerPushByKey } = useRouterPush()
const goDeviceDetails = (row) => {
  routerPushByKey('device_details', {
    query: {
      d_id: row.id
    }
  })
}

type DeviceManageQuickActionsExpose = {
  openEditDevice: (row: any) => void
  openShareDevice: (row: any) => void
}

type PendingQuickAction =
  | {
      kind: 'edit' | 'share'
      row: any
    }
  | null

const deviceManageQuickActionsRef = ref<DeviceManageQuickActionsExpose | null>(null)
const quickActionsVisited = ref(false)
const pendingQuickAction = ref<PendingQuickAction>(null)

const refreshDeviceTable = () => {
  tablePageRef.value?.handleSearch?.()
}

const flushPendingQuickAction = () => {
  const instance = deviceManageQuickActionsRef.value
  const nextAction = pendingQuickAction.value
  if (!instance || !nextAction) return
  pendingQuickAction.value = null
  if (nextAction.kind === 'edit') {
    instance.openEditDevice(nextAction.row)
    return
  }
  instance.openShareDevice(nextAction.row)
}

watch(deviceManageQuickActionsRef, flushPendingQuickAction)

const openDeviceQuickAction = (kind: 'edit' | 'share', row: any) => {
  quickActionsVisited.value = true
  pendingQuickAction.value = { kind, row }
  flushPendingQuickAction()
}

const openEditDevice = (row: any) => {
  openDeviceQuickAction('edit', row)
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

const openShareDevice = (row: any) => {
  openDeviceQuickAction('share', row)
}

const columns_to_show = ref(createDeviceManageColumns(goDeviceDetails, openEditDevice, confirmDeleteDevice, openShareDevice))
const actions = []

const { scheduleDeviceStatusSubscription } = useDeviceManageStatusSubscription({
  tablePageRef,
  logger
})

// searchConfigs is the device-management page query contract.
const searchConfigs = ref<SearchConfig[]>([
  {
    key: 'group_id',
    label: 'custom.devicePage.selectGroup',
    type: 'tree-select',
    multiple: false,
    initValue: query.group_id,
    options: [{ label: $t('custom.devicePage.group'), key: '' }],
    loadOptions: getDeviceGroupOptions
  },
  {
    key: 'device_config_id',
    label: 'custom.devicePage.unlimitedDeviceConfig',
    type: 'select',
    options: [],
    initValue: query.device_config_id,
    labelField: 'name',
    valueField: 'id',
    loadOptions: getDeviceConfigOptions
  },
  {
    key: 'is_online',
    label: 'custom.devicePage.unlimitedOnlineStatus',
    type: 'select',
    initValue: query.is_online,
    options: [
      { label: () => $t('custom.devicePage.unlimitedOnlineStatus'), value: '' },
      { label: () => $t('custom.devicePage.online'), value: 1 },
      { label: () => $t('custom.devicePage.offline'), value: 0 }
    ]
  },
  {
    key: 'never_reported',
    label: 'custom.devicePage.reportHistoryStatus',
    type: 'select',
    initValue: query.never_reported,
    options: [
      { label: () => $t('custom.devicePage.allReportHistory'), value: '' },
      { label: () => $t('custom.devicePage.neverReported'), value: true },
      { label: () => $t('custom.devicePage.hasReported'), value: false }
    ]
  },
  {
    key: 'lifecycle_status',
    label: 'custom.devicePage.lifecycleStatus',
    type: 'select',
    initValue: query.lifecycle_status,
    options: buildLifecycleStatusOptions($t)
  },
  {
    key: 'last_reported_after',
    label: 'custom.devicePage.lastReportedAfter',
    type: 'date',
    initValue: query.last_reported_after
  },
  {
    key: 'last_reported_before',
    label: 'custom.devicePage.lastReportedBefore',
    type: 'date',
    initValue: query.last_reported_before
  },
  {
    key: 'warn_status',
    label: 'custom.devicePage.unlimitedAlarmStatus',
    type: 'select',
    initValue: query.warn_status,
    options: [
      { label: () => $t('custom.devicePage.unlimitedAlarmStatus'), value: '' },
      { label: () => $t('custom.devicePage.alarm'), value: 'Y' },
      { label: () => $t('custom.devicePage.noAlarm'), value: 'N' }
    ]
  },
  {
    key: 'device_type',
    label: 'custom.devicePage.unlimitedAccessType',
    initValue: query.device_type,
    type: 'select',
    options: [
      { label: $t('custom.devicePage.unlimitedAccessType'), value: '' },
      { label: $t('custom.devicePage.directConnectedDevices'), value: '1' },
      { label: $t('custom.devicePage.gateway'), value: '2' },
      { label: $t('custom.devicePage.gatewaySubEquipment'), value: '3' }
      // { label: $t('custom.devicePage.byProtocol'), value: 'A' },
      // { label: $t('custom.devicePage.byService'), value: 'B' }
    ]
  },
  {
    key: 'service_identifier',
    label: 'card.anyProtocolService',
    type: 'select',
    initValue: query.service_identifier,
    options: [{ label: $t('card.anyProtocolService'), value: '' }]
  },
  {
    key: 'search',
    initValue: query.search,
    label: 'custom.devicePage.deviceSearch',
    type: 'input'
  },
  {
    key: 'name',
    initValue: query.name,
    label: 'custom.devicePage.deviceName',
    type: 'input'
  },
  {
    key: 'device_number',
    initValue: query.device_number,
    label: 'custom.devicePage.deviceNumber',
    type: 'input'
  },
  {
    key: 'pid_number',
    initValue: query.pid_number,
    label: 'custom.devicePage.pidNumber',
    type: 'input'
  },
  {
    key: 'firmware_version',
    initValue: query.firmware_version,
    label: 'custom.devicePage.firmwareVersion',
    type: 'input'
  },
  {
    key: 'description',
    initValue: query.description,
    label: 'custom.devicePage.description',
    type: 'input'
  },
  {
    key: 'shared_status',
    label: 'custom.devicePage.sharedStatus',
    type: 'select',
    initValue: query.shared_status,
    options: [
      { label: () => $t('custom.devicePage.allSharedStatus'), value: '' },
      { label: () => $t('custom.devicePage.shared'), value: 'shared' },
      { label: () => $t('custom.devicePage.unshared'), value: 'unshared' }
    ]
  },
  {
    key: 'label',
    initValue: query.label,
    label: 'custom.devicePage.label',
    type: 'input'
  }
])

const {
  initializeServiceAccessFiltersInBackground,
  paramsUpdateHandle,
  primeInitialServiceAccessFilter
} = useDeviceManageServiceAccessFilters({
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
  },
]

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
  getGroupOptions: getDeviceGroupOptions,
  assignDevicesToGroup: deviceGroupRelation
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
    element: () => (
      <n-dropdown options={dropOption} trigger="click" onSelect={handleSelect}>
        <n-button type="primary">+{$t('custom.devicePage.addDevice')}</n-button>
      </n-dropdown>
    )
  }
]
const active = ref(false)
const addDrawerVisited = ref(false)
const isSuccess = ref(false)

const setIsSuccess = (flag: boolean) => {
  isSuccess.value = flag
}
const placement = ref<DrawerPlacement>('right')
const current = ref<number>(1)
const currentStatus = ref<StepsProps['status']>('process')
const activate = (place: DrawerPlacement, key: string | number) => {
  if (key === 'server') {
    router.push('/device/service-access')
  } else {
    current.value = 1
    manualDeviceNumber.value = ''
    addDrawerVisited.value = true
    active.value = true
    addKey.value = key
    placement.value = place
  }
}

onMounted(() => {
  if (route.query?.onboarding === 'first-device' && route.query?.add) {
    activate('bottom', 'hands')
  }
})

const completeHandAdd = () => {
  tablePageRef.value?.handleSearch()
}

const { deviceNumber, buttonDisabled, showMessage, messageStyle, completeAdd } = useDeviceManageActivationFlow({
  checkDevice: checkDevice as unknown as Parameters<typeof useDeviceManageActivationFlow>[0]['checkDevice'],
  activateDevice: activateRdiDevice,
  logger,
  onActivated: () => {
    active.value = false
    tablePageRef.value?.handleSearch()
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
    <DeviceManageQuickActions
      v-if="quickActionsVisited"
      ref="deviceManageQuickActionsRef"
      @updated="refreshDeviceTable"
    />
    <NModal v-model:show="bulkGroupModalVisible" preset="card" class="max-w-520px">
      <template #header>{{ $t('custom.devicePage.addSelectedToGroupTitle') }}</template>
      <NFlex vertical :size="12">
        <NAlert type="info" :show-icon="false">
          {{ $t('custom.devicePage.addSelectedToGroupHint').replace('{count}', String(selectedFleetDeviceIds.length)) }}
        </NAlert>
        <NTreeSelect
          v-model:value="bulkGroupId"
          :options="bulkGroupOptions"
          :placeholder="$t('custom.devicePage.selectTargetGroup')"
          filterable
          clearable
        />
        <NFlex justify="end" :size="8">
          <NButton @click="bulkGroupModalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="bulkGroupAssigning" @click="assignSelectedDevicesToGroup">
            {{ $t('common.confirm') }}
          </NButton>
        </NFlex>
      </NFlex>
    </NModal>
    <NModal v-model:show="selectedFleetSummaryVisible" preset="card" class="max-w-640px">
      <template #header>{{ $t('custom.devicePage.selectedDeviceSummaryTitle') }}</template>
      <NFlex vertical :size="12">
        <NAlert type="info" :show-icon="false">
          {{ $t('custom.devicePage.selectedDeviceSummaryHint') }}
        </NAlert>
        <div class="selected-device-summary-grid">
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.selectedDeviceSummaryTotal') }}</span>
            <strong>{{ selectedFleetSummary.total }}</strong>
          </div>
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.selectedDeviceSummaryOnline') }}</span>
            <strong>{{ selectedFleetSummary.online }}</strong>
          </div>
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.selectedDeviceSummaryOffline') }}</span>
            <strong>{{ selectedFleetSummary.offline }}</strong>
          </div>
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.selectedDeviceSummaryAlarmed') }}</span>
            <strong>{{ selectedFleetSummary.alarmed }}</strong>
          </div>
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.selectedDeviceSummaryMissingVersion') }}</span>
            <strong>{{ selectedFleetSummary.missingVersion }}</strong>
          </div>
        </div>
        <NAlert
          v-if="selectedFleetSummary.offline || selectedFleetSummary.alarmed || selectedFleetSummary.missingVersion"
          type="warning"
          :show-icon="false"
        >
          {{ $t('custom.devicePage.selectedDeviceRiskHint') }}
        </NAlert>
        <NInput
          :value="selectedFleetDeviceIdentifiers"
          type="textarea"
          :rows="5"
          readonly
          :placeholder="$t('custom.devicePage.selectedDeviceIdentifiersPlaceholder')"
        />
        <NFlex justify="end" :size="8">
          <NButton @click="selectedFleetSummaryVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="copySelectedFleetDeviceIdentifiers">
            {{ $t('custom.devicePage.copySelectedDeviceIdentifiers') }}
          </NButton>
        </NFlex>
      </NFlex>
    </NModal>
    <NModal v-model:show="fleetScopeConfirmVisible" preset="card" class="max-w-560px">
      <template #header>{{ pendingFleetScopeAction ? $t(pendingFleetScopeAction.labelKey) : '' }}</template>
      <NFlex vertical :size="12">
        <NAlert type="warning" :show-icon="false">
          {{ $t('custom.devicePage.fleetActionHint') }}
        </NAlert>
        <div class="selected-device-summary-grid">
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.fleetCurrentPageCount') }}</span>
            <strong>{{ currentPageDeviceCount }}</strong>
          </div>
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.fleetTargetPreviewCount') }}</span>
            <strong>{{ targetPreviewTotal ?? '--' }}</strong>
          </div>
          <div class="selected-device-summary-item">
            <span>{{ $t('custom.devicePage.fleetSelectedCount') }}</span>
            <strong>{{ selectedFleetDeviceIds.length }}</strong>
          </div>
        </div>
        <NText depth="3">
          {{ $t('custom.devicePage.fleetCurrentPageOnly') }}
        </NText>
        <NFlex justify="end" :size="8">
          <NButton @click="cancelFleetCurrentPageAction">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="confirmFleetCurrentPageAction">
            {{ pendingFleetScopeAction ? $t(pendingFleetScopeAction.labelKey) : $t('common.confirm') }}
          </NButton>
        </NFlex>
      </NFlex>
    </NModal>
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

<style scoped lang="scss">
.selected-device-summary-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}

.selected-device-summary-item {
  min-width: 0;
  padding: 10px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #fafafa;
  display: grid;
  gap: 4px;

  span {
    color: #666;
    font-size: 12px;
    line-height: 1.4;
  }

  strong {
    color: #222;
    font-size: 20px;
    line-height: 1.2;
  }
}

@media (max-width: 720px) {
  .selected-device-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
