<!--
  Purpose: scene automation linkage editor.
  Core flow: edit grouped trigger conditions and actions, support create/edit echo, and submit a scene_automations payload.
  Important: trigger_condition_groups, actions, backType, and route query ids affect save behavior and post-save navigation.
-->
<!--
  文件说明：
  - 联动规则编辑页入口，负责整合触发条件、执行动作与基础表单信息。
  - 同时处理新建/编辑回显、提交前格式转换，以及保存后的路由返回。
  维护提示：
  - trigger_condition_groups 与 actions 是提交给 scene_automations 的核心字段。
  - route query 中的 id、backType、device_id、device_config_id 会影响回显与保存后跳转。
-->
<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { FormInst } from 'naive-ui'
import { NButton, NCard, useDialog } from 'naive-ui'
import AutomationDryRunPreview from '@/views/automation/linkage-edit/modules/AutomationDryRunPreview.vue'
import EditAction from '@/views/automation/linkage-edit/modules/edit-action.vue'
import EditPremise from '@/views/automation/linkage-edit/modules/edit-premise.vue'
import { buildAutomationEditEchoState } from '@/views/automation/linkage-edit/modules/automationEditEchoState'
import {
  createAutomationFormRules,
  createDefaultAutomationForm,
  readAutomationRouteContext
} from '@/views/automation/linkage-edit/modules/automationEditorState'
import {
  buildSubmitActions,
  buildSubmitConditionGroups
} from '@/views/automation/linkage-edit/modules/automationSubmitPayload'
import {
  getAutomationSubmitBlocker,
  resolveAutomationPostSaveRoute,
  saveAutomationDefinition
} from '@/views/automation/linkage-edit/modules/automationSaveFlow'
import { buildFirstAutomationTelemetryRecommendation } from '@/views/automation/linkage-edit/modules/automationStarterRecommendation'
import { buildFirstAutomationStarterChecklist } from '@/views/automation/linkage-edit/modules/automationStarterChecklist'
import { useAutomationExecutionPreview } from '@/views/automation/linkage-edit/modules/useAutomationExecutionPreview'
import { useAutomationSaveGate } from '@/views/automation/linkage-edit/modules/useAutomationSaveGate'
import { useAutomationDryRunQuickFixes } from '@/views/automation/linkage-edit/modules/useAutomationDryRunQuickFixes'

import {
  sceneAutomationsAdd,
  sceneAutomationsDryRun,
  sceneAutomationsEdit,
  sceneAutomationsInfo
} from '@/service/api/automation'
import type { SceneAutomationDryRunPayload } from '@/service/api/automation'
import { $t } from '@/locales'
import { useTabStore } from '@/store/modules/tab'

const dialog = useDialog()
const route = useRoute()
const router = useRouter()
const routeContext = readAutomationRouteContext(route.query)
const backType = ref(routeContext.backType)
const configFormRules = ref(createAutomationFormRules($t))
const configFormRef = ref<HTMLElement & FormInst>()
const configForm = ref(createDefaultAutomationForm())
const configId = ref(routeContext.configId)
const propsData = ref(routeContext.propsData)
const firstAutomationStarter = routeContext.starter
const isFirstDeviceAutomationStarter = computed(
  () =>
    !routeContext.configId &&
    (firstAutomationStarter.type === 'first-telemetry-rule' || routeContext.onboarding === 'first-device')
)
const firstAutomationStarterDeviceLabel = computed(
  () => firstAutomationStarter.deviceName || firstAutomationStarter.deviceNumber || $t('custom.automation.firstDevice')
)
const firstAutomationStarterTelemetryLabel = computed(() =>
  firstAutomationStarter.telemetryKey
    ? `${firstAutomationStarter.telemetryKey}${
        firstAutomationStarter.telemetryValue ? ` = ${firstAutomationStarter.telemetryValue}` : ''
      }`
    : $t('custom.automation.latestTelemetryField')
)
const firstAutomationStarterConditionText = computed(() =>
  firstAutomationStarter.telemetryKey
    ? $t('custom.automation.firstRuleConditionWithTelemetry').replace(
        '{telemetry}',
        firstAutomationStarterTelemetryLabel.value
      )
    : $t('custom.automation.firstRuleConditionWithoutTelemetry')
)
const firstAutomationStarterName = computed(() =>
  $t('custom.automation.firstRuleName').replace('{device}', firstAutomationStarterDeviceLabel.value)
)
const firstAutomationStarterDescription = computed(() =>
  [
    $t('custom.automation.firstRuleDescriptionDevice').replace('{device}', firstAutomationStarterDeviceLabel.value),
    firstAutomationStarter.telemetryKey
      ? $t('custom.automation.firstRuleDescriptionTelemetry').replace(
          '{telemetry}',
          firstAutomationStarterTelemetryLabel.value
        )
      : '',
    $t('custom.automation.firstRuleDescriptionDryRun')
  ]
    .filter(Boolean)
    .join(' ')
)
const firstAutomationTelemetryRecommendation = computed(() =>
  buildFirstAutomationTelemetryRecommendation(
    {
      telemetryKey: firstAutomationStarter.telemetryKey,
      telemetryValue: firstAutomationStarter.telemetryValue,
      telemetryAt: firstAutomationStarter.telemetryAt,
      deviceId: propsData.value.device_id,
      deviceConfigId: propsData.value.device_config_id
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
      conditionHint: firstAutomationStarter.telemetryKey
        ? $t('custom.automation.firstRuleTelemetryConditionHintWithKey').replace(
            '{telemetry}',
            firstAutomationStarterTelemetryLabel.value
          )
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
  configForm.value.name = firstAutomationStarterName.value
  configForm.value.description = firstAutomationStarterDescription.value
}

const tabStore = useTabStore()
const editPremise = ref()
const editAction = ref()

const buildAutomationExecutionPayload = (): SceneAutomationDryRunPayload => ({
  id: configForm.value.id || undefined,
  name: configForm.value.name,
  description: configForm.value.description,
  enabled: configForm.value.enabled,
  trigger_condition_groups: handleIfData(),
  actions: handleActionData()
})

const {
  backendDryRunStatus,
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
} = useAutomationExecutionPreview({
  buildPayload: buildAutomationExecutionPayload,
  dryRun: sceneAutomationsDryRun as any,
  getLocalBlocker: (payload) => getAutomationSubmitBlocker(payload, $t) || ''
})

const { ensureBackendDryRunCanSave, isSaveDryRunLoading } = useAutomationSaveGate({
  runBackendDryRunForPayload,
  t: $t
})

const firstAutomationStarterChecklist = computed(() => {
  return buildFirstAutomationStarterChecklist(
    {
      enabled: isFirstDeviceAutomationStarter.value,
      conditionCount: previewConditionCount.value,
      actionCount: previewActionCount.value,
      backendDryRunStatus: backendDryRunStatus.value,
      customerDryRunStatus: customerDryRunView.value.status,
      canSave: customerDryRunView.value.canSave
    },
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

const syncSubmitPayload = () => {
  const payload = refreshLocalExecutionExplanation()
  configForm.value.trigger_condition_groups = (payload.trigger_condition_groups ?? []) as any[]
  configForm.value.actions = payload.actions

  return payload
}


const submitData = async () => {
  const submitPayload = syncSubmitPayload()
  const blocker = getAutomationSubmitBlocker(submitPayload, $t)

  if (blocker) {
    window.$message?.error(blocker)
    return
  }

  await configFormRef?.value?.validate()
  await editPremise.value.premiseFormRefReturn()?.validate()
  await editAction.value.actionFormRefReturn()?.validate()
  if (!(await ensureBackendDryRunCanSave(submitPayload))) {
    return
  }

  dialog.warning({
    title: $t('common.tip'),
    content: $t('common.saveSceneInfo'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const saved = await saveAutomationDefinition({
        isEdit: Boolean(configId.value),
        payload: submitPayload,
        addAutomation: sceneAutomationsAdd,
        editAutomation: sceneAutomationsEdit
      })

      if (!saved) return

      await tabStore.removeTab(route.path)
      router.replace(resolveAutomationPostSaveRoute(backType.value, propsData.value) as any)
    }
  })
}

const conditionsType = ref(null as any)
const conditionChose = (data: any) => {
  if (data) {
    conditionsType.value = data
  }
}
const automationsInfo = ref(null as any)
const conditionData = ref([] as any)
const actionData = ref([] as any)
const firstAutomationRecommendedConditionApplied = ref(false)
const firstAutomationRecommendedActionApplied = ref(false)
const firstAutomationRecommendedConditionDraft = computed(
  () => firstAutomationTelemetryRecommendation.value.conditionDraft
)
const firstAutomationRecommendedActionDraft = computed(() => firstAutomationTelemetryRecommendation.value.actionDraft)

const applyFirstAutomationRecommendedCondition = () => {
  const draft = firstAutomationRecommendedConditionDraft.value
  if (!draft.available || !draft.condition) {
    window.$message?.warning($t('custom.automation.firstRuleRecommendedConditionMissing'))
    return
  }

  conditionData.value = [[{ ...draft.condition }]]
  conditionsType.value = draft.condition.trigger_conditions_type
  firstAutomationRecommendedConditionApplied.value = true
  window.$message?.success($t('custom.automation.firstRuleRecommendedConditionApplied'))

  void nextTick(() => {
    if (editPremise.value?.ifGroupsData && editAction.value?.actionGroupsReturn) {
      refreshLocalExecutionExplanation()
    }
  })
}

const applyFirstAutomationRecommendedAction = () => {
  const draft = firstAutomationRecommendedActionDraft.value
  actionData.value = [{ ...draft.action }]
  firstAutomationRecommendedActionApplied.value = true
  window.$message?.success($t('custom.automation.firstRuleRecommendedActionApplied'))

  void nextTick(() => {
    if (editPremise.value?.ifGroupsData && editAction.value?.actionGroupsReturn) {
      refreshLocalExecutionExplanation()
    }
  })
}

const openFirstAutomationAlarmCreator = () => {
  if (editAction.value?.openCreateAlarm) {
    editAction.value.openCreateAlarm()
    return
  }

  window.$message?.warning($t('custom.automation.firstRuleRecommendedActionCreateUnavailable'))
}

const { automationDryRunQuickFixActions, handleAutomationDryRunQuickFix } = useAutomationDryRunQuickFixes({
  isFirstDeviceAutomationStarter,
  previewConditionGroups,
  previewActions,
  conditionData,
  actionData,
  editPremise,
  editAction,
  applyFirstAutomationRecommendedAction,
  openFirstAutomationAlarmCreator,
  refreshLocalExecutionExplanation,
  t: $t
})
const getSceneAutomationsInfo = async () => {
  const res = await sceneAutomationsInfo(configId.value)
  const echoState = buildAutomationEditEchoState(res?.data)
  if (!echoState) return

  automationsInfo.value = echoState.automationsInfo
  configForm.value = echoState.configForm
  conditionData.value = echoState.conditionData
  actionData.value = echoState.actionData
}

const handleIfData = () => {
  if (!editPremise.value?.ifGroupsData) {
    console.error('EditPremise component ref is not available yet.')
    return [] // Return empty array if the ref has not exposed its API yet
  }
  return buildSubmitConditionGroups(editPremise.value.ifGroupsData())
}

const handleActionData = () => {
  if (!editAction.value?.actionGroupsReturn) {
    console.error('EditAction component ref is not available yet.')
    return [] // Return empty array if the ref has not exposed its API yet
  }
  return buildSubmitActions(editAction.value.actionGroupsReturn())
}

if (configId.value) {
  if (typeof configId.value === 'string') {
    configForm.value.id = configId.value
  }
  getSceneAutomationsInfo()
}
</script>

<template>
  <div class="linkage-edit">
    <NCard
      :bordered="false"
      :title="(configId ? $t('common.edit') : $t('common.add')) + $t('route.automation_scene-linkage')"
    >
      <n-alert v-if="isFirstDeviceAutomationStarter" type="info" :show-icon="false" class="mb-12px">
        <div class="first-automation-starter">
          <div>
            <div class="first-automation-starter__title">
              {{ $t('custom.automation.firstRuleStarterTitle') }}
            </div>
            <div class="first-automation-starter__desc">
              {{
                $t('custom.automation.firstRuleStarterDesc')
                  .replace('{device}', firstAutomationStarterDeviceLabel)
                  .replace('{telemetry}', firstAutomationStarterTelemetryLabel)
              }}
            </div>
          </div>
          <div class="first-automation-starter__steps">
            <span>{{ firstAutomationStarterConditionText }}</span>
            <span>{{ $t('custom.automation.firstRuleActionStep') }}</span>
          </div>
          <div class="first-automation-telemetry-guide">
            <div>
              <div class="first-automation-telemetry-guide__title">
                {{ $t('custom.automation.firstRuleTelemetryGuideTitle') }}
              </div>
              <div class="first-automation-telemetry-guide__desc">
                {{ $t('custom.automation.firstRuleTelemetryGuideDesc') }}
              </div>
            </div>
            <div class="first-automation-telemetry-guide__cards">
              <div
                v-for="item in firstAutomationTelemetryRecommendation.cards"
                :key="item.key"
                class="first-automation-telemetry-guide__card"
                :class="`first-automation-telemetry-guide__card--${item.status}`"
              >
                <span>{{ item.title }}</span>
                <strong>{{ item.value }}</strong>
              </div>
            </div>
            <div class="first-automation-telemetry-guide__hint">
              {{ firstAutomationTelemetryRecommendation.conditionHint }}
            </div>
            <div
              class="first-automation-telemetry-guide__next"
              :class="`first-automation-telemetry-guide__next--${firstAutomationTelemetryRecommendation.nextAction.status}`"
            >
              <strong>{{ firstAutomationTelemetryRecommendation.nextAction.title }}</strong>
              <span>{{ firstAutomationTelemetryRecommendation.nextAction.desc }}</span>
            </div>
            <div
              class="first-automation-telemetry-guide__draft"
              :class="`first-automation-telemetry-guide__draft--${firstAutomationRecommendedConditionDraft.status}`"
            >
              <div>
                <strong>{{ firstAutomationRecommendedConditionDraft.title }}</strong>
                <span>{{ firstAutomationRecommendedConditionDraft.desc }}</span>
                <small v-if="firstAutomationRecommendedConditionApplied">
                  {{ $t('custom.automation.firstRuleRecommendedConditionApplied') }}
                </small>
              </div>
              <NButton
                size="small"
                type="primary"
                secondary
                :disabled="!firstAutomationRecommendedConditionDraft.available"
                @click="applyFirstAutomationRecommendedCondition"
              >
                {{ $t('custom.automation.firstRuleApplyRecommendedCondition') }}
              </NButton>
            </div>
            <div class="first-automation-telemetry-guide__draft first-automation-telemetry-guide__draft--action">
              <div>
                <strong>{{ firstAutomationRecommendedActionDraft.title }}</strong>
                <span>{{ firstAutomationRecommendedActionDraft.desc }}</span>
                <small v-if="firstAutomationRecommendedActionApplied">
                  {{ $t('custom.automation.firstRuleRecommendedActionApplied') }}
                </small>
              </div>
              <div class="first-automation-telemetry-guide__draft-actions">
                <NButton size="small" type="primary" secondary @click="applyFirstAutomationRecommendedAction">
                  {{ $t('custom.automation.firstRuleApplyRecommendedAction') }}
                </NButton>
                <NButton size="small" tertiary @click="openFirstAutomationAlarmCreator">
                  {{ $t('custom.automation.firstRuleCreateAlarmTarget') }}
                </NButton>
              </div>
            </div>
          </div>
          <div class="first-automation-checklist" :aria-label="$t('custom.automation.firstRuleChecklistTitle')">
            <div
              v-for="(item, index) in firstAutomationStarterChecklist"
              :key="item.key"
              class="first-automation-checklist__item"
              :class="`first-automation-checklist__item--${item.status}`"
            >
              <span class="first-automation-checklist__marker">{{ index + 1 }}</span>
              <span class="first-automation-checklist__body">
                <strong>{{ item.title }}</strong>
                <small>{{ item.desc }}</small>
              </span>
            </div>
          </div>
        </div>
      </n-alert>
      <NForm
        ref="configFormRef"
        :model="configForm"
        :rules="configFormRules"
        label-placement="left"
        label-width="80"
        size="small"
      >
        <NFlex>
          <NFormItem :label="$t('generate.labelName')" path="name" class="w-150">
            <NInput v-model:value="configForm.name" :placeholder="$t('generate.enter-scene-linkage-name')" />
          </NFormItem>
          <NFormItem :label="$t('generate.description')" path="description" class="w-150">
            <NInput
              v-model:value="configForm.description"
              type="textarea"
              :placeholder="$t('generate.enter-description')"
              rows="1"
            />
          </NFormItem>
        </NFlex>
        <NFormItem :label="$t('generate.if')" class="w-100%" path="trigger_condition_groups" :show-feedback="false">
          <EditPremise
            ref="editPremise"
            :device_id="propsData.device_id"
            :device_config_id="propsData.device_config_id"
            :condition-data="conditionData"
            @condition-chose="conditionChose"
          />
        </NFormItem>
        <n-divider dashed class="divider-class" />
        <NFormItem :label="$t('generate.then')" class="w-100%" path="actions" :show-feedback="false">
          <EditAction ref="editAction" :conditions-type="conditionsType" :action-data="actionData" />
        </NFormItem>
      </NForm>
      <AutomationDryRunPreview
        :local-status-text="localPreviewStatusText"
        :backend-status-text="backendDryRunStatusText"
        :backend-alert-type="backendDryRunAlertType"
        :backend-error="backendDryRunError"
        :condition-group-count="previewConditionGroups.length"
        :condition-count="previewConditionCount"
        :action-count="previewActionCount"
        :condition-summary-items="conditionSummaryItems"
        :action-summary-items="actionSummaryItems"
        :operator-plan="operatorPlan"
        :backend-dry-run-view="backendDryRunView"
        :customer-dry-run-view="customerDryRunView"
        :beginner-guide-cards="beginnerGuideCards"
        :quick-fix-actions="automationDryRunQuickFixActions"
        :local-blocking-errors="localBlockingErrors"
        :dry-run-response-text="dryRunResponseText"
        :is-backend-dry-run-loading="isBackendDryRunLoading"
        @refresh="refreshLocalExecutionExplanation"
        @run-backend-dry-run="runBackendDryRun"
        @quick-fix="handleAutomationDryRunQuickFix"
      />
      <n-divider class="divider-class" />
      <NFlex justify="center">
        <NButton type="primary" :loading="isSaveDryRunLoading" @click="submitData">
          {{ $t('generate.save-scene-linkage') }}
        </NButton>
      </NFlex>
    </NCard>
  </div>
</template>

<style scoped>
.first-automation-starter {
  display: grid;
  gap: 10px;
}

.first-automation-starter__title {
  font-size: var(--font-size-lg);
  font-weight: 700;
  color: var(--text-color-1);
}

.first-automation-starter__desc {
  margin-top: 4px;
  color: var(--text-color-2);
  line-height: 1.5;
}

.first-automation-starter__steps {
  display: grid;
  gap: 6px;
  color: var(--text-color-2);
  line-height: 1.55;
}

.first-automation-telemetry-guide {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--card-color);
}

.first-automation-telemetry-guide__title {
  font-size: var(--font-size-base);
  font-weight: 700;
  color: var(--text-color-1);
}

.first-automation-telemetry-guide__desc {
  margin-top: 3px;
  color: var(--text-color-2);
  line-height: 1.45;
}

.first-automation-telemetry-guide__cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.first-automation-telemetry-guide__card {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 8px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
}

.first-automation-telemetry-guide__card--ready {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-automation-telemetry-guide__card span {
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.first-automation-telemetry-guide__card strong {
  overflow-wrap: anywhere;
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__hint {
  color: var(--text-color-2);
  line-height: 1.5;
}

.first-automation-telemetry-guide__next {
  display: grid;
  gap: 3px;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
  color: var(--text-color-2);
  line-height: 1.45;
}

.first-automation-telemetry-guide__next--ready {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-automation-telemetry-guide__next--missing {
  border-color: rgb(var(--warning-color) / 0.6);
  background: rgb(var(--warning-color) / 0.1);
}

.first-automation-telemetry-guide__next strong {
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__next span {
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__draft {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 9px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
}

.first-automation-telemetry-guide__draft--ready {
  border-color: rgb(var(--info-color) / 0.55);
  background: rgb(var(--info-color) / 0.08);
}

.first-automation-telemetry-guide__draft--missing {
  border-color: rgb(var(--warning-color) / 0.6);
  background: rgb(var(--warning-color) / 0.1);
}

.first-automation-telemetry-guide__draft--action {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-automation-telemetry-guide__draft > div:first-child {
  display: grid;
  gap: 3px;
  min-width: 0;
  color: var(--text-color-2);
  line-height: 1.45;
}

.first-automation-telemetry-guide__draft strong {
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__draft span,
.first-automation-telemetry-guide__draft small {
  overflow-wrap: anywhere;
  font-size: var(--font-size-secondary);
}

.first-automation-telemetry-guide__draft small {
  color: rgb(var(--success-600));
}

.first-automation-telemetry-guide__draft-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.first-automation-checklist {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.first-automation-checklist__item {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
}

.first-automation-checklist__item--done {
  border-color: rgb(var(--success-color) / 0.55);
  background: rgb(var(--success-color) / 0.06);
}

.first-automation-checklist__item--active {
  border-color: rgb(var(--info-color) / 0.55);
  background: rgb(var(--info-color) / 0.08);
}

.first-automation-checklist__marker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--radius-pill);
  background: var(--divider-color);
  color: var(--text-color-1);
  font-size: var(--font-size-caption);
  font-weight: 700;
}

.first-automation-checklist__item--done .first-automation-checklist__marker {
  background: rgb(var(--success-color));
  color: white;
}

.first-automation-checklist__item--active .first-automation-checklist__marker {
  background: rgb(var(--info-color));
  color: white;
}

.first-automation-checklist__body {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.first-automation-checklist__body strong {
  font-size: var(--font-size-secondary);
  color: var(--text-color-1);
}

.first-automation-checklist__body small {
  color: var(--text-color-2);
  line-height: 1.45;
}

@media (max-width: 960px) {
  .first-automation-telemetry-guide__cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .first-automation-checklist {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .first-automation-telemetry-guide__cards,
  .first-automation-checklist {
    grid-template-columns: 1fr;
  }

  .first-automation-telemetry-guide__draft {
    grid-template-columns: 1fr;
  }

  .first-automation-telemetry-guide__draft-actions {
    justify-content: flex-start;
  }
}
</style>
