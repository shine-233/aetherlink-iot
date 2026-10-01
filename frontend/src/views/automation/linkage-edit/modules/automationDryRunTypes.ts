/**
 * 联动预演视图层的公共类型。
 * 这些类型构成 automationDryRunPreview 桶文件的对外契约，被 Vue 组件与 composable 直接引用。
 */

export type BackendDryRunStatus = 'waiting' | 'ready' | 'pending' | 'available' | 'unavailable'
export type AutomationDryRunCustomerStatus = 'unchecked' | 'passed' | 'risk'
export type AutomationDryRunTone = 'default' | 'error' | 'success' | 'warning' | 'info'

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
  tagType: AutomationDryRunTone
  alertType: AutomationDryRunTone
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
