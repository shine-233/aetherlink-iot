<!--
  文件用途：Integration 新建/编辑表单弹窗（从 index.vue 拆出）。
  核心逻辑：名称/连接器类型/上下行转换器绑定/设备多选/高级配置 JSON 表单；
  config JSONB 与表单互转（device_ids 由设备多选托管，其余键保留在 configExtra），
  提交时按 editingId 走 create/update，成功后 emit success 由页面回刷列表。
  转换器与设备选项由页面在打开弹窗时注入，保持“打开即刷新绑定选项”的旧行为。
-->
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { NButton, NForm, NFormItem, NInput, NModal, NSelect, NSpace, NSwitch, useMessage } from 'naive-ui'
import type { FormInst, FormRules, SelectOption } from 'naive-ui'
import {
  createIntegration,
  updateIntegration,
  type IntegrationConnectorType,
  type IntegrationItem
} from '@/service/api'
import { $t } from '@/locales'

defineOptions({ name: 'IntegrationFormModal' })

const props = defineProps<{
  show: boolean
  /** 编辑目标；null 表示新建。 */
  integration: IntegrationItem | null
  uplinkOptions: SelectOption[]
  downlinkOptions: SelectOption[]
  deviceOptions: SelectOption[]
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  success: []
}>()

const message = useMessage()

const submitting = ref(false)
const formRef = ref<FormInst | null>(null)

// 连接器类型选项：与后端 CHECK 约束（130.sql）保持一致。
const connectorOptions: SelectOption[] = [
  { label: $t('page.integration.connector.opcua'), value: 'opcua' },
  { label: $t('page.integration.connector.snmp'), value: 'snmp' },
  { label: $t('page.integration.connector.plugin'), value: 'plugin' }
]

const formModel = reactive({
  name: '',
  connector_type: 'opcua' as IntegrationConnectorType,
  converter_uplink_id: null as string | null,
  converter_downlink_id: null as string | null,
  deviceIds: [] as string[],
  configExtra: '',
  enabled: true
})

const formRules: FormRules = {
  name: { required: true, message: $t('page.integration.nameRequired'), trigger: 'blur' }
}

/** config JSONB 与表单互转：device_ids 由设备多选托管，其余键保留在 configExtra。 */
function applyConfigToForm(configJSON: string) {
  let parsed: Record<string, unknown> = {}
  try {
    parsed = JSON.parse(configJSON || '{}') as Record<string, unknown>
  } catch {
    parsed = {}
  }
  const { device_ids: deviceIds, ...rest } = parsed
  formModel.deviceIds = Array.isArray(deviceIds) ? (deviceIds as string[]) : []
  const restKeys = Object.keys(rest)
  formModel.configExtra = restKeys.length > 0 ? JSON.stringify(rest, null, 2) : ''
}

function configFromForm() {
  let extra: Record<string, unknown> = {}
  if (formModel.configExtra.trim() !== '') {
    extra = JSON.parse(formModel.configExtra) as Record<string, unknown>
  }
  extra.device_ids = formModel.deviceIds
  return JSON.stringify(extra)
}

// 打开时按“新建/编辑”回填表单。
watch(
  () => props.show,
  (show) => {
    if (!show) return
    if (props.integration) {
      Object.assign(formModel, {
        name: props.integration.name,
        connector_type: props.integration.connector_type,
        converter_uplink_id: props.integration.converter_uplink_id ?? null,
        converter_downlink_id: props.integration.converter_downlink_id ?? null,
        enabled: props.integration.enabled
      })
      applyConfigToForm(props.integration.config)
    } else {
      Object.assign(formModel, {
        name: '',
        connector_type: 'opcua',
        converter_uplink_id: null,
        converter_downlink_id: null,
        deviceIds: [],
        configExtra: '',
        enabled: true
      })
    }
  },
  { immediate: true }
)

const close = () => {
  emit('update:show', false)
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  if (formModel.configExtra.trim() !== '') {
    try {
      JSON.parse(formModel.configExtra)
    } catch {
      message.error($t('page.integration.invalidConfig'))
      return
    }
  }
  submitting.value = true
  try {
    const payload = {
      name: formModel.name,
      connector_type: formModel.connector_type,
      converter_uplink_id: formModel.converter_uplink_id ?? '',
      converter_downlink_id: formModel.converter_downlink_id ?? '',
      config: configFromForm(),
      enabled: formModel.enabled
    }
    const { error } = props.integration
      ? await updateIntegration({ id: props.integration.id, ...payload })
      : await createIntegration(payload)
    if (!error) {
      message.success($t('page.integration.saveSuccess'))
      close()
      emit('success')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <NModal
    :show="props.show"
    preset="card"
    :title="props.integration ? $t('page.integration.editTitle') : $t('page.integration.createTitle')"
    class="w-640px"
    @update:show="emit('update:show', $event)"
  >
    <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" label-width="120">
      <NFormItem :label="$t('page.integration.name')" path="name">
        <NInput v-model:value="formModel.name" :placeholder="$t('page.integration.namePlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('page.integration.connectorType')" path="connector_type">
        <NSelect v-model:value="formModel.connector_type" :options="connectorOptions" />
      </NFormItem>
      <NFormItem :label="$t('page.integration.converterUplink')" path="converter_uplink_id">
        <NSelect
          v-model:value="formModel.converter_uplink_id"
          :options="props.uplinkOptions"
          :placeholder="$t('page.integration.unbound')"
          clearable
          filterable
        />
      </NFormItem>
      <NFormItem :label="$t('page.integration.converterDownlink')" path="converter_downlink_id">
        <NSelect
          v-model:value="formModel.converter_downlink_id"
          :options="props.downlinkOptions"
          :placeholder="$t('page.integration.unbound')"
          clearable
          filterable
        />
      </NFormItem>
      <NFormItem :label="$t('page.integration.devices')" path="deviceIds">
        <NSelect
          v-model:value="formModel.deviceIds"
          :options="props.deviceOptions"
          multiple
          filterable
          clearable
          :placeholder="$t('page.integration.devicesPlaceholder')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.integration.configExtra')" path="configExtra">
        <NInput
          v-model:value="formModel.configExtra"
          type="textarea"
          :rows="4"
          :placeholder="$t('page.integration.configHint')"
        />
      </NFormItem>
      <NFormItem :label="$t('page.integration.enabled')" path="enabled">
        <NSwitch v-model:value="formModel.enabled" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="close">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped></style>
