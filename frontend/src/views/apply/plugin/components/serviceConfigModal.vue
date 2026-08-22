<!--
文件用途：提供 接入插件管理 页面内的 serviceConfigModal 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="ts">
import { ref } from 'vue'
import type { FormInst, FormRules, SelectOption } from 'naive-ui'
import { $t } from '@/locales'
import { putRegisterService } from '@/service/api/plugin'

type PluginServiceRow = {
  service_type?: number
  service_config?: string
  [key: string]: unknown
}

const serviceType = ref($t('card.accessProtocol'))
// 是否为“接入协议”类型由后端 service_type 数值决定，不能依赖已翻译的展示文案做判断。
const isAccessProtocol = ref(true)
const emit = defineEmits(['getList'])
const serviceModal = ref(false)
const formRef = ref<FormInst | null>(null)
const details = ref<PluginServiceRow>({})

const loading = ref(false)
const defaultForm = {
  http_address: '',
  device_type: 1,
  sub_topic_prefix: '',
  access_address: ''
}
const form = ref({ ...defaultForm })

const rules = ref<FormRules>({
  http_address: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('card.httpAddress')
  },
  device_type: {
    required: true,
    message: $t('card.chooseDeviceType')
  },
  sub_topic_prefix: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('card.subscribeSubjectPrefix')
  }
})
const options = ref<SelectOption[]>([
  {
    label: $t('card.directConnectDevice'),
    value: 1
  },
  {
    label: $t('card.gatewayDevice'),
    value: 2
  },
  {
    label: $t('card.gatewaySubDevice'),
    value: 3
  }
])

const openModal = (row: PluginServiceRow | null | undefined) => {
  if (row) {
    isAccessProtocol.value = row.service_type === 1
    serviceType.value = row.service_type === 1 ? $t('card.accessProtocol') : $t('card.accessService')
    details.value = { ...row }
    if (details.value.service_config === '') return
    Object.assign(form.value, JSON.parse(row.service_config!))
  }
  serviceModal.value = true
}
const close: () => void = () => {
  serviceModal.value = false
  details.value = {}
  Object.assign(form.value, defaultForm)
}

const submitSevice: () => void = () => {
  formRef.value?.validate(async errors => {
    if (errors) return
    loading.value = true
    const params = details.value
    params.service_config = JSON.stringify(form.value)
    const data = await putRegisterService(params)
    if (data.data) {
      emit('getList')
      close()
    }
    loading.value = false
  })
}

defineExpose({ openModal })
</script>

<template>
  <n-modal
    v-model:show="serviceModal"
    preset="dialog"
    :title="`${$t('common.pluginConfig')}(${serviceType})`"
    @after-leave="close"
  >
    <n-space vertical>
      <n-spin :show="loading">
        <n-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-placement="left"
          label-width="auto"
          require-mark-placement="right-hanging"
          :disabled="loading"
        >
          <n-form-item :label="$t('card.httpServerAddress')" path="http_address">
            <n-input v-model:value="form.http_address" placeholder="plugin-service:503" />
          </n-form-item>
          <n-form-item v-if="isAccessProtocol" :label="$t('generate.device-type')" path="device_type">
            <n-select v-model:value="form.device_type" :placeholder="$t('card.chooseDeviceType')" :options="options" />
          </n-form-item>
          <n-form-item :label="$t('card.serverSubscribeSubjectPrefix')" path="sub_topic_prefix">
            <n-input v-model:value="form.sub_topic_prefix" placeholder="tenant/{tenantId}/plugin/{service}/" />
          </n-form-item>
          <n-form-item v-if="isAccessProtocol" :label="$t('card.deviceAccessAddress')" path="access_address">
            <n-input
              v-model:value="form.access_address"
              :placeholder="$t('card.fillDeviceAccessAddress')"
              type="textarea"
            />
          </n-form-item>
        </n-form>
        <div class="footer">
          <NButton type="primary" class="btn" @click="submitSevice">{{ $t('common.confirm') }}</NButton>
          <NButton @click="close">{{ $t('common.cancel') }}</NButton>
        </div>
      </n-spin>
    </n-space>
  </n-modal>
</template>

<style lang="scss" scoped>
.selectType {
  width: 100%;
}
.footer {
  display: flex;
  flex-direction: row-reverse;
  .btn {
    margin-left: 10px;
  }
}
</style>
