<!--
  文件用途：联动规则（scene_automations）编辑页入口。
  核心逻辑：整合触发条件（EditPremise）、执行动作（EditAction）与基础信息；新建/编辑回显、提交前格式转换、
    本地+后端试运行门禁、保存后路由返回。首台设备引导在 useFirstAutomationStarter + FirstAutomationStarterPanel。
  关键注意事项：trigger_condition_groups 与 actions 是提交核心字段；route query 中的 id、backType、
    device_id、device_config_id 会影响回显与保存后跳转。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { FormInst } from 'naive-ui'
import { NButton, NCard, useDialog } from 'naive-ui'
import AutomationDryRunPreview from '@/views/automation/linkage-edit/modules/AutomationDryRunPreview.vue'
import EditAction from '@/views/automation/linkage-edit/modules/edit-action.vue'
import EditPremise from '@/views/automation/linkage-edit/modules/edit-premise.vue'
import FirstAutomationStarterPanel from '@/views/automation/linkage-edit/modules/FirstAutomationStarterPanel.vue'
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
import { useAutomationExecutionPreview } from '@/views/automation/linkage-edit/modules/useAutomationExecutionPreview'
import { useAutomationSaveGate } from '@/views/automation/linkage-edit/modules/useAutomationSaveGate'
import { useAutomationDryRunQuickFixes } from '@/views/automation/linkage-edit/modules/useAutomationDryRunQuickFixes'
import { useFirstAutomationStarter } from '@/views/automation/linkage-edit/modules/useFirstAutomationStarter'
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
const tabStore = useTabStore()
const routeContext = readAutomationRouteContext(route.query)
const backType = ref(routeContext.backType)
const configFormRules = ref(createAutomationFormRules($t))
const configFormRef = ref<HTMLElement & FormInst>()
const configForm = ref(createDefaultAutomationForm())
const configId = ref(routeContext.configId)
const propsData = ref(routeContext.propsData)

const editPremise = ref()
const editAction = ref()
const conditionsType = ref(null as any)
const automationsInfo = ref(null as any)
const conditionData = ref([] as any)
const actionData = ref([] as any)

const conditionChose = (data: any) => {
  if (data) conditionsType.value = data
}

// 编辑器子组件通过 ref 暴露当前条件/动作；ref 未就绪时返回空数组，由提交阻断器给出提示。
const readEditorGroups = <T,>(editor: any, method: string, build: (raw: any) => T[]): T[] => {
  if (!editor?.[method]) {
    console.error(`Automation editor ref is not ready: ${method}`)
    return []
  }
  return build(editor[method]())
}
const handleIfData = () => readEditorGroups(editPremise.value, 'ifGroupsData', buildSubmitConditionGroups)
const handleActionData = () => readEditorGroups(editAction.value, 'actionGroupsReturn', buildSubmitActions)

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

const { ensureBackendDryRunCanSave, isSaveDryRunLoading } = useAutomationSaveGate({ runBackendDryRunForPayload, t: $t })

const firstAutomationStarter = useFirstAutomationStarter({
  routeContext,
  propsData,
  configForm,
  conditionData,
  actionData,
  conditionsType,
  editPremise,
  editAction,
  preview: () => ({
    conditionCount: previewConditionCount.value,
    actionCount: previewActionCount.value,
    backendDryRunStatus: backendDryRunStatus.value,
    customerDryRunStatus: customerDryRunView.value.status,
    canSave: customerDryRunView.value.canSave
  }),
  refreshLocalExecutionExplanation
})
const {
  isFirstDeviceAutomationStarter,
  firstAutomationStarterTelemetryLabel,
  firstAutomationTelemetryRecommendation,
  firstAutomationStarterChecklist,
  firstAutomationRecommendedConditionApplied,
  firstAutomationRecommendedActionApplied,
  firstAutomationRecommendedConditionDraft,
  firstAutomationRecommendedActionDraft,
  applyFirstAutomationRecommendedCondition,
  applyFirstAutomationRecommendedAction,
  openFirstAutomationAlarmCreator
} = firstAutomationStarter

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
  if (!(await ensureBackendDryRunCanSave(submitPayload))) return

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

const getSceneAutomationsInfo = async () => {
  const res = await sceneAutomationsInfo(configId.value)
  const echoState = buildAutomationEditEchoState(res?.data)
  if (!echoState) return
  automationsInfo.value = echoState.automationsInfo
  configForm.value = echoState.configForm
  conditionData.value = echoState.conditionData
  actionData.value = echoState.actionData
}

if (configId.value) {
  if (typeof configId.value === 'string') configForm.value.id = configId.value
  getSceneAutomationsInfo()
}

// 页面测试经 setupState 读取引导状态；这些绑定保持在页面作用域。
defineExpose({
  firstAutomationStarterTelemetryLabel,
  firstAutomationTelemetryRecommendation,
  firstAutomationStarterChecklist,
  firstAutomationRecommendedConditionApplied,
  firstAutomationRecommendedActionApplied,
  firstAutomationRecommendedConditionDraft,
  firstAutomationRecommendedActionDraft,
  applyFirstAutomationRecommendedCondition
})
</script>

<template>
  <div class="linkage-edit">
    <NCard
      :bordered="false"
      :title="(configId ? $t('common.edit') : $t('common.add')) + $t('route.automation_scene-linkage')"
    >
      <FirstAutomationStarterPanel v-if="isFirstDeviceAutomationStarter" :starter="firstAutomationStarter" />
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
