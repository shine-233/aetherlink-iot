<!--
  Automation premise editor: keeps condition type, source, parameter, time-window,
  and edit-state wiring together while delegating source-specific option loading
  to helper modules.
-->
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NFlex, useMessage } from 'naive-ui'
import type { FormInst } from 'naive-ui'
import { deviceGroupTree } from '@/service/api'
import {
  configMetricsConditionMenu,
  deviceConfigAll,
  deviceListAll,
  deviceMetricsConditionMenu
} from '@/service/api/automation'
import { $t } from '@/locales'
import { useI18n } from 'vue-i18n'
import { createPremiseDeviceConditionState } from './premise-device-condition-state'
import {
  applyTriggerParamSelectionChange,
  handleTriggerParamOptionsShow,
  loadPremiseTriggerParamOptions
} from './premise-trigger-lifecycle'
import {
  applyTriggerParamSelectionState,
  commitSelectedTriggerParam,
  normalizeIfItemForEcho,
  validateEventTriggerJsonValue
} from './premise-trigger-param-state'
import {
  addEventParamCondition,
  buildEventExistsOptions,
  deleteEventParamCondition,
  eventConditionOperatorChange,
  getEventOperatorOptions as resolveEventOperatorOptions,
  getEventParamOptions,
  syncSelectedEventParams
} from './premise-event-param-conditions'
import { createPremiseLocalizedConditionOptions } from './premise-localized-condition-options'
import { createPremiseConditionGroupsState } from './premise-condition-groups-state'
import PremiseEventParamConditionEditor from './PremiseEventParamConditionEditor.vue'
import PremiseScheduleConditionEditor from './PremiseScheduleConditionEditor.vue'

interface Emits {
  (e: 'conditionChose', data: any): void
}

const route = useRoute()
const emit = defineEmits<Emits>()
const { locale } = useI18n()

const premiseFormRef = ref<FormInst | null>(null)
const premiseForm = ref<any>({
  ifGroups: []
})
// Premise form validation rules.
const premiseFormRules = ref({
  ifType: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  trigger_conditions_type: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  trigger_source: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  trigger_param: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  trigger_operator: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  trigger_value: {
    required: true,
    message: $t('common.input'),
    trigger: 'blur'
  },
  minValue: {
    required: true,
    message: $t('common.input'),
    trigger: 'blur'
  },
  maxValue: {
    required: true,
    message: $t('common.input'),
    trigger: 'blur'
  },
  onceTimeValue: {
    required: true,
    message: $t('common.select')
  },
  expiration_time: {
    required: true,
    message: $t('common.select')
  },
  task_type: {
    required: true,
    message: $t('common.select'),
    trigger: 'change'
  },
  hourTimeValue: {
    required: true,
    message: $t('common.select')
  },
  dayTimeValue: {
    required: true,
    message: $t('common.select')
  },
  weekChoseValue: {
    required: true,
    message: $t('common.select')
  },
  weekTimeValue: {
    required: true,
    message: $t('common.select')
  },
  monthChoseValue: {
    required: true,
    message: $t('common.select')
  },
  monthTimeValue: {
    required: true,
    message: $t('common.select')
  },
  startTimeValue: {
    required: true,
    message: $t('common.select')
  },
  endTimeValue: {
    required: true,
    message: $t('common.select')
  }
})

const getIfTypeOptions = (ifGroup, ifIndex) => {
  return [
    {
      label: $t('common.deviceConditions'),
      value: '1',
      disabled: ifGroup.some((item) => {
        return (item.trigger_conditions_type === '20' || item.trigger_conditions_type === '21') && ifIndex > 0
      })
    },
    {
      label: $t('common.timeConditions'),
      value: '2'
    }
  ]
}
const ifTypeChange = (ifItem: any, data: any) => {
  ifItem.trigger_conditions_type = null
  ifItem = judgeItem.value
  ifItem.ifType = data
}

// Reset derived device trigger state when type or source changes.
const resetDeviceTriggerSelection = (ifItem: any) => {
  applyTriggerParamSelectionState(ifItem, {}, { resetComparatorState: true })
}

const {
  btnloading,
  deviceConditionOptions,
  deviceConfigDisabled,
  deviceConfigOption,
  deviceGroupOptions,
  deviceOptions,
  ensureDeviceConfigsLoaded,
  ensureDevicesLoaded,
  getDevice,
  getDeviceConfig,
  handleFocus,
  onDeviceKeydownEnter,
  onKeydownEnter,
  onTapInput,
  queryDevice,
  queryDeviceConfig,
  setQueryDeviceNameRef,
  triggerConditionsTypeChange,
  triggerSourceChange
} = createPremiseDeviceConditionState({
  t: $t,
  resetTriggerSelection: resetDeviceTriggerSelection,
  deviceGroupTreeRequest: deviceGroupTree,
  deviceListRequest: deviceListAll,
  deviceConfigRequest: deviceConfigAll,
  emitConditionChose: (data) => emit('conditionChose', data)
})

// Device source search/switching lives in helpers; parameter options load on demand.
const loadTriggerParamOptions = async (ifItem: any) => {
  await loadPremiseTriggerParamOptions(ifItem, {
    deviceMetricsConditionMenu: deviceMetricsConditionMenu as (payload: Record<string, any>) => Promise<{ data?: any[] } | null | undefined>,
    configMetricsConditionMenu: configMetricsConditionMenu as (payload: Record<string, any>) => Promise<{ data?: any[] } | null | undefined>,
    statusOption: statusData.value,
    syncSelectedEventParams,
    onError: (error) => {
      console.error('Failed to load trigger param options', error)
    }
  })
}

const actionParamShow = async (ifItem: any, data: any) => {
  await handleTriggerParamOptionsShow(ifItem, data === true, loadTriggerParamOptions)
}

const {
  getTimeConditionOptions,
  statusData,
  cycleOptions,
  weekOptions,
  determineOptions,
  expirationTimeOptions,
  monthRangeOptions
} = createPremiseLocalizedConditionOptions($t)

const message = useMessage()

// Validate event-condition JSON immediately so users see errors before submit.
const actionValueChange = (ifItem: any) => {
  if (ifItem.trigger_param_type === 'event') {
    try {
      if (validateEventTriggerJsonValue(ifItem.trigger_value)) {
        ifItem.inputFeedback = ''
        ifItem.inputValidationStatus = undefined
      } else {
        message.error($t('common.enterJson'))
        ifItem.inputValidationStatus = 'error'
      }
    } catch (e) {
      message.error($t('common.enterJson'))
      ifItem.inputValidationStatus = 'error'
    }
  }
}

const eventExistsOptions = computed(() => buildEventExistsOptions())

const getEventOperatorOptions = (ifItem: any, condition: any) => {
  return resolveEventOperatorOptions(ifItem, condition, determineOptions.value)
}

const triggerParamChange = (ifItem: any, data: any) => {
  applyTriggerParamSelectionChange(ifItem, data, commitSelectedTriggerParam)
}

interface Props {
  conditionData?: any[]
  device_id?: string
  device_config_id?: string
}

const props = withDefaults(defineProps<Props>(), {
  conditionData: () => [],
  device_id: '',
  device_config_id: ''
})
const configId = ref(route.query.id || null)

const {
  judgeItem,
  addIfGroupsSubItem,
  deleteIfGroupsSubItem,
  deleteIfGroupsItem,
  addConditionGroup,
  addIfGroupItem,
  ifGroupsData,
  premiseFormRefReturn
} = createPremiseConditionGroupsState({
  premiseForm,
  premiseFormRef,
  props,
  hasConfigId: Boolean(configId.value),
  locale,
  deviceConfigDisabled,
  statusData,
  ensureDevicesLoaded,
  ensureDeviceConfigsLoaded,
  loadTriggerParamOptions,
  normalizeIfItemForEcho,
  emitConditionChose: (data) => emit('conditionChose', data)
})

defineExpose({
  ifGroupsData,
  addConditionGroup,
  premiseFormRefReturn
})
</script>
<template>
  <NFlex vertical class="mt-1 w-100%">
    <NForm
      ref="premiseFormRef"
      :model="premiseForm"
      :rules="premiseFormRules"
      :submit-on-enter="false"
      label-placement="left"
      size="small"
      :show-feedback="false"
      @keydown.enter="onKeydownEnter"
    >
      {{ $t('generate.condition-trigger') }}
      <NFlex v-for="(ifGroupItem, ifGroupIndex) in premiseForm.ifGroups" :key="ifGroupIndex" class="w-100%">
        <NCard class="mb-2 w-[calc(100%-78px)]">
          <NFlex v-for="(ifItem, ifIndex) in ifGroupItem" :key="ifIndex" class="ifGroupItem-class mb-2 w-100%">
            <NFlex class="flex-1" align="center">
              <NTag v-if="ifIndex !== 0" type="success" class="tag-class" size="small">{{ $t('generate.and') }}</NTag>
              <!-- Condition type selection. -->
              <NFormItem
                :show-label="false"
                :path="`ifGroups[${ifGroupIndex}][${ifIndex}].ifType`"
                :rule="premiseFormRules.ifType"
                class="ml-10 max-w-25 w-full"
              >
                <NSelect
                  v-model:value="ifItem.ifType"
                  :options="getIfTypeOptions(ifGroupItem, ifIndex)"
                  :placeholder="$t('common.select')"
                  @update-value="(data) => ifTypeChange(ifItem, data)"
                />
              </NFormItem>
              <NFlex v-if="ifItem.ifType === '1'" class="flex-1">
                <!-- Condition type selection. -->
                <NFormItem
                  :show-label="false"
                  :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_conditions_type`"
                  :rule="premiseFormRules.trigger_conditions_type"
                  class="max-w-25 w-full"
                >
                  <NSelect
                    v-model:value="ifItem.trigger_conditions_type"
                    :options="deviceConditionOptions"
                    :placeholder="$t('common.select')"
                    clearable
                    @update:value="(data) => triggerConditionsTypeChange(ifItem, data)"
                  />
                </NFormItem>
                <template v-if="ifItem.trigger_conditions_type === '10'">
                  <NFormItem
                    :show-label="false"
                    :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_source`"
                    :rule="premiseFormRules.trigger_source"
                    class="max-w-40 w-full"
                  >
                    <NSelect
                      v-model:value="ifItem.trigger_source"
                      :options="deviceOptions"
                      value-field="id"
                      label-field="name"
                      clearable
                      :consistent-menu-width="false"
                      @update:show="(show) => show && ensureDevicesLoaded()"
                      @click.prevent="
                        (e) => {
                          onDeviceKeydownEnter(e, ifIndex)
                        }
                      "
                      @keydown.enter="
                        (e) => {
                          onDeviceKeydownEnter(e, ifIndex)
                        }
                      "
                      @update:value="() => triggerSourceChange(ifItem, ifIndex)"
                    >
                      <template #header>
                        <NFlex align="center" class="w-500px">
                          {{ $t('generate.group') }}
                          <NSelect
                            v-model:value="queryDevice.group_id"
                            :options="deviceGroupOptions"
                            label-field="name"
                            value-field="id"
                            class="max-w-40"
                            clearable
                            :placeholder="$t('common.select')"
                            @keydown.enter="onKeydownEnter"
                            @update:value="(data) => getDevice(data, queryDevice.device_name)"
                          />
                          <NInput
                            :ref="(el) => setQueryDeviceNameRef(el, ifIndex)"
                            v-model:value="queryDevice.device_name"
                            class="flex-1"
                            clearable
                            :placeholder="$t('common.input')"
                            @keydown.enter="onTapInput(queryDevice, ifIndex)"
                            @click="handleFocus(ifIndex)"
                          ></NInput>
                          <NButton
                            :disabled="!btnloading"
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
                <template v-if="ifItem.trigger_conditions_type === '11'">
                  <!-- Condition type selection. -->
                  <NFormItem
                    :show-label="false"
                    :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_source`"
                    :rule="premiseFormRules.trigger_source"
                    class="max-w-40 w-full"
                  >
                    <NSelect
                      v-model:value="ifItem.trigger_source"
                      :options="deviceConfigOption"
                      label-field="name"
                      value-field="id"
                      :placeholder="$t('common.select')"
                      remote
                      filterable
                      @update:show="(show) => show && ensureDeviceConfigsLoaded()"
                      @search="getDeviceConfig"
                      @update:value="() => triggerSourceChange(ifItem, ifIndex)"
                    />
                  </NFormItem>
                </template>
                <template v-if="ifItem.trigger_source">
                  <!-- Trigger parameter selection. -->
                  <NFormItem
                    :show-label="false"
                    :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_param`"
                    :rule="premiseFormRules.trigger_param"
                    class="max-w-40 w-full"
                  >
                    <NCascader
                      v-model:value="ifItem.trigger_param_key"
                      :placeholder="$t('common.select')"
                      :options="ifItem.triggerParamOptions"
                      check-strategy="child"
                      children-field="options"
                      size="small"
                      @update:show="(data) => actionParamShow(ifItem, data)"
                      @update:value="(value, option, pathValues) => triggerParamChange(ifItem, pathValues)"
                    />
                  </NFormItem>
                  <template
                    v-if="ifItem.trigger_param_type === 'telemetry' || ifItem.trigger_param_type === 'attributes'"
                  >
                    <!-- Comparator selection for telemetry or attribute conditions. -->
                    <NFormItem
                      :show-label="false"
                      :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_operator`"
                      :rule="premiseFormRules.trigger_operator"
                      class="max-w-35 w-full"
                    >
                      <NSelect v-model:value="ifItem.trigger_operator" :options="determineOptions" />
                    </NFormItem>
                    <template v-if="ifItem.trigger_operator === 'in'">
                      <!-- List value input. -->
                      <NFormItem
                        :show-label="false"
                        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_value`"
                        :rule="premiseFormRules.trigger_value"
                        class="max-w-50 w-full"
                      >
                        <NInput
                          v-model:value="ifItem.trigger_value"
                          :placeholder="$t('generate.separated-by-commas')"
                        />
                      </NFormItem>
                    </template>
                    <template v-else-if="ifItem.trigger_operator == 'between'">
                      <!-- Range value input. -->
                      <NFormItem
                        :show-label="false"
                        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].minValue`"
                        :rule="premiseFormRules.minValue"
                        class="max-w-35 w-full"
                      >
                        <NInput v-model:value="ifItem.minValue" :placeholder="$t('generate.min-value')" />
                      </NFormItem>
                      <NFormItem
                        :show-label="false"
                        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].maxValue`"
                        :rule="premiseFormRules.maxValue"
                        class="max-w-30 w-full"
                      >
                        <NInput v-model:value="ifItem.maxValue" :placeholder="$t('generate.max-value')" />
                      </NFormItem>
                    </template>
                    <template v-else>
                      <!-- Single value input. -->
                      <NFormItem
                        :show-label="false"
                        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_value`"
                        :rule="premiseFormRules.trigger_value"
                        class="max-w-40 w-full"
                      >
                        <NInput v-model:value="ifItem.trigger_value" :placeholder="$t('generate.value')" />
                      </NFormItem>
                    </template>
                  </template>
                  <template v-if="ifItem.trigger_param_type === 'event'">
                    <PremiseEventParamConditionEditor
                      :if-item="ifItem"
                      :event-exists-options="eventExistsOptions"
                      :get-event-param-options="getEventParamOptions"
                      :get-event-operator-options="getEventOperatorOptions"
                      @add-condition="addEventParamCondition(ifItem)"
                      @delete-condition="(conditionIndex) => deleteEventParamCondition(ifItem, conditionIndex)"
                      @operator-change="eventConditionOperatorChange"
                    />
                  </template>
                  <template v-if="ifItem.trigger_param_type === 'status'">
                    <!-- Status conditions have no extra input. -->
                  </template>
                </template>
              </NFlex>
              <PremiseScheduleConditionEditor
                v-if="ifItem.ifType === '2'"
                :if-item="ifItem"
                :if-group-index="ifGroupIndex"
                :if-index="ifIndex"
                :premise-form-rules="premiseFormRules"
                :time-condition-options="getTimeConditionOptions(ifGroupItem)"
                :cycle-options="cycleOptions"
                :week-options="weekOptions"
                :expiration-time-options="expirationTimeOptions"
                :month-range-options="monthRangeOptions"
              />
            </NFlex>
            <NFlex class="w-100px">
              <NButton
                v-if="ifIndex === 0"
                type="primary"
                class="absolute right-0"
                @click="addIfGroupsSubItem(ifGroupIndex)"
              >
                {{ $t('generate.add-condition') }}
              </NButton>
              <NButton
                v-if="ifIndex !== 0"
                type="error"
                class="absolute right-0"
                @click="deleteIfGroupsSubItem(ifGroupIndex, ifIndex)"
              >
                {{ $t('generate.delete-condition') }}
              </NButton>
            </NFlex>
          </NFlex>
        </NCard>
        <NButton v-if="ifGroupIndex > 0" type="error" class="relative" @click="deleteIfGroupsItem(ifGroupIndex)">
          {{ $t('generate.delete-group') }}
        </NButton>
      </NFlex>
    </NForm>
    <NButton type="primary" class="w-30" @click="addIfGroupItem(null)">{{ $t('generate.add-group') }}</NButton>
  </NFlex>
</template>

<style scoped>
.ifGroupItem-class {
  position: relative;

  .tag-class {
    position: absolute;
    top: 5px;
  }
}

:deep(.n-card__content) {
  padding: 10px 10px 4px 10px !important;
}

</style>
