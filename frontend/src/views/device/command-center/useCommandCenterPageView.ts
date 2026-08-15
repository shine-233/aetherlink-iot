import { computed, type Ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import type {
  CommandJobRowsStatusFilter,
  FleetCommandJobListItem,
  FleetCommandJobListResult,
  FleetCommandJobPreviewResult
} from '@/service/api/device'
import { buildCommandCenterGuideSteps } from './commandCenterGuide'
import {
  buildCommandJobHistoryAttentionOptions,
  buildCommandJobHistoryAttentionTotalSummary,
  buildCommandJobHistoryStatusOptions
} from './commandCenterJobView'
import {
  buildCommandCenterContractRows,
  commandCenterImmediateChecks,
  commandCenterJobRequirements,
  commandCenterPostSubmitChecklist,
  formatCommandCenterRecommendedPath,
  formatCommandCenterTelemetryEvidence
} from './commandCenterPageView'
import { buildCommandCenterPreviewExplanationRows } from './commandCenterPreviewExplanation'
import {
  buildCommandJobReadiness,
  buildFilterExecutionCapSummary,
  commandPreviewCoversFullFilterScope
} from './commandCenterSubmitGate'
import {
  buildCommandJobEligibilityImpactPreview,
  buildCommandJobPreviewActionPlan,
  buildFilteredFleetEligibilityPreview,
  buildRouteDecisionSummary,
  type FleetCommandScopeType
} from './commandCenterState'
import {
  createCommandJobHistoryColumns,
  createCommandJobPreviewColumns,
  createCommandJobSubmitColumns
} from './commandCenterTableColumns'

type Translate = (key: any) => string

type ScopeContextLike = {
  savedFilterName?: string
}

type UseCommandCenterPageViewInput = {
  activeSavedFleetFilterName: () => string | undefined
  commandIdentify: Ref<string>
  currentPageCount: Ref<number | null>
  currentPayloadFingerprint: Ref<string>
  filterSummaryCount: () => number
  hasCommandJobScope: Ref<boolean>
  isDeviceFilterScope: Ref<boolean>
  jobHistory: Ref<FleetCommandJobListResult>
  maxDevices: Ref<number | null | undefined>
  openCommandJobDetail: (
    jobId: string,
    options?: { rowsStatusFilter?: CommandJobRowsStatusFilter; rowsSearch?: string }
  ) => void
  openFleet: () => void
  previewCommandJob: () => void
  previewLoading: Ref<boolean>
  previewPayloadFingerprint: Ref<string>
  previewResult: Ref<FleetCommandJobPreviewResult | null | undefined>
  requestedTotal: Ref<number | null>
  reuseCommandJobDraft: (job: FleetCommandJobListItem) => void
  saveCommandJobTemplate: (job: FleetCommandJobListItem) => void
  routeScope: Ref<string>
  subsetLimit: Ref<number | null | undefined>
  scope: Ref<string>
  scopeContext: () => ScopeContextLike
  selectedCount: Ref<number>
  submitCommandJob: () => void
  submitLoading: Ref<boolean>
  submitResult: Ref<unknown>
  t: Translate
}

export function useCommandCenterPageView(input: UseCommandCenterPageViewInput) {
  const routeDecisionSummary = computed(() => buildRouteDecisionSummary(input.previewResult.value?.rows ?? []))
  const filteredFleetEligibilityPreview = computed(() =>
    buildFilteredFleetEligibilityPreview({
      isDeviceFilterScope: input.isDeviceFilterScope.value,
      previewResult: input.previewResult.value
    })
  )
  const previewTokenShort = computed(() => input.previewResult.value?.preview_token?.slice(0, 12) || '--')
  const previewCoversFullFilterScope = computed(() =>
    commandPreviewCoversFullFilterScope({
      isDeviceFilterScope: input.isDeviceFilterScope.value,
      previewResult: input.previewResult.value
    })
  )
  const commandJobReadiness = computed(() =>
    buildCommandJobReadiness(
      {
        hasCommandJobScope: input.hasCommandJobScope.value,
        commandIdentify: input.commandIdentify.value,
        previewResult: input.previewResult.value,
        previewPayloadFingerprint: input.previewPayloadFingerprint.value,
        currentPayloadFingerprint: input.currentPayloadFingerprint.value,
        previewCoversFullFilterScope: previewCoversFullFilterScope.value,
        maxDevices: input.maxDevices.value
      },
      input.t
    )
  )
  const commandSubmitDisabledHint = computed(() => commandJobReadiness.value.blockingReason)
  const commandJobPreviewActionPlan = computed(() =>
    buildCommandJobPreviewActionPlan({
      previewResult: input.previewResult.value,
      fallbackNextAction: commandJobReadiness.value.requiredNextAction
    })
  )
  const commandJobEligibilityImpactPreview = computed(() =>
    buildCommandJobEligibilityImpactPreview({
      isDeviceFilterScope: input.isDeviceFilterScope.value,
      previewResult: input.previewResult.value,
      fallbackNextAction: commandJobReadiness.value.requiredNextAction
    })
  )
  const canPreviewCommandJobNow = computed(() => commandJobReadiness.value.canPreview)
  const canSubmitCommandJobNow = computed(() => commandJobReadiness.value.canSubmit)
  const showPreviewRecoveryAction = computed(
    () =>
      Boolean(commandSubmitDisabledHint.value) && !canSubmitCommandJobNow.value && canPreviewCommandJobNow.value
  )
  const commandJobReadinessTagType = computed(() => {
    if (commandJobReadiness.value.customerRiskLevel === 'ready') return 'success'
    if (commandJobReadiness.value.customerRiskLevel === 'warning') return 'warning'
    return 'error'
  })
  const filterExecutionCapSummary = computed(() =>
    buildFilterExecutionCapSummary(
      {
        requestedTotal: input.requestedTotal.value,
        maxDevices: input.maxDevices.value,
        subsetLimit: input.subsetLimit.value
      },
      input.t
    )
  )
  const previewExplanationRows = computed(() =>
    buildCommandCenterPreviewExplanationRows(
      {
        scopeType: input.scope.value as FleetCommandScopeType,
        selectedCount: input.selectedCount.value,
        currentPageCount: input.currentPageCount.value,
        requestedTotal: input.requestedTotal.value,
        previewRequestedCount: input.previewResult.value?.requested_count,
        previewShownCount: input.previewResult.value?.rows.length,
        activeSavedFilterName: input.activeSavedFleetFilterName() || input.scopeContext().savedFilterName,
        maxDevices: input.maxDevices.value,
        subsetLimit: input.subsetLimit.value,
        canSubmitCommandJob: canSubmitCommandJobNow.value,
        submitDisabledHint: commandSubmitDisabledHint.value
      },
      input.t
    )
  )
  const jobHistoryStatusOptions = computed(() => buildCommandJobHistoryStatusOptions(input.t))
  const jobHistoryAttentionOptions = computed(() =>
    buildCommandJobHistoryAttentionOptions(input.t, input.jobHistory.value.attention_counts)
  )
  const jobHistoryAttentionSummary = computed(() =>
    buildCommandJobHistoryAttentionTotalSummary(input.jobHistory.value.attention_counts, input.t)
  )
  const operatorGuideSteps = computed(() =>
    buildCommandCenterGuideSteps(
      {
        hasCommandJobScope: input.hasCommandJobScope.value,
        hasCommandIdentifier: Boolean(input.commandIdentify.value.trim()),
        hasPreviewResult: Boolean(input.previewResult.value),
        hasSubmitResult: Boolean(input.submitResult.value),
        canPreviewCommandJob: canPreviewCommandJobNow.value,
        canSubmitCommandJob: canSubmitCommandJobNow.value,
        previewLoading: input.previewLoading.value,
        submitLoading: input.submitLoading.value
      },
      {
        openFleet: input.openFleet,
        previewCommandJob: input.previewCommandJob,
        submitCommandJob: input.submitCommandJob
      }
    )
  )
  const previewColumns = computed(() =>
    createCommandJobPreviewColumns({
      formatRecommendedPath: path => formatCommandCenterRecommendedPath(path, input.t),
      formatTelemetryEvidence: row => formatCommandCenterTelemetryEvidence(row, input.t),
      t: input.t
    })
  )
  const submitColumns = computed(() => createCommandJobSubmitColumns(input.t))
  const jobHistoryColumns = computed<DataTableColumns<FleetCommandJobListItem>>(() =>
    createCommandJobHistoryColumns({
      openCommandJobDetail: input.openCommandJobDetail,
      reuseCommandJobDraft: input.reuseCommandJobDraft,
      saveCommandJobTemplate: input.saveCommandJobTemplate,
      t: input.t
    })
  )
  const contractRows = computed(() =>
    buildCommandCenterContractRows(
      {
        currentPageCount: input.currentPageCount.value,
        filterSummaryCount: input.filterSummaryCount(),
        requestedTotal: input.requestedTotal.value,
        routeScope: input.routeScope.value,
        scope: input.scope.value,
        selectedCount: input.selectedCount.value
      },
      input.t
    )
  )

  return {
    canPreviewCommandJobNow,
    canSubmitCommandJobNow,
    commandJobReadiness,
    commandJobReadinessTagType,
    commandJobEligibilityImpactPreview,
    commandJobPreviewActionPlan,
    commandSubmitDisabledHint,
    contractRows,
    filterExecutionCapSummary,
    filteredFleetEligibilityPreview,
    immediateChecks: commandCenterImmediateChecks,
    jobHistoryColumns,
    jobHistoryAttentionOptions,
    jobHistoryAttentionSummary,
    jobHistoryStatusOptions,
    jobRequirements: commandCenterJobRequirements,
    operatorGuideSteps,
    postSubmitChecklist: commandCenterPostSubmitChecklist,
    previewColumns,
    previewCoversFullFilterScope,
    previewExplanationRows,
    previewTokenShort,
    routeDecisionSummary,
    showPreviewRecoveryAction,
    submitColumns
  }
}
