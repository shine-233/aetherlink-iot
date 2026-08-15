<!--
文件用途: 承载Add Devices Step1相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { deviceAdd } from '@/service/api/device'
import { $t } from '@/locales'

const props = defineProps<{
  configOptions: any[]
  nextCallback: () => void
  setIdCallback: (dId: string, cId: string, dobj: string, deviceNumber: string) => void
}>()
const formRef = ref<FormInst | null>(null)
const message = useMessage()
const formValue = ref({
  name: '',
  pid_number: '',
  label: [],
  device_config_id: ''
})
const rules = {
  name: {
    required: true,
    message: $t('custom.devicePage.enterDeviceName'),
    trigger: 'blur'
  },
  pid_number: {
    required: true,
    pattern: /^[A-Za-z0-9]{12}$/,
    message: $t('rdi.device.pidInvalid'),
    trigger: ['input', 'blur']
  }
}

async function handleValidateClick(e: MouseEvent) {
  e.preventDefault()
  try {
    await formRef.value?.validate()

    const payload = {
      ...formValue.value,
      pid_number: formValue.value.pid_number.trim().toUpperCase(),
      label: formValue.value.label.join(','),
      access_way: 'A'
    }
    const res = await deviceAdd(payload)
    const configId = formValue.value.device_config_id
    const deviceId = res.data.id
    props.setIdCallback(deviceId, configId, res.data.voucher, res.data.device_number || payload.pid_number)
    props.nextCallback()
  } catch (error) {
    if (Array.isArray(error)) {
      message.error($t('custom.devicePage.validationFailed'))
      return
    }

    message.error($t('generate.addFailed'))
  }
}
</script>

<template>
  <div>
    <n-card :bordered="false">
      <n-form ref="formRef" :label-width="80" :model="formValue" :rules="rules" size="small">
        <n-form-item :label="$t('custom.devicePage.deviceName')" path="name">
          <n-input v-model:value="formValue.name" :placeholder="$t('custom.devicePage.inputDeviceName')" />
        </n-form-item>
        <n-form-item label="PID" path="pid_number">
          <n-input
            v-model:value="formValue.pid_number"
            maxlength="12"
            :placeholder="$t('rdi.device.pidPlaceholder')"
            @update:value="value => (formValue.pid_number = value.toUpperCase())"
          />
        </n-form-item>
        <n-form-item :label="$t('custom.devicePage.label')" path="label">
          <n-dynamic-tags v-model:value="formValue.label" />
        </n-form-item>
        <n-form-item :label="$t('device_template.equipmentConfig')" path="device_config_id">
          <n-select
            v-model:value="formValue.device_config_id"
            :placeholder="$t('custom.devicePage.selectDeviceConfig')"
            label-field="name"
            value-field="id"
            :options="configOptions"
            filterable
          />
        </n-form-item>
        <n-form-item>
          <n-button type="primary" attr-type="button" @click="handleValidateClick">
            {{ $t('custom.devicePage.saveAndNext') }}
          </n-button>
        </n-form-item>
      </n-form>
    </n-card>
  </div>
</template>

<style scoped></style>
