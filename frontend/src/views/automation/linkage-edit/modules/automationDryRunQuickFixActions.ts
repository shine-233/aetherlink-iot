import type { AutomationDryRunQuickFixAction } from './automationDryRunPreview'

export const AUTOMATION_DRY_RUN_QUICK_FIX_KEYS = {
  addConditionGroup: 'add-condition-group',
  addAlarmActionSlot: 'add-alarm-action-slot',
  createAlarmTarget: 'create-alarm-target',
  applyFirstAlarmAction: 'apply-first-alarm-action',
  createFirstAlarmTarget: 'create-first-alarm-target'
} as const

export type AutomationDryRunQuickFixKey =
  (typeof AUTOMATION_DRY_RUN_QUICK_FIX_KEYS)[keyof typeof AUTOMATION_DRY_RUN_QUICK_FIX_KEYS]

export type AutomationDryRunQuickFixTexts = {
  addConditionTitle: string
  addConditionDesc: string
  addConditionButton: string
  addAlarmActionTitle: string
  addAlarmActionDesc: string
  addAlarmActionButton: string
  createAlarmTargetTitle: string
  createAlarmTargetDesc: string
  createAlarmTargetButton: string
}

const getActionType = (action: any) => String(action?.actionType || action?.action_type || '')

export const hasAlarmActionSlot = (actions: any[] = []) => actions.some((item: any) => getActionType(item) === '30')

export const hasAlarmActionMissingTarget = (actions: any[] = []) =>
  actions.some((item: any) => getActionType(item) === '30' && !item?.action_target)

export const createAlarmActionSlot = () => ({
  actionType: '30',
  action_type: '30',
  action_target: null
})

export const buildGeneralAutomationDryRunQuickFixActions = (options: {
  conditionCount: number
  actionCount: number
  actions: any[]
  texts: AutomationDryRunQuickFixTexts
}): AutomationDryRunQuickFixAction[] => {
  const fixes: AutomationDryRunQuickFixAction[] = []

  if (options.conditionCount <= 0) {
    fixes.push({
      key: AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addConditionGroup,
      title: options.texts.addConditionTitle,
      desc: options.texts.addConditionDesc,
      buttonLabel: options.texts.addConditionButton,
      type: 'primary'
    })
  }

  if (options.actionCount <= 0) {
    fixes.push({
      key: AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addAlarmActionSlot,
      title: options.texts.addAlarmActionTitle,
      desc: options.texts.addAlarmActionDesc,
      buttonLabel: options.texts.addAlarmActionButton,
      type: 'primary'
    })
    return fixes
  }

  if (hasAlarmActionMissingTarget(options.actions)) {
    fixes.push({
      key: AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.createAlarmTarget,
      title: options.texts.createAlarmTargetTitle,
      desc: options.texts.createAlarmTargetDesc,
      buttonLabel: options.texts.createAlarmTargetButton,
      type: 'warning'
    })
  }

  return fixes
}

export type FirstAutomationDryRunQuickFixTexts = {
  applyActionTitle: string
  applyActionDesc: string
  applyActionButton: string
  createAlarmTargetTitle: string
  createAlarmTargetDesc: string
  createAlarmTargetButton: string
}

export const buildFirstAutomationDryRunQuickFixActions = (options: {
  actions: any[]
  texts: FirstAutomationDryRunQuickFixTexts
}): AutomationDryRunQuickFixAction[] => {
  if (!hasAlarmActionSlot(options.actions)) {
    return [
      {
        key: AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.applyFirstAlarmAction,
        title: options.texts.applyActionTitle,
        desc: options.texts.applyActionDesc,
        buttonLabel: options.texts.applyActionButton,
        type: 'primary'
      }
    ]
  }

  if (hasAlarmActionMissingTarget(options.actions)) {
    return [
      {
        key: AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.createFirstAlarmTarget,
        title: options.texts.createAlarmTargetTitle,
        desc: options.texts.createAlarmTargetDesc,
        buttonLabel: options.texts.createAlarmTargetButton,
        type: 'warning'
      }
    ]
  }

  return []
}