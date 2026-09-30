<!--
文件用途: 承载 OTA 升级相关的产品升级页面或业务组件。
核心逻辑: 页面状态、接口调用与列表分页全部收敛在 useOtaTaskPage(内部由 useListPage 驱动),
         本文件只负责模板组装与把子组件动作回抛给页面编排层。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示,避免只改前端状态。
-->
<script setup lang="ts">
import PageHeader from '@/components/common/page-header/index.vue'
import OtaTaskCreateModal from './OtaTaskCreateModal.vue'
import OtaTaskDetailDialog from './OtaTaskDetailDialog.vue'
import OtaTaskNextStepCard from './OtaTaskNextStepCard.vue'
import OtaTaskPackageSelect from './OtaTaskPackageSelect.vue'
import { useOtaTaskPage } from './useOtaTaskPage'

const {
  // 升级包与任务列表(useListPage 承担分页/加载态/过期请求丢弃)
  packageLoading,
  packageOptions,
  selectedPackageId,
  selectedPackage,
  taskList,
  taskLoading,
  taskPagination,
  taskColumns,
  fetchPackages,
  fetchTasks,
  handlePackageSearch,
  // 创建任务弹窗
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
  isFleetFilterScope,
  savedFleetFiltersLoading,
  savedFleetFilterLoadFailed,
  savedFleetFilterOptions,
  selectedSavedFleetFilterId,
  selectedSavedFleetFilter,
  fleetFilterSummaryItems,
  filterPreviewSubsetRows,
  filterPreviewSubsetColumns,
  taskPrimaryActionLabel,
  deviceLoading,
  deviceOptions,
  handleDeviceSearch,
  openTaskModal,
  saveTask,
  // 任务明细弹窗
  detailModalVisible,
  selectedTask,
  detailList,
  detailLoading,
  detailQuery,
  detailPagination,
  detailColumns,
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
  fetchTaskDetails,
  resetDetailQuery,
  openTaskDetail,
  exportFailedDevices,
  copyFailedDevices,
  copyFailureSupportBundle,
  downloadTaskSupportBundle,
  // 失败设备诊断跳转
  firstFailedDiagnosticDevice,
  openFailedDeviceDiagnostics,
  openFirstFailedDeviceDiagnostics,
  // Ready Check 深链上下文
  readyCheckOtaContextVisible,
  readyCheckOtaContextType,
  readyCheckOtaContextMessage,
  readyCheckOtaDetailContextMessage,
  readyCheckOtaContextStatus,
  readyCheckOtaDetailMatched,
  // 新手引导
  otaNextStep,
  handleOtaNextStep
} = useOtaTaskPage()
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

      <OtaTaskPackageSelect
        v-model:selected-package-id="selectedPackageId"
        :loading="packageLoading"
        :package-options="packageOptions"
        :selected-package="selectedPackage"
        @search="handlePackageSearch"
      />

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
</style>
