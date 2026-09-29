<!--
  文件用途: 设备详情页“修改设备信息”弹窗。
  核心逻辑: 直接编辑父级 deviceData 的 name / device_number / description 与标签列表；取消与保存动作交回父级。
  关键注意事项: 校验与提交（validateDeviceUpdate / deviceUpdate）仍在详情页壳层，弹窗不直接调用接口。
-->
<script setup lang="ts">
import { $t } from '@/locales'
import type { DeviceDetailData } from '../device-edit-state'

defineProps<{
  deviceData: DeviceDetailData
  compact: boolean
}>()

defineEmits<{
  cancel: []
  save: []
}>()

const show = defineModel<boolean>('show', { required: true })
const labels = defineModel<string[]>('labels', { required: true })

const rules = {
  name: { required: true, message: $t('custom.devicePage.enterDeviceName'), trigger: 'blur' },
  device_number: { required: true, message: $t('custom.devicePage.enterDeviceNumber'), trigger: 'blur' }
}
</script>

<template>
  <n-modal
    v-model:show="show"
    aria-label="dialog"
    :title="$t('generate.issue-attribute')"
    :class="compact ? 'w-90%' : 'w-400px'"
  >
    <n-card>
      <n-form :model="deviceData" :rules="rules">
        <div>
          <NH3>{{ $t('generate.modify-device-info') }}</NH3>
        </div>
        <n-form-item :label="$t('custom.devicePage.deviceName')" path="name">
          <n-input v-model:value="deviceData.name" aria-required="true" />
        </n-form-item>
        <n-form-item :label="$t('generate.device-code')" path="device_number">
          <n-input v-model:value="deviceData.device_number" />
        </n-form-item>
        <n-form-item :label="$t('custom.devicePage.label')" path="label">
          <n-dynamic-tags v-model:value="labels" />
        </n-form-item>
        <n-form-item :label="$t('generate.device-description')">
          <NInput v-model:value="deviceData.description" type="textarea" />
        </n-form-item>
        <n-space>
          <n-button @click="$emit('cancel')">{{ $t('generate.cancel') }}</n-button>
          <n-button @click="$emit('save')">{{ $t('common.save') }}</n-button>
        </n-space>
      </n-form>
    </n-card>
  </n-modal>
</template>
