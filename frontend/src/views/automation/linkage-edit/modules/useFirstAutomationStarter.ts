/**
 * 文件用途：「首台设备第一条遥测规则」引导（从首页工作台跳入新建联动时启用）。
 * 核心逻辑：
 *   - 由路由 starter 上下文派生设备/遥测文案、默认规则名称与描述、推荐条件/动作草稿；
 *   - 一键套用推荐条件/动作后，等编辑器子组件暴露 API 再刷新本地执行解释；
 *   - 清单进度由执行预览（条件数、动作数、试运行状态）驱动。
 * 关键注意事项：编辑已有规则（带 id）时永不启用引导。
 */
import { computed, nextTick, ref, type Ref } from 'vue'
import { $t } from '@/locales'
import { buildFirstAutomationTelemetryRecommendation } from './automationStarterRecommendation'
import { buildFirstAutomationStarterChecklist } from './automationStarterChecklist'
import type { readAutomationRouteContext } from './automationEditorState'

type AutomationRouteContext = ReturnType<typeof readAutomationRouteContext>

/** 清单进度所需的执行预览快照（与 buildFirstAutomationStarterChecklist 入参同源，去掉 enabled）。 */
export type FirstAutomationStarterPreview = Omit<Parameters<typeof buildFirstAutomationStarterChecklist>[0], 'enabled'>

export interface UseFirstAutomationStarterOptions {
  routeContext: AutomationRouteContext
  propsData: Ref<{ device_id?: unknown; device_config_id?: unknown }>
  configForm: Ref<{ name: string | null; description: string | null }>
  conditionData: Ref<any[]>
  actionData: Ref<any[]>
  conditionsType: Ref<any>
  editPremise: Ref<any>
  editAction: Ref<any>
  preview: () => FirstAutomationStarterPreview
  refreshLocalExecutionExplanation: () => unknown
}

const fillTemplate = (template: string, values: Record<string, string>) =>
  Object.entries(values).reduce((text, [key, value]) => text.replace(`{${key}}`, value), template)

export function useFirstAutomationStarter(options: UseFirstAutomationStarterOptions) {
  const { routeContext, propsData } = options
  const starter = routeContext.starter

  const isFirstDeviceAutomationStarter = computed(
    () =>
      !routeContext.configId && (starter.type === 'first-telemetry-rule' || routeContext.onboarding === 'first-device')
  )
  const firstAutomationStarterDeviceLabel = computed(
    () => starter.deviceName || starter.deviceNumber || $t('custom.automation.firstDevice')
  )
  const firstAutomationStarterTelemetryLabel = computed(() =>
    starter.telemetryKey
      ? `${starter.telemetryKey}${starter.telemetryValue ? ` = ${starter.telemetryValue}` : ''}`
      : $t('custom.automation.latestTelemetryField')
  )
  const telemetryVars = () => ({ telemetry: firstAutomationStarterTelemetryLabel.value })
  const firstAutomationStarterConditionText = computed(() =>
    starter.telemetryKey
      ? fillTemplate($t('custom.automation.firstRuleConditionWithTelemetry'), telemetryVars())
      : $t('custom.automation.firstRuleConditionWithoutTelemetry')
  )
  const firstAutomationStarterDesc = computed(() =>
    fillTemplate($t('custom.automation.firstRuleStarterDesc'), {
      device: firstAutomationStarterDeviceLabel.value,
      ...telemetryVars()
    })
  )

  const firstAutomationTelemetryRecommendation = computed(() =>
    buildFirstAutomationTelemetryRecommendation(
      {
        telemetryKey: starter.telemetryKey,
        telemetryValue: starter.telemetryValue,
        telemetryAt: starter.telemetryAt,
        deviceId: propsData.value.device_id as any,
        deviceConfigId: propsData.value.device_config_id as any
      },
      {
        keyTitle: $t('custom.automation.firstRuleTelemetryKeyLabel'),
        valueTitle: $t('custom.automation.firstRuleTelemetryValueLabel'),
        timeTitle: $t('custom.automation.firstRuleTelemetryTimeLabel'),
        keyFallback: $t('custom.automation.latestTelemetryField'),
        valueFallback: $t('custom.automation.firstRuleTelemetryMissingValue'),
        timeFallback: $t('custom.automation.firstRuleTelemetryMissingTime'),
        sourceTitle: $t('custom.automation.firstRuleTelemetrySourceLabel'),
        sourceDevice: $t('custom.automation.firstRuleTelemetrySourceDevice'),
        sourceTemplate: $t('custom.automation.firstRuleTelemetrySourceTemplate'),
        sourceFallback: $t('custom.automation.firstRuleTelemetrySourceMissing'),
        conditionHint: starter.telemetryKey
          ? fillTemplate($t('custom.automation.firstRuleTelemetryConditionHintWithKey'), telemetryVars())
          : $t('custom.automation.firstRuleTelemetryConditionHintWithoutKey'),
        nextActionTitle: $t('custom.automation.firstRuleTelemetryNextActionTitle'),
        nextActionWithKey: $t('custom.automation.firstRuleTelemetryNextActionWithKey'),
        nextActionWithoutKey: $t('custom.automation.firstRuleTelemetryNextActionWithoutKey'),
        conditionDraftTitle: $t('custom.automation.firstRuleRecommendedConditionTitle'),
        conditionDraftWithValue: $t('custom.automation.firstRuleRecommendedConditionDesc'),
        conditionDraftWithoutValue: $t('custom.automation.firstRuleRecommendedConditionReady'),
        conditionDraftMissing: $t('custom.automation.firstRuleRecommendedConditionMissing'),
        actionDraftTitle: $t('custom.automation.firstRuleRecommendedActionTitle'),
        actionDraftDesc: $t('custom.automation.firstRuleRecommendedActionDesc')
      }
    )
  )

  if (isFirstDeviceAutomationStarter.value) {
    const device = { device: firstAutomationStarterDeviceLabel.value }
    options.configForm.value.name = fillTemplate($t('custom.automation.firstRuleName'), device)
    options.configForm.value.description = [
      fillTemplate($t('custom.automation.firstRuleDescriptionDevice'), device),
      starter.telemetryKey ? fillTemplate($t('custom.automation.firstRuleDescriptionTelemetry'), telemetryVars()) : '',
      $t('custom.automation.firstRuleDescriptionDryRun')
    ]
      .filter(Boolean)
      .join(' ')
  }

  const firstAutomationStarterChecklist = computed(() => {
    const preview = options.preview()
    return buildFirstAutomationStarterChecklist(
      { enabled: isFirstDeviceAutomationStarter.value, ...preview },
      {
        conditionTitle: $t('custom.automation.firstRuleChecklistConditionTitle'),
        conditionDesc: firstAutomationStarterConditionText.value,
        actionTitle: $t('custom.automation.firstRuleChecklistActionTitle'),
        actionDesc: $t('custom.automation.firstRuleChecklistActionDesc'),
        dryRunTitle: $t('custom.automation.firstRuleChecklistDryRunTitle'),
        dryRunDesc: $t('custom.automation.firstRuleChecklistDryRunDesc'),
        saveTitle: $t('custom.automation.firstRuleChecklistSaveTitle'),
        saveDesc: $t('custom.automation.firstRuleChecklistSaveDesc')
      }
    )
  })

  const firstAutomationRecommendedConditionApplied = ref(false)
  const firstAutomationRecommendedActionApplied = ref(false)
  const firstAutomationRecommendedConditionDraft = computed(
    () => firstAutomationTelemetryRecommendation.value.conditionDraft
  )
  const firstAutomationRecommendedActionDraft = computed(() => firstAutomationTelemetryRecommendation.value.actionDraft)

  // 套用草稿后编辑器需要一个 tick 才会基于新 props 重建内部状态并暴露 API。
  const refreshWhenEditorsReady = () =>
    nextTick(() => {
      if (options.editPremise.value?.ifGroupsData && options.editAction.value?.actionGroupsReturn) {
        options.refreshLocalExecutionExplanation()
      }
    })

  const applyFirstAutomationRecommendedCondition = () => {
    const draft = firstAutomationRecommendedConditionDraft.value
    if (!draft.available || !draft.condition) {
      window.$message?.warning($t('custom.automation.firstRuleRecommendedConditionMissing'))
      return
    }
    options.conditionData.value = [[{ ...draft.condition }]]
    options.conditionsType.value = draft.condition.trigger_conditions_type
    firstAutomationRecommendedConditionApplied.value = true
    window.$message?.success($t('custom.automation.firstRuleRecommendedConditionApplied'))
    void refreshWhenEditorsReady()
  }

  const applyFirstAutomationRecommendedAction = () => {
    options.actionData.value = [{ ...firstAutomationRecommendedActionDraft.value.action }]
    firstAutomationRecommendedActionApplied.value = true
    window.$message?.success($t('custom.automation.firstRuleRecommendedActionApplied'))
    void refreshWhenEditorsReady()
  }

  const openFirstAutomationAlarmCreator = () => {
    if (options.editAction.value?.openCreateAlarm) {
      options.editAction.value.openCreateAlarm()
      return
    }
    window.$message?.warning($t('custom.automation.firstRuleRecommendedActionCreateUnavailable'))
  }

  return {
    isFirstDeviceAutomationStarter,
    firstAutomationStarterDeviceLabel,
    firstAutomationStarterTelemetryLabel,
    firstAutomationStarterConditionText,
    firstAutomationStarterDesc,
    firstAutomationTelemetryRecommendation,
    firstAutomationStarterChecklist,
    firstAutomationRecommendedConditionApplied,
    firstAutomationRecommendedActionApplied,
    firstAutomationRecommendedConditionDraft,
    firstAutomationRecommendedActionDraft,
    applyFirstAutomationRecommendedCondition,
    applyFirstAutomationRecommendedAction,
    openFirstAutomationAlarmCreator
  }
}
