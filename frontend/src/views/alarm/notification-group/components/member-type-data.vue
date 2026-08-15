<!--
文件用途：提供 告警通知组管理 页面内的 member-type-data 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { FormItemRule } from 'naive-ui'
import { MemberNotificationOptions } from '@/constants/business'
import { createRequiredFormRule } from '@/utils/form/rule'
import { $t } from '@/locales'
import {
  getCurrentName,
  handleDeleteMember,
  handleScroll,
  handleSearch,
  handleUpdateMember,
  memberOptionsLoading,
  notificationTypeOptions
} from '../utils'

type FormModel = Pick<DataService.Data, any>

const formModel = reactive<FormModel>(createDefaultFormModel())

function createDefaultFormModel(): FormModel {
  return {
    name: '',
    signMode: null
  }
}

const rules: Record<keyof FormModel, FormItemRule | FormItemRule[]> = {
  name: createRequiredFormRule($t('generate.ruleName')),
  signMode: createRequiredFormRule($t('generate.signatureMethod')),
  ip: createRequiredFormRule($t('generate.IPwhitelist'))
}

const props = withDefaults(
  defineProps<{
    index: number
    selectedNotificationType: string[]
  }>(),
  {
    index: 0,
    notificationType: []
  }
)
const selectedMember = ref(getCurrentName(props.index))
const selectNotificationType = ref(props.selectedNotificationType)

const handleDelete = (index: number) => {
  handleDeleteMember(index)
}

const handleUpdate = () => {
  handleUpdateMember(props.index, { name: selectedMember.value || '', notificationType: selectNotificationType.value })
}

const handleChange = () => {
  handleUpdate()
}
</script>

<template>
  <NForm ref="formRef" label-placement="left" :label-width="120" :model="formModel" :rules="rules">
    <NFormItem path="name">
      <NSelect
        v-model:value="selectedMember"
        :placeholder="$t('generate.select-user')"
        :options="notificationTypeOptions"
        clearable
        remote
        filterable
        :loading="memberOptionsLoading"
        style="width: 240px; margin-right: 16px"
        @search="handleSearch"
        @scroll="handleScroll"
        @update:value="handleChange"
      />
      <NCheckboxGroup v-model:value="selectNotificationType" @click="handleUpdate">
        <NSpace item-style="display: flex;">
          <NCheckbox
            v-for="item in MemberNotificationOptions"
            :key="item.value"
            :value="item.value"
            :label="item.label"
          >
            {{ item.label }}
          </NCheckbox>
        </NSpace>
      </NCheckboxGroup>
      <NButton type="error" size="small" style="margin-left: 12px" @click="() => handleDelete(index)">
        {{ $t('common.delete') }}
      </NButton>
    </NFormItem>
  </NForm>
</template>

<style lang="scss" scoped></style>
