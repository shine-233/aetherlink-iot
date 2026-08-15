import { echoActionGroups, echoConditionGroups } from '@/views/automation/linkage-edit/modules/automationEchoPayload'

export type AutomationEditEchoState = {
  automationsInfo: any
  configForm: any
  conditionData: any[]
  actionData: any[]
}

export function buildAutomationEditEchoState(detail: any): AutomationEditEchoState | null {
  if (!detail) return null

  const triggerConditionGroups = Array.isArray(detail.trigger_condition_groups) ? detail.trigger_condition_groups : []
  const actions = Array.isArray(detail.actions) ? detail.actions : []

  return {
    automationsInfo: detail,
    configForm: detail,
    conditionData: echoConditionGroups(triggerConditionGroups),
    actionData: echoActionGroups(actions)
  }
}
