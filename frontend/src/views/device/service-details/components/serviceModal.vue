<!--
文件用途: 承载ServiceModal相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { createServiceDrop, getServiceAccessForm, putServiceDrop, getServiceListDrop } from '@/service/api/plugin'
import { $t } from '@/locales'
import FormInput from './form.vue'

const isEdit = ref<any>(false)
const emit = defineEmits(['getList', 'isEdit'])
const serviceModals = ref<any>(false)
const formRef = ref<any>(null)
const currentStep = ref(1)

const service_plugin_id = ref<any>('')
const formElements = ref<any>([])
const defaultForm = {
  name: '',
  service_plugin_id: '',
  voucher: {},
  vouchers: {},
  auth_type: 'manual' // 添加模式字段，默认为手动
}
const form = ref<any>({ ...defaultForm })
const rules = ref<any>({
  name: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('custom.serviceAccess.accessPointNameRequired')
  },
  auth_type: {
    required: true,
    trigger: ['change'],
    message: $t('custom.serviceAccess.modeRequired')
  }
})

const parseVoucher = (voucher: unknown): Record<string, unknown> => {
  if (!voucher) return {}
  if (typeof voucher === 'object') return voucher as Record<string, unknown>

  try {
    const parsed = JSON.parse(String(voucher))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    // Third-party access points store credentials as backend JSON; keep the modal repairable when an old row is malformed.
    return {}
  }
}

const openModal: (id: any, row?: any) => void = async (id, row) => {
  if (row) {
    // 编辑模式：设置 isEdit 为 true 并填充表单数据
    isEdit.value = true
    Object.assign(form.value, row)
    const voucherData = parseVoucher(row.voucher)
    Object.assign(form.value.vouchers, voucherData)
    // 从 voucher 解析的数据中回显 auth_type 到选择模式字段
    if (voucherData.auth_type) {
      form.value.auth_type = voucherData.auth_type
    }
  } else {
    // 新增模式：重置 isEdit 为 false
    isEdit.value = false
  }
  service_plugin_id.value = id
  form.value.service_plugin_id = id
  const data = await getServiceAccessForm({
    service_plugin_id: service_plugin_id.value
  })
  if (data.data) {
    formElements.value = data.data
    serviceModals.value = true
  }
}
const close: () => void = () => {
  serviceModals.value = false
  form.value = { ...defaultForm }
  form.value.vouchers = {}
  currentStep.value = 1
  // 重置编辑状态
  isEdit.value = false
}

const submitSevice: () => void = async () => {
  formRef.value?.validate(async (errors) => {
    if (errors) return

    form.value.vouchers.auth_type = form.value.auth_type
    form.value.voucher = JSON.stringify(form.value.vouchers)

    // Automatic discovery depends on the optional adapter. Probe it before
    // persisting the access point so an unavailable adapter cannot leave a
    // half-created record behind.
    if (form.value.auth_type === 'auto') {
      try {
        const probe = await getServiceListDrop({
          voucher: form.value.voucher,
          service_type: '',
          page: 1,
          page_size: 10
        })
        if (probe.error) {
          window.$message?.error($t('common.loadFailed'))
          return
        }
      } catch {
        window.$message?.error($t('common.loadFailed'))
        return
      }
    }

    const data: any = isEdit.value ? await putServiceDrop(form.value) : await createServiceDrop(form.value)
    if (data.error || (!isEdit.value && !data.data?.id)) {
      window.$message?.error($t('common.saveFailed'))
      return
    }

    serviceModals.value = false
    const id = isEdit.value ? form.value.id : data.data.id
    if (form.value.auth_type === 'auto') {
      emit(
        'isEdit',
        form.value.voucher,
        {
          id,
          auth_type: form.value.auth_type,
          name: form.value.name
        },
        true
      )
    } else {
      emit('isEdit', form.value.voucher, id, isEdit.value)
    }

    form.value = { ...defaultForm }
    form.value.vouchers = {}
  })
}

defineExpose({ openModal })
</script>

<template>
  <n-modal
    v-model:show="serviceModals"
    preset="dialog"
    :title="$t('card.addNewAccessPoint')"
    class="w"
    @after-leave="close"
  >
    <n-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-placement="left"
      label-width="auto"
      require-mark-placement="right-hanging"
    >
      <n-form-item :label="$t('card.accessPointName')" path="name">
        <n-input v-model:value="form.name" :placeholder="$t('custom.serviceAccess.accessPointNameRequired')" />
      </n-form-item>
      <n-form-item :label="$t('common.selectionMode')" path="auth_type">
        <n-radio-group v-model:value="form.auth_type">
          <n-radio value="manual">{{ $t('common.manual') }}</n-radio>
          <n-radio value="auto">{{ $t('common.automatic') }}</n-radio>
        </n-radio-group>
      </n-form-item>
    </n-form>
    <div class="box">
      <FormInput v-model:protocol-config="form.vouchers" :form-elements="formElements"></FormInput>
    </div>
    <div class="footer">
      <NButton type="primary" class="btn" @click="submitSevice">{{ $t('card.saveNext') }}</NButton>
      <NButton @click="close">{{ $t('common.cancel') }}</NButton>
    </div>
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
.box {
  width: 100%;
  height: 100%;
}
</style>

<style lang="scss">
.w {
  width: 70% !important;
  margin-top: 15vh;
  height: max-content !important;
  max-height: 800px !important;
  overflow: auto;
}
</style>
