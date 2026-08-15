<!--
鏂囦欢鐢ㄩ€? 鎵胯浇OTA 鍗囩骇鐩稿叧鐨勪骇鍝佸崌绾ч〉闈㈡垨涓氬姟缁勪欢銆?鏍稿績閫昏緫: 缁勭粐椤甸潰鐘舵€併€佹帴鍙ｈ皟鐢ㄣ€佽〃鍗?鍒楄〃浜や簰鍜屽瓙缁勪欢鍗忎綔锛屽悜鐢ㄦ埛鍛堢幇鍙搷浣滅殑涓氬姟娴佺▼銆?鍏抽敭娉ㄦ剰浜嬮」: 淇敼鏃惰鍚屾鏍稿璺敱鍙傛暟銆佹帴鍙ｈ浇鑽枫€佹潈闄愮姸鎬佸拰鐢ㄦ埛鍙鎻愮ず锛岄伩鍏嶅彧鏀瑰墠绔姸鎬併€?閲嶆瀯寤鸿: 鍙€愭鎶婃煡璇€佹彁浜ゅ拰寮圭獥鐘舵€佹媶鎴愮粍鍚堝嚱鏁帮紝璁╃粍浠舵洿涓撴敞浜庡竷灞€涓庝簨浠剁紪鎺掋€?-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'
import { debounce } from 'lodash-es'
import { addOtaTask, editOtaTaskDetail, getOtaTaskSupportBundle, previewOtaTask } from '@/service/product/update-ota'
import { listFleetSavedFilters } from '@/service/api/device'
import { $t } from '@/locales'
import { createOtaTaskColumns } from './ota-task-table-columns'
import { useOtaTaskData } from './useOtaTaskData'
import { useOtaTaskDetail } from './useOtaTaskDetail'
import { useOtaTaskFlow } from './useOtaTaskFlow'
import { buildOtaFilterSummaryItems, buildOtaPreviewDeviceRows } from './ota-task-state'
import {
  FLEET_CURRENT_PAGE_SCOPE,
  FLEET_DEVICE_FILTER_SCOPE,
  FLEET_FILTER_RESULT_SCOPE,
  parseFleetRolloutContext
} from '../../device/modules/fleet-rollout-context'
import type { FleetRolloutRouteQueryValue } from '../../device/modules/fleet-rollout-context'
import OtaTaskPreflightCard from './OtaTaskPreflightCard.vue'
import OtaTaskDetailDialog from './OtaTaskDetailDialog.vue'
import OtaTaskFilterRolloutSummary from './OtaTaskFilterRolloutSummary.vue'
import OtaTaskLaunchContext from './OtaTaskLaunchContext.vue'
import OtaTaskNextStepCard from './OtaTaskNextStepCard.vue'

const route = useRoute()
const router = useRouter()
// route.query 的值可能是 null 或 (string|null)[]，与 FleetRolloutRouteQueryValue
// 兼容但 LocationQuery 的索引签名更宽，需显式收窄后再传入解析器。
const fleetRolloutContext = computed(() =>
  parseFleetRolloutContext(route.query as Record<string, FleetRolloutRouteQueryValue>)
)

type ReadyCheckOtaContextStatus = 'idle' | 'matched' | 'not-found'

function normalizeRouteQueryText(value: unknown) {
  const rawValue = Array.isArray(value) ? value[0] : value
  return typeof rawValue === 'string' ? rawValue.trim() : ''
}

const isReadyCheckOtaSource = computed(() => normalizeRouteQueryText(route.query.source) === 'ready-check')
const readyCheckOtaTaskId = computed(() => normalizeRouteQueryText(route.query.ota_task_id))
const readyCheckOtaDetailId = computed(() => normalizeRouteQueryText(route.query.ota_detail_id))
const readyCheckOtaContextStatus = ref<ReadyCheckOtaContextStatus>('idle')

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
    success: (message) => window.$message?.success(message),
    warning: (message) => window.$message?.warning(message)
  },
  fleetRolloutContext
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
  return scope === FLEET_FILTER_RESULT_SCOPE || scope === FLEET_DEVICE_FILTER_SCOPE || scope === FLEET_CURRENT_PAGE_SCOPE
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
    render: (row) => row.label || row.id || '--'
  },
  {
    title: $t('page.product.update-ota.previewSubsetDeviceNumber'),
    key: 'deviceNumber',
    render: (row) => row.deviceNumber || '--'
  },
  {
    title: $t('page.product.update-ota.previewSubsetVersion'),
    key: 'currentVersion',
    render: (row) => row.currentVersion || '--'
  },
  {
    title: $t('page.product.update-ota.previewSubsetOnline'),
    key: 'online'
  }
])

const readyCheckOtaDetailMatched = computed(() => {
  if (!readyCheckOtaDetailId.value) return false
  return detailList.value.some((item) => item.id === readyCheckOtaDetailId.value)
})

const readyCheckOtaContextVisible = computed(
  () => isReadyCheckOtaSource.value && Boolean(readyCheckOtaTaskId.value || readyCheckOtaDetailId.value)
)

const readyCheckOtaContextType = computed(() => {
  if (readyCheckOtaContextStatus.value === 'not-found') return 'warning'
  return readyCheckOtaContextStatus.value === 'matched' ? 'success' : 'info'
})

const readyCheckOtaContextMessage = computed(() => {
  if (!readyCheckOtaContextVisible.value) return ''
  if (readyCheckOtaContextStatus.value === 'not-found') {
    return $t('page.product.update-ota.readyCheckContextTaskMissing').replace('{taskId}', readyCheckOtaTaskId.value || '--')
  }
  if (readyCheckOtaContextStatus.value === 'matched') {
    return $t('page.product.update-ota.readyCheckContextTaskMatched')
      .replace('{taskId}', readyCheckOtaTaskId.value || '--')
      .replace('{detailId}', readyCheckOtaDetailId.value || '--')
  }
  return $t('page.product.update-ota.readyCheckContextPreserved')
    .replace('{taskId}', readyCheckOtaTaskId.value || '--')
    .replace('{detailId}', readyCheckOtaDetailId.value || '--')
})

const readyCheckOtaDetailContextMessage = computed(() => {
  if (!readyCheckOtaContextVisible.value || !readyCheckOtaDetailId.value) return ''
  return readyCheckOtaDetailMatched.value
    ? $t('page.product.update-ota.readyCheckDetailMatched').replace('{detailId}', readyCheckOtaDetailId.value)
    : $t('page.product.update-ota.readyCheckDetailPreserved').replace('{detailId}', readyCheckOtaDetailId.value)
})

const otaNextStep = computed(() => {
  if (packageLoading.value) {
    return {
      type: 'info' as const,
      step: $t('page.product.update-ota.onboardingStepLoading'),
      title: $t('common.loading'),
      description: $t('page.product.update-ota.onboardingLoadingDesc'),
      actionLabel: $t('common.refresh'),
      action: 'refresh'
    }
  }

  if (!packageOptions.value.length) {
    return {
      type: 'warning' as const,
      step: $t('page.product.update-ota.onboardingStepPackage'),
      title: $t('page.product.update-ota.onboardingNoPackageTitle'),
      description: $t('page.product.update-ota.onboardingNoPackageDesc'),
      actionLabel: $t('page.product.update-ota.onboardingUploadPackageAction'),
      action: 'upload'
    }
  }

  if (!selectedPackageId.value) {
    return {
      type: 'info' as const,
      step: $t('page.product.update-ota.onboardingStepSelect'),
      title: $t('page.product.update-ota.onboardingSelectPackageTitle'),
      description: $t('page.product.update-ota.onboardingSelectPackageDesc').replace(
        '{count}',
        String(packageOptions.value.length)
      ),
      actionLabel: $t('common.refresh'),
      action: 'refresh'
    }
  }

  return {
    type: 'success' as const,
    step: $t('page.product.update-ota.onboardingStepCreate'),
    title: $t('page.product.update-ota.onboardingReadyTitle'),
    description: $t('page.product.update-ota.onboardingReadyDesc'),
    actionLabel: $t('page.product.update-ota.onboardingCreateTaskAction'),
    action: 'create'
  }
})

function handleOtaNextStep() {
  if (otaNextStep.value.action === 'upload') {
    router.push({ name: 'product_update-package', query: { return_to: 'ota_task' } })
    return
  }

  if (otaNextStep.value.action === 'create') {
    openTaskModal()
    return
  }

  fetchPackages()
}

async function applyReadyCheckOtaContext() {
  if (!isReadyCheckOtaSource.value || !readyCheckOtaTaskId.value) return
  const matchedTask = taskList.value.find((item) => item.id === readyCheckOtaTaskId.value)
  if (!matchedTask) {
    readyCheckOtaContextStatus.value = 'not-found'
    return
  }
  readyCheckOtaContextStatus.value = 'matched'
  if (selectedTask.value?.id === matchedTask.id && detailModalVisible.value) return
  await openTaskDetail(matchedTask)
}

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
  updateTaskDetailStatus,
  statusLabel,
  statusTagType,
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
    success: (message) => window.$message?.success(message),
    warning: (message) => window.$message?.warning(message)
  },
  dialog: {
    warning: (options) => window.$dialog?.warning(options)
  }
})

const firstFailedDiagnosticDevice = computed(() => {
  for (const group of failureGroups.value) {
    const device = group.devices.find((item) => item.device_id)
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
      <div class="page-header">
        <div>
          <div class="page-title">{{ $t('page.product.update-ota.otaTitle') }}</div>
          <div class="page-subtitle">
            {{
              selectedPackage?.name || selectedPackage?.version || $t('page.product.update-package.packagePlaceholder')
            }}
          </div>
        </div>
        <NSpace>
          <!-- 必须显式无参调用：fetchPackages(search = keyword) 的首参是搜索词，
               直接绑定会把 MouseEvent 当搜索词传进去，触发 search.trim() 类型错误。 -->
          <NButton :loading="packageLoading" @click="() => fetchPackages()">{{ $t('common.refresh') }}</NButton>
          <NButton type="primary" :disabled="!selectedPackageId" @click="openTaskModal">
            {{ $t('page.product.update-ota.updateTask') }}
          </NButton>
        </NSpace>
      </div>

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
      />
    </NSpace>

    <NModal
      v-model:show="taskModalVisible"
      preset="card"
      class="task-modal"
      :title="$t('page.product.update-ota.updateTask')"
    >
      <NForm label-placement="top">
        <OtaTaskLaunchContext
          :selected-package="selectedPackage"
          :show-no-eligible-device-alert="showNoEligibleDeviceAlert"
          :fleet-preselection-result="fleetPreselectionResult"
          :is-fleet-filter-scope="isFleetFilterScope"
          :is-fleet-filter-rollout="isFleetFilterRollout"
          :filter-preview-result="filterPreviewResult"
          :saved-fleet-filters-loading="savedFleetFiltersLoading"
          :saved-fleet-filter-load-failed="savedFleetFilterLoadFailed"
          :saved-fleet-filter-options="savedFleetFilterOptions"
          :selected-saved-fleet-filter-id="selectedSavedFleetFilterId"
          :selected-saved-fleet-filter="selectedSavedFleetFilter"
          @update:selected-saved-fleet-filter-id="selectedSavedFleetFilterId = $event"
        />
        <NFormItem :label="$t('page.product.update-ota.taskName')" required>
          <NInput v-model:value="taskForm.name" />
        </NFormItem>
        <OtaTaskFilterRolloutSummary
          v-if="isFleetFilterRollout"
          :selected-saved-fleet-filter="selectedSavedFleetFilter"
          :fleet-preselection-result="fleetPreselectionResult"
          :fleet-filter-summary-items="fleetFilterSummaryItems"
          :filter-preview-result="filterPreviewResult"
          :filter-preview-subset-columns="filterPreviewSubsetColumns"
          :filter-preview-subset-rows="filterPreviewSubsetRows"
        />
        <NFormItem v-else :label="$t('page.product.update-ota.selectDevice')" required>
          <NSelect
            v-model:value="taskForm.device_id_list"
            multiple
            filterable
            remote
            :loading="deviceLoading"
            :disabled="deviceLoading"
            :options="deviceOptions"
            :virtual-scroll="true"
            max-tag-count="responsive"
            :placeholder="deviceLoading ? $t('common.loading') : $t('page.product.update-ota.selectDevice')"
            @search="handleDeviceSearch"
          />
        </NFormItem>
        <OtaTaskPreflightCard
          class="mb-3"
          :summary="taskPreflight"
          :items="taskPreflightItems"
          :risk-devices="taskRiskDevices"
        />
        <NFormItem :label="$t('page.product.update-ota.desc')">
          <NInput v-model:value="taskForm.description" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="taskModalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="saving" :disabled="!canSaveTask" @click="saveTask">
            {{ taskPrimaryActionLabel }}
          </NButton>
        </NSpace>
      </template>
    </NModal>

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

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
}

.page-subtitle {
  margin-top: 4px;
  color: var(--text-color-3);
}

.package-select {
  width: 320px;
}

.detail-filter {
  width: 220px;
}

.rollout-refresh-meta {
  color: var(--text-color-3);
  font-size: 12px;
  line-height: 1.5;
}

.action-row {
  display: flex;
  gap: 8px;
}

.task-modal {
  width: min(620px, calc(100vw - 32px));
}

.detail-modal {
  width: min(1100px, calc(100vw - 32px));
}

.rollout-summary-card :deep(.n-card__content) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 12px;
}

.rollout-summary-card__label {
  min-width: 0;
  color: var(--text-color-2);
  font-size: 12px;
}

.rollout-guidance-card {
  border: 1px solid var(--border-color);
}

.rollout-guidance-card__title {
  min-width: 0;
  font-weight: 600;
}

.rollout-guidance-card__desc {
  margin-top: 8px;
  color: var(--text-color-2);
  font-size: 12px;
  line-height: 1.5;
}

.failure-workbench-desc {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: 12px;
}

.first-failure-diagnostics-cta :deep(.n-alert-body__content) {
  width: 100%;
}

.first-failure-diagnostics-cta__title {
  font-weight: 700;
}

.first-failure-diagnostics-cta__desc {
  margin-top: 4px;
  color: var(--text-color-2);
  font-size: 12px;
  line-height: 1.5;
}

.retry-recommendation-card {
  border: 1px solid var(--border-color);
  background: var(--card-color);
}

.retry-recommendation-card__title {
  min-width: 0;
  font-weight: 600;
}

.retry-recommendation-card__desc,
.retry-recommendation-card__devices {
  color: var(--text-color-2);
  font-size: 12px;
  line-height: 1.5;
}

.retry-recommendation-card__devices {
  overflow-wrap: anywhere;
}

.failure-group-card {
  border: 1px solid var(--border-color);
}

.failure-group-card__reason {
  min-width: 0;
  overflow-wrap: anywhere;
  font-weight: 600;
}

.failure-group-card__devices {
  display: grid;
  gap: 8px;
}

.failure-group-card__device {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: var(--text-color-3);
  font-size: 12px;
  line-height: 1.5;
}

.failure-group-card__device span {
  min-width: 0;
  overflow-wrap: anywhere;
}

@media (max-width: 720px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .package-select,
  .detail-filter {
    width: 100%;
  }

}
</style>
