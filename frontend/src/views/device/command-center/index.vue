<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import type { Ref } from 'vue'
import { NButton } from 'naive-ui'
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
import {
  buildClearedSavedFilterQuery,
  buildRenamedSavedFilterQuery
} from './commandCenterRouteQuery'
import { buildCommandJobProgressSteps } from './commandCenterProgressFlow'
import CommandCenterDraftNotices from './CommandCenterDraftNotices.vue'
import CommandCenterJobHistorySection from './CommandCenterJobHistorySection.vue'
import CommandCenterPreflightSection from './CommandCenterPreflightSection.vue'
import CommandCenterProgressSection from './CommandCenterProgressSection.vue'

const CommandJobPreviewWorkbench = defineAsyncComponent(() => import('./CommandJobPreviewWorkbench.vue'))
const CommandJobResultView = defineAsyncComponent(() => import('./CommandJobResultView.vue'))
const CommandCenterSavedFilterChooser = defineAsyncComponent(() => import('./CommandCenterSavedFilterChooser.vue'))

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

const scheduleIdleCommandCenterTask = (task: () => void, fallbackDelay = 120) => {
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
  setError: (message) => setCommandJobError(message),
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

setCommandJobError = (message) => {
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
const {
  shouldMount: shouldMountJobHistoryPanel,
  mountNow: mountJobHistoryPanelNow
} = useViewportDeferredMount(jobHistoryViewportRef, {
  rootMargin: '360px 0px',
  fallbackDelay: 500
})
const {
  shouldMount: shouldMountPreflightPanel,
  mountNow: mountPreflightPanelNow
} = useViewportDeferredMount(preflightViewportRef, {
  rootMargin: '420px 0px',
  fallbackDelay: 1800
})
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

const commandScopeSafety = computed(() => {
  return buildCommandScopeSafety({
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
})

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
  scheduleIdleCommandCenterTask(() => {
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
    return
  }
})

watch(activeCommandJobId, (jobId) => {
  if (!jobId || jobId === submitResult.value?.job_id) return
  void openCommandJobDetail(jobId)
})

watch(shouldMountJobHistoryPanel, (shouldMount) => {
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


</script>

<template>
  <div class="command-center-page">
    <div class="command-center-header">
      <div>
        <h1>{{ $t('custom.commandCenter.title') }}</h1>
        <p>{{ $t('custom.commandCenter.subtitle') }}</p>
      </div>
      <NButton secondary @click="openFleet">
        {{ $t('custom.commandCenter.backToFleet') }}
      </NButton>
    </div>

    <NAlert type="info" :show-icon="false">
      {{ $t('custom.commandCenter.contractHint') }}
    </NAlert>
    <CommandCenterProgressSection
      :steps="commandJobProgressSteps"
      :preview-loading="previewLoading"
      :can-preview-command-job-now="canPreviewCommandJobNow"
      @preview="previewCommandJob"
    />

    <NAlert v-if="showRecentRunningCommandJob" type="info" :show-icon="false" class="command-recent-running-job">
      <div>
        <strong>{{ $t('custom.commandCenter.recentRunningJobTitle') }}</strong>
        <span>
          {{
            $t('custom.commandCenter.recentRunningJobDesc').replace(
              '{jobId}',
              recentRunningCommandJobId
            )
          }}
        </span>
      </div>
      <NSpace :size="[8, 8]">
        <NButton size="small" type="primary" :loading="jobActionLoading" @click="openRecentRunningCommandJob">
          {{ $t('custom.commandCenter.recentRunningJobOpen') }}
        </NButton>
        <NButton size="small" secondary @click="clearRecentRunningCommandJob">
          {{ $t('custom.commandCenter.recentRunningJobDismiss') }}
        </NButton>
      </NSpace>
    </NAlert>

    <section class="command-center-section command-center-guide">
      <div class="command-center-section__head">
        <NTag type="info" size="small">{{ $t('custom.commandCenter.guideTag') }}</NTag>
        <h2>{{ $t('custom.commandCenter.guideTitle') }}</h2>
      </div>
      <p>{{ $t('custom.commandCenter.guideDesc') }}</p>
      <div class="command-guide-steps">
        <div v-for="step in operatorGuideSteps" :key="step.key" class="command-guide-step">
          <div class="command-guide-step__top">
            <span class="command-guide-step__index">{{ step.index }}</span>
            <NTag :type="step.statusType" size="small">{{ $t(step.statusKey) }}</NTag>
          </div>
          <h3>{{ $t(step.titleKey) }}</h3>
          <p>{{ $t(step.descKey) }}</p>
          <NButton v-if="step.actionLabelKey" size="small" secondary :disabled="step.disabled" @click="step.action?.()">
            {{ $t(step.actionLabelKey) }}
          </NButton>
        </div>
      </div>
    </section>

    <div class="command-center-grid">
      <section class="command-center-section">
        <div class="command-center-section__head">
          <NTag type="success" size="small">{{ $t('custom.commandCenter.immediateTag') }}</NTag>
          <h2>{{ $t('custom.commandCenter.immediateTitle') }}</h2>
        </div>
        <p>{{ $t('custom.commandCenter.immediateDesc') }}</p>
        <ul>
          <li v-for="item in immediateChecks" :key="item">{{ $t(item) }}</li>
        </ul>
        <NButton type="primary" :disabled="!hasSelectedDevices" @click="openImmediateCommand">
          {{ $t('custom.commandCenter.openImmediateCommand') }}
        </NButton>
      </section>

      <section class="command-center-section">
        <div class="command-center-section__head">
          <NTag type="warning" size="small">{{ $t('custom.commandCenter.jobsTag') }}</NTag>
          <h2>{{ $t('custom.commandCenter.jobsTitle') }}</h2>
        </div>
        <p>{{ $t('custom.commandCenter.jobsDesc') }}</p>
        <CommandCenterSavedFilterChooser
          v-model:selected-saved-fleet-filter-id="selectedSavedFleetFilterId"
          :active-saved-fleet-filter="activeSavedFleetFilter"
          :apply-saved-fleet-filter="applySavedFleetFilterInCommandCenter"
          :clear-saved-fleet-filter-identity="clearSavedFleetFilterIdentity"
          :delete-saved-fleet-filter="deleteCommandCenterSavedFilter"
          :refresh-saved-fleet-filters="refreshCommandCenterSavedFilters"
          :rename-saved-fleet-filter="renameSavedFleetFilterFromView"
          :saved-fleet-filter-action-error="savedFleetFilterActionError"
          :saved-fleet-filter-loading="savedFleetFilterLoading"
          :saved-fleet-filter-notice-key="savedFleetFilterNoticeKey"
          :saved-fleet-filter-options="savedFleetFilterOptions"
          :stale-route-saved-filter="staleRouteSavedFilter"
        />
        <NAlert v-if="isDeviceFilterScope" :type="filterScopeBackendRejected ? 'error' : 'warning'" :show-icon="false">
          {{ $t('custom.commandCenter.filterScopeDraftWarning') }}
        </NAlert>
        <CommandCenterJobHistorySection
          :set-history-viewport-ref="setJobHistoryViewportRef"
          :should-mount-job-history-panel="shouldMountJobHistoryPanel"
          :is-device-filter-scope="isDeviceFilterScope"
          :filter-summary-items="filterSummaryItems"
          :requested-total="requestedTotal"
          :current-page-count="currentPageCount"
          :job-history-search="jobHistorySearch"
          :job-history-loading="jobHistoryLoading"
          :job-history-status="jobHistoryStatus"
          :job-history-status-options="jobHistoryStatusOptions"
          :job-history-attention-filter="jobHistoryAttentionFilter"
          :job-history-attention-options="jobHistoryAttentionOptions"
          :job-history-attention-aggregate-rows="jobHistoryAttentionAggregateRows"
          :job-history-initial-load-queued="jobHistoryInitialLoadQueued"
          :job-history="jobHistory"
          :job-history-columns="jobHistoryColumns"
          :preview-loading="previewLoading"
          :can-preview-command-job-now="canPreviewCommandJobNow"
          :can-load-more-job-history="canLoadMoreJobHistory"
          @update:job-history-search="jobHistorySearch = $event"
          @update:job-history-status="jobHistoryStatus = $event"
          @update:job-history-attention-filter="setJobHistoryAttentionFilter"
          @search="setJobHistorySearch(jobHistorySearch)"
          @clear-search="clearJobHistorySearch"
          @refresh="loadCommandJobHistory()"
          @open-fleet="openFleet"
          @preview="previewCommandJob"
          @load-more="loadMoreCommandJobHistory"
          @mount-panel-now="mountJobHistoryPanelNow"
        />
        <CommandCenterDraftNotices
          :reused-draft="reusedCommandJobDraft"
          :route-draft-notice="routeCommandDraftNotice"
          :preview-loading="previewLoading"
          :can-preview-now="canPreviewCommandJobNow"
          @preview="previewCommandJob"
          @dismiss-reused-draft="clearReusedCommandJobDraft"
          @dismiss-route-draft="clearRouteCommandDraftNotice"
        />
        <CommandJobPreviewWorkbench
          v-model:command-identify="commandIdentify"
          v-model:command-template-name="commandTemplateName"
          v-model:command-value="commandValue"
          v-model:max-devices="maxDevices"
          v-model:scheduled-at="scheduledAt"
          v-model:timeout-seconds="timeoutSeconds"
          :active-job-warnings="activeJobWarnings"
          :can-preview-command-job-now="canPreviewCommandJobNow"
          :can-submit-command-job-now="canSubmitCommandJobNow"
          :command-job-error="commandJobError"
          :command-job-eligibility-impact-preview="commandJobEligibilityImpactPreview"
          :command-job-preview-action-plan="commandJobPreviewActionPlan"
          :command-job-readiness="commandJobReadiness"
          :command-job-readiness-tag-type="commandJobReadinessTagType"
          :command-submit-disabled-hint="commandSubmitDisabledHint"
          :filter-execution-cap-summary="filterExecutionCapSummary"
          :filtered-fleet-eligibility-preview="filteredFleetEligibilityPreview"
          :has-command-job-scope="hasCommandJobScope"
          :is-device-filter-scope="isDeviceFilterScope"
          :job-requirements="jobRequirements"
          :preview-columns="previewColumns"
          :preview-explanation-rows="previewExplanationRows"
          :preview-loading="previewLoading"
          :preview-result="previewResult"
          :preview-token-short="previewTokenShort"
          :route-decision-summary="routeDecisionSummary"
          :saved-command-templates="savedCommandTemplates"
          :scope-safety-description="commandScopeSafety.description"
          :scope-safety-meta="commandScopeSafety.meta"
          :scope-safety-tag="commandScopeSafety.tag"
          :scope-safety-tag-type="commandScopeSafety.tagType"
          :show-preview-recovery-action="showPreviewRecoveryAction"
          :submit-loading="submitLoading"
          @apply-built-in-command-template="applyBuiltInCommandTemplate"
          @apply-saved-command-template="applySavedCommandTemplate"
          @copy-eligibility-impact-summary="copyCommandJobEligibilityImpactSummary"
          @copy-saved-command-template="template => copyCommandTemplateExport([template])"
          @copy-saved-command-templates="() => copyCommandTemplateExport(savedCommandTemplates)"
          @delete-saved-command-template="deleteSavedCommandTemplate"
          @import-saved-command-templates="importSavedCommandTemplates"
          @open-ota-jobs="openOtaJobs"
          @preview-command-job="previewCommandJob"
          @save-command-template="saveCurrentCommandTemplate"
          @submit-command-job="submitCommandJob"
        />
        <CommandJobResultView
          v-if="submitResult"
          :job-result="commandJobResult"
          :job-actions="commandJobActions"
        />
      </section>
    </div>

    <CommandCenterPreflightSection
      :set-preflight-viewport-ref="setPreflightViewportRef"
      :should-mount-preflight-panel="shouldMountPreflightPanel"
      :contract-rows="contractRows"
      :current-page-count="currentPageCount"
      :filter-summary-items="filterSummaryItems"
      :has-command-job-scope="hasCommandJobScope"
      :is-device-filter-scope="isDeviceFilterScope"
      :requested-total="requestedTotal"
      @mount-panel-now="mountPreflightPanelNow"
    />
  </div>
</template>

<style scoped>
.command-center-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
}

.command-center-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.command-center-header h1,
.command-center-section h2 {
  margin: 0;
  color: #0f172a;
}

.command-center-header h1 {
  font-size: 22px;
}

.command-center-header p,
.command-center-section p,
.command-center-section li {
  color: #475569;
  font-size: 13px;
  line-height: 1.6;
}

.command-center-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.command-center-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.command-center-section__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.command-center-section ul {
  margin: 0;
  padding-left: 18px;
}

.command-center-guide {
  gap: 10px;
}

.command-guide-steps {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.command-guide-step {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f8fafc;
}

.command-guide-step__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.command-guide-step__index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 999px;
  background: #0f172a;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.command-guide-step h3 {
  margin: 0;
  color: #0f172a;
  font-size: 14px;
}

.command-guide-step p {
  margin: 0;
}

.command-recent-running-job {
  border-color: #7dd3fc;
  background: #f0f9ff;
}

.command-recent-running-job :deep(.n-alert-body__content) {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-recent-running-job strong {
  display: block;
  margin-bottom: 4px;
  color: #075985;
}

.command-recent-running-job span {
  color: #0369a1;
  font-size: 12px;
}

.mt-3 {
  margin-top: 12px;
}

@media (max-width: 900px) {
  .command-center-header {
    flex-direction: column;
  }

  .command-center-grid {
    grid-template-columns: 1fr;
  }

  .command-guide-steps {
    grid-template-columns: 1fr;
  }

  .command-recent-running-job :deep(.n-alert-body__content) {
    flex-direction: column;
    align-items: flex-start;
  }

}
</style>
