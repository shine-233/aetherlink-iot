<!--
文件用途：提供 接入插件管理 页面内的 serviceModal 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { putRegisterService, registerService } from '@/service/api/plugin'
import { $t } from '@/locales'
const isEdit = ref<any>(false)
const emit = defineEmits(['getList'])
const serviceModal = ref<any>(false)
const formRef = ref<any>(null)
const loading = ref<any>(false)
const defaultForm = {
  name: '',
  service_identifier: '',
  service_type: null,
  version: '',
  description: '',
  service_config: '',
  remark: ''
}
const form = ref<any>({ ...defaultForm })
const rules = ref<any>({
  name: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('card.enterServiceName')
  },
  service_identifier: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('card.enterServiceIdentifier')
  },
  service_type: {
    required: true,
    message: $t('card.selectServiceType')
  }
})
const options = ref<any>([
  {
    label: $t('card.accessProtocol'),
    value: 1
  },
  {
    label: $t('card.accessService'),
    value: 2
  }
])
const openModal: (row: any) => void = row => {
  if (row) {
    isEdit.value = true
    Object.assign(form.value, row)
  } else {
    isEdit.value = false
    form.value = { ...defaultForm }
  }
  serviceModal.value = true
}
const close: () => void = () => {
  serviceModal.value = false
  form.value = { ...defaultForm }
}
const submitSevice: () => void = async () => {
  formRef.value?.validate(async errors => {
    if (errors) return
    loading.value = true
    const data: any = isEdit.value ? await putRegisterService(form.value) : await registerService(form.value)
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
  <n-modal aria-label="dialog" v-model:show="serviceModal" preset="dialog" :title="$t('common.serviceConfi')">
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
          @after-leave="close"
        >
          <n-form-item :label="$t('card.serviceName')" path="name">
            <n-input v-model:value="form.name" :placeholder="$t('card.enterServiceName')" />
          </n-form-item>
          <n-form-item :label="$t('card.serviceIdentifier')" path="service_identifier">
            <n-input v-model:value="form.service_identifier" :placeholder="$t('card.enterServiceIdentifier')" />
          </n-form-item>
          <n-form-item :label="$t('card.type')" path="service_type">
            <n-space vertical class="selectType">
              <n-select v-model:value="form.service_type" :options="options" :disabled="isEdit" />
            </n-space>
          </n-form-item>
          <n-form-item :label="$t('card.version')" path="version">
            <n-input v-model:value="form.version" :placeholder="$t('card.enterVersion')" />
          </n-form-item>
          <n-form-item :label="$t('card.description')" path="description">
            <n-input v-model:value="form.description" :placeholder="$t('card.enterDescription')" type="textarea" />
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
