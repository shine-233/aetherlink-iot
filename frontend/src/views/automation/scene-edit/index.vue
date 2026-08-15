<!--
  文件用途: 普通自动化场景编辑页，负责新增/编辑场景时的动作配置与提交。
  关键流程:
  1. 新增态只初始化空动作组；设备、配置、场景、告警等目录在打开/搜索或编辑回显需要时加载。
  2. 编辑态先取场景详情，再把接口 actions 回显成前端表单结构。
  3. 提交时再把表单结构重新压平为接口要求的 actions payload。
  静态审查关注点:
  1. 页面同时维护“表单态”和“接口态”两套动作结构，回显与提交映射必须保持对称。
  2. `actionValue` 的 JSON 校验依赖 blur 事件，未覆盖接口异常和脏数据回显场景。
  3. 当前 `actionOptions` 只开放“操作设备”，但模板仍保留激活场景/触发告警分支，后续若恢复入口需先核对联动逻辑。
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NCard, NFlex, useDialog, useMessage } from 'naive-ui'
import type { FormInst } from 'naive-ui'
import { warningMessageList } from '@/service/api/alarm'
import PopUp from '@/views/alarm/warning-message/components/pop-up.vue'
import { sceneAdd, sceneDryRun, sceneEdit, sceneGet, sceneInfo } from '@/service/api/automation'
import { $t } from '@/locales'
import { useTabStore } from '@/store/modules/tab'
import {
  OPERATE_DEVICE_ACTION_TYPE,
  type SceneActionGroupLike,
  type SceneInstructionLike
} from './scene-action-mappers'
import {
  buildSceneSubmitPayload,
  duplicateSceneActionGroup,
  formatSceneActionsForEdit
} from './scene-edit-form-orchestration'
import {
  createEmptySceneActionGroup,
  createEmptySceneInstruction
} from './scene-action-form-factories'
import { validateSceneActionJsonValues } from './scene-action-form-state'
import AutomationDryRunPreview from '../linkage-edit/modules/AutomationDryRunPreview.vue'
import LinkageActionExecutionSummary from '../linkage-edit/modules/LinkageActionExecutionSummary.vue'
import type { AutomationDryRunQuickFixAction } from '../linkage-edit/modules/automationDryRunPreview'
import { runAutomationDryRunSaveGate } from '../linkage-edit/modules/automationSaveFlow'
import { useAutomationExecutionPreview } from '../linkage-edit/modules/useAutomationExecutionPreview'
import SceneOperateDeviceActionGroupEditor from './scene-operate-device-action-group-editor.vue'
import {
  SCENE_DRY_RUN_QUICK_FIX_KEYS,
  buildSceneActionDryRunPayload,
  buildSceneDryRunQuickFixActions,
  getSceneActionLocalBlocker
} from './scene-dry-run-preview'
import { useSceneActionTargetCatalog, type SelectOption } from './useSceneActionTargetCatalog'

const route = useRoute()
const router = useRouter()
const dialog = useDialog()
const message = useMessage()

// 场景动作协议里会混用“动作组类型”和“设备指令类型”，这里集中声明常量避免散落 magic value。
// 这几类参数在表单中直接输入 JSON，不再展示二级标识符选择器。
// 这些参数在失焦时需要做 JSON 格式校验，避免把无效字符串直接提交给后端。
// 这两类参数提交给后端时需要封装为 { key: value } 结构。
const configId = ref(route.query.id || '')
type SceneInstruction = SceneInstructionLike
type SceneActionGroup = SceneActionGroupLike
type SceneConfigForm = {
  id: string
  name: string
  description: string
  actions: SceneActionGroup[]
}
type NamePageQuery = {
  page: number
  page_size: number
  name: string
}

const getCurrentSceneId = () => {
  return typeof configId.value === 'string' ? configId.value : ''
}

// 告警弹窗创建成功后刷新下拉，保证用户可立刻选中新建项。
const popUpVisible = ref(false)
const handleAlarmCreated = () => {
  getAlarmList('')
}

// `configForm` 是整页唯一的可提交状态，动作组与设备指令均挂在 `actions` 内。
const configFormRef = ref<FormInst | null>(null)
const configForm = ref<SceneConfigForm>({
  id: '',
  name: '',
  description: '',
  actions: []
})
const isSaveDryRunLoading = ref(false)

// 规则只覆盖“有值校验”，更细的 JSON 合法性由 `actionValueChange` 补充处理。
const configFormRules = ref({
  name: {
    required: true,
    message: $t('generate.enter-scene-name')
  },
  description: {
    required: false,
    message: $t('generate.enterSceneDesc')
  },
  actionType: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  action_type: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  action_target: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  action_param_type: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  action_param: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  actionValue: {
    required: true,
    message: $t('common.select')
  }
})

// 激活场景/触发告警分支共用一个远程加载态，和设备动作目录的加载态分开维护。
const remoteSelectLoading = ref(false)

// 当前页面只开放“操作设备”动作；其余动作分支先保留模板兼容性，不在入口暴露。
const actionOptions = ref([
  {
    label: $t('common.operateDevice'),
    value: OPERATE_DEVICE_ACTION_TYPE,
    disabled: false
  }
])

// 提交阶段把表单中的 `actionValue` 转成接口要求的 `action_value`。
// 不同参数类型的序列化规则不同，这里是最关键的协议收口点之一。
const navigateToSceneManage = async () => {
  await tabStore.removeTab(route.path)
  router.replace({ path: '/automation/scene-manage' })
}

const resetActionOptionsDisabledState = () => {
  actionOptions.value.forEach((item) => {
    item.disabled = false
  })
}

const resetActionGroupSelection = (actionGroupItem: SceneActionGroup) => {
  actionGroupItem.actionInstructList = []
  actionGroupItem.action_type = null
  actionGroupItem.action_target = null
}

const syncActionOptionsForGroupChange = (actionType: string | null) => {
  resetActionOptionsDisabledState()

  if (actionType === OPERATE_DEVICE_ACTION_TYPE) {
    const operateDeviceOption = actionOptions.value.find((item) => item.value === OPERATE_DEVICE_ACTION_TYPE)
    if (operateDeviceOption) {
      operateDeviceOption.disabled = true
    }
  }
}

// 切换动作组类型时，先重置当前组，再按新类型补最小可编辑结构。
const actionChange = (actionGroupItem: SceneActionGroup, _actionGroupIndex: number, data: string | null) => {
  syncActionOptionsForGroupChange(data)
  resetActionGroupSelection(actionGroupItem)
  if (data === OPERATE_DEVICE_ACTION_TYPE) {
    actionGroupItem.actionInstructList = [createEmptySceneInstruction()]
  }
}

// 操作设备动作内部再区分“单个设备”和“单类设备”两条取数路径。
const actionTypeOptions = ref([
  {
    label: $t('common.singleDevice'),
    value: '10'
  },
  {
    label: $t('common.singleClassDevice'),
    value: '11'
  }
])
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

// 激活场景/触发告警分支虽然暂未开放入口，但其远程选项加载逻辑仍需保留。
const sceneList = ref<SelectOption[]>([])
const queryScene = ref<NamePageQuery>({
  page: 1,
  page_size: 10,
  name: ''
})
const getSceneList = async (name: string) => {
  queryScene.value.name = name || ''
  remoteSelectLoading.value = true
  try {
    const res = await sceneGet(queryScene.value)
    sceneList.value = res.data?.list || []
  } finally {
    remoteSelectLoading.value = false
  }
}

const ensureSceneListLoaded = () => {
  if (sceneList.value.length > 0) return
  void getSceneList('')
}

const alarmList = ref<SelectOption[]>([])
const queryAlarm = ref<NamePageQuery>({
  page: 1,
  page_size: 10,
  name: ''
})

const getAlarmList = async (name: string) => {
  queryAlarm.value.name = name || ''
  remoteSelectLoading.value = true
  try {
    const res = await warningMessageList(queryAlarm.value)
    alarmList.value = res.data?.list || []
  } finally {
    remoteSelectLoading.value = false
  }
}

const ensureAlarmListLoaded = () => {
  if (alarmList.value.length > 0) return
  void getAlarmList('')
}

// 新增动作组前先触发表单校验，避免页面出现多组半成品配置。
const addActionGroupItem = async () => {
  if (configForm.value.actions.length !== 0) {
    await configFormRef.value?.validate()
  }
  configForm.value.actions.push(createEmptySceneActionGroup() as SceneActionGroup)
}

const deleteActionGroupItem = (actionGroupIndex: any) => {
  configForm.value.actions.splice(actionGroupIndex, 1)
}

const duplicateActionGroupItem = (actionGroupIndex: number) => {
  const duplicatedGroup = duplicateSceneActionGroup(configForm.value.actions[actionGroupIndex])
  configForm.value.actions.splice(actionGroupIndex + 1, 0, duplicatedGroup)
}

const tabStore = useTabStore()

// 提交前把页面维护的嵌套动作结构转回接口需要的扁平数组。
const buildSubmitPayload = () => {
  return buildSceneSubmitPayload(configForm.value as SceneConfigForm)
}

const buildSceneExecutionPreviewPayload = () => {
  return buildSceneActionDryRunPayload({
    form: configForm.value,
    buildSubmitPayload
  })
}

const {
  backendDryRunError,
  isBackendDryRunLoading,
  executionPreview,
  previewConditionGroups,
  previewActions,
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

const sceneDryRunQuickFixActions = computed<AutomationDryRunQuickFixAction[]>(() =>
  buildSceneDryRunQuickFixActions({
    actionGroups: configForm.value.actions,
    texts: {
      addActionGroupTitle: $t('generate.sceneDryRunQuickFixAddActionGroupTitle'),
      addActionGroupDesc: $t('generate.sceneDryRunQuickFixAddActionGroupDesc'),
      addActionGroupButton: $t('generate.sceneDryRunQuickFixAddActionGroupButton'),
      selectOperateDeviceTitle: $t('generate.sceneDryRunQuickFixSelectOperateDeviceTitle'),
      selectOperateDeviceDesc: $t('generate.sceneDryRunQuickFixSelectOperateDeviceDesc'),
      selectOperateDeviceButton: $t('generate.sceneDryRunQuickFixSelectOperateDeviceButton'),
      addDeviceInstructionTitle: $t('generate.sceneDryRunQuickFixAddDeviceInstructionTitle'),
      addDeviceInstructionDesc: $t('generate.sceneDryRunQuickFixAddDeviceInstructionDesc'),
      addDeviceInstructionButton: $t('generate.sceneDryRunQuickFixAddDeviceInstructionButton')
    }
  })
)

const refreshSceneDryRunExplanationAfterQuickFix = () => {
  void nextTick(() => {
    refreshLocalExecutionExplanation()
  })
}

const addSceneActionGroupFromQuickFix = () => {
  configForm.value.actions.push(createEmptySceneActionGroup() as SceneActionGroup)
  message.success($t('generate.sceneDryRunQuickFixAddActionGroupAdded'))
  refreshSceneDryRunExplanationAfterQuickFix()
}

const selectOperateDeviceFromQuickFix = () => {
  if (!configForm.value.actions[0]) {
    configForm.value.actions.push(createEmptySceneActionGroup() as SceneActionGroup)
  }

  const firstGroup = configForm.value.actions[0]
  firstGroup.actionType = OPERATE_DEVICE_ACTION_TYPE
  actionChange(firstGroup, 0, OPERATE_DEVICE_ACTION_TYPE)
  message.success($t('generate.sceneDryRunQuickFixSelectOperateDeviceApplied'))
  refreshSceneDryRunExplanationAfterQuickFix()
}

const addDeviceInstructionFromQuickFix = () => {
  const firstGroup = configForm.value.actions[0]
  if (!firstGroup) {
    addSceneActionGroupFromQuickFix()
    return
  }

  firstGroup.actionInstructList.push(createEmptySceneInstruction())
  message.success($t('generate.sceneDryRunQuickFixAddDeviceInstructionAdded'))
  refreshSceneDryRunExplanationAfterQuickFix()
}

const handleSceneDryRunQuickFix = (key: string) => {
  if (key === SCENE_DRY_RUN_QUICK_FIX_KEYS.addActionGroup) {
    addSceneActionGroupFromQuickFix()
    return
  }

  if (key === SCENE_DRY_RUN_QUICK_FIX_KEYS.selectOperateDevice) {
    selectOperateDeviceFromQuickFix()
    return
  }

  if (key === SCENE_DRY_RUN_QUICK_FIX_KEYS.addDeviceInstruction) {
    addDeviceInstructionFromQuickFix()
  }
}

const ensureSceneDryRunCanSave = async () => {
  const payload = buildSceneExecutionPreviewPayload()
  const localBlocker = getSceneActionLocalBlocker(payload, $t)
  if (localBlocker) {
    message.error(localBlocker)
    return false
  }

  isSaveDryRunLoading.value = true
  try {
    const result = await runAutomationDryRunSaveGate({
      payload,
      runBackendDryRunForPayload,
      backendUnavailableMessage: $t('generate.automationDryRunBackendUnavailable'),
      saveBlockedMessage: $t('generate.automationDryRunSaveBlocked')
    })
    if (!result.canSave) message.error(result.message)
    return result.canSave
  } finally {
    isSaveDryRunLoading.value = false
  }
}

// 新增与编辑共用同一套保存逻辑，只在接口入口上做分流。
const saveScene = async () => {
  const configFormData = buildSubmitPayload()
  const res = getCurrentSceneId() ? await sceneEdit(configFormData) : await sceneAdd(configFormData)

  if (!res.error) {
    await navigateToSceneManage()
  }
}

// 提交分两步: 先做前端校验，再弹确认框，最终由确认回调触发真正保存。
const submitData = async () => {
  await configFormRef.value?.validate()
  const jsonIssues = validateSceneActionJsonValues(configForm.value.actions, $t('common.enterJson'))
  if (jsonIssues.length > 0) {
    const firstIssue = jsonIssues[0]
    message.error(`${$t('common.enterJson')} #${firstIssue.actionGroupIndex + 1}.${firstIssue.instructIndex + 1}`)
    return
  }

  if (!(await ensureSceneDryRunCanSave())) {
    return
  }

  dialog.warning({
    title: $t('common.tip'),
    content: $t('common.saveSceneInfo'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: saveScene
  })
}

// 编辑态只依赖 sceneId 拉详情，然后复用回显映射把接口数据还原回表单结构。
const getSceneInfo = async () => {
  const sceneId = getCurrentSceneId()
  if (!sceneId) {
    return
  }

  const res = await sceneInfo(sceneId)
  const info = res.data?.info || {}
  const actions = res.data?.actions || []
  configForm.value = { ...configForm.value, ...info, actions }
  dataEcho(actions)
}

// 场景详情回显的真正收口点，后续若拆纯函数，这里是最合适的落点。
const dataEcho = (actionsData: SceneInstruction[]) => {
  configForm.value.actions = formatSceneActionsForEdit<SceneActionGroup>(actionsData)
  configForm.value.actions
    .filter((actionGroup: SceneActionGroup) => actionGroup.actionType === OPERATE_DEVICE_ACTION_TYPE)
    .forEach((actionGroup: SceneActionGroup) => {
      actionGroup.actionInstructList.forEach((instructItem: SceneInstruction) => {
        if (instructItem.action_type === '10') {
          ensureDeviceTargetCatalogsLoaded()
        }
        if (instructItem.action_type === '11') {
          ensureDeviceConfigOptionsLoaded()
        }
        void actionParamShow(instructItem)
      })
    })
  configForm.value.actions.forEach((actionGroup: SceneActionGroup) => {
    if (actionGroup.actionType === '20') {
      ensureSceneListLoaded()
    }
    if (actionGroup.actionType === '30') {
      ensureAlarmListLoaded()
    }
  })
}

// 新增态默认插入一个空动作组，编辑态则先补 id 再拉详情。
const initializeSceneForm = () => {
  const sceneId = getCurrentSceneId()
  if (sceneId) {
    configForm.value.id = sceneId
    void getSceneInfo()
    return
  }

  void addActionGroupItem()
}

onMounted(() => {
  initializeSceneForm()
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
