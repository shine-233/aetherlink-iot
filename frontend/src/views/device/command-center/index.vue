<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { NButton } from 'naive-ui'
import { $t } from '@/locales'
import { useCommandCenterPageController } from './useCommandCenterPageController'
import CommandCenterDraftNotices from './CommandCenterDraftNotices.vue'
import CommandCenterJobHistorySection from './CommandCenterJobHistorySection.vue'
import CommandCenterPreflightSection from './CommandCenterPreflightSection.vue'
import CommandCenterProgressSection from './CommandCenterProgressSection.vue'
import './command-center-page.css'

const CommandJobPreviewWorkbench = defineAsyncComponent(() => import('./CommandJobPreviewWorkbench.vue'))
const CommandJobResultView = defineAsyncComponent(() => import('./CommandJobResultView.vue'))
const CommandCenterSavedFilterChooser = defineAsyncComponent(() => import('./CommandCenterSavedFilterChooser.vue'))

const {
  activeJobWarnings,
  activeSavedFleetFilter,
  applyBuiltInCommandTemplate,
  applySavedCommandTemplate,
  applySavedFleetFilterInCommandCenter,
  canLoadMoreJobHistory,
  canPreviewCommandJobNow,
  canSubmitCommandJobNow,
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
} = useCommandCenterPageController()
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
          {{ $t('custom.commandCenter.recentRunningJobDesc').replace('{jobId}', recentRunningCommandJobId) }}
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
          @copy-saved-command-template="(template) => copyCommandTemplateExport([template])"
          @copy-saved-command-templates="() => copyCommandTemplateExport(savedCommandTemplates)"
          @delete-saved-command-template="deleteSavedCommandTemplate"
          @import-saved-command-templates="importSavedCommandTemplates"
          @open-ota-jobs="openOtaJobs"
          @preview-command-job="previewCommandJob"
          @save-command-template="saveCurrentCommandTemplate"
          @submit-command-job="submitCommandJob"
        />
        <CommandJobResultView v-if="submitResult" :job-result="commandJobResult" :job-actions="commandJobActions" />
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
