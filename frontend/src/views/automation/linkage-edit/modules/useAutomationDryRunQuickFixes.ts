import { computed, nextTick, type ComputedRef, type Ref } from 'vue'
import {
  AUTOMATION_DRY_RUN_QUICK_FIX_KEYS,
  buildFirstAutomationDryRunQuickFixActions,
  buildGeneralAutomationDryRunQuickFixActions,
  createAlarmActionSlot
} from './automationDryRunQuickFixActions'
import type { AutomationDryRunQuickFixAction } from './automationDryRunPreview'
import { buildSubmitActions, buildSubmitConditionGroups } from './automationSubmitPayload'

type Translate = (key: string) => string

type AutomationDraftComponentRef = Ref<any>

export function useAutomationDryRunQuickFixes(options: {
  isFirstDeviceAutomationStarter: ComputedRef<boolean>
  previewConditionGroups: ComputedRef<any[]>
  previewActions: ComputedRef<any[]>
  conditionData: Ref<any[]>
  actionData: Ref<any[]>
  editPremise: AutomationDraftComponentRef
  editAction: AutomationDraftComponentRef
  applyFirstAutomationRecommendedAction: () => void
  openFirstAutomationAlarmCreator: () => void
  refreshLocalExecutionExplanation: () => unknown
  t: Translate
}) {
  const currentDraftConditionGroups = () => {
    if (options.editPremise.value?.ifGroupsData) {
      return buildSubmitConditionGroups(options.editPremise.value.ifGroupsData())
    }

    return buildSubmitConditionGroups(options.conditionData.value || [])
  }

  const currentDraftActions = () => {
    if (options.editAction.value?.actionGroupsReturn) {
      return buildSubmitActions(options.editAction.value.actionGroupsReturn())
    }

    return buildSubmitActions(options.actionData.value || [])
  }

  const firstAutomationDryRunQuickFixActions = computed<AutomationDryRunQuickFixAction[]>(() => {
    if (!options.isFirstDeviceAutomationStarter.value) return []

    const knownActions = options.previewActions.value.length > 0 ? options.previewActions.value : options.actionData.value
    return buildFirstAutomationDryRunQuickFixActions({
      actions: knownActions,
      texts: {
        applyActionTitle: options.t('custom.automation.firstRuleRecommendedActionTitle'),
        applyActionDesc: options.t('custom.automation.firstRuleDryRunQuickFixApplyActionDesc'),
        applyActionButton: options.t('custom.automation.firstRuleApplyRecommendedAction'),
        createAlarmTargetTitle: options.t('custom.automation.firstRuleCreateAlarmTarget'),
        createAlarmTargetDesc: options.t('custom.automation.firstRuleDryRunQuickFixCreateAlarmDesc'),
        createAlarmTargetButton: options.t('custom.automation.firstRuleCreateAlarmTarget')
      }
    })
  })

  const generalAutomationDryRunQuickFixActions = computed<AutomationDryRunQuickFixAction[]>(() => {
    if (options.isFirstDeviceAutomationStarter.value) return []

    const conditionGroups =
      options.previewConditionGroups.value.length > 0
        ? options.previewConditionGroups.value
        : currentDraftConditionGroups()
    const actions = options.previewActions.value.length > 0 ? options.previewActions.value : currentDraftActions()
    const conditionCount = conditionGroups.reduce((count: number, group: any[]) => count + group.length, 0)

    return buildGeneralAutomationDryRunQuickFixActions({
      conditionCount,
      actionCount: actions.length,
      actions,
      texts: {
        addConditionTitle: options.t('generate.automationDryRunQuickFixAddConditionTitle'),
        addConditionDesc: options.t('generate.automationDryRunQuickFixAddConditionDesc'),
        addConditionButton: options.t('generate.automationDryRunQuickFixAddConditionButton'),
        addAlarmActionTitle: options.t('generate.automationDryRunQuickFixAddAlarmActionTitle'),
        addAlarmActionDesc: options.t('generate.automationDryRunQuickFixAddAlarmActionDesc'),
        addAlarmActionButton: options.t('generate.automationDryRunQuickFixAddAlarmActionButton'),
        createAlarmTargetTitle: options.t('generate.automationDryRunQuickFixCreateAlarmTargetTitle'),
        createAlarmTargetDesc: options.t('generate.automationDryRunQuickFixCreateAlarmTargetDesc'),
        createAlarmTargetButton: options.t('generate.automationDryRunQuickFixCreateAlarmTargetButton')
      }
    })
  })

  const automationDryRunQuickFixActions = computed<AutomationDryRunQuickFixAction[]>(() =>
    options.isFirstDeviceAutomationStarter.value
      ? firstAutomationDryRunQuickFixActions.value
      : generalAutomationDryRunQuickFixActions.value
  )

  const refreshDryRunExplanationAfterQuickFix = () => {
    void nextTick(() => {
      options.refreshLocalExecutionExplanation()
    })
  }

  const addConditionGroupFromQuickFix = () => {
    if (options.editPremise.value?.addConditionGroup) {
      options.editPremise.value.addConditionGroup()
      window.$message?.success(options.t('generate.automationDryRunQuickFixConditionAdded'))
      refreshDryRunExplanationAfterQuickFix()
      return
    }

    options.conditionData.value = [...options.conditionData.value, [{ ifType: null }]]
    window.$message?.success(options.t('generate.automationDryRunQuickFixConditionAdded'))
    refreshDryRunExplanationAfterQuickFix()
  }

  const addAlarmActionSlotFromQuickFix = () => {
    if (options.editAction.value?.addAlarmActionSlot) {
      options.editAction.value.addAlarmActionSlot()
      window.$message?.success(options.t('generate.automationDryRunQuickFixAlarmActionAdded'))
      refreshDryRunExplanationAfterQuickFix()
      return
    }

    options.actionData.value = [...options.actionData.value, createAlarmActionSlot()]
    window.$message?.success(options.t('generate.automationDryRunQuickFixAlarmActionAdded'))
    refreshDryRunExplanationAfterQuickFix()
  }

  const handleAutomationDryRunQuickFix = (key: string) => {
    if (key === AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.applyFirstAlarmAction) {
      options.applyFirstAutomationRecommendedAction()
      return
    }

    if (key === AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.createFirstAlarmTarget) {
      options.openFirstAutomationAlarmCreator()
      return
    }

    if (key === AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addConditionGroup) {
      addConditionGroupFromQuickFix()
      return
    }

    if (key === AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addAlarmActionSlot) {
      addAlarmActionSlotFromQuickFix()
      return
    }

    if (key === AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.createAlarmTarget) {
      options.openFirstAutomationAlarmCreator()
    }
  }

  return {
    automationDryRunQuickFixActions,
    handleAutomationDryRunQuickFix
  }
}