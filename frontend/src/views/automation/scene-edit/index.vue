<!--
  文件用途: 普通自动化场景编辑页，负责新增/编辑场景时的动作配置与提交。
  关键流程:
  1. 新增态只初始化空动作组；设备、配置、场景、告警等目录在打开/搜索或编辑回显需要时加载。
  2. 编辑态先取场景详情，再把接口 actions 回显成前端表单结构。
  3. 提交时再把表单结构重新压平为接口要求的 actions payload。
  模块划分:
  - 设备动作目录 useSceneActionTargetCatalog；激活场景/触发告警远程目录 useSceneRemoteTargetCatalog；
  - 试运行快速修复与保存门禁 useSceneDryRunActions；表单/接口映射 scene-edit-form-orchestration。
  关键注意事项: 页面同时维护“表单态”和“接口态”两套动作结构，回显与提交映射必须保持对称。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NCard, NFlex, useDialog, useMessage } from 'naive-ui'
import type { FormInst } from 'naive-ui'
import PopUp from '@/views/alarm/warning-message/components/pop-up.vue'
import { sceneAdd, sceneDryRun, sceneEdit, sceneInfo } from '@/service/api/automation'
import { $t } from '@/locales'
import { useTabStore } from '@/store/modules/tab'
import {
  OPERATE_DEVICE_ACTION_TYPE,
  type SceneActionGroupLike as SceneActionGroup,
  type SceneInstructionLike as SceneInstruction
} from './scene-action-mappers'
import {
  buildSceneSubmitPayload,
  duplicateSceneActionGroup,
  formatSceneActionsForEdit
} from './scene-edit-form-orchestration'
import { createEmptySceneActionGroup, createEmptySceneInstruction } from './scene-action-form-factories'
import { validateSceneActionJsonValues } from './scene-action-form-state'
import AutomationDryRunPreview from '../linkage-edit/modules/AutomationDryRunPreview.vue'
import LinkageActionExecutionSummary from '../linkage-edit/modules/LinkageActionExecutionSummary.vue'
import { useAutomationExecutionPreview } from '../linkage-edit/modules/useAutomationExecutionPreview'
import SceneOperateDeviceActionGroupEditor from './scene-operate-device-action-group-editor.vue'
import { buildSceneActionDryRunPayload, getSceneActionLocalBlocker } from './scene-dry-run-preview'
import { useSceneActionTargetCatalog } from './useSceneActionTargetCatalog'
import { useSceneRemoteTargetCatalog } from './useSceneRemoteTargetCatalog'
import { useSceneDryRunActions } from './useSceneDryRunActions'

type SceneConfigForm = { id: string; name: string; description: string; actions: SceneActionGroup[] }

const route = useRoute()
const router = useRouter()
const dialog = useDialog()
const message = useMessage()
const tabStore = useTabStore()

const configId = ref(route.query.id || '')
const getCurrentSceneId = () => (typeof configId.value === 'string' ? configId.value : '')

// `configForm` 是整页唯一的可提交状态，动作组与设备指令均挂在 `actions` 内。
const configFormRef = ref<FormInst | null>(null)
const configForm = ref<SceneConfigForm>({ id: '', name: '', description: '', actions: [] })

// 规则只覆盖“有值校验”，JSON 合法性在提交时由 validateSceneActionJsonValues 补充。
const selectRule = (trigger?: string) => ({ required: true, message: $t('common.select'), ...(trigger && { trigger }) })
const configFormRules = ref({
  name: { required: true, message: $t('generate.enter-scene-name') },
  description: { required: false, message: $t('generate.enterSceneDesc') },
  actionType: selectRule('change'),
  action_type: selectRule('change'),
  action_target: selectRule('change'),
  action_param_type: selectRule('change'),
  action_param: selectRule('change'),
  actionValue: selectRule()
})

// 当前页面只开放“操作设备”动作；激活场景/触发告警分支保留模板兼容性，不在入口暴露。
const actionOptions = ref([{ label: $t('common.operateDevice'), value: OPERATE_DEVICE_ACTION_TYPE, disabled: false }])
// 操作设备动作内部再区分“单个设备”和“单类设备”两条取数路径。
const actionTypeOptions = ref([
  { label: $t('common.singleDevice'), value: '10' },
  { label: $t('common.singleClassDevice'), value: '11' }
])

// 切换动作组类型时，先重置当前组，再按新类型补最小可编辑结构。
const actionChange = (actionGroupItem: SceneActionGroup, _actionGroupIndex: number, data: string | null) => {
  actionOptions.value.forEach((item) => {
    item.disabled = data === OPERATE_DEVICE_ACTION_TYPE && item.value === OPERATE_DEVICE_ACTION_TYPE
  })
  actionGroupItem.actionInstructList = data === OPERATE_DEVICE_ACTION_TYPE ? [createEmptySceneInstruction()] : []
  actionGroupItem.action_type = null
  actionGroupItem.action_target = null
}

const {
  actionParamShow,
  actionTargetChange,
  actionTypeChange,
  deviceConfigOption,
  deviceGroupOptions,
  deviceOptions,
  ensureDeviceConfigOptionsLoaded,
  ensureDeviceGroupsLoaded,
  ensureDeviceOptionsLoaded,
  ensureDeviceTargetCatalogsLoaded,
  getDevice,
  getDeviceConfig,
  loadingSelect: actionTargetLoading,
  queryDevice
} = useSceneActionTargetCatalog()

const {
  remoteSelectLoading,
  sceneList,
  getSceneList,
  ensureSceneListLoaded,
  alarmList,
  getAlarmList,
  ensureAlarmListLoaded
} = useSceneRemoteTargetCatalog()

// 告警弹窗创建成功后刷新下拉，保证用户可立刻选中新建项。
const popUpVisible = ref(false)
const handleAlarmCreated = () => getAlarmList('')

// 新增动作组前先触发表单校验，避免页面出现多组半成品配置。
const addActionGroupItem = async () => {
  if (configForm.value.actions.length !== 0) await configFormRef.value?.validate()
  configForm.value.actions.push(createEmptySceneActionGroup() as SceneActionGroup)
}
const deleteActionGroupItem = (actionGroupIndex: number) => {
  configForm.value.actions.splice(actionGroupIndex, 1)
}
const duplicateActionGroupItem = (actionGroupIndex: number) => {
  const duplicatedGroup = duplicateSceneActionGroup(configForm.value.actions[actionGroupIndex])
  configForm.value.actions.splice(actionGroupIndex + 1, 0, duplicatedGroup)
}

// 提交前把页面维护的嵌套动作结构转回接口需要的扁平数组。
const buildSubmitPayload = () => buildSceneSubmitPayload(configForm.value)
const buildSceneExecutionPreviewPayload = () =>
  buildSceneActionDryRunPayload({ form: configForm.value, buildSubmitPayload })

const {
  backendDryRunError,
  isBackendDryRunLoading,
  executionPreview,
  previewConditionGroups,
  previewConditionCount,
  previewActionCount,
  localBlockingErrors,
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
  buildPayload: buildSceneExecutionPreviewPayload,
  dryRun: sceneDryRun,
  getLocalBlocker: (payload) => getSceneActionLocalBlocker(payload, $t)
})

const sceneLocalPreviewStatusText = computed(() => {
  if (localBlockingErrors.value.length > 0) return $t('generate.sceneDryRunLocalHasBlocker')
  if (executionPreview.value) return $t('generate.sceneDryRunLocalReady')
  return $t('generate.sceneDryRunLocalEmpty')
})

const { isSaveDryRunLoading, sceneDryRunQuickFixActions, handleSceneDryRunQuickFix, ensureSceneDryRunCanSave } =
  useSceneDryRunActions({
    actions: computed(() => configForm.value.actions) as any,
    message,
    buildPayload: buildSceneExecutionPreviewPayload,
    refreshLocalExecutionExplanation,
    runBackendDryRunForPayload,
    onActionTypeChange: actionChange
  })

const saveScene = async () => {
  const payload = buildSubmitPayload()
  const res = getCurrentSceneId() ? await sceneEdit(payload) : await sceneAdd(payload)
  if (res.error) return
  await tabStore.removeTab(route.path)
  router.replace({ path: '/automation/scene-manage' })
}

// 提交分两步: 先做前端校验与试运行门禁，再弹确认框，最终由确认回调触发真正保存。
const submitData = async () => {
  await configFormRef.value?.validate()
  const [firstIssue] = validateSceneActionJsonValues(configForm.value.actions, $t('common.enterJson'))
  if (firstIssue) {
    message.error(`${$t('common.enterJson')} #${firstIssue.actionGroupIndex + 1}.${firstIssue.instructIndex + 1}`)
    return
  }
  if (!(await ensureSceneDryRunCanSave())) return
  dialog.warning({
    title: $t('common.tip'),
    content: $t('common.saveSceneInfo'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: saveScene
  })
}

// 回显后按动作类型按需加载目录，保证下拉能显示已选项的名称。
const loadCatalogsForEcho = (groups: SceneActionGroup[]) => {
  groups.forEach((group) => {
    if (group.actionType === '20') ensureSceneListLoaded()
    if (group.actionType === '30') ensureAlarmListLoaded()
    if (group.actionType !== OPERATE_DEVICE_ACTION_TYPE) return
    group.actionInstructList.forEach((instruct: SceneInstruction) => {
      if (instruct.action_type === '10') ensureDeviceTargetCatalogsLoaded()
      if (instruct.action_type === '11') ensureDeviceConfigOptionsLoaded()
      void actionParamShow(instruct)
    })
  })
}

const getSceneInfo = async () => {
  const sceneId = getCurrentSceneId()
  if (!sceneId) return
  const res = await sceneInfo(sceneId)
  const actions = res.data?.actions || []
  configForm.value = {
    ...configForm.value,
    ...(res.data?.info || {}),
    actions: formatSceneActionsForEdit<SceneActionGroup>(actions)
  }
  loadCatalogsForEcho(configForm.value.actions)
}

// 新增态默认插入一个空动作组，编辑态则先补 id 再拉详情。
onMounted(() => {
  const sceneId = getCurrentSceneId()
  if (!sceneId) {
    void addActionGroupItem()
    return
  }
  configForm.value.id = sceneId
  void getSceneInfo()
})
</script>

<template>
  <div class="scene-edit">
    <NCard :bordered="false" :title="`${configId ? $t('card.editScene') : $t('card.addScene')}`">
      <NForm
        ref="configFormRef"
        :model="configForm"
        :rules="configFormRules"
        label-placement="left"
        label-width="100"
        size="small"
      >
        <NFormItem :label="$t('generate.labelName')" path="name" class="w-150">
          <NInput v-model:value="configForm.name" :placeholder="$t('generate.enterSceneName')" />
        </NFormItem>
        <NFormItem :label="$t('generate.description')" path="description" class="w-150">
          <NInput
            v-model:value="configForm.description"
            type="textarea"
            :placeholder="$t('generate.enter-description')"
            rows="1"
          />
        </NFormItem>
        <NFormItem :label="$t('generate.action')" required class="w-100%" :show-feedback="false">
          <NFlex vertical class="mt-1 w-100%">
            <NFlex
              v-for="(actionGroupItem, actionGroupIndex) in configForm.actions"
              :key="actionGroupIndex"
              class="mt-1 w-100%"
            >
              <NFormItem
                :show-label="false"
                :show-feedback="false"
                :path="`actions[${actionGroupIndex}].actionType`"
                :rule="configFormRules.actionType"
                class="max-w-30 w-full"
              >
                <NSelect
                  v-model:value="actionGroupItem.actionType"
                  :options="actionOptions"
                  @update:value="(data) => actionChange(actionGroupItem, actionGroupIndex, data)"
                />
              </NFormItem>
              <template v-if="actionGroupItem.actionType === OPERATE_DEVICE_ACTION_TYPE">
                <SceneOperateDeviceActionGroupEditor
                  :action-group-item="actionGroupItem"
                  :action-group-index="actionGroupIndex"
                  :action-type-options="actionTypeOptions"
                  :config-form-rules="configFormRules"
                  :device-config-option="deviceConfigOption"
                  :device-group-options="deviceGroupOptions"
                  :device-options="deviceOptions"
                  :ensure-device-config-options-loaded="ensureDeviceConfigOptionsLoaded"
                  :ensure-device-groups-loaded="ensureDeviceGroupsLoaded"
                  :ensure-device-options-loaded="ensureDeviceOptionsLoaded"
                  :ensure-device-target-catalogs-loaded="ensureDeviceTargetCatalogsLoaded"
                  :get-device="getDevice"
                  :get-device-config="getDeviceConfig"
                  :loading-select="actionTargetLoading"
                  :query-device="queryDevice"
                  :action-target-change="actionTargetChange"
                  :action-type-change="actionTypeChange"
                  :create-instruction="createEmptySceneInstruction"
                />
              </template>
              <template v-if="actionGroupItem.actionType === '20'">
                <NFlex class="ml-6 w-auto" align="center">
                  <NFormItem
                    :label="$t('generate.activate')"
                    label-width="60px"
                    :show-feedback="false"
                    :path="`actions[${actionGroupIndex}].action_target`"
                    :rule="configFormRules.action_target"
                    class="w-full"
                  >
                    <NSelect
                      v-model:value="actionGroupItem.action_target"
                      :options="sceneList"
                      label-field="name"
                      value-field="id"
                      :placeholder="$t('common.select')"
                      :loading="remoteSelectLoading"
                      filterable
                      class="max-w-50"
                      remote
                      @search="getSceneList"
                      @update:show="(show) => show && ensureSceneListLoaded()"
                    />
                  </NFormItem>
                </NFlex>
              </template>
              <template v-if="actionGroupItem.actionType === '30'">
                <NFlex class="ml-6 w-auto">
                  <NFormItem
                    :label="$t('generate.trigger')"
                    label-width="60px"
                    :show-feedback="false"
                    :path="`actions[${actionGroupIndex}].action_target`"
                    :rule="configFormRules.action_target"
                  >
                    <NSelect
                      v-model:value="actionGroupItem.action_target"
                      :options="alarmList"
                      label-field="name"
                      value-field="id"
                      :placeholder="$t('common.select')"
                      class="max-w-50"
                      filterable
                      remote
                      :loading="remoteSelectLoading"
                      @search="getAlarmList"
                      @update:show="(show) => show && ensureAlarmListLoaded()"
                    />
                  </NFormItem>
                  <NButton class="w-20" dashed type="info" @click="popUpVisible = true">
                    {{ $t('generate.create-alarm') }}
                  </NButton>
                </NFlex>
              </template>
              <NButton type="default" @click="duplicateActionGroupItem(actionGroupIndex)">
                {{ $t('generate.copy') }}
              </NButton>
              <NButton v-if="actionGroupIndex > 0" type="error" @click="deleteActionGroupItem(actionGroupIndex)">
                {{ $t('generate.delete-execution-action') }}
              </NButton>
            </NFlex>
            <NButton type="primary" class="w-30" @click="addActionGroupItem()">
              {{ $t('generate.add-execution-action') }}
            </NButton>
          </NFlex>
        </NFormItem>
      </NForm>
      <LinkageActionExecutionSummary
        :action-groups="configForm.actions"
        :device-options="deviceOptions"
        :device-config-options="deviceConfigOption"
        :scene-options="sceneList"
        :alarm-options="alarmList"
      />
      <AutomationDryRunPreview
        :local-status-text="sceneLocalPreviewStatusText"
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
        :quick-fix-actions="sceneDryRunQuickFixActions"
        :local-blocking-errors="localBlockingErrors"
        :dry-run-response-text="dryRunResponseText"
        :is-backend-dry-run-loading="isBackendDryRunLoading"
        :scene-action-only="true"
        @refresh="refreshLocalExecutionExplanation"
        @run-backend-dry-run="runBackendDryRun"
        @quick-fix="handleSceneDryRunQuickFix"
      />
      <n-divider class="divider-class" />
      <NFlex justify="center" class="mb-5">
        <NButton type="primary" :loading="isSaveDryRunLoading" @click="submitData">
          {{ $t('generate.save-scene-configuration') }}
        </NButton>
      </NFlex>
    </NCard>
    <PopUp v-model:visible="popUpVisible" type="add" :edit-data="null" @new-edit="handleAlarmCreated" />
  </div>
</template>

<style scoped>
:deep(.n-card__content) {
  padding: 10px 10px 4px 10px !important;
}
</style>
