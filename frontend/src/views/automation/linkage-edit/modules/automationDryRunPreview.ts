export type BackendDryRunStatus = 'waiting' | 'ready' | 'pending' | 'available' | 'unavailable'
export type AutomationDryRunCustomerStatus = 'unchecked' | 'passed' | 'risk'

export interface AutomationDryRunLine {
  key: string
  text: string
}

export interface AutomationConditionSummaryGroup {
  key: string
  lines: AutomationDryRunLine[]
}

export interface AutomationDryRunDiagnosticItem {
  key: string
  type: 'success' | 'error' | 'warning' | 'info'
  scope: string
  message: string
}

export interface AutomationDryRunIssueLine {
  key: string
  text: string
}

export interface AutomationDryRunCustomerView {
  status: AutomationDryRunCustomerStatus
  tagType: 'default' | 'error' | 'success' | 'warning' | 'info'
  alertType: 'default' | 'error' | 'success' | 'warning' | 'info'
  blockingErrors: AutomationDryRunIssueLine[]
  warnings: AutomationDryRunIssueLine[]
  referenceCounts: AutomationDryRunLine[]
  nextSteps: AutomationDryRunLine[]
  responseAvailable: boolean
  canSave: boolean | null
}

export interface AutomationDryRunTraceStep {
  key: string
  index: number
  phase: 'trigger' | 'action' | string
  status: 'evaluated' | 'skipped' | 'blocked' | string
  statusType: 'success' | 'warning' | 'error' | 'info'
  label: string
  kind: string
  target: string
  detail: string
  notes: string[]
}

export interface AutomationDryRunTraceView {
  steps: AutomationDryRunTraceStep[]
  stepCount: number
  evaluatedAt: string
  explanation: string
  isSimulation: boolean
}

export interface AutomationDryRunBackendView {
  metrics: AutomationDryRunLine[]
  conditionTypes: AutomationDryRunLine[]
  actionTypes: AutomationDryRunLine[]
  targetKinds: AutomationDryRunLine[]
  diagnostics: AutomationDryRunDiagnosticItem[]
  nextSteps: AutomationDryRunLine[]
  trace: AutomationDryRunTraceView
}

export interface AutomationDryRunOperatorPlan {
  source: AutomationDryRunLine[]
  conditions: AutomationDryRunLine[]
  actions: AutomationDryRunLine[]
  limits: AutomationDryRunLine[]
}

export interface AutomationDryRunBeginnerGuideCard {
  key: 'save' | 'match' | 'skipped' | 'actions'
  type: 'success' | 'error' | 'warning' | 'info'
  titleKey: string
  textKey: string
  detail: string
}

export interface AutomationDryRunQuickFixAction {
  key: string
  title: string
  desc: string
  buttonLabel: string
  type?: 'primary' | 'info' | 'success' | 'warning' | 'error'
  disabled?: boolean
}

export const getAutomationDryRunStatusText = (status: BackendDryRunStatus) => {
  if (status === 'pending') return '正在请求后端预演...'
  if (status === 'available') return '后端预演已返回结果。'
  if (status === 'unavailable') return '后端预演暂不可用，仅显示本地说明。'
  if (status === 'ready') return '本地说明已生成，后端预演尚未运行。'

  return '尚未请求后端预演。'
}

export const getAutomationDryRunAlertType = (
  status: BackendDryRunStatus
): 'default' | 'error' | 'success' | 'warning' | 'info' => {
  if (status === 'available') return 'success'
  if (status === 'unavailable') return 'warning'

  return 'info'
}

export const stringifyDryRunResponse = (response: unknown) => {
  if (!response) return ''

  return JSON.stringify(response, null, 2)
}

export const getPreviewErrorText = (error: any) => {
  return error?.error?.message || error?.message || error?.response?.data?.message || '后端预演暂不可用。'
}

const formatPreviewValue = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '--'
  if (typeof value === 'object') return JSON.stringify(value)

  return String(value)
}

const conditionTypeLabel = (type: any) => {
  const labels: Record<string, string> = {
    '10': 'Single-device condition',
    '11': 'Thing model condition',
    '20': 'One-time schedule',
    '21': 'Recurring schedule',
    '22': 'Time range'
  }

  return labels[String(type)] || `Condition type ${formatPreviewValue(type)}`
}

const actionTypeLabel = (type: any) => {
  const labels: Record<string, string> = {
    '10': 'Single-device action',
    '11': 'Thing model action',
    '20': 'Activate scene',
    '30': 'Trigger alarm'
  }

  return labels[String(type)] || `Action type ${formatPreviewValue(type)}`
}

const describeCondition = (condition: any) => {
  const type = conditionTypeLabel(condition.trigger_conditions_type)
  if (condition.trigger_conditions_type === '20') {
    return `${type}: ${formatPreviewValue(condition.execution_time)}`
  }
  if (condition.trigger_conditions_type === '21') {
    return `${type}: ${formatPreviewValue(condition.task_type)} / ${formatPreviewValue(condition.params)}`
  }
  if (condition.trigger_conditions_type === '22') {
    return `${type}: ${formatPreviewValue(condition.trigger_value)}`
  }

  const source = formatPreviewValue(condition.trigger_source)
  const param = `${formatPreviewValue(condition.trigger_param_type)}:${formatPreviewValue(condition.trigger_param)}`
  const operator = formatPreviewValue(condition.trigger_operator)
  const value = formatPreviewValue(condition.trigger_value)

  return `${type}: ${source} / ${param} ${operator} ${value}`
}

const describeAction = (action: any) => {
  const type = actionTypeLabel(action.action_type || action.actionType)
  const target = formatPreviewValue(action.action_target)
  if (action.action_type === '10' || action.action_type === '11') {
    const param = `${formatPreviewValue(action.action_param_type)}:${formatPreviewValue(action.action_param)}`
    return `${type}: ${target} / ${param} = ${formatPreviewValue(action.action_value)}`
  }

  return `${type}: ${target}`
}

export const buildConditionSummaryItems = (conditionGroups: any[]): AutomationConditionSummaryGroup[] =>
  conditionGroups.map((group: any[], groupIndex: number) => ({
    key: `condition-group-${groupIndex}`,
    lines: group.map((condition, conditionIndex) => ({
      key: `condition-${groupIndex}-${conditionIndex}`,
      text: describeCondition(condition)
    }))
  }))

export const buildActionSummaryItems = (actions: any[]): AutomationDryRunLine[] =>
  actions.map((action: any, actionIndex: number) => ({
    key: `action-${actionIndex}`,
    text: describeAction(action)
  }))

const normalizeCountRecord = (record: unknown): Record<string, number> => {
  if (!record || typeof record !== 'object') return {}

  return record as Record<string, number>
}

const buildCountLines = (prefix: string, record: unknown): AutomationDryRunLine[] =>
  Object.entries(normalizeCountRecord(record)).map(([label, value]) => ({
    key: `${prefix}-${label}`,
    text: `${label}: ${value}`
  }))

const normalizeStringList = (value: unknown): string[] => {
  if (!value) return []
  if (Array.isArray(value)) {
    return value.map((item) => {
      if (typeof item === 'string') return item
      if (item?.message) return item.message

      return String(item)
    })
  }
  if (typeof value === 'string') return [value]

  return []
}

const buildIssueLines = (prefix: string, values: unknown): AutomationDryRunIssueLine[] =>
  normalizeStringList(values).map((text, index) => ({
    key: `${prefix}-${index}`,
    text
  }))

const getResponseDryRun = (response: any) => response?.dry_run || response?.dryRun || {}

const getNumericResponseValue = (response: any, keys: string[]) => {
  for (const key of keys) {
    const value = response?.[key] ?? getResponseDryRun(response)?.[key]
    if (typeof value === 'number') return value
  }

  return null
}

export const buildAutomationOperatorPlan = (
  payload: any,
  status: BackendDryRunStatus,
  response: any,
  backendError = ''
): AutomationDryRunOperatorPlan => {
  const conditionGroups = Array.isArray(payload?.trigger_condition_groups) ? payload.trigger_condition_groups : []
  const actions = Array.isArray(payload?.actions) ? payload.actions : []
  const conditionCount = conditionGroups.reduce((count: number, group: any[]) => count + group.length, 0)
  const responseDryRun = getResponseDryRun(response)
  const referenceSource =
    response?.reference_counts || response?.referenceCounts || responseDryRun.reference_counts || responseDryRun.target_kinds
  const referenceLines = buildCountLines('operator-reference', referenceSource)
  const canSave = getCanSave(response)

  return {
    source: [
      {
        key: 'rule-name',
        text: `规则：${formatPreviewValue(payload?.name)}`
      },
      {
        key: 'rule-enabled',
        text: `保存后启用：${payload?.enabled === false ? '否' : '是'}`
      },
      {
        key: 'dry-run-state',
        text:
          status === 'available'
            ? '当前载荷已有后端预演结果。'
            : status === 'unavailable'
              ? `后端预演暂不可用${backendError ? `：${backendError}` : '。'}`
              : '当前载荷尚未运行后端预演。'
      }
    ],
    conditions: [
      {
        key: 'condition-shape',
        text: `载荷包含 ${conditionGroups.length} 个条件组和 ${conditionCount} 条条件。`
      },
      ...buildCountLines('operator-condition-type', responseDryRun.condition_types)
    ],
    actions: [
      {
        key: 'action-shape',
        text: `如果规则触发，保存后的定义会包含 ${actions.length} 条已配置动作。`
      },
      ...buildCountLines('operator-action-type', responseDryRun.action_types),
      ...referenceLines
    ],
    limits: [
      {
        key: 'save-readiness',
        text:
          canSave === false
            ? '后端判断这条规则暂不适合保存。'
            : canSave === true
              ? '后端判断这条规则可以保存。'
              : '后端预演返回 can_save 前，保存状态仍未知。'
      },
      {
        key: 'no-side-effect',
        text: '预演不会保存规则、发布命令、触发报警，也不能证明实时设备遥测已经命中。'
      },
      {
        key: 'runtime-evidence',
        text: '保存后，请用自动化日志或设备/报警证据证明规则确实触发。'
      }
    ]
  }
}

const getBlockingErrorMessages = (response: any) => {
  const diagnostics = Array.isArray(response?.diagnostics) ? response.diagnostics : []
  const diagnosticErrors = diagnostics
    .filter((item: any) => item?.severity === 'error')
    .map((item: any) => item?.message || String(item))

  return [
    ...normalizeStringList(response?.blocking_errors),
    ...normalizeStringList(response?.blockers),
    ...normalizeStringList(response?.errors),
    ...diagnosticErrors
  ]
}

const getCanSave = (response: any) => {
  if (typeof response?.can_save === 'boolean') return response.can_save
  if (typeof response?.canSave === 'boolean') return response.canSave
  if (typeof response?.valid === 'boolean') return response.valid

  return null
}

const getWarningMessages = (response: any) => {
  const diagnostics = Array.isArray(response?.diagnostics) ? response.diagnostics : []
  const diagnosticWarnings = diagnostics
    .filter((item: any) => item?.severity === 'warning')
    .map((item: any) => item?.message || String(item))

  return [...normalizeStringList(response?.warnings), ...diagnosticWarnings]
}

const getSkippedConditionMessages = (response: any) => [
  ...normalizeStringList(response?.skipped_conditions),
  ...normalizeStringList(response?.skippedConditions)
]

const getUnavailableActionMessages = (response: any) => [
  ...normalizeStringList(response?.unavailable_actions),
  ...normalizeStringList(response?.unavailableActions)
]

const firstLine = (...lineGroups: AutomationDryRunLine[][]) => {
  for (const lines of lineGroups) {
    if (lines.length > 0) return lines[0].text
  }

  return ''
}

export const buildAutomationDryRunBeginnerGuide = (options: {
  status: BackendDryRunStatus
  response: any
  backendError: string
  customerView: AutomationDryRunCustomerView
  localBlockingErrors: AutomationDryRunLine[]
  conditionGroupCount: number
  conditionCount: number
  actionCount: number
}): AutomationDryRunBeginnerGuideCard[] => {
  const matchedDeviceCount = getNumericResponseValue(options.response, ['matched_devices', 'matchedDevices'])
  const skippedConditions = getSkippedConditionMessages(options.response)
  const unavailableActions = getUnavailableActionMessages(options.response)
  const firstBlocker = firstLine(options.localBlockingErrors, options.customerView.blockingErrors)
  const firstWarning = firstLine(options.customerView.warnings)
  const shapeDetail = `${options.conditionGroupCount} group(s) / ${options.conditionCount} condition row(s) / ${options.actionCount} action row(s)`

  let saveCard: AutomationDryRunBeginnerGuideCard
  if (options.status === 'pending') {
    saveCard = {
      key: 'save',
      type: 'info',
      titleKey: 'generate.automationDryRunBeginnerSaveTitle',
      textKey: 'generate.automationDryRunBeginnerSaveRunning',
      detail: shapeDetail
    }
  } else if (firstBlocker || options.customerView.canSave === false) {
    saveCard = {
      key: 'save',
      type: 'error',
      titleKey: 'generate.automationDryRunBeginnerSaveTitle',
      textKey: 'generate.automationDryRunBeginnerSaveBlocked',
      detail: firstBlocker || shapeDetail
    }
  } else if (options.customerView.canSave === true) {
    saveCard = {
      key: 'save',
      type: options.customerView.status === 'passed' ? 'success' : 'warning',
      titleKey: 'generate.automationDryRunBeginnerSaveTitle',
      textKey: 'generate.automationDryRunBeginnerSaveReady',
      detail: firstWarning || shapeDetail
    }
  } else if (options.status === 'unavailable') {
    saveCard = {
      key: 'save',
      type: 'warning',
      titleKey: 'generate.automationDryRunBeginnerSaveTitle',
      textKey: 'generate.automationDryRunBeginnerSaveBackendUnavailable',
      detail: options.backendError || shapeDetail
    }
  } else {
    saveCard = {
      key: 'save',
      type: 'info',
      titleKey: 'generate.automationDryRunBeginnerSaveTitle',
      textKey: 'generate.automationDryRunBeginnerSaveRunFirst',
      detail: shapeDetail
    }
  }

  const matchCard: AutomationDryRunBeginnerGuideCard =
    matchedDeviceCount !== null
      ? {
          key: 'match',
          type: matchedDeviceCount > 0 ? 'success' : 'warning',
          titleKey: 'generate.automationDryRunBeginnerMatchTitle',
          textKey: 'generate.automationDryRunBeginnerMatchKnown',
          detail: String(matchedDeviceCount)
        }
      : {
          key: 'match',
          type: options.status === 'available' ? 'warning' : 'info',
          titleKey: 'generate.automationDryRunBeginnerMatchTitle',
          textKey:
            options.status === 'available'
              ? 'generate.automationDryRunBeginnerMatchNotEvaluated'
              : 'generate.automationDryRunBeginnerMatchRunFirst',
          detail: shapeDetail
        }

  const skippedCard: AutomationDryRunBeginnerGuideCard =
    skippedConditions.length > 0 || firstWarning
      ? {
          key: 'skipped',
          type: 'warning',
          titleKey: 'generate.automationDryRunBeginnerSkippedTitle',
          textKey: 'generate.automationDryRunBeginnerSkippedKnown',
          detail: skippedConditions[0] || firstWarning
        }
      : {
          key: 'skipped',
          type: options.status === 'available' ? 'success' : 'info',
          titleKey: 'generate.automationDryRunBeginnerSkippedTitle',
          textKey:
            options.status === 'available'
              ? 'generate.automationDryRunBeginnerSkippedNone'
              : 'generate.automationDryRunBeginnerSkippedRunFirst',
          detail: shapeDetail
        }

  let actionCard: AutomationDryRunBeginnerGuideCard
  if (options.actionCount === 0) {
    actionCard = {
      key: 'actions',
      type: 'error',
      titleKey: 'generate.automationDryRunBeginnerActionTitle',
      textKey: 'generate.automationDryRunBeginnerActionMissing',
      detail: shapeDetail
    }
  } else if (unavailableActions.length > 0) {
    actionCard = {
      key: 'actions',
      type: 'warning',
      titleKey: 'generate.automationDryRunBeginnerActionTitle',
      textKey: 'generate.automationDryRunBeginnerActionUnavailable',
      detail: unavailableActions[0]
    }
  } else if (options.status === 'available') {
    actionCard = {
      key: 'actions',
      type: 'success',
      titleKey: 'generate.automationDryRunBeginnerActionTitle',
      textKey: 'generate.automationDryRunBeginnerActionReady',
      detail: shapeDetail
    }
  } else {
    actionCard = {
      key: 'actions',
      type: 'info',
      titleKey: 'generate.automationDryRunBeginnerActionTitle',
      textKey: 'generate.automationDryRunBeginnerActionRunFirst',
      detail: shapeDetail
    }
  }

  return [saveCard, matchCard, skippedCard, actionCard]
}

const toDiagnosticType = (severity: unknown): AutomationDryRunDiagnosticItem['type'] => {
  if (severity === 'success' || severity === 'error' || severity === 'warning') return severity

  return 'info'
}

const emptyTraceView = (): AutomationDryRunTraceView => ({
  steps: [],
  stepCount: 0,
  evaluatedAt: '',
  explanation: '',
  isSimulation: true
})

const toTraceStatusType = (status: unknown): AutomationDryRunTraceStep['statusType'] => {
  if (status === 'evaluated') return 'success'
  if (status === 'skipped') return 'warning'
  if (status === 'blocked') return 'error'

  return 'info'
}

export const buildTraceView = (response: any): AutomationDryRunTraceView => {
  const rawTrace = response?.execution_trace || response?.executionTrace
  if (!rawTrace) return emptyTraceView()

  const rawSteps = Array.isArray(rawTrace.steps) ? rawTrace.steps : []

  return {
    steps: rawSteps.map((step: any, index: number) => ({
      key: `trace-step-${step?.index ?? index}`,
      index: typeof step?.index === 'number' ? step.index : index + 1,
      phase: step?.phase || 'trigger',
      status: step?.status || 'evaluated',
      statusType: toTraceStatusType(step?.status),
      label: step?.label || `step ${index + 1}`,
      kind: step?.kind || '',
      target: step?.target || '',
      detail: step?.detail || '',
      notes: Array.isArray(step?.notes) ? step.notes.filter((note: unknown) => typeof note === 'string') : []
    })),
    stepCount: typeof rawTrace.step_count === 'number' ? rawTrace.step_count : rawSteps.length,
    evaluatedAt: rawTrace.evaluated_at || rawTrace.evaluatedAt || '',
    explanation: rawTrace.explanation || '',
    isSimulation: rawTrace.is_simulation !== false
  }
}

export const buildBackendDryRunView = (response: any): AutomationDryRunBackendView => {
  if (!response) {
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

  const dryRun = getResponseDryRun(response)
  const diagnostics = Array.isArray(response.diagnostics) ? response.diagnostics : []
  const fallbackDiagnostics = [
    ...(response.errors || []).map((message: string) => ({ severity: 'error', scope: 'validation', message })),
    ...(response.warnings || []).map((message: string) => ({ severity: 'warning', scope: 'warning', message }))
  ]
  const diagnosticSource = diagnostics.length > 0 ? diagnostics : fallbackDiagnostics

  return {
    metrics: [
      {
        key: 'valid',
        text: response.valid === false ? 'Validation: failed' : 'Validation: passed'
      },
      {
        key: 'can-save',
        text: getCanSave(response) === false ? 'Save readiness: blocked' : 'Save readiness: allowed'
      },
      {
        key: 'conditions',
        text: `Conditions: ${dryRun.condition_group_count || 0} groups / ${dryRun.condition_count || 0} rows`
      },
      {
        key: 'actions',
        text: `Actions: ${dryRun.action_count || 0} rows`
      }
    ],
    conditionTypes: buildCountLines('condition-type', dryRun.condition_types),
    actionTypes: buildCountLines('action-type', dryRun.action_types),
    targetKinds: buildCountLines('target-kind', dryRun.target_kinds),
    diagnostics: diagnosticSource.map((item: any, index: number) => ({
      key: `diagnostic-${index}`,
      type: toDiagnosticType(item.severity),
      scope: item.scope || 'dry-run',
      message: item.message || String(item)
    })),
    nextSteps: (response.next_steps || []).map((step: string, index: number) => ({
      key: `next-step-${index}`,
      text: step
    })),
    trace: buildTraceView(response)
  }
}

export const buildAutomationDryRunCustomerView = (
  status: BackendDryRunStatus,
  response: any,
  backendError: string
): AutomationDryRunCustomerView => {
  const responseAvailable = status === 'available' && !!response
  const dryRun = getResponseDryRun(response)
  const blockingErrors = responseAvailable
    ? buildIssueLines('blocking-error', getBlockingErrorMessages(response))
    : backendError
      ? [{ key: 'backend-error', text: backendError }]
      : []
  const warnings = responseAvailable ? buildIssueLines('warning', getWarningMessages(response)) : []
  const referenceSource =
    response?.reference_counts || response?.referenceCounts || dryRun.reference_counts || dryRun.target_kinds
  const referenceCounts = buildCountLines('reference', referenceSource)
  const nextSteps = responseAvailable
    ? normalizeStringList(response?.next_steps || response?.nextSteps).map((step, index) => ({
        key: `next-step-${index}`,
        text: step
      }))
    : []

  if (status === 'available') {
    const canSave = getCanSave(response)
    const hasRisk = canSave === false || response?.valid === false || blockingErrors.length > 0 || warnings.length > 0

    return {
      status: hasRisk ? 'risk' : 'passed',
      tagType: hasRisk ? 'warning' : 'success',
      alertType: hasRisk ? 'warning' : 'success',
      blockingErrors,
      warnings,
      referenceCounts,
      nextSteps,
      responseAvailable,
      canSave
    }
  }

  if (status === 'unavailable') {
    return {
      status: 'risk',
      tagType: 'warning',
      alertType: 'warning',
      blockingErrors,
      warnings,
      referenceCounts,
      nextSteps,
      responseAvailable,
      canSave: null
    }
  }

  return {
    status: 'unchecked',
    tagType: 'default',
    alertType: 'info',
    blockingErrors,
    warnings,
    referenceCounts,
    nextSteps,
    responseAvailable,
    canSave: null
  }
}
