<!--
发布到市场确认弹窗，负责在正式发布设备配置前补充市场展示信息并提交发布请求。
核心链路：打开弹窗时接收 device_config_id -> 拉取设备配置详情填充默认值 -> 校验市场名称/品牌/版本等字段 -> 携带 market token 调用发布接口。
静态维护重点：
1. 发布入口的真实主体是 `device_config_id`，不是模板 ID；修改这条链路时要同时和后端 `PublishToMarket` 的入参约定保持一致。
2. 当前默认值来自设备配置详情，但部分品牌/型号字段是否存在取决于后端返回结构，后续若数据不稳定应补显式映射或兜底说明。
3. token 失效时这里只删除 sessionStorage 并提示重新登录，后续若要优化体验，建议和市场登录弹窗链路统一。
-->
<script setup lang="ts">
import { ref, reactive } from 'vue'
import { NModal, NForm, NFormItem, NInput, NButton, NAlert, NSelect, FormInst, FormRules } from 'naive-ui'
import { $t } from '@/locales'
import { publishToMarket } from '@/service/api/market'
import { deviceConfigInfo } from '@/service/api/device'

const emit = defineEmits(['publish-success'])

const visible = ref(false)
const loading = ref(false)
const formRef = ref<FormInst | null>(null)
// 发布动作的核心标识是 device_config_id，后端会据此继续追到模板与协议配置。
const deviceConfigIdValue = ref('')

const formModel = reactive({
  market_name: '',
  brand: '',
  model: '',
  category: '',
  version: '1.0.0',
  author: '',
  description: ''
})

const rules: FormRules = {
  market_name: [
    { required: true, message: () => $t('device_template.requireName'), trigger: 'blur' },
    { max: 50, message: () => $t('common.maxLength', { length: 50 }), trigger: 'blur' }
  ],
  brand: [{ required: true, message: () => $t('device_template.requireBrand'), trigger: 'blur' }],
  model: [{ required: true, message: () => $t('device_template.requireModel'), trigger: 'blur' }],
  category: [{ required: true, message: () => $t('device_template.requireCategory'), trigger: ['blur', 'change'] }],
  version: [
    { required: true, message: () => $t('device_template.requireVersion'), trigger: 'blur' },
    {
      pattern: /^\d+\.\d+\.\d+$/,
      message: () => $t('device_template.versionFormatError', { format: 'x.y.z' }),
      trigger: 'blur'
    }
  ],
  author: [{ required: true, message: () => $t('device_template.requireAuthor'), trigger: 'blur' }],
  description: [{ required: true, message: () => $t('device_template.requireDescription'), trigger: 'blur' }]
}

const categoryOptions = [
  { label: () => $t('device_template.marketCatIoT'), value: 'IoT' },
  { label: () => $t('device_template.marketCatIndustrial'), value: '工业' },
  { label: () => $t('device_template.marketCatAgriculture'), value: '农业' },
  { label: () => $t('device_template.marketCatSmartCity'), value: '智慧城市' },
  { label: () => $t('device_template.marketCatOther'), value: '其他' }
]

// 打开时先重置表单，再尝试用设备配置详情自动填默认值，减少用户重复录入。
const open = async (deviceConfigId: string, defaultName?: string) => {
  deviceConfigIdValue.value = deviceConfigId

  formModel.market_name = defaultName || ''
  formModel.brand = ''
  formModel.model = ''
  formModel.category = ''
  formModel.version = '1.0.0'
  formModel.author = ''
  formModel.description = ''

  try {
    const dcRes: any = await deviceConfigInfo({ id: deviceConfigId })
    if (!dcRes.error && dcRes.data) {
      const dc = dcRes.data
      if (!formModel.market_name) {
        formModel.market_name = dc.name || ''
      }
      formModel.brand = dc.brand || ''
      formModel.model = dc.model_number || dc.product_model || ''
      formModel.version = dc.version || '1.0.0'
      formModel.author = dc.author || ''
      formModel.description = dc.description || ''
    }
  } catch (e) {
    console.error('Failed to get device config detail', e)
  }
  visible.value = true
}

// 真正发布前必须同时满足表单校验通过和市场 token 仍然有效。
const handlePublish = async () => {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  const token = sessionStorage.getItem('market_token')
  if (!token) {
    window.$message?.error($t('market.loginRequired'))
    visible.value = false
    return
  }

  loading.value = true
  try {
    const res: any = await publishToMarket({
      device_config_id: deviceConfigIdValue.value,
      market_token: token,
      market_name: formModel.market_name,
      brand: formModel.brand,
      model: formModel.model,
      category: formModel.category,
      version: formModel.version,
      author: formModel.author,
      description: formModel.description
    })
    if (!res.error) {
      window.$message?.success($t('device_template.publishSuccess'))
      visible.value = false
      emit('publish-success')
    } else {
      window.$message?.error($t('device_template.publishFailed') + ': ' + (res.error?.msg || ''))
    }
  } catch (e: any) {
    if (e?.response?.status === 401) {
      sessionStorage.removeItem('market_token')
      window.$message?.error($t('market.tokenExpired'))
    } else {
      window.$message?.error($t('device_template.publishFailed') + ': ' + (e?.message || ''))
    }
  } finally {
    loading.value = false
  }
}

// 取消仅关闭弹窗，不主动清空最近一次填写内容，下一次 open 会重新覆盖默认值。
const handleCancel = () => {
  visible.value = false
}

defineExpose({ open })
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="dialog"
    :title="$t('device_template.publishConfirmTitle')"
    style="width: 550px"
  >
    <div style="margin-top: 20px">
      <NForm
        ref="formRef"
        :model="formModel"
        :rules="rules"
        label-placement="left"
        label-width="110"
        require-mark-placement="right-hanging"
      >
        <NFormItem :label="$t('device_template.marketName')" path="market_name">
          <NInput
            v-model:value="formModel.market_name"
            :placeholder="$t('device_template.inputMarketName')"
            maxlength="50"
            show-count
            clearable
          />
        </NFormItem>
        <NFormItem :label="$t('device_template.brand')" path="brand">
          <NInput v-model:value="formModel.brand" :placeholder="$t('device_template.inputBrand')" clearable />
        </NFormItem>
        <NFormItem :label="$t('device_template.modelNumber')" path="model">
          <NInput v-model:value="formModel.model" :placeholder="$t('device_template.inputModelNumber')" clearable />
        </NFormItem>
        <NFormItem :label="$t('device_template.category')" path="category">
          <NSelect
            v-model:value="formModel.category"
            :options="categoryOptions"
            :placeholder="$t('device_template.selectCategory')"
            clearable
          />
        </NFormItem>
        <NFormItem :label="$t('device_template.version')" path="version">
          <NInput v-model:value="formModel.version" :placeholder="$t('device_template.inputVersion')" clearable />
        </NFormItem>
        <NFormItem :label="$t('device_template.author')" path="author">
          <NInput v-model:value="formModel.author" :placeholder="$t('device_template.inputAuthor')" clearable />
        </NFormItem>
        <NFormItem :label="$t('generate.description')" path="description">
          <NInput
            v-model:value="formModel.description"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 6 }"
            :placeholder="$t('device_template.inputDescription')"
            clearable
          />
        </NFormItem>
      </NForm>

      <NAlert type="info" style="margin-top: 12px">
        {{ $t('device_template.publishConfirmMessage') }}
      </NAlert>
    </div>

    <template #action>
      <NButton @click="handleCancel">{{ $t('common.cancel') }}</NButton>
      <NButton type="primary" :loading="loading" @click="handlePublish">
        {{ $t('device_template.confirmPublish') }}
      </NButton>
    </template>
  </NModal>
</template>

<style scoped></style>
