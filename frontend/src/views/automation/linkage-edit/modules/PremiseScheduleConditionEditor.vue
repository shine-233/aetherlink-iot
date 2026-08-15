<script setup lang="ts">
import { AlertCircleOutline, RefreshOutline } from '@vicons/ionicons5'
import { $t } from '@/locales'
import { resetRepeatScheduleFields } from './premise-schedule-condition-state'

defineProps<{
  ifItem: any
  ifGroupIndex: number
  ifIndex: number
  premiseFormRules: any
  timeConditionOptions: any[]
  cycleOptions: any[]
  weekOptions: any[]
  expirationTimeOptions: any[]
  monthRangeOptions: any[]
}>()
</script>

<template>
  <NFlex class="flex-1">
    <NFormItem
      :show-label="false"
      :path="`ifGroups[${ifGroupIndex}][${ifIndex}].trigger_conditions_type`"
      :rule="premiseFormRules.trigger_conditions_type"
      class="max-w-25 w-full"
    >
      <NSelect
        v-model:value="ifItem.trigger_conditions_type"
        :options="timeConditionOptions"
        :placeholder="$t('common.select')"
        @update:value="ifItem.task_type = null"
      />
    </NFormItem>

    <template v-if="ifItem.trigger_conditions_type === '20'">
      <NFormItem
        :show-label="false"
        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].onceTimeValue`"
        :rule="premiseFormRules.onceTimeValue"
        class="max-w-40 w-full"
      >
        <n-date-picker
          v-model:value="ifItem.onceTimeValue"
          type="datetime"
          :time-picker-props="{ format: 'HH:mm' }"
          format="yyyy-MM-dd HH:mm"
          :placeholder="$t('generate.please-select-day-hour-minute')"
        />
      </NFormItem>
      <NFlex align="center">
        {{ $t('generate.not-executed') }}
        <NButton text class="refresh-class">
          <n-icon>
            <RefreshOutline />
          </n-icon>
        </NButton>
      </NFlex>
      <NFormItem
        :label="$t('generate.expiration-time')"
        label-width="80px"
        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].expiration_time`"
        :rule="premiseFormRules.expiration_time"
      >
        <NSelect
          v-model:value="ifItem.expiration_time"
          :options="expirationTimeOptions"
          :placeholder="$t('generate.please-select')"
          class="w-25"
        />
        <n-tooltip placement="top-start" trigger="hover">
          <template #trigger>
            <n-icon size="24" class="ml-2">
              <AlertCircleOutline />
            </n-icon>
          </template>
          {{ $t('generate.expiration-time') }}
          {{ expirationTimeOptions.find((data) => ifItem.expiration_time)?.label || '' }}
        </n-tooltip>
      </NFormItem>
    </template>

    <template v-if="ifItem.trigger_conditions_type === '21'">
      <NFormItem
        :show-label="false"
        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].task_type`"
        :rule="premiseFormRules.task_type"
        class="max-w-25 w-full"
      >
        <NSelect
          v-model:value="ifItem.task_type"
          :options="cycleOptions"
          :placeholder="$t('generate.please-select')"
          @update:value="() => resetRepeatScheduleFields(ifItem)"
        />
      </NFormItem>

      <template v-if="ifItem.task_type === 'HOUR'">
        <NFormItem
          key="hourTimeValue"
          :show-label="false"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].hourTimeValue`"
          :rule="premiseFormRules.hourTimeValue"
          class="max-w-25 w-full"
        >
          <NTimePicker v-model:value="ifItem.hourTimeValue" :placeholder="$t('common.select')" format="mm" />
        </NFormItem>
        <NFormItem
          key="expiration_time0"
          :label="$t('generate.expiration-time')"
          label-width="80px"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].expiration_time`"
          :rule="premiseFormRules.expiration_time"
        >
          <NSelect
            v-model:value="ifItem.expiration_time"
            :options="expirationTimeOptions"
            :placeholder="$t('generate.please-select')"
            class="w-25"
          />
          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <n-icon size="24" class="ml-2">
                <AlertCircleOutline />
              </n-icon>
            </template>
            {{ $t('generate.expiration-time') }}
            {{ expirationTimeOptions.find((data) => ifItem.expiration_time)?.label || '' }}
          </n-tooltip>
        </NFormItem>
      </template>

      <template v-if="ifItem.task_type === 'DAY'">
        <NFormItem
          key="dayTimeValue"
          :show-label="false"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].dayTimeValue`"
          :rule="premiseFormRules.dayTimeValue"
          class="max-w-25 w-full"
        >
          <NTimePicker
            v-model:value="ifItem.dayTimeValue"
            :placeholder="$t('common.select')"
            value-format="HH:mm"
            format="HH:mm"
          />
        </NFormItem>
        <NFormItem
          key="expiration_time1"
          :label="$t('generate.expiration-time')"
          label-width="80px"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].expiration_time`"
          :rule="premiseFormRules.expiration_time"
        >
          <NSelect
            v-model:value="ifItem.expiration_time"
            :options="expirationTimeOptions"
            :placeholder="$t('generate.please-select')"
            class="w-25"
          />
          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <n-icon size="24" class="ml-2">
                <AlertCircleOutline />
              </n-icon>
            </template>
            {{ $t('generate.expiration-time') }}
            {{ expirationTimeOptions.find((data) => ifItem.expiration_time)?.label || '' }}
          </n-tooltip>
        </NFormItem>
      </template>

      <template v-if="ifItem.task_type === 'WEEK'">
        <div class="weekChoseValue-box w-120">
          <NFormItem
            key="weekChoseValue"
            :show-label="false"
            :path="`ifGroups[${ifGroupIndex}][${ifIndex}].weekChoseValue`"
            :rule="premiseFormRules.weekChoseValue"
            :show-feedback="true"
            class="w-full"
          >
            <NCheckboxGroup v-model:value="ifItem.weekChoseValue">
              <NSpace item-style="display: flex;">
                <n-checkbox
                  v-for="(weekItem, weekIndex) in weekOptions"
                  :key="weekIndex"
                  :value="weekItem.value"
                  :label="weekItem.label"
                />
              </NSpace>
            </NCheckboxGroup>
          </NFormItem>
        </div>
        <NFormItem
          key="weekTimeValue"
          :show-label="false"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].weekTimeValue`"
          :rule="premiseFormRules.weekTimeValue"
          class="max-w-25 w-full"
        >
          <NTimePicker
            v-model:value="ifItem.weekTimeValue"
            :placeholder="$t('common.select')"
            value-format="HH:mm"
            format="HH:mm"
          />
        </NFormItem>
        <NFormItem
          key="expiration_time2"
          :label="$t('generate.expiration-time')"
          label-width="80px"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].expiration_time`"
          :rule="premiseFormRules.expiration_time"
        >
          <NSelect
            v-model:value="ifItem.expiration_time"
            :options="expirationTimeOptions"
            :placeholder="$t('generate.please-select')"
            class="w-25"
          />
          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <n-icon size="24" class="ml-2">
                <AlertCircleOutline />
              </n-icon>
            </template>
            {{ $t('generate.expiration-time') }}
            {{ expirationTimeOptions.find((data) => ifItem.expiration_time)?.label || '' }}
          </n-tooltip>
        </NFormItem>
      </template>

      <template v-if="ifItem.task_type === 'MONTH'">
        <NFormItem
          key="monthChoseValue"
          :show-label="false"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].monthChoseValue`"
          :rule="premiseFormRules.monthChoseValue"
          class="max-w-25 w-full"
        >
          <NSelect
            v-model:value="ifItem.monthChoseValue"
            :options="monthRangeOptions"
            :placeholder="$t('generate.please-select-date')"
          />
        </NFormItem>
        <NFormItem
          key="monthTimeValue"
          :show-label="false"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].monthTimeValue`"
          :rule="premiseFormRules.monthTimeValue"
          class="max-w-25 w-full"
        >
          <NTimePicker
            v-model:value="ifItem.monthTimeValue"
            :placeholder="$t('common.select')"
            value-format="HH:mm"
            format="HH:mm"
          />
        </NFormItem>
        <NFormItem
          key="expiration_time3"
          :label="$t('generate.expiration-time')"
          label-width="80px"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].expiration_time`"
          :rule="premiseFormRules.expiration_time"
        >
          <NSelect
            v-model:value="ifItem.expiration_time"
            :options="expirationTimeOptions"
            :placeholder="$t('generate.please-select')"
            class="w-25"
          />
          <n-tooltip placement="top-start" trigger="hover">
            <template #trigger>
              <n-icon size="24" class="ml-2">
                <AlertCircleOutline />
              </n-icon>
            </template>
            {{ $t('generate.expiration-time') }}
            {{ expirationTimeOptions.find((data) => ifItem.expiration_time)?.label || '' }}
          </n-tooltip>
        </NFormItem>
      </template>
    </template>

    <template v-if="ifItem.trigger_conditions_type === '22'">
      <div class="weekChoseValue-box w-120">
        <NFormItem
          :show-label="false"
          :path="`ifGroups[${ifGroupIndex}][${ifIndex}].weekChoseValue`"
          :rule="premiseFormRules.weekChoseValue"
          :show-feedback="true"
          class="w-full"
        >
          <NCheckboxGroup v-model:value="ifItem.weekChoseValue">
            <NSpace item-style="display: flex;">
              <NCheckbox
                v-for="(weekItem, weekIndex) in weekOptions"
                :key="weekIndex"
                :value="weekItem.value"
                :label="weekItem.label"
              />
            </NSpace>
          </NCheckboxGroup>
        </NFormItem>
      </div>
      <NFormItem
        :show-label="false"
        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].startTimeValue`"
        :rule="premiseFormRules.startTimeValue"
        class="max-w-25 w-full"
      >
        <NTimePicker
          v-model:value="ifItem.startTimeValue"
          :placeholder="$t('common.select')"
          value-format="HH:mm:ss"
          format="HH:mm:ss"
        />
      </NFormItem>
      -
      <NFormItem
        :show-label="false"
        :path="`ifGroups[${ifGroupIndex}][${ifIndex}].endTimeValue`"
        :rule="premiseFormRules.endTimeValue"
        class="max-w-25 w-full"
      >
        <NTimePicker
          v-model:value="ifItem.endTimeValue"
          :placeholder="$t('common.select')"
          value-format="HH:mm:ss"
          format="HH:mm:ss"
        />
      </NFormItem>
    </template>
  </NFlex>
</template>

<style scoped>
.refresh-class {
  font-size: 24px;
}

.weekChoseValue-box {
  :deep(.n-form-item-feedback-wrapper) {
    position: absolute;
    top: 20px;
  }
}
</style>
