import { computed, ref } from 'vue'
import type { ComputedRef } from 'vue'
import { writeClipboardText } from '@/utils/clipboard'
import {
  cancelFleetCommandJob,
  getFleetCommandJobRows,
  getFleetCommandJobSummary,
  listFleetCommandJobs,
  previewFleetCommandJob,
  retryFleetCommandJob,
  submitFleetCommandJob
} from '@/service/api/device'
import type {
  CommandJobRowsStatusFilter,
  FleetCommandJobListResult,
  FleetCommandJobPayload,
  FleetCommandJobPreviewResult,
  FleetCommandJobSubmitResult
} from '@/service/api/device'
import { normalizeApiData, serializeFleetCommandPayload } from './commandCenterState'
import { commandPreviewCoversFullFilterScope } from './commandCenterSubmitGate'
import { useCommandCenterJobSupportBundle } from './useCommandCenterJobSupportBundle'

const COMMAND_JOB_DETAIL_ROWS_PAGE_SIZE = 50
const COMMAND_JOB_HISTORY_PAGE_SIZE = 10
const commandJobRowsStatusFilters: CommandJobRowsStatusFilter[] = [
  'all',
  'needs_attention',
  'retryable',
  'retry_ready',
  'retry_waiting',
  'retry_exhausted',
  'device_failed',
  'failed',
  'missing_log',
  'in_progress',
  'canceled'
]

interface UseCommandCenterJobWorkbenchOptions {
  buildPayload: () => FleetCommandJobPayload
  validatePayload: () => boolean
  currentPayloadFingerprint: ComputedRef<string>
  isDeviceFilterScope: ComputedRef<boolean>
  setActiveCommandJobQuery: (jobId: string) => Promise<void>
  t: (key: string) => string
}

interface OpenCommandJobDetailOptions {
  rowsStatusFilter?: CommandJobRowsStatusFilter
  rowsSearch?: string
}

export const useCommandCenterJobWorkbench = (options: UseCommandCenterJobWorkbenchOptions) => {
  const previewLoading = ref(false)
  const submitLoading = ref(false)
  const jobActionLoading = ref(false)
  const previewResult = ref<FleetCommandJobPreviewResult | null>(null)
  const submitResult = ref<FleetCommandJobSubmitResult | null>(null)
  const jobHistory = ref<FleetCommandJobListResult>({ total: 0, list: [] })
  const jobHistoryPage = ref(1)
  const jobHistoryLoading = ref(false)
  const jobHistoryStatus = ref<string | null>(null)
  const jobHistoryAttentionFilter = ref<string | null>(null)
  const jobHistorySearch = ref('')
  const commandJobRowsLoading = ref(false)
  const commandJobRowsStatusFilter = ref<CommandJobRowsStatusFilter>('all')
  const commandJobRowsSearch = ref('')
  const commandJobError = ref('')
  const filterScopeBackendRejected = ref(false)
  const previewPayloadFingerprint = ref('')
  const {
    clearCommandJobSupportBundle,
    copyCommandJobSupportBundle,
    downloadCommandJobSupportBundle,
    loadCommandJobSupportBundle,
    supportBundle,
    supportBundleLoading
  } = useCommandCenterJobSupportBundle({
    activeJobId: () => submitResult.value?.job_id,
    setError: message => {
      commandJobError.value = message
    },
    t: options.t
  })
  let historyRequestSeq = 0
  let previewRequestSeq = 0
  let submitRequestSeq = 0
  let jobActionRequestSeq = 0
  let jobRowsRequestSeq = 0

  const retryableFailedRows = computed(() => submitResult.value?.rows.filter(row => row.can_retry) ?? [])
  const canLoadMoreCommandJobRows = computed(() => {
    const result = submitResult.value
    if (!result?.job_id) return false
    const total = result.rows_total ?? result.rows.length
    return result.rows.length < total
  })
  const canRetryLoadedCommandJob = computed(() => {
    const result = submitResult.value
    const readyCount = result?.retry_ready_count ?? retryableFailedRows.value.filter(row => {
      if (row.retry_state) return row.retry_state === 'retryable'
      if (!row.next_retry_after) return true
      const retryAt = Date.parse(row.next_retry_after)
      return !Number.isFinite(retryAt) || retryAt <= Date.now()
    }).length
    return Boolean(result?.can_retry_failed && readyCount > 0)
  })
  const commandJobRowsStatusFilterOptions = computed(() =>
    commandJobRowsStatusFilters.map(value => ({
      label: options.t(`custom.commandCenter.rowsFilter.${value}`),
      value
    }))
  )
  const canLoadMoreJobHistory = computed(() => jobHistory.value.list.length < jobHistory.value.total)
  const canAutoRefreshCommandJob = computed(
    () =>
      !jobActionLoading.value &&
      !commandJobRowsLoading.value &&
      !supportBundleLoading.value &&
      !jobHistoryLoading.value
  )
  const activeJobWarnings = computed(() => {
    const result = submitResult.value ?? previewResult.value
    if (!result) return []
    return [...(result.warnings ?? []), ...(result.scope_limits ?? [])]
  })

  const resetCommandJobDraft = () => {
    previewRequestSeq++
    submitRequestSeq++
    jobActionRequestSeq++
    jobRowsRequestSeq++
    previewResult.value = null
    submitResult.value = null
    clearCommandJobSupportBundle()
    previewPayloadFingerprint.value = ''
    commandJobError.value = ''
    filterScopeBackendRejected.value = false
    commandJobRowsStatusFilter.value = 'all'
    commandJobRowsSearch.value = ''
  }

  const formatCommandJobError = (error: unknown, fallbackKey: string) => {
    const message = error instanceof Error ? error.message : ''
    if (options.isDeviceFilterScope.value && message.includes('selected_devices')) {
      filterScopeBackendRejected.value = true
      return `${options.t('custom.commandCenter.filterScopeBackendRejected')} ${message}`
    }
    return message || options.t(fallbackKey)
  }

  const mergeCommandJobSummary = (
    current: FleetCommandJobSubmitResult,
    summary: FleetCommandJobSubmitResult
  ): FleetCommandJobSubmitResult => ({
    ...current,
    ...summary,
    rows: current.rows,
    rows_total: summary.rows_total ?? current.rows_total ?? current.rows.length,
    rows_truncated: summary.rows_truncated ?? current.rows_truncated
  })

  const loadCommandJobHistory = async (page = 1, append = false) => {
    const requestSeq = ++historyRequestSeq
    const status = jobHistoryStatus.value || undefined
    const attentionFilter = jobHistoryAttentionFilter.value || undefined
    const search = jobHistorySearch.value.trim()
    jobHistoryLoading.value = true
    try {
      const result = normalizeApiData(
        await listFleetCommandJobs({
          page,
          page_size: COMMAND_JOB_HISTORY_PAGE_SIZE,
          status,
          attention_filter: attentionFilter,
          search: search || undefined
        })
      )
      if (requestSeq !== historyRequestSeq) return
      jobHistorySearch.value = result.search ?? search
      jobHistoryAttentionFilter.value = result.attention_filter ?? attentionFilter ?? null
      jobHistoryPage.value = page
      jobHistory.value = append
        ? {
            ...result,
            list: [...jobHistory.value.list, ...result.list]
          }
        : result
    } catch (error) {
      if (requestSeq === historyRequestSeq) {
        commandJobError.value =
          error instanceof Error ? error.message : options.t('custom.commandCenter.jobHistoryLoadFailed')
      }
    } finally {
      if (requestSeq === historyRequestSeq) {
        jobHistoryLoading.value = false
      }
    }
  }

  const loadMoreCommandJobHistory = async () => {
    if (jobHistoryLoading.value || !canLoadMoreJobHistory.value) return
    await loadCommandJobHistory(jobHistoryPage.value + 1, true)
  }

  const setJobHistorySearch = async (search: string) => {
    const nextSearch = search.trim()
    jobHistorySearch.value = nextSearch
    await loadCommandJobHistory()
  }

  const clearJobHistorySearch = async () => {
    jobHistorySearch.value = ''
    await loadCommandJobHistory()
  }

  const setJobHistoryAttentionFilter = async (attentionFilter: string | null) => {
    jobHistoryAttentionFilter.value = attentionFilter || null
    await loadCommandJobHistory()
  }

  const previewCommandJob = async () => {
    if (!options.validatePayload()) return
    const payload = options.buildPayload()
    const payloadFingerprint = serializeFleetCommandPayload(payload)
    const requestSeq = ++previewRequestSeq
    previewLoading.value = true
    filterScopeBackendRejected.value = false
    submitResult.value = null
    try {
      const result = normalizeApiData(await previewFleetCommandJob(payload))
      if (requestSeq !== previewRequestSeq) return
      previewResult.value = result
      previewPayloadFingerprint.value = payloadFingerprint
    } catch (error) {
      if (requestSeq === previewRequestSeq) {
        commandJobError.value = formatCommandJobError(error, 'custom.commandCenter.previewFailed')
      }
    } finally {
      if (requestSeq === previewRequestSeq) {
        previewLoading.value = false
      }
    }
  }

  const submitCommandJob = async () => {
    if (!options.validatePayload()) return
    const payload = options.buildPayload()
    if (previewPayloadFingerprint.value !== serializeFleetCommandPayload(payload)) {
      commandJobError.value = options.t('custom.commandCenter.previewBeforeSubmit')
      return
    }
    if (
      !commandPreviewCoversFullFilterScope({
        isDeviceFilterScope: options.isDeviceFilterScope.value,
        previewResult: previewResult.value
      })
    ) {
      commandJobError.value = options
        .t('custom.commandCenter.submitBlockedSubsetOnly')
        .replace('{shown}', String(previewResult.value?.rows.length ?? 0))
        .replace('{matched}', String(previewResult.value?.requested_count ?? 0))
        .replace('{max}', String(payload.max_devices ?? '--'))
      return
    }
    if (previewResult.value?.preview_token) {
      payload.preview_token = previewResult.value.preview_token
    }
    submitLoading.value = true
    filterScopeBackendRejected.value = false
    const requestSeq = ++submitRequestSeq
    try {
      const result = normalizeApiData(await submitFleetCommandJob(payload, { include_rows: false }))
      if (requestSeq !== submitRequestSeq) return
      commandJobRowsStatusFilter.value = 'all'
      commandJobRowsSearch.value = ''
      submitResult.value = commandJobActionResultAsPagedSummary(result)
      clearCommandJobSupportBundle()
      if (result.job_id) void loadCommandJobRows(result.job_id)
      void loadCommandJobHistory()
    } catch (error) {
      if (requestSeq === submitRequestSeq) {
        commandJobError.value = formatCommandJobError(error, 'custom.commandCenter.submitFailed')
      }
    } finally {
      if (requestSeq === submitRequestSeq) {
        submitLoading.value = false
      }
    }
  }

  const refreshCommandJob = async () => {
    const currentJob = submitResult.value
    if (!currentJob?.job_id) return
    const requestSeq = ++jobActionRequestSeq
    jobActionLoading.value = true
    try {
      const summary = normalizeApiData(await getFleetCommandJobSummary(currentJob.job_id))
      const latestJob = submitResult.value
      if (requestSeq !== jobActionRequestSeq) return
      if (latestJob?.job_id !== currentJob.job_id) return
      submitResult.value = mergeCommandJobSummary(latestJob, summary)
      clearCommandJobSupportBundle()
    } catch (error) {
      if (requestSeq === jobActionRequestSeq) {
        commandJobError.value = error instanceof Error ? error.message : options.t('custom.commandCenter.jobStatusLoadFailed')
      }
    } finally {
      if (requestSeq === jobActionRequestSeq) {
        jobActionLoading.value = false
      }
    }
  }

  const loadCommandJobRows = async (
    jobId: string,
    page = 1,
    append = false,
    statusFilter = commandJobRowsStatusFilter.value,
    search = commandJobRowsSearch.value
  ) => {
    const requestSeq = ++jobRowsRequestSeq
    commandJobRowsLoading.value = true
    const normalizedSearch = search.trim()
    try {
      const rowsResult = normalizeApiData(
        await getFleetCommandJobRows(jobId, {
          page,
          page_size: COMMAND_JOB_DETAIL_ROWS_PAGE_SIZE,
          status_filter: statusFilter,
          search: normalizedSearch || undefined
        })
      )
      const currentJob = submitResult.value
      if (requestSeq !== jobRowsRequestSeq || currentJob?.job_id !== jobId) return
      const normalizedStatusFilter = rowsResult.status_filter || statusFilter
      commandJobRowsStatusFilter.value = normalizedStatusFilter
      commandJobRowsSearch.value = rowsResult.search ?? normalizedSearch
      const rows = append ? [...currentJob.rows, ...rowsResult.rows] : rowsResult.rows
      submitResult.value = {
        ...currentJob,
        rows,
        rows_total: rowsResult.total,
        rows_truncated: rowsResult.rows_truncated
      }
    } catch (error) {
      if (requestSeq === jobRowsRequestSeq) {
        commandJobError.value = error instanceof Error ? error.message : options.t('custom.commandCenter.jobStatusLoadFailed')
      }
    } finally {
      if (requestSeq === jobRowsRequestSeq) {
        commandJobRowsLoading.value = false
      }
    }
  }

  const loadMoreCommandJobRows = async () => {
    const currentJob = submitResult.value
    if (!currentJob?.job_id || commandJobRowsLoading.value || !canLoadMoreCommandJobRows.value) return
    const nextPage = Math.floor(currentJob.rows.length / COMMAND_JOB_DETAIL_ROWS_PAGE_SIZE) + 1
    await loadCommandJobRows(
      currentJob.job_id,
      nextPage,
      true,
      commandJobRowsStatusFilter.value,
      commandJobRowsSearch.value
    )
  }

  const setCommandJobRowsStatusFilter = async (statusFilter: CommandJobRowsStatusFilter) => {
    if (commandJobRowsStatusFilter.value === statusFilter) return
    commandJobRowsStatusFilter.value = statusFilter
    const currentJob = submitResult.value
    if (!currentJob?.job_id) return
    await loadCommandJobRows(currentJob.job_id, 1, false, statusFilter)
  }

  const setCommandJobRowsSearch = async (search: string) => {
    const nextSearch = search.trim()
    if (commandJobRowsSearch.value === nextSearch) return
    commandJobRowsSearch.value = nextSearch
    const currentJob = submitResult.value
    if (!currentJob?.job_id) return
    await loadCommandJobRows(currentJob.job_id, 1, false, commandJobRowsStatusFilter.value, nextSearch)
  }

  const clearCommandJobRowsSearch = async () => {
    if (!commandJobRowsSearch.value) return
    await setCommandJobRowsSearch('')
  }

  const reviewCommandJobRows = async (statusFilter: CommandJobRowsStatusFilter) => {
    commandJobRowsStatusFilter.value = statusFilter
    commandJobRowsSearch.value = ''
    const currentJob = submitResult.value
    if (!currentJob?.job_id) return
    await loadCommandJobRows(currentJob.job_id, 1, false, statusFilter, '')
  }

  const commandJobActionResultAsPagedSummary = (
    result: FleetCommandJobSubmitResult
  ): FleetCommandJobSubmitResult => {
    const rowsTotal = result.rows_total ?? result.rows.length
    return {
      ...result,
      rows: [],
      rows_total: rowsTotal,
      rows_truncated: rowsTotal > 0
    }
  }

  const cancelCommandJob = async () => {
    if (!submitResult.value?.job_id || !submitResult.value.can_cancel) return
    const jobId = submitResult.value.job_id
    const requestSeq = ++jobActionRequestSeq
    jobActionLoading.value = true
    try {
      const result = normalizeApiData(await cancelFleetCommandJob(jobId, { include_rows: false }))
      if (requestSeq !== jobActionRequestSeq) return
      commandJobRowsSearch.value = ''
      submitResult.value = commandJobActionResultAsPagedSummary(result)
      clearCommandJobSupportBundle()
      void loadCommandJobRows(jobId)
      void loadCommandJobHistory()
    } catch (error) {
      if (requestSeq === jobActionRequestSeq) {
        commandJobError.value = error instanceof Error ? error.message : options.t('custom.commandCenter.jobCancelFailed')
      }
    } finally {
      if (requestSeq === jobActionRequestSeq) {
        jobActionLoading.value = false
      }
    }
  }

  const retryCommandJob = async () => {
    if (!submitResult.value?.job_id || !canRetryLoadedCommandJob.value) return
    const jobId = submitResult.value.job_id
    const requestSeq = ++jobActionRequestSeq
    jobActionLoading.value = true
    try {
      const result = normalizeApiData(await retryFleetCommandJob(jobId, { include_rows: false }))
      if (requestSeq !== jobActionRequestSeq) return
      commandJobRowsSearch.value = ''
      submitResult.value = commandJobActionResultAsPagedSummary(result)
      clearCommandJobSupportBundle()
      void loadCommandJobRows(jobId)
      void loadCommandJobHistory()
    } catch (error) {
      if (requestSeq === jobActionRequestSeq) {
        commandJobError.value = error instanceof Error ? error.message : options.t('custom.commandCenter.jobRetryFailed')
      }
    } finally {
      if (requestSeq === jobActionRequestSeq) {
        jobActionLoading.value = false
      }
    }
  }

  const copyRetryableDeviceIds = async () => {
    const deviceIds = retryableFailedRows.value.map(row => row.device_id).filter(Boolean)
    if (deviceIds.length === 0) {
      window.$message?.warning(options.t('custom.commandCenter.copyFailedDeviceIdsEmpty'))
      return
    }
    const ok = await writeClipboardText(deviceIds.join(','))
    if (ok) window.$message?.success(options.t('custom.commandCenter.copyFailedDeviceIdsSuccess'))
    else window.$message?.warning(options.t('common.copyFailed'))
  }

  const openCommandJobDetail = async (jobId: string, detailOptions: OpenCommandJobDetailOptions = {}) => {
    const rowsStatusFilter = detailOptions.rowsStatusFilter ?? 'all'
    const rowsSearch = detailOptions.rowsSearch ?? ''
    const requestSeq = ++jobActionRequestSeq
    jobActionLoading.value = true
    try {
      const result = normalizeApiData(await getFleetCommandJobSummary(jobId))
      if (requestSeq !== jobActionRequestSeq) return
      submitResult.value = result
      clearCommandJobSupportBundle()
      commandJobRowsSearch.value = rowsSearch
      commandJobRowsStatusFilter.value = rowsStatusFilter
      await options.setActiveCommandJobQuery(jobId)
      await loadCommandJobRows(jobId, 1, false, rowsStatusFilter, rowsSearch)
    } catch (error) {
      if (requestSeq === jobActionRequestSeq) {
        commandJobError.value = error instanceof Error ? error.message : options.t('custom.commandCenter.jobStatusLoadFailed')
      }
    } finally {
      if (requestSeq === jobActionRequestSeq) {
        jobActionLoading.value = false
      }
    }
  }

  return {
    activeJobWarnings,
    canLoadMoreJobHistory,
    canLoadMoreCommandJobRows,
    canAutoRefreshCommandJob,
    commandJobError,
    commandJobRowsLoading,
    commandJobRowsSearch,
    commandJobRowsStatusFilter,
    commandJobRowsStatusFilterOptions,
    copyCommandJobSupportBundle,
    copyRetryableDeviceIds,
    clearCommandJobRowsSearch,
    cancelCommandJob,
    downloadCommandJobSupportBundle,
    filterScopeBackendRejected,
    jobActionLoading,
    jobHistory,
    jobHistoryAttentionFilter,
    jobHistoryLoading,
    jobHistorySearch,
    jobHistoryStatus,
    loadCommandJobHistory,
    loadMoreCommandJobHistory,
    clearJobHistorySearch,
    loadCommandJobSupportBundle,
    loadMoreCommandJobRows,
    openCommandJobDetail,
    previewCommandJob,
    previewLoading,
    previewPayloadFingerprint,
    previewResult,
    resetCommandJobDraft,
    refreshCommandJob,
    reviewCommandJobRows,
    retryableFailedRows,
    retryCommandJob,
    setCommandJobRowsStatusFilter,
    setCommandJobRowsSearch,
    setJobHistoryAttentionFilter,
    setJobHistorySearch,
    submitCommandJob,
    submitLoading,
    submitResult,
    supportBundle,
    supportBundleLoading
  }
}
