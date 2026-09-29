/*
 * Wiring layer for the command-center page.
 *
 * The page composes a dozen feature composables (scope/draft/templates/saved filters/job
 * workbench/evidence view/session). Keeping that wiring here leaves `index.vue` free to focus on
 * layout and event binding, and makes the whole page state machine testable without mounting it.
 */
import { computed, onMounted, ref, watch } from 'vue'
import type { Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { $t } from '@/locales'
import { useViewportDeferredMount } from '@/hooks/common/useViewportDeferredMount'
import type { FleetCommandJobListItem } from '@/service/api/device'
import { useCommandCenterRouteScope } from './useCommandCenterRouteScope'
import { useCommandCenterRouteDraftSync } from './useCommandCenterRouteDraftSync'
import { useCommandCenterJobFollowUpActions } from './useCommandCenterJobFollowUpActions'
import { useCommandCenterJobWorkbench } from './useCommandCenterJobWorkbench'
import { useCommandCenterSubmitEvidenceView } from './useCommandCenterSubmitEvidenceView'
import { useCommandCenterDraft } from './useCommandCenterDraft'
import { useCommandCenterCommandTemplates } from './useCommandCenterCommandTemplates'
import { useCommandCenterTemplateActions } from './useCommandCenterTemplateActions'
import { useCommandCenterNavigation } from './useCommandCenterNavigation'
import { useCommandCenterSavedFleetFilters } from './useCommandCenterSavedFleetFilters'
import { buildCommandJobHistoryAttentionAggregateRows } from './commandCenterJobView'
import { buildCommandScopeSafety } from './commandCenterScopeSafety'
import { useCommandCenterPageView } from './useCommandCenterPageView'
import { useCommandCenterJobSession } from './useCommandCenterJobSession'
import { buildCommandJobResultViewModel } from './commandCenterJobResultViewModel'
import type { CommandJobResultActions } from './commandCenterJobResultViewModel'
import { buildClearedSavedFilterQuery, buildRenamedSavedFilterQuery } from './commandCenterRouteQuery'
import { buildCommandJobProgressSteps } from './commandCenterProgressFlow'

const scheduleIdleTask = (task: () => void, fallbackDelay = 120) => {
  if (typeof window === 'undefined') {
    task()
    return
  }
  if ('requestIdleCallback' in window) {
    window.requestIdleCallback(task, { timeout: 2000 })
    return
  }
  ;(window as Window).setTimeout(task, fallbackDelay)
}

export function useCommandCenterPageController() {
  const route = useRoute()
  const router = useRouter()

  const {
    activeCommandJobId,
    currentPageCount,
    deviceFilter,
    filterSummaryItems,
    hasCommandJobScope,
    hasDeviceFilter,
    hasSelectedDevices,
    isDeviceFilterScope,
    requestedTotal,
    routeCommandDraft,
    routeScope,
    scope,
    scopeContext,
    selectedCount,
    selectedDeviceIds,
    setActiveCommandJobQuery
  } = useCommandCenterRouteScope()

  let setCommandJobError: (message: string) => void = () => undefined

  const {
    buildCurrentFleetCommandPayload,
    commandIdentify,
    commandValue,
    currentPayloadFingerprint,
    maxDevices,
    scheduledAt,
    subsetLimit,
    timeoutSeconds,
    validateFleetCommandPayload
  } = useCommandCenterDraft({
    selectedDeviceIds: () => selectedDeviceIds.value,
    scopeType: () => scope.value,
    deviceFilter: () => deviceFilter.value,
    requestedTotal: () => requestedTotal.value,
    currentPageCount: () => currentPageCount.value,
    source: () => scopeContext.value.source,
    hasSelectedDevices: () => hasSelectedDevices.value,
    hasDeviceFilter: () => hasDeviceFilter.value,
    setError: message => setCommandJobError(message),
    t: $t
  })

  const {
    activeJobWarnings,
    canAutoRefreshCommandJob,
    canLoadMoreJobHistory,
    canLoadMoreCommandJobRows,
    commandJobError,
    commandJobRowsLoading,
    commandJobRowsSearch,
    commandJobRowsStatusFilter,
    commandJobRowsStatusFilterOptions,
    copyCommandJobSupportBundle,
    copyRetryableDeviceIds,
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
    clearCommandJobRowsSearch,
    setCommandJobRowsStatusFilter,
    setCommandJobRowsSearch,
    setJobHistoryAttentionFilter,
    setJobHistorySearch,
    submitCommandJob,
    submitLoading,
    submitResult,
    supportBundle,
    supportBundleLoading
  } = useCommandCenterJobWorkbench({
    buildPayload: buildCurrentFleetCommandPayload,
    currentPayloadFingerprint,
    isDeviceFilterScope,
    setActiveCommandJobQuery,
    t: $t,
    validatePayload: validateFleetCommandPayload
  })

  const jobHistoryAttentionAggregateRows = computed(() =>
    buildCommandJobHistoryAttentionAggregateRows(jobHistory.value.attention_counts, $t)
  )

  setCommandJobError = message => {
    commandJobError.value = message
  }

  const {
    activeSavedFleetFilter,
    clearCommandCenterSavedFilterSelection,
    deleteCommandCenterSavedFilter,
    renameCommandCenterSavedFilter,
    refreshCommandCenterSavedFilters,
    savedFleetFilterActionError,
    savedFleetFilterLoading,
    savedFleetFilterNoticeKey,
    savedFleetFilterOptions,
    savedFleetFilters,
    selectedSavedFleetFilterId,
    staleRouteSavedFilter,
    syncSelectedSavedFleetFilterFromRoute
  } = useCommandCenterSavedFleetFilters({
    getRouteSavedFilterId: () => scopeContext.value.savedFilterId
  })

  const {
    applyRouteCommandDraft,
    clearReusedCommandJobDraft,
    clearRouteCommandDraftNotice,
    reusedCommandJobDraft,
    routeCommandDraftNotice
  } = useCommandCenterRouteDraftSync({
    routeCommandDraft,
    commandIdentify,
    commandValue,
    timeoutSeconds,
    resetCommandJobDraft
  })

  const jobHistoryInitialLoadQueued = ref(false)
  const jobHistoryViewportRef = ref<HTMLElement | null>(null)
  const setJobHistoryViewportRef = (element: HTMLElement | null) => {
    jobHistoryViewportRef.value = element
  }
  const preflightViewportRef = ref<HTMLElement | null>(null)
  const setPreflightViewportRef = (element: HTMLElement | null) => {
    preflightViewportRef.value = element
  }
  const initialJobHistoryLoadRequested = ref(false)
  const { shouldMount: shouldMountJobHistoryPanel, mountNow: mountJobHistoryPanelNow } = useViewportDeferredMount(
    jobHistoryViewportRef,
    {
      rootMargin: '360px 0px',
      fallbackDelay: 500
    }
  )
  const { shouldMount: shouldMountPreflightPanel, mountNow: mountPreflightPanelNow } = useViewportDeferredMount(
    preflightViewportRef,
    {
      rootMargin: '420px 0px',
      fallbackDelay: 1800
    }
  )

  const {
    commandTemplateName,
    deleteCommandTemplate,
    importCommandTemplates,
    saveCommandTemplate,
    savedCommandTemplates
  } = useCommandCenterCommandTemplates()

  const {
    applyBuiltInCommandTemplate,
    applySavedCommandTemplate,
    copyCommandTemplateExport,
    deleteSavedCommandTemplate,
    importSavedCommandTemplates,
    saveCommandJobTemplate,
    saveCurrentCommandTemplate
  } = useCommandCenterTemplateActions({
    commandIdentify,
    commandValue,
    timeoutSeconds: timeoutSeconds as Ref<number>,
    commandTemplateName,
    saveCommandTemplate,
    deleteCommandTemplate,
    importCommandTemplates,
    resetCommandJobDraft,
    clearReusedCommandJobDraft,
    t: $t
  })

  const { applySavedFleetFilterInCommandCenter, openFleet, openImmediateCommand, openOtaJobs } =
    useCommandCenterNavigation({
      router,
      commandIdentify: () => commandIdentify.value,
      deviceFilter: () => deviceFilter.value,
      isDeviceFilterScope: () => isDeviceFilterScope.value,
      previewCommandJob,
      previewResult: () => previewResult.value,
      requestedTotal: () => requestedTotal.value,
      resetCommandJobDraft,
      savedFleetFilters: () => savedFleetFilters.value,
      selectedCount: () => selectedCount.value,
      selectedDeviceIds: () => selectedDeviceIds.value,
      selectedSavedFleetFilterId,
      t: $t
    })

  const {
    canRetryCurrentCommandJob,
    jobAuditSummaryCard,
    jobExecutionSummaryCard,
    jobGovernanceSummaryCard,
    jobDeviceProgressTracks,
    jobOutcomeGroups,
    jobOperatorNextAction,
    jobProgressHealthCard,
    jobProgressPercent,
    jobProgressSummary,
    jobHandoffSummary,
    jobStatusCountRows,
    jobStatusLabel,
    jobStatusRows,
    jobTimelineRows,
    jobTroubleshootingRows,
    submitCapabilitySummary,
    jobActionConsequenceRows,
    submitEvidenceAlertType,
    submitEvidenceSummary,
    submitRowsHiddenCount,
    submitRowsForCustomer,
    supportBundlePreview
  } = useCommandCenterSubmitEvidenceView({
    submitResult,
    supportBundle,
    t: $t
  })

  const {
    canPreviewCommandJobNow,
    canSubmitCommandJobNow,
    commandJobEligibilityImpactPreview,
    commandJobPreviewActionPlan,
    commandJobReadiness,
    commandJobReadinessTagType,
    commandSubmitDisabledHint,
    contractRows,
    filterExecutionCapSummary,
    filteredFleetEligibilityPreview,
    immediateChecks,
    jobHistoryColumns,
    jobHistoryAttentionOptions,
    jobHistoryStatusOptions,
    jobRequirements,
    operatorGuideSteps,
    postSubmitChecklist,
    previewColumns,
    previewExplanationRows,
    previewTokenShort,
    routeDecisionSummary,
    showPreviewRecoveryAction,
    submitColumns
  } = useCommandCenterPageView({
    activeSavedFleetFilterName: () => activeSavedFleetFilter.value?.name,
    commandIdentify,
    currentPageCount,
    currentPayloadFingerprint,
    filterSummaryCount: () => filterSummaryItems.value.length,
    hasCommandJobScope,
    isDeviceFilterScope,
    jobHistory,
    maxDevices,
    openCommandJobDetail,
    openFleet,
    previewCommandJob,
    previewLoading,
    previewPayloadFingerprint,
    previewResult,
    requestedTotal,
    reuseCommandJobDraft,
    saveCommandJobTemplate,
    routeScope,
    subsetLimit,
    scope,
    scopeContext: () => scopeContext.value,
    selectedCount,
    submitCommandJob,
    submitLoading,
    submitResult,
    t: $t
  })

  const {
    copyCommandJobLink,
    copyCommandJobHandoffSummary,
    copyCommandJobCloseoutPacket,
    copyCommandJobEligibilityImpactSummary,
    openCommandJobDeviceDiagnosis
  } = useCommandCenterJobFollowUpActions({
    router,
    t: $t,
    submitResult,
    supportBundle,
    loadCommandJobSupportBundle,
    jobHandoffSummary,
    commandJobEligibilityImpactPreview
  })

  function reuseCommandJobDraft(job: FleetCommandJobListItem) {
    commandIdentify.value = job.identify || ''
    commandValue.value = job.command_value || ''
    timeoutSeconds.value = job.timeout_seconds || 60
    scheduledAt.value = null
    reusedCommandJobDraft.value = {
      jobId: job.job_id,
      identify: job.identify || ''
    }
    resetCommandJobDraft()
    window.$message?.success($t('custom.commandCenter.reuseJobDraftSuccess'))
  }

  const commandScopeSafety = computed(() =>
    buildCommandScopeSafety({
      hasCommandJobScope: hasCommandJobScope.value,
      isDeviceFilterScope: isDeviceFilterScope.value,
      selectedCount: selectedCount.value,
      savedFilterName: activeSavedFleetFilter.value?.name,
      routeSavedFilterName: scopeContext.value.savedFilterName,
      requestedTotal: requestedTotal.value,
      currentPageCount: currentPageCount.value,
      maxDevices: maxDevices.value as number,
      filterCount: filterSummaryItems.value.length,
      t: $t
    })
  )

  const commandJobProgressSteps = computed(() =>
    buildCommandJobProgressSteps({
      scopeReady: hasCommandJobScope.value,
      previewReady: Boolean(previewResult.value),
      submitted: Boolean(submitResult.value),
      supportReady: Boolean(supportBundle.value)
    })
  )

  const {
    clearRecentRunningCommandJob,
    commandJobAutoRefreshActive,
    commandJobAutoRefreshDeferred,
    openRecentRunningCommandJob,
    recentRunningCommandJobId,
    showRecentRunningCommandJob
  } = useCommandCenterJobSession({
    activeCommandJobId,
    canRefreshCommandJob: canAutoRefreshCommandJob,
    jobActionLoading,
    refreshCommandJob,
    openCommandJobDetail,
    submitResult
  })

  const commandJobResult = computed(() =>
    buildCommandJobResultViewModel({
      canLoadMoreCommandJobRows: canLoadMoreCommandJobRows.value,
      canRetryCurrentCommandJob: canRetryCurrentCommandJob.value,
      commandJobRowsLoading: commandJobRowsLoading.value,
      commandJobRowsSearch: commandJobRowsSearch.value,
      commandJobRowsStatusFilter: commandJobRowsStatusFilter.value,
      commandJobRowsStatusFilterOptions: commandJobRowsStatusFilterOptions.value,
      jobActionConsequenceRows: jobActionConsequenceRows.value,
      jobAuditSummaryCard: jobAuditSummaryCard.value,
      jobExecutionSummaryCard: jobExecutionSummaryCard.value,
      jobGovernanceSummaryCard: jobGovernanceSummaryCard.value,
      jobActionLoading: jobActionLoading.value,
      jobAutoRefreshActive: commandJobAutoRefreshActive.value,
      jobAutoRefreshDeferred: commandJobAutoRefreshDeferred.value,
      jobDeviceProgressTracks: jobDeviceProgressTracks.value,
      jobOutcomeGroups: jobOutcomeGroups.value,
      jobOperatorNextAction: jobOperatorNextAction.value,
      jobProgressHealthCard: jobProgressHealthCard.value,
      jobProgressPercent: jobProgressPercent.value,
      jobProgressSummary: jobProgressSummary.value,
      jobHandoffSummary: jobHandoffSummary.value,
      jobStatusCountRows: jobStatusCountRows.value,
      jobStatusLabel: jobStatusLabel.value,
      jobStatusRows: jobStatusRows.value,
      jobTimelineRows: jobTimelineRows.value,
      jobTroubleshootingRows: jobTroubleshootingRows.value,
      postSubmitChecklist,
      retryableFailedRows: retryableFailedRows.value,
      submitCapabilitySummary: submitCapabilitySummary.value,
      submitColumns: submitColumns.value,
      submitEvidenceAlertType: submitEvidenceAlertType.value,
      submitEvidenceSummary: submitEvidenceSummary.value,
      submitResult: submitResult.value,
      submitRowsHiddenCount: submitRowsHiddenCount.value,
      submitRowsForCustomer: submitRowsForCustomer.value,
      supportBundleLoading: supportBundleLoading.value,
      supportBundlePreview: supportBundlePreview.value
    })
  )

  const commandJobActions: CommandJobResultActions = {
    cancelCommandJob,
    copyCommandJobCloseoutPacket,
    copyCommandJobHandoffSummary,
    copyCommandJobLink,
    copyCommandJobSupportBundle,
    copyRetryableDeviceIds,
    clearCommandJobRowsSearch,
    downloadCommandJobSupportBundle,
    loadCommandJobSupportBundle,
    loadMoreCommandJobRows,
    openCommandJobDeviceDiagnosis,
    refreshCommandJob,
    reviewCommandJobRows,
    retryCommandJob,
    setCommandJobRowsSearch,
    setCommandJobRowsStatusFilter
  }

  const renameSavedFleetFilterFromView = async (filterId: string | number, nextName: string) => {
    const renamed = await renameCommandCenterSavedFilter(filterId, nextName)
    if (renamed && String(filterId) === scopeContext.value.savedFilterId) {
      await router.replace({ query: buildRenamedSavedFilterQuery(route.query, nextName) })
    }
    return renamed
  }

  const clearSavedFleetFilterIdentity = async () => {
    clearCommandCenterSavedFilterSelection()
    selectedSavedFleetFilterId.value = null
    resetCommandJobDraft()
    await router.replace({
      path: '/device/command-center',
      query: buildClearedSavedFilterQuery(route.query)
    })
  }

  const queueInitialCommandJobHistoryLoad = () => {
    if (initialJobHistoryLoadRequested.value) return
    initialJobHistoryLoadRequested.value = true
    jobHistoryInitialLoadQueued.value = true
    scheduleIdleTask(() => {
      jobHistoryInitialLoadQueued.value = false
      void loadCommandJobHistory()
    })
  }

  onMounted(() => {
    applyRouteCommandDraft()
    void refreshCommandCenterSavedFilters()
    if (activeCommandJobId.value) {
      void openCommandJobDetail(activeCommandJobId.value)
      mountJobHistoryPanelNow()
      queueInitialCommandJobHistoryLoad()
    }
  })

  watch(activeCommandJobId, jobId => {
    if (!jobId || jobId === submitResult.value?.job_id) return
    void openCommandJobDetail(jobId)
  })

  watch(shouldMountJobHistoryPanel, shouldMount => {
    if (shouldMount) {
      queueInitialCommandJobHistoryLoad()
    }
  })

  watch(
    () => scopeContext.value.savedFilterId,
    () => {
      syncSelectedSavedFleetFilterFromRoute()
    }
  )

  return {
    activeJobWarnings,
    activeSavedFleetFilter,
    applyBuiltInCommandTemplate,
    applySavedCommandTemplate,
    applySavedFleetFilterInCommandCenter,
    canLoadMoreJobHistory,
    canPreviewCommandJobNow,
    canSubmitCommandJobNow,
    clearCommandJobRowsSearch,
    clearRecentRunningCommandJob,
    clearReusedCommandJobDraft,
    clearRouteCommandDraftNotice,
    clearSavedFleetFilterIdentity,
    commandIdentify,
    commandJobActions,
    commandJobEligibilityImpactPreview,
    commandJobError,
    commandJobPreviewActionPlan,
    commandJobProgressSteps,
    commandJobReadiness,
    commandJobReadinessTagType,
    commandJobResult,
    commandScopeSafety,
    commandSubmitDisabledHint,
    commandTemplateName,
    commandValue,
    contractRows,
    copyCommandJobEligibilityImpactSummary,
    copyCommandTemplateExport,
    currentPageCount,
    deleteCommandCenterSavedFilter,
    deleteSavedCommandTemplate,
    filterExecutionCapSummary,
    filterScopeBackendRejected,
    filteredFleetEligibilityPreview,
    filterSummaryItems,
    hasCommandJobScope,
    hasSelectedDevices,
    immediateChecks,
    importSavedCommandTemplates,
    isDeviceFilterScope,
    jobActionLoading,
    jobHistory,
    jobHistoryAttentionAggregateRows,
    jobHistoryAttentionFilter,
    jobHistoryAttentionOptions,
    jobHistoryColumns,
    jobHistoryInitialLoadQueued,
    jobHistoryLoading,
    jobHistorySearch,
    jobHistoryStatus,
    jobHistoryStatusOptions,
    jobRequirements,
    loadCommandJobHistory,
    loadMoreCommandJobHistory,
    clearJobHistorySearch,
    maxDevices,
    mountJobHistoryPanelNow,
    mountPreflightPanelNow,
    openCommandJobDeviceDiagnosis,
    openFleet,
    openImmediateCommand,
    openOtaJobs,
    openRecentRunningCommandJob,
    operatorGuideSteps,
    previewColumns,
    previewCommandJob,
    previewExplanationRows,
    previewLoading,
    previewResult,
    previewTokenShort,
    refreshCommandCenterSavedFilters,
    renameSavedFleetFilterFromView,
    requestedTotal,
    reusedCommandJobDraft,
    routeCommandDraftNotice,
    routeDecisionSummary,
    saveCurrentCommandTemplate,
    saveCommandJobTemplate,
    savedCommandTemplates,
    savedFleetFilterActionError,
    savedFleetFilterLoading,
    savedFleetFilterNoticeKey,
    savedFleetFilterOptions,
    scheduledAt,
    selectedSavedFleetFilterId,
    setJobHistoryAttentionFilter,
    setJobHistorySearch,
    setJobHistoryViewportRef,
    setPreflightViewportRef,
    shouldMountJobHistoryPanel,
    shouldMountPreflightPanel,
    showPreviewRecoveryAction,
    showRecentRunningCommandJob,
    staleRouteSavedFilter,
    submitCommandJob,
    submitLoading,
    submitResult,
    timeoutSeconds,
    recentRunningCommandJobId
  }
}

export type CommandCenterPageController = ReturnType<typeof useCommandCenterPageController>
