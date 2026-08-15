import { computed, onScopeDispose, ref, watch, type Ref } from 'vue'
import type { SelectOption } from 'naive-ui'
import dayjs from 'dayjs'
import { writeClipboardText } from '@/utils/clipboard'
import { createOtaTaskDetailColumns } from './ota-task-table-columns'
import {
  buildOtaFailureClipboardText,
  buildOtaFailureGroups,
  buildOtaFailureSupportBundle,
  buildOtaRetryRecommendationCards,
  downloadOtaTaskSupportBundleJson,
  downloadOtaFailureCsv,
  getOtaFailedDevices
} from './ota-task-failure-workbench'
import {
  buildOtaRolloutGuidance,
  buildOtaRolloutSummary,
  buildOtaTaskStatusOptions,
  getOtaTaskStatusLabel,
  getOtaTaskStatusTagType
} from './ota-rollout-summary'
import type {
  OtaPackageRecord,
  OtaTaskDetailRecord,
  OtaTaskRecord,
  OtaTaskStatisticsItem,
  RolloutSummaryTagType
} from './ota-task-types'
import {
  getOtaTaskDetailActionDeviceLabel,
  getOtaTaskDetailActionTitleKey,
  type OtaTaskDetailAction
} from './ota-task-actions'

type Translate = (key: string) => string

type MessageApi = {
  success?: (message: string) => void
  warning?: (message: string) => void
}

type DialogApi = {
  warning?: (options: {
    title: string
    content: string
    positiveText: string
    negativeText: string
    onPositiveClick: () => Promise<void>
  }) => void
}

type UseOtaTaskDetailOptions = {
  selectedPackage: Readonly<Ref<OtaPackageRecord | null>>
  selectedTask?: Readonly<Ref<OtaTaskRecord | null>>
  detailLoading: Readonly<Ref<boolean>>
  detailList: Readonly<Ref<OtaTaskDetailRecord[]>>
  detailStatistics: Readonly<Ref<OtaTaskStatisticsItem[]>>
  loadTaskDetail: (row: OtaTaskRecord) => Promise<void>
  fetchTaskDetails: () => Promise<void>
  editTaskDetail: (payload: { id: string; action: OtaTaskDetailAction }) => Promise<{ error?: unknown }>
  getTaskSupportBundle?: (taskId: string) => Promise<{ data?: unknown; error?: unknown }>
  openFailedDeviceDiagnostics?: (row: OtaTaskDetailRecord) => void
  t: Translate
  message: MessageApi
  dialog: DialogApi
}

const OTA_ACTIVE_DETAIL_STATUSES = new Set([1, 2, 3])
const OTA_DETAIL_AUTO_REFRESH_MS = 10000

export const formatOtaTaskTime = (value?: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-')

export const useOtaTaskDetail = (options: UseOtaTaskDetailOptions) => {
  const detailModalVisible = ref(false)
  const detailAutoRefreshEnabled = ref(true)
  const detailLastRefreshedAt = ref('')
  const detailAutoRefreshTimer = ref<number | null>(null)
  const detailPageVisible = ref(typeof document === 'undefined' ? true : !document.hidden)
  const supportBundleLoading = ref(false)
  let supportBundleRequestSeq = 0
  const statusOptions = computed<SelectOption[]>(() => buildOtaTaskStatusOptions(options.t))
  const statusLabel = (status?: number) => getOtaTaskStatusLabel(status, options.t)
  const statusTagType = (status?: number): RolloutSummaryTagType => getOtaTaskStatusTagType(status)
  const rolloutSummary = computed(() => buildOtaRolloutSummary(options.detailStatistics.value, options.t))
  const rolloutFailedCount = computed(() => rolloutSummary.value.failedCount)
  const rolloutSuccessRate = computed(() => rolloutSummary.value.successRate)
  const rolloutSummaryItems = computed(() => rolloutSummary.value.items)
  const rolloutGuidanceItems = computed(() => buildOtaRolloutGuidance(options.detailStatistics.value, options.t))
  const failureFallbackReason = computed(() => options.t('page.product.update-ota.failureUnknownReason'))
  const failedDevices = computed(() => getOtaFailedDevices(options.detailList.value))
  const failureGroups = computed(() => buildOtaFailureGroups(options.detailList.value, failureFallbackReason.value))
  const retryRecommendationCards = computed(() =>
    buildOtaRetryRecommendationCards(options.detailList.value, failureFallbackReason.value, options.selectedPackage.value, (key) => ({
      title: options.t(`page.product.update-ota.retryRecommendation.${key}.title`),
      description: options.t(`page.product.update-ota.retryRecommendation.${key}.description`)
    }))
  )
  const failedDeviceCount = computed(() => failedDevices.value.length)
  const selectedTaskId = computed(() => options.selectedTask?.value?.id || '')
  const canCopyFailureSupportBundle = computed(() =>
    Boolean(failedDevices.value.length || (selectedTaskId.value && options.getTaskSupportBundle))
  )
  const rolloutActiveCount = computed(() =>
    options.detailStatistics.value.reduce((total, item) => {
      const status = Number(item.status)
      return OTA_ACTIVE_DETAIL_STATUSES.has(status) ? total + Number(item.count || 0) : total
    }, 0)
  )
  const detailAutoRefreshActive = computed(
    () => detailModalVisible.value && detailAutoRefreshEnabled.value && detailPageVisible.value && rolloutActiveCount.value > 0
  )
  const detailLastRefreshLabel = computed(() =>
    detailLastRefreshedAt.value
      ? options.t('page.product.update-ota.lastProgressRefreshAt').replace('{time}', detailLastRefreshedAt.value)
      : options.t('page.product.update-ota.lastProgressRefreshNever')
  )
  const markDetailRefreshed = () => {
    detailLastRefreshedAt.value = formatOtaTaskTime(new Date().toISOString())
  }
  const refreshTaskDetails = async () => {
    await options.fetchTaskDetails()
    markDetailRefreshed()
  }
  const handleDetailVisibilityChange = () => {
    if (typeof document === 'undefined') return
    detailPageVisible.value = !document.hidden
    if (detailPageVisible.value && detailAutoRefreshActive.value && !options.detailLoading.value) {
      void refreshTaskDetails()
    }
  }
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', handleDetailVisibilityChange)
  }
  const openTaskDetail = async (row: OtaTaskRecord) => {
    detailModalVisible.value = true
    await options.loadTaskDetail(row)
    markDetailRefreshed()
  }
  const clearDetailAutoRefresh = () => {
    if (detailAutoRefreshTimer.value === null) return
    if (typeof window !== 'undefined') {
      window.clearInterval(detailAutoRefreshTimer.value)
    }
    detailAutoRefreshTimer.value = null
  }
  const scheduleDetailAutoRefresh = () => {
    if (typeof window === 'undefined' || detailAutoRefreshTimer.value !== null) return
    detailAutoRefreshTimer.value = window.setInterval(() => {
      if (options.detailLoading.value) return
      void refreshTaskDetails()
    }, OTA_DETAIL_AUTO_REFRESH_MS)
  }
  const exportFailedDevices = () => {
    if (failedDevices.value.length === 0) {
      return
    }

    downloadOtaFailureCsv(options.detailList.value, failureFallbackReason.value)
    options.message.success?.(options.t('page.product.update-ota.failureExportStarted'))
  }
  const copyFailedDevices = async () => {
    if (failedDevices.value.length === 0) {
      return
    }

    const ok = await writeClipboardText(
      buildOtaFailureClipboardText(options.detailList.value, failureFallbackReason.value)
    )
    if (ok) {
      options.message.success?.(options.t('page.product.update-ota.failureCopied'))
    } else {
      options.message.warning?.(options.t('common.copyFailed'))
    }
  }
  const buildLocalFailureSupportBundle = () =>
    buildOtaFailureSupportBundle({
      task: options.selectedTask?.value,
      selectedPackage: options.selectedPackage.value,
      rows: options.detailList.value,
      statistics: options.detailStatistics.value,
      fallbackReason: failureFallbackReason.value,
      lastRefreshLabel: detailLastRefreshLabel.value
    })
  const copyLocalFailureSupportBundle = async () => {
    const ok = await writeClipboardText(buildLocalFailureSupportBundle())
    if (ok) {
      options.message.success?.(options.t('page.product.update-ota.failureSupportBundleCopied'))
    } else {
      options.message.warning?.(options.t('common.copyFailed'))
    }
  }
  const copyFailureSupportBundle = async () => {
    const taskId = selectedTaskId.value
    if (!taskId || !options.getTaskSupportBundle) {
      if (failedDevices.value.length) {
        await copyLocalFailureSupportBundle()
      }
      return
    }

    const requestSeq = ++supportBundleRequestSeq
    supportBundleLoading.value = true
    try {
      const { data, error } = await options.getTaskSupportBundle(taskId)
      if (requestSeq !== supportBundleRequestSeq || selectedTaskId.value !== taskId) return
      if (!error && data) {
        const ok = await writeClipboardText(JSON.stringify(data, null, 2))
        if (ok) {
          options.message.success?.(options.t('page.product.update-ota.failureSupportBundleCopied'))
        } else {
          options.message.warning?.(options.t('common.copyFailed'))
        }
        return
      }
      if (failedDevices.value.length) {
        await copyLocalFailureSupportBundle()
      } else {
        options.message.warning?.(options.t('page.product.update-ota.taskSupportBundleLoadFailed'))
      }
    } catch {
      if (requestSeq !== supportBundleRequestSeq || selectedTaskId.value !== taskId) return
      if (failedDevices.value.length) {
        await copyLocalFailureSupportBundle()
      } else {
        options.message.warning?.(options.t('page.product.update-ota.taskSupportBundleLoadFailed'))
      }
    } finally {
      if (requestSeq === supportBundleRequestSeq) {
        supportBundleLoading.value = false
      }
    }
  }
  const downloadTaskSupportBundle = async () => {
    const taskId = selectedTaskId.value
    if (!taskId || !options.getTaskSupportBundle) return

    const requestSeq = ++supportBundleRequestSeq
    supportBundleLoading.value = true
    try {
      const { data, error } = await options.getTaskSupportBundle(taskId)
      if (requestSeq !== supportBundleRequestSeq) return
      if (error || !data) {
        options.message.warning?.(options.t('page.product.update-ota.taskSupportBundleLoadFailed'))
        return
      }
      downloadOtaTaskSupportBundleJson(data, `aetherlink-ota-task-${taskId}-support-bundle.json`)
      options.message.success?.(options.t('page.product.update-ota.taskSupportBundleDownloaded'))
    } finally {
      if (requestSeq === supportBundleRequestSeq) {
        supportBundleLoading.value = false
      }
    }
  }
  const updateTaskDetailStatus = (row: OtaTaskDetailRecord, action: OtaTaskDetailAction) => {
    options.dialog.warning?.({
      title: options.t(getOtaTaskDetailActionTitleKey(action)),
      content: getOtaTaskDetailActionDeviceLabel(row),
      positiveText: options.t('common.confirm'),
      negativeText: options.t('common.cancel'),
      onPositiveClick: async () => {
        const { error } = await options.editTaskDetail({ id: row.id, action })
        if (!error) {
          options.message.success?.(options.t('common.operationSuccess'))
          await refreshTaskDetails()
        }
      }
    })
  }
  watch(detailAutoRefreshActive, (active) => {
    if (active) {
      scheduleDetailAutoRefresh()
      return
    }
    clearDetailAutoRefresh()
  })
  watch(detailModalVisible, (visible) => {
    if (!visible) {
      clearDetailAutoRefresh()
    }
  })
  onScopeDispose(() => {
    clearDetailAutoRefresh()
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', handleDetailVisibilityChange)
    }
  })
  const detailColumns = createOtaTaskDetailColumns({
    getSelectedPackage: () => options.selectedPackage.value,
    formatTime: formatOtaTaskTime,
    statusLabel,
    statusTagType,
    openFailedDeviceDiagnostics: options.openFailedDeviceDiagnostics,
    updateTaskDetailStatus
  })

  return {
    detailModalVisible,
    statusOptions,
    formatTime: formatOtaTaskTime,
    statusLabel,
    statusTagType,
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
    detailColumns
  }
}
