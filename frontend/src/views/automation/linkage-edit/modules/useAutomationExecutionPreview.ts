import { computed, ref } from 'vue'
import type { SceneAutomationDryRunPayload } from '@/service/api/automation'
import {
  buildAutomationDryRunBeginnerGuide,
  buildAutomationDryRunCustomerView,
  buildActionSummaryItems,
  buildAutomationOperatorPlan,
  buildBackendDryRunView,
  buildConditionSummaryItems,
  getAutomationDryRunAlertType,
  getAutomationDryRunStatusText,
  getPreviewErrorText,
  stringifyDryRunResponse,
  type BackendDryRunStatus
} from './automationDryRunPreview'

type ExecutionPreviewPayload = SceneAutomationDryRunPayload | (Partial<SceneAutomationDryRunPayload> & { actions: any[] })
type DryRunService = (payload: ExecutionPreviewPayload) => Promise<any>

export function useAutomationExecutionPreview(options: {
  buildPayload: () => ExecutionPreviewPayload
  dryRun: DryRunService
  getLocalBlocker?: (payload: ExecutionPreviewPayload) => string
}) {
  const executionPreview = ref<ExecutionPreviewPayload | null>(null)
  const backendDryRunStatus = ref<BackendDryRunStatus>('waiting')
  const backendDryRunResponse = ref<any>(null)
  const backendDryRunError = ref('')
  const isBackendDryRunLoading = ref(false)

  const previewConditionGroups = computed(() => executionPreview.value?.trigger_condition_groups || [])
  const previewActions = computed(() => executionPreview.value?.actions || [])
  const previewConditionCount = computed(() =>
    previewConditionGroups.value.reduce((count: number, group: any[]) => count + group.length, 0)
  )
  const previewActionCount = computed(() => previewActions.value.length)
  const localBlockingErrors = computed(() => {
    if (!executionPreview.value || !options.getLocalBlocker) return []
    const blocker = options.getLocalBlocker(executionPreview.value)

    return blocker ? [{ key: 'local-submit-blocker', text: blocker }] : []
  })
  const localPreviewStatusText = computed(() =>
    localBlockingErrors.value.length > 0
      ? 'Local explanation found a save blocker. Fix it before saving or running backend dry-run again.'
      : executionPreview.value
      ? 'Local explanation was generated from the current form. It is not a backend execution result.'
      : 'No local explanation yet. Refresh or save to generate one from the current form.'
  )
  const backendDryRunStatusText = computed(() => getAutomationDryRunStatusText(backendDryRunStatus.value))
  const backendDryRunAlertType = computed(() => getAutomationDryRunAlertType(backendDryRunStatus.value))
  const conditionSummaryItems = computed(() => buildConditionSummaryItems(previewConditionGroups.value))
  const actionSummaryItems = computed(() => buildActionSummaryItems(previewActions.value))
  const operatorPlan = computed(() =>
    buildAutomationOperatorPlan(
      executionPreview.value,
      backendDryRunStatus.value,
      backendDryRunResponse.value,
      backendDryRunError.value
    )
  )
  const backendDryRunView = computed(() => buildBackendDryRunView(backendDryRunResponse.value))
  const customerDryRunView = computed(() =>
    buildAutomationDryRunCustomerView(backendDryRunStatus.value, backendDryRunResponse.value, backendDryRunError.value)
  )
  const beginnerGuideCards = computed(() =>
    buildAutomationDryRunBeginnerGuide({
      status: backendDryRunStatus.value,
      response: backendDryRunResponse.value,
      backendError: backendDryRunError.value,
      customerView: customerDryRunView.value,
      localBlockingErrors: localBlockingErrors.value,
      conditionGroupCount: previewConditionGroups.value.length,
      conditionCount: previewConditionCount.value,
      actionCount: previewActionCount.value
    })
  )
  const dryRunResponseText = computed(() => stringifyDryRunResponse(backendDryRunResponse.value))

  const refreshLocalExecutionExplanation = () => {
    const payload = options.buildPayload()
    executionPreview.value = payload
    backendDryRunStatus.value = 'ready'
    backendDryRunResponse.value = null
    backendDryRunError.value = ''

    return payload
  }

  const runBackendDryRunForPayload = async (payload: ExecutionPreviewPayload) => {
    executionPreview.value = payload
    isBackendDryRunLoading.value = true
    backendDryRunStatus.value = 'pending'
    backendDryRunResponse.value = null
    backendDryRunError.value = ''

    try {
      const res = await options.dryRun(payload)
      if (!res || res?.error) {
        backendDryRunStatus.value = 'unavailable'
        backendDryRunError.value = getPreviewErrorText(res)
        return null
      }

      backendDryRunStatus.value = 'available'
      backendDryRunResponse.value = res?.data ?? res
      return backendDryRunResponse.value
    } catch (error: any) {
      backendDryRunStatus.value = 'unavailable'
      backendDryRunError.value = getPreviewErrorText(error)
      return null
    } finally {
      isBackendDryRunLoading.value = false
    }
  }

  const runBackendDryRun = async () => {
    const payload = refreshLocalExecutionExplanation()
    await runBackendDryRunForPayload(payload)
  }

  return {
    executionPreview,
    backendDryRunStatus,
    backendDryRunResponse,
    backendDryRunError,
    isBackendDryRunLoading,
    previewConditionGroups,
    previewActions,
    previewConditionCount,
    previewActionCount,
    localBlockingErrors,
    localPreviewStatusText,
    backendDryRunStatusText,
    backendDryRunAlertType,
    conditionSummaryItems,
    actionSummaryItems,
    operatorPlan,
    backendDryRunView,
    customerDryRunView,
    beginnerGuideCards,
    dryRunResponseText,
    refreshLocalExecutionExplanation,
    runBackendDryRunForPayload,
    runBackendDryRun
  }
}
