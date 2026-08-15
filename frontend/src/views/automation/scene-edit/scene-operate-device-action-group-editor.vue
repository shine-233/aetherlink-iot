<script setup lang="ts">
import { ref, toRefs } from 'vue'
import { useMessage } from 'naive-ui'
import { $t } from '@/locales'
import {
  ACTION_PARAM_TYPES_WITH_JSON_VALIDATION,
  SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE,
  SINGLE_DEVICE_ACTION_TARGET_TYPE,
  type SceneActionGroupLike,
  type SceneInstructionLike
} from './scene-action-mappers'
import {
  applyActionParamSelection,
  applyActionParamTypeChange,
  clearActionValueValidationState,
  markInvalidJsonActionValue,
  validateJsonActionValue
} from './scene-action-form-state'
import type { SelectOption } from './useSceneActionTargetCatalog'

type DeviceQueryModel = {
  group_id: string | null
  device_name: string | null
  bind_config: number
}

const props = defineProps<{
  actionGroupItem: SceneActionGroupLike
  actionGroupIndex: number
  actionTypeOptions: Array<{ label: string; value: string }>
  configFormRules: Record<string, any>
  deviceConfigOption: SelectOption[]
  deviceGroupOptions: SelectOption[]
  deviceOptions: SelectOption[]
  loadingSelect: boolean
  queryDevice: DeviceQueryModel
  createInstruction: () => SceneInstructionLike
  getDevice: (groupId: string | null, name: string | null) => void | Promise<void>
  getDeviceConfig: (name: string | null) => void | Promise<void>
  ensureDeviceConfigOptionsLoaded: () => void
  ensureDeviceGroupsLoaded: () => void
  ensureDeviceOptionsLoaded: () => void
  ensureDeviceTargetCatalogsLoaded: () => void
  actionTargetChange: (instructItem: SceneInstructionLike) => void
  actionTypeChange: (instructItem: SceneInstructionLike, actionType: string | null) => void
}>()

const {
  actionGroupItem,
  actionGroupIndex,
  actionTypeOptions,
  configFormRules,
  deviceConfigOption,
  deviceGroupOptions,
  deviceOptions,
  loadingSelect,
  queryDevice
} = toRefs(props)

const queryDeviceName = ref<Array<{ focus: () => void } | null>>([])
const handleFocus = (ifIndex: number) => {
  queryDeviceName.value[ifIndex]?.focus?.()
}

const actionParamTypeChange = (instructItem: SceneInstructionLike, data: string) => {
  applyActionParamTypeChange(instructItem, data)
}

const actionParamChange = (instructItem: SceneInstructionLike, data: string) => {
  applyActionParamSelection(instructItem, data)
}

const message = useMessage()

const setActionValueJsonError = (instructItem: SceneInstructionLike) => {
  message.error($t('common.enterJson'))
  markInvalidJsonActionValue(instructItem, $t('common.enterJson'))
}

const actionValueChange = (instructItem: SceneInstructionLike) => {
  if (ACTION_PARAM_TYPES_WITH_JSON_VALIDATION.has(instructItem.action_param_type as string)) {
    if (validateJsonActionValue(instructItem.action_param_type, instructItem.actionValue)) {
      clearActionValueValidationState(instructItem)
    } else {
      setActionValueJsonError(instructItem)
    }
  }
}

const addIfGroupsSubItem = () => {
  props.actionGroupItem.actionInstructList.push(props.createInstruction())
}

const deleteIfGroupsSubItem = (ifIndex: number) => {
  props.actionGroupItem.actionInstructList.splice(ifIndex, 1)
}
</script>

<template>
  <NCard class="flex-1">
    <NFlex
      v-for="(instructItem, instructIndex) in actionGroupItem.actionInstructList"
      :key="instructIndex"
      class="mb-2 mr-30"
    >
      <NFormItem
        :show-label="false"
        :show-feedback="false"
        :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].action_type`"
        :rule="configFormRules.action_type"
        class="max-w-30 w-full"
      >
        <NSelect
          v-model:value="instructItem.action_type"
          :options="actionTypeOptions"
          @update:value="(data) => props.actionTypeChange(instructItem, data)"
        />
      </NFormItem>
      <template v-if="instructItem.action_type === SINGLE_DEVICE_ACTION_TARGET_TYPE">
        <NFormItem
          :show-label="false"
          :show-feedback="false"
          :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].action_target`"
          :rule="configFormRules.action_target"
          class="max-w-40 w-full"
        >
          <NSelect
            v-model:value="instructItem.action_target"
            :options="deviceOptions"
            value-field="id"
            label-field="name"
            :consistent-menu-width="false"
            :loading="loadingSelect"
            @update:value="() => props.actionTargetChange(instructItem)"
            @update:show="(show) => show && props.ensureDeviceTargetCatalogsLoaded()"
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
                  @update:show="(show) => show && props.ensureDeviceGroupsLoaded()"
                  @update:value="(data) => props.getDevice(data, queryDevice.device_name)"
                />
                <NInput
                  ref="queryDeviceName"
                  v-model:value="queryDevice.device_name"
                  class="flex-1"
                  clearable
                  autofocus
                  @click="handleFocus(instructIndex)"
                ></NInput>
                <NButton type="primary" @click.stop="props.getDevice(queryDevice.group_id, queryDevice.device_name)">
                  {{ $t('common.search') }}
                </NButton>
              </NFlex>
            </template>
          </NSelect>
        </NFormItem>
      </template>
      <template v-if="instructItem.action_type === SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE">
        <NFormItem
          :show-label="false"
          :show-feedback="false"
          :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].action_target`"
          :rule="configFormRules.action_target"
          class="max-w-40 w-full"
        >
          <NSelect
            v-model:value="instructItem.action_target"
            :options="deviceConfigOption"
            label-field="name"
            value-field="id"
            :placeholder="$t('common.select')"
            remote
            filterable
            @search="props.getDeviceConfig"
            @update:value="() => props.actionTargetChange(instructItem)"
            @update:show="(show) => show && props.ensureDeviceConfigOptionsLoaded()"
          />
        </NFormItem>
      </template>
      <template v-if="instructItem.action_type">
        <NFormItem
          :show-label="false"
          :show-feedback="false"
          :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].action_param_type`"
          :rule="configFormRules.action_param_type"
          class="max-w-30 w-full"
        >
          <NSelect
            v-model:value="instructItem.action_param_type"
            :options="(instructItem.actionParamTypeOptions as any)"
            @update:value="(data) => actionParamTypeChange(instructItem, data)"
          />
        </NFormItem>
        <NFormItem
          v-if="instructItem.showSubSelect"
          :show-label="false"
          :show-feedback="false"
          :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].action_param`"
          :rule="configFormRules.action_param"
          class="max-w-40 w-full"
        >
          <NSelect
            v-model:value="instructItem.action_param"
            :options="instructItem.actionParamOptions"
            @update:value="(data) => actionParamChange(instructItem, data)"
          />
        </NFormItem>
        <NFormItem
          v-if="instructItem.showSubSelect && instructItem.actionParamData"
          :show-label="false"
          :show-feedback="false"
          :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].actionValue`"
          :rule="configFormRules.actionValue"
          :validation-status="(instructItem.inputValidationStatus as any)"
          :feedback="instructItem.inputFeedback"
          class="max-w-60 w-full"
        >
          <NInput
            v-if="instructItem.actionParamData.data_type === 'string'"
            v-model:value="instructItem.actionValue"
            :placeholder="$t('common.as') + '：' + instructItem.placeholder"
            class="w-full"
            @blur="actionValueChange(instructItem)"
          />
          <n-input-number
            v-if="instructItem.actionParamData.data_type === 'number'"
            v-model:value="instructItem.actionValue"
            class="w-full"
            :placeholder="$t('common.as') + '：' + instructItem.placeholder"
            :show-button="false"
          />
          <n-radio-group
            v-if="instructItem.actionParamData.data_type === 'boolean'"
            v-model:value="instructItem.actionValue"
            name="radiogroup"
          >
            <n-space>
              <n-radio :value="true">true</n-radio>
              <n-radio :value="false">false</n-radio>
            </n-space>
          </n-radio-group>
        </NFormItem>
        <NFormItem
          v-if="!instructItem.showSubSelect"
          :show-label="false"
          :show-feedback="false"
          :path="`actions[${actionGroupIndex}].actionInstructList[${instructIndex}].actionValue`"
          :rule="configFormRules.actionValue"
          :validation-status="(instructItem.inputValidationStatus as any)"
          :feedback="instructItem.inputFeedback"
          class="w-60"
        >
          <NInput
            v-model:value="instructItem.actionValue"
            :placeholder="$t('common.as') + '：' + instructItem.placeholder"
            class="w-full"
            @blur="actionValueChange(instructItem)"
          />
        </NFormItem>
      </template>
      <NButton v-if="instructIndex === 0" type="primary" class="absolute right-5" @click="addIfGroupsSubItem">
        {{ $t('generate.add-row') }}
      </NButton>
      <NButton
        v-if="instructIndex !== 0"
        type="error"
        class="absolute right-5"
        @click="deleteIfGroupsSubItem(instructIndex)"
      >
        {{ $t('common.delete') }}
      </NButton>
    </NFlex>
  </NCard>
</template>
