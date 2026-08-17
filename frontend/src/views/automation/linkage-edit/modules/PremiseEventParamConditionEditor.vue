<script setup lang="ts">
type EventParamCondition = Record<string, any>

type Props = {
  ifItem: Record<string, any>
  eventExistsOptions: any[]
  getEventParamOptions: (ifItem: Record<string, any>) => any[]
  getEventOperatorOptions: (ifItem: Record<string, any>, condition: EventParamCondition) => any[]
}

defineProps<Props>()

const emit = defineEmits<{
  addCondition: []
  deleteCondition: [index: number]
  operatorChange: [condition: EventParamCondition]
}>()
</script>

<template>
  <NFlex vertical class="event-param-condition-section">
    <NFlex
      v-for="(condition, conditionIndex) in ifItem.eventParamConditions"
      :key="conditionIndex"
      align="center"
      class="event-param-condition-row"
    >
      <NFormItem :show-label="false" class="max-w-40 w-full">
        <NSelect
          v-model:value="condition.field"
          :options="getEventParamOptions(ifItem)"
          filterable
          tag
          clearable
          :placeholder="$t('common.param')"
        />
      </NFormItem>
      <NFormItem :show-label="false" class="max-w-35 w-full">
        <NSelect
          v-model:value="condition.operator"
          :options="getEventOperatorOptions(ifItem, condition)"
          @update:value="emit('operatorChange', condition)"
        />
      </NFormItem>
      <template v-if="condition.operator === 'exists'">
        <NFormItem :show-label="false" class="max-w-30 w-full">
          <NSelect v-model:value="condition.value" :options="eventExistsOptions" />
        </NFormItem>
      </template>
      <template v-else-if="condition.operator === 'between'">
        <NFormItem :show-label="false" class="max-w-30 w-full">
          <NInput v-model:value="condition.minValue" :placeholder="$t('generate.min-value')" />
        </NFormItem>
        <NFormItem :show-label="false" class="max-w-30 w-full">
          <NInput v-model:value="condition.maxValue" :placeholder="$t('generate.max-value')" />
        </NFormItem>
      </template>
      <template v-else>
        <NFormItem :show-label="false" class="max-w-40 w-full">
          <NInput
            v-model:value="condition.value"
            :placeholder="condition.operator === 'in' ? $t('generate.separated-by-commas') : $t('generate.value')"
          />
        </NFormItem>
      </template>
       <NButton quaternary type="error" @click="emit('deleteCondition', Number(conditionIndex))">
        {{ $t('common.delete') }}
      </NButton>
    </NFlex>
    <NFlex align="center">
      <NButton dashed size="small" @click="emit('addCondition')">
        {{ $t('common.add') }}{{ $t('common.param') }}
      </NButton>
      <NTag v-if="!ifItem.eventParamConditions?.length" type="warning" size="small">
        请至少添加一条事件参数条件
      </NTag>
    </NFlex>
  </NFlex>
</template>

<style scoped>
.event-param-condition-section,
.event-param-condition-row {
  width: 100%;
}
</style>
