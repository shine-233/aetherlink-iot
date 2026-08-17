<!--
  文件用途: 自动化联动动作编辑模块。
  核心逻辑: 根据条件类型维护动作组，加载设备/配置指标、场景和告警消息，并向父页面输出 actions payload。
  关键注意事项: 条件类型变化会重置动作数据，动作 payload 字段必须与后端自动化执行语义一致。
  重构建议: 抽出动作默认值、重置策略和 payload normalize，并补条件切换导致动作重置的测试。
-->
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { type FormInst, NButton, NCard, NFlex, useMessage } from 'naive-ui'
import PopUp from '@/views/alarm/warning-message/components/pop-up.vue'
import { $t } from '@/locales'
import LinkageActionExecutionSummary from './LinkageActionExecutionSummary.vue'
import { useLinkageActionGroupState } from './linkage-action-group-state'
import { useLinkageActionParamState } from './linkage-action-param-state'
import { useLinkageActionTargetOptions } from './linkage-action-target-options'

const route = useRoute()

interface Props {
  conditionsType?: object | any
  actionData?: any
}

const props = withDefaults(defineProps<Props>(), {
  conditionsType: null,
  actionData: []
})

const configId = ref<any>(route.query.id || null)

// 新建告警弹窗显示状态
const popUpVisible = ref(false)
// 新建告警回执
const newEdit = () => {
  getAlarmList('')
}
// 场景表单实例
const configFormRef = ref<FormInst | null>(null)
const actionGroupsReturn = () => {
  return actionForm.value.actionGroups
}
const actionFormRefReturn = () => {
  return configFormRef.value
}
const openCreateAlarm = () => {
  popUpVisible.value = true
}
// 场景表单规则
const configFormRules = ref({
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
const {
  loadingSelect,
  deviceGroupOptions,
  deviceOptions,
  queryDevice,
  getDevice,
  ensureDevicesLoaded,
  deviceConfigOption,
  getDeviceConfig,
  ensureDeviceConfigsLoaded,
  sceneList,
  getSceneList,
  ensureScenesLoaded,
  alarmList,
  getAlarmList,
  ensureAlarmsLoaded,
  hydrateActionTargetCatalogsForEcho
} = useLinkageActionTargetOptions()
const message = useMessage()
const { actionParamShow, actionParamTypeChange, actionParamChange, actionValueChange } =
  useLinkageActionParamState(message)
const {
  actionForm,
  actionOptions,
  actionTypeOptions,
  resetActionData,
  applyActionData,
  actionChange,
  actionTypeChange,
  addActionGroupItem,
  addAlarmActionSlot,
  deleteActionGroupItem,
  addIfGroupsSubItem,
  deleteIfGroupsSubItem
} = useLinkageActionGroupState({
  configFormRef,
  getConditionsType: () => props.conditionsType,
  hydrateActionParam: actionParamShow,
  preloadDevices: ensureDevicesLoaded,
  preloadDeviceConfigs: ensureDeviceConfigsLoaded
})

defineExpose({
  actionGroupsReturn,
  actionFormRefReturn,
  openCreateAlarm,
  addAlarmActionSlot
})

const normalizeCreatedAlarmOption = (alarm: any) => {
  const id = alarm?.id || alarm?.alarm_config_id || alarm?.alarmConfigId
  if (!id) return null
  return {
    ...alarm,
    id,
    name: alarm?.name || alarm?.alarm_config_name || alarm?.title || id
  }
}
const ensureCreatedAlarmOptionVisible = (alarm: any) => {
  const option = normalizeCreatedAlarmOption(alarm)
  if (!option) return null
  const exists = alarmList.value.some((item: any) => String(item?.id) === String(option.id))
  if (!exists) {
    alarmList.value = [option, ...alarmList.value]
  }
  return option
}
const applyCreatedAlarmToFirstEmptyTarget = (alarm: any) => {
  const option = ensureCreatedAlarmOptionVisible(alarm)
  if (!option) return false
  const targetGroup = actionForm.value.actionGroups.find((item: any) => {
    const actionType = String(item?.actionType || item?.action_type || '')
    return actionType === '30' && !item?.action_target
  })
  if (!targetGroup) return false
  targetGroup.action_target = option.id
  return true
}
const handleAlarmSaved = async (alarm: any) => {
  await getAlarmList('')
  applyCreatedAlarmToFirstEmptyTarget(alarm)
}

const queryDeviceName = ref<any[]>([])
const handleFocus = (ifIndex: any) => {
  queryDeviceName.value[ifIndex].focus()
}

// 选择动作目标
const actionTargetChange = (instructItem: any) => {
  instructItem.action_param_type = null
  instructItem.action_param = null
  instructItem.actionValue = null
  instructItem.actionParamOptionsData = []
  instructItem.actionParamTypeOptions = []
  instructItem.actionParamOptions = []
  actionParamShow(instructItem)
}

watch(
  () => props.conditionsType,
  (newValue) => {
    if (newValue) {
      resetActionData()
    }
  }
)
watch(
  () => props.actionData,
  async (newValue) => {
    await hydrateActionTargetCatalogsForEcho(newValue)
    applyActionData(newValue)
  }
)

onMounted(async () => {
  // A populated actionData is an edit echo even when the parent route does not
  // carry an id (for example embedded editors and API-driven forms).
  if (!configId.value && (!Array.isArray(props.actionData) || props.actionData.length === 0)) {
    addActionGroupItem()
  } else {
    await hydrateActionTargetCatalogsForEcho(props.actionData)
    applyActionData(props.actionData)
  }
})
</script>

<template>
  <div class="actions-box w-100%">
    <LinkageActionExecutionSummary
      :action-groups="actionForm.actionGroups"
      :device-options="deviceOptions"
      :device-config-options="deviceConfigOption"
      :scene-options="sceneList"
      :alarm-options="alarmList"
    />
    <NForm
      ref="configFormRef"
      :model="actionForm"
      :rules="configFormRules"
      label-placement="left"
      label-width="150"
      size="small"
      :show-feedback="false"
    >
      <NFlex vertical class="mt-1 w-100%">
        <NFlex
          v-for="(actionGroupItem, actionGroupIndex) in actionForm.actionGroups"
          :key="actionGroupIndex"
          class="mt-1 w-100%"
        >
          <NFormItem
            :show-label="false"
            :path="`actionGroups[${actionGroupIndex}].actionType`"
            :rule="configFormRules.actionType"
            class="w-100%"
          >
            <NFlex class="w-100%">
              <NSelect
                v-model:value="actionGroupItem.actionType"
                :options="actionOptions"
                class="max-w-40"
                @update:value="(data) => actionChange(actionGroupItem, actionGroupIndex, data)"
              />
              <template v-if="actionGroupItem.actionType === '1'">
                <!--          执行动作是操作设备->添加指令--->
                <NCard class="flex-1">
                  <NFlex
                    v-for="(instructItem, instructIndex) in actionGroupItem.actionInstructList"
                    :key="instructIndex"
                    class="mb-2 mr-30"
                  >
                    <template v-if="props.conditionsType !== '11'">
                      <NFormItem
                        :show-label="false"
                        :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].action_type`"
                        :rule="configFormRules.action_type"
                        class="w-40"
                      >
                        <NSelect
                          v-model:value="instructItem.action_type"
                          :options="actionTypeOptions"
                          class="max-w-40"
                          @update:value="(data) => actionTypeChange(instructItem, data)"
                        />
                      </NFormItem>
                    </template>
                    <template v-if="instructItem.action_type === '10'">
                      <!--                      选择单个设备-->
                      <NFormItem
                        :show-label="false"
                        :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].action_target`"
                        :rule="configFormRules.action_target"
                        class="w-40"
                      >
                        <NSelect
                          v-model:value="instructItem.action_target"
                          :options="deviceOptions"
                          value-field="id"
                          label-field="name"
                          :consistent-menu-width="false"
                          :loading="loadingSelect"
                          class="max-w-40"
                          @update:show="(show) => show && ensureDevicesLoaded()"
                          @update:value="() => actionTargetChange(instructItem)"
                        >
                          <template #header>
                            <NFlex align="center" class="w-500px">
                              {{ $t('generate.group') }}
                              <n-select
                                v-model:value="queryDevice.group_id"
                                :options="deviceGroupOptions"
                                label-field="name"
                                value-field="id"
                                class="max-w-40"
                                clearable
                                @update:value="(data) => getDevice(data, queryDevice.device_name)"
                              />
                              <NInput
                                ref="queryDeviceName"
                                v-model:value="queryDevice.device_name"
                                class="flex-1"
                                clearable
                                @click="handleFocus(instructIndex)"
                              ></NInput>
                              <NButton
                                type="primary"
                                @click.stop="getDevice(queryDevice.group_id, queryDevice.device_name)"
                              >
                                {{ $t('common.search') }}
                              </NButton>
                            </NFlex>
                          </template>
                        </NSelect>
                      </NFormItem>
                    </template>
                    <template v-if="instructItem.action_type === '11'">
                      <!--                      选择单类设备-->
                      <NFormItem
                        :show-label="false"
                        :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].action_target`"
                        :rule="configFormRules.action_target"
                        class="w-40"
                      >
                        <NSelect
                          v-model:value="instructItem.action_target"
                          :options="deviceConfigOption"
                          label-field="name"
                          value-field="id"
                          class="max-w-40"
                          :placeholder="$t('common.select')"
                          remote
                          filterable
                          @update:show="(show) => show && ensureDeviceConfigsLoaded()"
                          @search="getDeviceConfig"
                          @update:value="() => actionTargetChange(instructItem)"
                        />
                      </NFormItem>
                    </template>
                    <template v-if="instructItem.action_type">
                      <!--                      选择属性-->
                      <NFormItem
                        :show-label="false"
                        :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].action_param_type`"
                        :rule="configFormRules.action_param_type"
                        class="w-40"
                      >
                        <NSelect
                          v-model:value="instructItem.action_param_type"
                          :options="instructItem.actionParamTypeOptions"
                          class="max-w-40"
                          @update:value="(data) => actionParamTypeChange(instructItem, data)"
                        />
                      </NFormItem>
                      <NFormItem
                        v-if="instructItem.showSubSelect"
                        :show-label="false"
                        :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].action_param`"
                        :rule="configFormRules.action_param"
                        class="w-40"
                      >
                        <NSelect
                          v-model:value="instructItem.action_param"
                          :options="instructItem.actionParamOptions"
                          @update:value="(data) => actionParamChange(instructItem, data)"
                        />
                      </NFormItem>
                      <template v-if="instructItem.showSubSelect && instructItem.actionParamData">
                        <NFormItem
                          :show-label="false"
                          :show-feedback="instructItem.actionParamData?.data_type === 'boolean'"
                          :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].actionValue`"
                          :rule="configFormRules.actionValue"
                          :validation-status="instructItem.inputValidationStatus"
                          :feedback="instructItem.inputFeedback"
                          class="w-60"
                        >
                          <NInput
                            v-if="instructItem.actionParamData.data_type === 'string'"
                            v-model:value="instructItem.actionValue"
                            :placeholder="$t('common.as') + ': ' + (instructItem.placeholder || '--')"
                            class="w-full"
                            @blur="actionValueChange(instructItem)"
                          />
                          <n-input-number
                            v-if="instructItem.actionParamData && instructItem.actionParamData.data_type === 'number'"
                            v-model:value="instructItem.actionValue"
                            class="w-full"
                            :placeholder="$t('common.as') + ': ' + (instructItem.placeholder || '--')"
                            :show-button="false"
                          />
                          <n-radio-group
                            v-if="instructItem.actionParamData && instructItem.actionParamData.data_type === 'boolean'"
                            v-model:value="instructItem.actionValue"
                            name="radiogroup"
                          >
                            <n-space>
                              <n-radio :value="true">true</n-radio>
                              <n-radio :value="false">false</n-radio>
                            </n-space>
                          </n-radio-group>
                        </NFormItem>
                      </template>
                      <template v-if="!instructItem.showSubSelect">
                        <NFormItem
                          :show-label="false"
                          :show-feedback="false"
                          :path="`actionGroups[${actionGroupIndex}].actionInstructList[${instructIndex}].actionValue`"
                          :rule="configFormRules.actionValue"
                          :validation-status="instructItem.inputValidationStatus"
                          :feedback="instructItem.inputFeedback"
                          class="w-60"
                        >
                          <NInput
                            v-model:value="instructItem.actionValue"
                            :placeholder="$t('common.as') + ': ' + (instructItem.placeholder || '--')"
                            class="w-full"
                            @blur="actionValueChange(instructItem)"
                          />
                        </NFormItem>
                      </template>
                    </template>
                    <NButton
                      v-if="instructIndex === 0"
                      type="primary"
                      class="absolute right-5"
                      @click="addIfGroupsSubItem(actionGroupIndex)"
                    >
                      {{ $t('generate.add-operation') }}
                    </NButton>
                    <NButton
                      v-if="instructIndex !== 0"
                      type="error"
                      class="absolute right-5"
                      @click="deleteIfGroupsSubItem(actionGroupIndex, instructIndex)"
                    >
                      {{ $t('generate.delete-operation') }}
                    </NButton>
                  </NFlex>
                </NCard>
              </template>
              <template v-if="actionGroupItem.actionType === '20'">
                <!--          执行动作是激活场景->添加指令--->
                <NFlex class="ml-6" align="center">
                  <NFormItem
                    label-width="60"
                    :label="$t('generate.activate')"
                    :path="`actionGroups[${actionGroupIndex}].action_target`"
                    :rule="configFormRules.action_target"
                  >
                    <NSelect
                      v-model:value="actionGroupItem.action_target"
                      :options="sceneList"
                      label-field="name"
                      value-field="id"
                      :placeholder="$t('common.select')"
                      class="max-w-60"
                      :loading="loadingSelect"
                      filterable
                      remote
                      @update:show="(show) => show && ensureScenesLoaded()"
                      @search="getSceneList"
                    />
                  </NFormItem>
                </NFlex>
              </template>
              <template v-if="actionGroupItem.actionType === '30'">
                <!--          执行动作是触发告警->添加指令--->
                <NFlex class="ml-6">
                  <NFormItem
                    label-width="60"
                    :label="$t('generate.trigger')"
                    :path="`actionGroups[${actionGroupIndex}].action_target`"
                    :rule="configFormRules.action_target"
                  >
                    <NSelect
                      v-model:value="actionGroupItem.action_target"
                      :options="alarmList"
                      label-field="name"
                      value-field="id"
                      class="max-w-60"
                      :placeholder="$t('common.select')"
                      filterable
                      remote
                      :loading="loadingSelect"
                      @update:show="(show) => show && ensureAlarmsLoaded()"
                      @search="getAlarmList"
                    />
                  </NFormItem>
                  <NButton class="w-20" dashed type="info" @click="popUpVisible = true">
                    {{ $t('generate.create-alarm') }}
                  </NButton>
                </NFlex>
              </template>
              <NButton v-if="Number(actionGroupIndex) > 0" type="error" @click="deleteActionGroupItem(Number(actionGroupIndex))">
                {{ $t('generate.delete-execution-action') }}
              </NButton>
            </NFlex>
          </NFormItem>
        </NFlex>
        <NButton type="primary" class="w-30" @click="addActionGroupItem()">
          {{ $t('generate.add-execution-action') }}
        </NButton>
      </NFlex>
    </NForm>
    <PopUp
      v-model:visible="popUpVisible"
      type="add"
      :edit-data="null"
      @new-edit="newEdit"
      @saved="handleAlarmSaved"
    />
  </div>
</template>

<style scoped lang="scss">
:deep(.n-card__content) {
  padding: 10px 10px 4px 10px !important;
}
</style>
