import type { AutomationDryRunCustomerStatus } from './automationDryRunPreview'
import type { BackendDryRunStatus } from './automationDryRunPreview'

export type FirstAutomationChecklistStatus = 'done' | 'active' | 'todo'

export type FirstAutomationChecklistItem = {
  key: 'condition' | 'action' | 'dry-run' | 'save'
  status: FirstAutomationChecklistStatus
  title: string
  desc: string
}

export type FirstAutomationChecklistTexts = {
  conditionTitle: string
  conditionDesc: string
  actionTitle: string
  actionDesc: string
  dryRunTitle: string
  dryRunDesc: string
  saveTitle: string
  saveDesc: string
}

export type FirstAutomationChecklistState = {
  enabled: boolean
  conditionCount: number
  actionCount: number
  backendDryRunStatus: BackendDryRunStatus
  customerDryRunStatus: AutomationDryRunCustomerStatus
  canSave: boolean | null
}

export const buildFirstAutomationStarterChecklist = (
  state: FirstAutomationChecklistState,
  texts: FirstAutomationChecklistTexts
): FirstAutomationChecklistItem[] => {
  if (!state.enabled) return []

  const hasCondition = state.conditionCount > 0
  const hasAction = state.actionCount > 0
  const hasDryRunFeedback = state.backendDryRunStatus === 'available' || state.backendDryRunStatus === 'unavailable'
  const dryRunPassed = state.customerDryRunStatus === 'passed' || state.canSave === true

  const getStatus = (step: FirstAutomationChecklistItem['key']): FirstAutomationChecklistStatus => {
    if (step === 'condition') return hasCondition ? 'done' : 'active'
    if (step === 'action') return !hasCondition ? 'todo' : hasAction ? 'done' : 'active'
    if (step === 'dry-run') return !hasCondition || !hasAction ? 'todo' : dryRunPassed ? 'done' : 'active'

    return dryRunPassed ? 'active' : 'todo'
  }

  return [
    {
      key: 'condition',
      status: getStatus('condition'),
      title: texts.conditionTitle,
      desc: texts.conditionDesc
    },
    {
      key: 'action',
      status: getStatus('action'),
      title: texts.actionTitle,
      desc: texts.actionDesc
    },
    {
      key: 'dry-run',
      status: getStatus('dry-run'),
      title: texts.dryRunTitle,
      desc: texts.dryRunDesc
    },
    {
      key: 'save',
      status: getStatus('save'),
      title: texts.saveTitle,
      desc: texts.saveDesc
    }
  ]
}
