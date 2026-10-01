/*
 * 文件用途: OTA 升级任务页(index.vue)的页面级编排 composable。
 * 核心逻辑: 串联 useOtaTaskData / useOtaTaskFlow / useOtaTaskDetail / useOtaReadyCheckContext /
 *           useOtaOnboardingStep,并持有失败诊断跳转、筛选预览展示行、远程搜索防抖和首屏加载时序。
 * 关键注意事项: 任务列表与明细列表的筛选/分页/加载态/过期请求丢弃全部由 useListPage 承担
 *              (见 useOtaTaskData),本文件不得再自管 page/pageSize/rows 状态。
 * 重构建议: 若继续膨胀,可把"创建弹窗的预览派生态"或"失败诊断跳转"再拆为独立 composable。
 */
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { debounce } from 'lodash-es'
import { useRoute, useRouter } from 'vue-router'
import { addOtaTask, editOtaTaskDetail, getOtaTaskSupportBundle, previewOtaTask } from '@/service/product/update-ota'
import { listFleetSavedFilters } from '@/service/api/device'
import { $t } from '@/locales'
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

export function useOtaTaskPage() {
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
    readyCheckOtaContextStatus,
    readyCheckOtaDetailMatched,
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

  return {
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
    deviceCandidates,
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
  }
}

export type OtaTaskPage = ReturnType<typeof useOtaTaskPage>
