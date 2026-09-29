<!--
文件用途: 承载 OTA 升级相关的产品升级页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'
import { debounce } from 'lodash-es'
import { addOtaTask, editOtaTaskDetail, getOtaTaskSupportBundle, previewOtaTask } from '@/service/product/update-ota'
import { listFleetSavedFilters } from '@/service/api/device'
import { $t } from '@/locales'
import PageHeader from '@/components/common/page-header/index.vue'
import { createOtaTaskColumns } from './ota-task-table-columns'
import { useOtaTaskData } from './useOtaTaskData'
import { useOtaTaskDetail } from './useOtaTaskDetail'
import { useOtaTaskFlow } from './useOtaTaskFlow'
import { useOtaOnboardingStep } from './useOtaOnboardingStep'
import { normalizeRouteQueryText, useOtaReadyCheckContext } from './useOtaReadyCheckContext'
import { buildOtaFilterSummaryItems, buildOtaPreviewDeviceRows } from './ota-task-state'
import {
  FLEET_CURRENT_PAGE_SCOPE,
  FLEET_DEVICE_FILTER_SCOPE,
  FLEET_FILTER_RESULT_SCOPE,
  parseFleetRolloutContext
} from '../../device/modules/fleet-rollout-context'
import type { FleetRolloutRouteQueryValue } from '../../device/modules/fleet-rollout-context'
import OtaTaskCreateModal from './OtaTaskCreateModal.vue'
import OtaTaskDetailDialog from './OtaTaskDetailDialog.vue'
import OtaTaskNextStepCard from './OtaTaskNextStepCard.vue'

const route = useRoute()
const router = useRouter()

const {
  packageLoading,
  taskLoading,
  detailLoading,
  deviceLoading,
  taskList,
  detailList,
  detailStatistics,
  deviceCandidates,
  deviceOptions,
  selectedPackageId,
  selectedTask,
  selectedPackage,
  packageOptions,
  detailQuery,
  taskPagination,
  detailPagination,
  fetchPackages,
  fetchTasks,
  fetchDevices,
  fetchTaskDetails,
  openTaskDetail: loadTaskDetail,
  resetTaskPage,
  resetDetailQuery,
  clearDeviceCandidates
} = useOtaTaskData()

const routeOtaPackageId = normalizeRouteQueryText(route.query.ota_package_id)
if (routeOtaPackageId) {
  selectedPackageId.value = routeOtaPackageId
}

const {
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
  savedFleetFiltersLoading,
  savedFleetFilterLoadFailed,
  savedFleetFilterOptions,
  selectedSavedFleetFilterId,
  selectedSavedFleetFilter,
  openTaskModal,
  saveTask
} = useOtaTaskFlow({
  data: {
    selectedPackageId,
    selectedPackage,
    deviceCandidates,
    deviceOptions,
    deviceLoading,
    fetchDevices,
    fetchTasks,
    clearDeviceCandidates
  },
  services: {
    addTask: addOtaTask,
    previewTask: previewOtaTask,
    listFleetSavedFilters
  },
  t: $t,
  message: {
    success: message => window.$message?.success(message),
    warning: message => window.$message?.warning(message)
  },
  fleetRolloutContext: computed(() =>
    parseFleetRolloutContext(route.query as Record<string, FleetRolloutRouteQueryValue>)
  )
})

const taskPrimaryActionLabel = computed(() => {
  if (!isFleetFilterRollout.value) return $t('common.save')
  return filterPreviewResult.value
    ? $t('page.product.update-ota.confirmCreateTask')
    : $t('page.product.update-ota.previewFilterTask')
})

const fleetFilterSummaryItems = computed(() =>
  buildOtaFilterSummaryItems(fleetPreselectionResult.value?.deviceFilter || {})
)

const isFleetFilterScope = computed(() => {
  const scope = fleetPreselectionResult.value?.scope
  return (
    scope === FLEET_FILTER_RESULT_SCOPE || scope === FLEET_DEVICE_FILTER_SCOPE || scope === FLEET_CURRENT_PAGE_SCOPE
  )
})

const filterPreviewSubsetRows = computed(() => {
  const previewRows = filterPreviewResult.value?.preview_devices
  if (Array.isArray(previewRows) && previewRows.length) {
    return buildOtaPreviewDeviceRows(previewRows)
  }
  return buildOtaPreviewDeviceRows(deviceCandidates.value)
})

const filterPreviewSubsetColumns = computed<DataTableColumns<any>>(() => [
  {
    title: $t('page.product.update-ota.previewSubsetDevice'),
    key: 'label',
    render: row => row.label || row.id || '--'
  },
  {
    title: $t('page.product.update-ota.previewSubsetDeviceNumber'),
    key: 'deviceNumber',
    render: row => row.deviceNumber || '--'
  },
  {
    title: $t('page.product.update-ota.previewSubsetVersion'),
    key: 'currentVersion',
    render: row => row.currentVersion || '--'
  },
  {
    title: $t('page.product.update-ota.previewSubsetOnline'),
    key: 'online'
  }
])

function openFailedDeviceDiagnostics(row: { id: string; device_id?: string }) {
  if (!row.device_id) {
    window.$message?.warning($t('page.product.update-ota.failureDiagnosticsMissingDevice'))
    return
  }

  router.push({
    name: 'device_details',
    query: {
      d_id: row.device_id,
      tab: 'ready-check',
      source: 'ota',
      ...(selectedTask.value?.id ? { ota_task_id: selectedTask.value.id } : {}),
      ota_detail_id: row.id
    }
  })
}

const {
  detailModalVisible,
  statusOptions,
  formatTime,
  rolloutFailedCount,
  rolloutSuccessRate,
  rolloutSummaryItems,
  rolloutGuidanceItems,
  rolloutActiveCount,
  detailAutoRefreshEnabled,
  detailAutoRefreshActive,
  detailLastRefreshLabel,
  refreshTaskDetails,
  failedDeviceCount,
  canCopyFailureSupportBundle,
  failureGroups,
  retryRecommendationCards,
  supportBundleLoading,
  exportFailedDevices,
  copyFailedDevices,
  copyFailureSupportBundle,
  downloadTaskSupportBundle,
  openTaskDetail,
  detailColumns
} = useOtaTaskDetail({
  selectedPackage,
  selectedTask,
  detailLoading,
  detailList,
  detailStatistics,
  loadTaskDetail,
  fetchTaskDetails,
  editTaskDetail: editOtaTaskDetail,
  getTaskSupportBundle: getOtaTaskSupportBundle,
  openFailedDeviceDiagnostics,
  t: $t,
  message: {
    success: message => window.$message?.success(message),
    warning: message => window.$message?.warning(message)
  },
  dialog: {
    warning: options => window.$dialog?.warning(options)
  }
})

const {
  readyCheckOtaContextVisible,
  readyCheckOtaContextType,
  readyCheckOtaContextMessage,
  readyCheckOtaDetailContextMessage,
  applyReadyCheckOtaContext
} = useOtaReadyCheckContext({
  taskList,
  detailList,
  selectedTask,
  detailModalVisible,
  openTaskDetail
})

const { nextStep: otaNextStep, handleNextStep: handleOtaNextStep } = useOtaOnboardingStep({
  packageLoading,
  packageOptions,
  selectedPackageId,
  onUploadPackage: () => router.push({ name: 'product_update-package', query: { return_to: 'ota_task' } }),
  onCreateTask: openTaskModal,
  onRefreshPackages: () => fetchPackages()
})

const firstFailedDiagnosticDevice = computed(() => {
  for (const group of failureGroups.value) {
    const device = group.devices.find(item => item.device_id)
    if (device) return device
  }
  return null
})

function openFirstFailedDeviceDiagnostics() {
  if (!firstFailedDiagnosticDevice.value) {
    window.$message?.warning($t('page.product.update-ota.failureDiagnosticsMissingDevice'))
    return
  }

  openFailedDeviceDiagnostics(firstFailedDiagnosticDevice.value)
}

const taskColumns = createOtaTaskColumns({
  getSelectedPackage: () => selectedPackage.value,
  formatTime,
  openTaskDetail
})

const searchDeviceOptions = debounce((query: string) => {
  if (!taskModalVisible.value || isFleetFilterRollout.value) return
  fetchDevices(query)
}, 300)

const searchPackageOptions = debounce((query: string) => {
  fetchPackages(query)
}, 300)

function handleDeviceSearch(query: string) {
  searchDeviceOptions(query)
}

function handlePackageSearch(query: string) {
  searchPackageOptions(query)
}

watch(selectedPackageId, async () => {
  resetTaskPage()
  await fetchTasks()
  await applyReadyCheckOtaContext()
})

onBeforeUnmount(() => {
  searchDeviceOptions.cancel()
  searchPackageOptions.cancel()
})

onMounted(async () => {
  const packageSelectionChanged = await fetchPackages()
  if (!packageSelectionChanged) {
    await fetchTasks()
    await applyReadyCheckOtaContext()
  }
})
</script>

<template>
  <div class="product-page">
    <NSpace vertical size="medium">
      <PageHeader
        :title="$t('page.product.update-ota.otaTitle')"
        :subtitle="
          selectedPackage?.name || selectedPackage?.version || $t('page.product.update-package.packagePlaceholder')
        "
      >
        <!-- 必须显式无参调用：fetchPackages(search = keyword) 的首参是搜索词，
             直接绑定会把 MouseEvent 当搜索词传进去，触发 search.trim() 类型错误。 -->
        <NButton :loading="packageLoading" @click="() => fetchPackages()">{{ $t('common.refresh') }}</NButton>
        <NButton type="primary" :disabled="!selectedPackageId" @click="openTaskModal">
          {{ $t('page.product.update-ota.updateTask') }}
        </NButton>
      </PageHeader>

      <NCard :bordered="false">
        <NSpace align="center" :wrap="true">
          <NSelect
            v-model:value="selectedPackageId"
            class="package-select"
            filterable
            remote
            clearable
            :loading="packageLoading"
            :options="packageOptions"
            :placeholder="$t('page.product.update-package.packagePlaceholder')"
            @search="handlePackageSearch"
          />
          <NTag v-if="selectedPackage?.device_config_name" type="info">{{ selectedPackage.device_config_name }}</NTag>
          <NTag v-if="selectedPackage?.signature" type="success">
            {{ $t('page.product.update-ota.packageSign') }}: {{ selectedPackage.signature }}
          </NTag>
        </NSpace>
      </NCard>

      <OtaTaskNextStepCard
        :next-step="otaNextStep"
        :has-packages="Boolean(packageOptions.length)"
        :has-selected-package="Boolean(selectedPackageId)"
        @action="handleOtaNextStep"
      />

      <NAlert v-if="readyCheckOtaContextVisible" :type="readyCheckOtaContextType" :show-icon="true">
        {{ readyCheckOtaContextMessage }}
      </NAlert>

      <NDataTable
        remote
        :columns="taskColumns"
        :data="taskList"
        :loading="taskLoading"
        :pagination="taskPagination"
        :scroll-x="980"
      >
        <template #empty>
          <NEmpty :description="$t('common.noData')" class="py-24px" />
        </template>
      </NDataTable>
    </NSpace>

    <OtaTaskCreateModal
      v-model:show="taskModalVisible"
      v-model:task-form="taskForm"
      v-model:selected-saved-fleet-filter-id="selectedSavedFleetFilterId"
      :selected-package="selectedPackage"
      :show-no-eligible-device-alert="showNoEligibleDeviceAlert"
      :fleet-preselection-result="fleetPreselectionResult"
      :is-fleet-filter-scope="isFleetFilterScope"
      :is-fleet-filter-rollout="isFleetFilterRollout"
      :filter-preview-result="filterPreviewResult"
      :saved-fleet-filters-loading="savedFleetFiltersLoading"
      :saved-fleet-filter-load-failed="savedFleetFilterLoadFailed"
      :saved-fleet-filter-options="savedFleetFilterOptions"
      :selected-saved-fleet-filter="selectedSavedFleetFilter"
      :fleet-filter-summary-items="fleetFilterSummaryItems"
      :filter-preview-subset-columns="filterPreviewSubsetColumns"
      :filter-preview-subset-rows="filterPreviewSubsetRows"
      :device-loading="deviceLoading"
      :device-options="deviceOptions"
      :preflight="taskPreflight"
      :preflight-items="taskPreflightItems"
      :risk-devices="taskRiskDevices"
      :saving="saving"
      :can-save-task="canSaveTask"
      :primary-action-label="taskPrimaryActionLabel"
      @search-devices="handleDeviceSearch"
      @save="saveTask"
    />

    <OtaTaskDetailDialog
      :show="detailModalVisible"
      :ready-check-ota-detail-context-message="readyCheckOtaDetailContextMessage"
      :detail-last-refresh-label="detailLastRefreshLabel"
      :detail-auto-refresh-active="detailAutoRefreshActive"
      :rollout-failed-count="rolloutFailedCount"
      :rollout-success-rate="rolloutSuccessRate"
      :detail-loading="detailLoading"
      :rollout-active-count="rolloutActiveCount"
      :detail-auto-refresh-enabled="detailAutoRefreshEnabled"
      :rollout-summary-items="rolloutSummaryItems"
      :rollout-guidance-items="rolloutGuidanceItems"
      :failed-device-count="failedDeviceCount"
      :support-bundle-loading="supportBundleLoading"
      :can-copy-failure-support-bundle="canCopyFailureSupportBundle"
      :has-first-failed-diagnostic-device="Boolean(firstFailedDiagnosticDevice)"
      :retry-recommendation-cards="retryRecommendationCards"
      :failure-groups="failureGroups"
      :detail-query="detailQuery"
      :status-options="statusOptions"
      :detail-columns="detailColumns"
      :detail-list="detailList"
      :detail-pagination="detailPagination"
      @update:show="detailModalVisible = $event"
      @update:detail-auto-refresh-enabled="detailAutoRefreshEnabled = $event"
      @update:detail-query-device-name="detailQuery.device_name = $event"
      @update:detail-query-task-status="detailQuery.task_status = $event"
      @refresh="refreshTaskDetails"
      @reset-detail-query="resetDetailQuery"
      @copy-failed-devices="copyFailedDevices"
      @copy-failure-support-bundle="copyFailureSupportBundle"
      @download-task-support-bundle="downloadTaskSupportBundle"
      @export-failed-devices="exportFailedDevices"
      @open-first-failed-diagnostics="openFirstFailedDeviceDiagnostics"
      @open-failed-device-diagnostics="openFailedDeviceDiagnostics"
    />
  </div>
</template>

<style scoped>
.product-page {
  padding: 16px;
}

.package-select {
  width: 320px;
}

@media (max-width: 720px) {
  .package-select {
    width: 100%;
  }
}
</style>
