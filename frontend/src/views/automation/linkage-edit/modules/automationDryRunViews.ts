/**
 * 后端预演结果的三种视图：执行 trace、工程视图（backend view）、客户视图（customer view）。
 * 只消费 NormalizedDryRun；原始响应在入口处经 normalizeDryRunResponse 统一转换。
 */
import { buildCountLines, buildIssueLines, buildStepLines } from './automationDryRunFormat'
import { normalizeDryRunResponse } from './automationDryRunNormalize'
import type {
  DryRunResponseInput,
  NormalizedDryRun,
  NormalizedDryRunDiagnostic,
  NormalizedDryRunTrace
} from './automationDryRunNormalize'
import type {
  AutomationDryRunBackendView,
  AutomationDryRunCustomerView,
  AutomationDryRunDiagnosticItem,
  AutomationDryRunTraceStep,
  AutomationDryRunTraceView,
  BackendDryRunStatus
} from './automationDryRunTypes'

const emptyTraceView = (): AutomationDryRunTraceView => ({
  steps: [],
  stepCount: 0,
  evaluatedAt: '',
  explanation: '',
  isSimulation: true
})

const toTraceStatusType = (status: string): AutomationDryRunTraceStep['statusType'] => {
  if (status === 'evaluated') return 'success'
  if (status === 'skipped') return 'warning'
  if (status === 'blocked') return 'error'

  return 'info'
}

const toTraceView = (trace: NormalizedDryRunTrace | null): AutomationDryRunTraceView => {
  if (!trace) return emptyTraceView()

  return {
    steps: trace.steps.map((step, position) => ({
      key: `trace-step-${step.index ?? position}`,
      index: step.index ?? position + 1,
      phase: step.phase || 'trigger',
      status: step.status || 'evaluated',
      // 缺失 status 时展示为 evaluated，但语气保持中性 info，避免把未知状态渲染为"通过"
      statusType: toTraceStatusType(step.status),
      label: step.label || `step ${position + 1}`,
      kind: step.kind,
      target: step.target,
      detail: step.detail,
      notes: step.notes
    })),
    stepCount: trace.stepCount,
    evaluatedAt: trace.evaluatedAt,
    explanation: trace.explanation,
    isSimulation: trace.isSimulation
  }
}

export const buildTraceView = (response: DryRunResponseInput): AutomationDryRunTraceView =>
  toTraceView(normalizeDryRunResponse(response).trace)

/** 诊断列表为空时，用原始 errors/warnings 合成诊断，保证工程视图不出现空白 */
const resolveDiagnostics = (dryRun: NormalizedDryRun): NormalizedDryRunDiagnostic[] => {
  if (dryRun.diagnostics.length > 0) return dryRun.diagnostics

  return [
    ...dryRun.errors.map((message) => ({
      severity: 'error' as const,
      rawSeverity: 'error',
      scope: 'validation',
      message
    })),
    ...dryRun.rawWarnings.map((message) => ({
      severity: 'warning' as const,
      rawSeverity: 'warning',
      scope: 'warning',
      message
    }))
  ]
}

const toDiagnosticItem = (item: NormalizedDryRunDiagnostic, index: number): AutomationDryRunDiagnosticItem => ({
  key: `diagnostic-${index}`,
  type: item.severity,
  scope: item.scope || 'dry-run',
  message: item.message
})

export const buildBackendDryRunView = (response: DryRunResponseInput): AutomationDryRunBackendView => {
  const dryRun = normalizeDryRunResponse(response)
  if (!dryRun.present) {
    return {
      metrics: [],
      conditionTypes: [],
      actionTypes: [],
      targetKinds: [],
      diagnostics: [],
      nextSteps: [],
      trace: emptyTraceView()
    }
  }

  const { stats } = dryRun

  return {
    metrics: [
      { key: 'valid', text: dryRun.valid === false ? 'Validation: failed' : 'Validation: passed' },
      { key: 'can-save', text: dryRun.canSave === false ? 'Save readiness: blocked' : 'Save readiness: allowed' },
      {
        key: 'conditions',
        text: `Conditions: ${stats.conditionGroupCount} groups / ${stats.conditionCount} rows`
      },
      { key: 'actions', text: `Actions: ${stats.actionCount} rows` }
    ],
    conditionTypes: buildCountLines('condition-type', stats.conditionTypes),
    actionTypes: buildCountLines('action-type', stats.actionTypes),
    targetKinds: buildCountLines('target-kind', stats.targetKinds),
    diagnostics: resolveDiagnostics(dryRun).map(toDiagnosticItem),
    nextSteps: buildStepLines(dryRun.nextSteps),
    trace: toTraceView(dryRun.trace)
  }
}

type CustomerVerdict = Pick<AutomationDryRunCustomerView, 'status' | 'tagType' | 'alertType' | 'canSave'>

const resolveCustomerVerdict = (
  status: BackendDryRunStatus,
  dryRun: NormalizedDryRun,
  hasIssues: boolean
): CustomerVerdict => {
  if (status === 'available') {
    const hasRisk = dryRun.canSave === false || dryRun.valid === false || hasIssues
    const tone = hasRisk ? 'warning' : 'success'

    return { status: hasRisk ? 'risk' : 'passed', tagType: tone, alertType: tone, canSave: dryRun.canSave }
  }
  if (status === 'unavailable') {
    return { status: 'risk', tagType: 'warning', alertType: 'warning', canSave: null }
  }

  return { status: 'unchecked', tagType: 'default', alertType: 'info', canSave: null }
}

export const buildAutomationDryRunCustomerView = (
  status: BackendDryRunStatus,
  response: DryRunResponseInput,
  backendError: string
): AutomationDryRunCustomerView => {
  const dryRun = normalizeDryRunResponse(response)
  const responseAvailable = status === 'available' && dryRun.present
  const blockingErrors = responseAvailable
    ? buildIssueLines('blocking-error', dryRun.blockingErrors)
    : backendError
      ? [{ key: 'backend-error', text: backendError }]
      : []
  const warnings = responseAvailable ? buildIssueLines('warning', dryRun.warnings) : []

  return {
    ...resolveCustomerVerdict(status, dryRun, blockingErrors.length > 0 || warnings.length > 0),
    blockingErrors,
    warnings,
    referenceCounts: buildCountLines('reference', dryRun.referenceCounts),
    nextSteps: responseAvailable ? buildStepLines(dryRun.nextSteps) : [],
    responseAvailable
  }
}
