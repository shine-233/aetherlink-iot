<!--
设备配置新增/编辑页，负责设备配置基础信息、物模型绑定、协议插件表单和凭证类型的统一编辑。
核心链路：根据路由 id 区分新增或编辑 -> 拉取物模型、协议列表、协议表单与凭证类型 -> 回显或编辑配置 -> 序列化 protocol_config 后提交保存。
静态维护重点：
1. `configForm` 与 `protocol_config` 当前拆成两套状态，提交时再合并，后续若协议表单继续复杂化，建议抽 composable 统一管理。
2. 空字符串物模型 ID 在这里代表“解绑物模型”，修改物模型选择逻辑时必须同步后端 `device_config.go` 的解绑约定。
3. 页面既处理新增又处理编辑，错误提示和 loading 收口分支较多，后续应优先补 `try/finally` 风格的统一提交流程。
-->
<script lang="ts" setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import type { AxiosError } from 'axios'
import type { FormInst, SelectOption, FormRules, SelectGroupOption } from 'naive-ui'
import { NTooltip, NIcon, NFlex, useMessage } from 'naive-ui'
import { HelpCircle } from '@vicons/ionicons5'
import { router } from '@/router'
import {
  deviceConfigAdd,
  deviceConfigEdit,
  deviceConfigInfo,
  deviceConfigVoucherType,
  deviceProtocolServiceList,
  deviceTemplate,
  protocolPluginConfigForm
} from '@/service/api/device'
import { $t } from '@/locales'
import FormInput from '../config-detail/modules/form.vue'

const message = useMessage()

const route = useRoute()
const configId = ref(route.query.id || null)
const modalTitle = ref('generate.add')

let loading = ref(false)

// 页面主表单负责基础字段，协议动态表单单独放在 protocol_config 中维护。
interface ConfigFormData {
  id: string | number | null
  additional_info: string | null
  description: string | null
  device_conn_type: string | number | null
  device_template_id: string | number | '' | null // 允许空字符串
  device_type: string | number | null
  name: string | null
  protocol_config: Record<string, any> | string | null // 允许对象、字符串或 null
  protocol_type: string | number | null
  remark: string | null
  voucher_type: string | number | null
}

function defaultConfigForm(): ConfigFormData {
  return {
    id: null,
    additional_info: null,
    description: null,
    device_conn_type: null,
    device_template_id: '', // 默认值是空字符串
    device_type: null,
    name: null,
    protocol_config: null, // 默认是 null
    protocol_type: null,
    remark: null,
    voucher_type: null
  }
}

const configForm = ref<ConfigFormData>(defaultConfigForm())
const isEdit = ref(false)

interface Option {
  name: string
  id: string | number
  [key: string]: any // 允许其他属性
}

// 协议类型、连接方式、动态表单项会随着设备类型和协议类型变化而重新装配。
const typeOptions = ref<(SelectOption | SelectGroupOption)[]>([])
const connectOptions = ref<SelectOption[]>([])
const protocol_config = ref<Record<string, any>>({})
type FormElementType = 'input' | 'table' | 'select'

interface FormElement {
  type: FormElementType
  dataKey: string
  label: string
  options?: Option[]
  placeholder?: string
  // validate?: Validate; // 暂时注释掉，除非能找到 Validate 定义
  array?: FormElement[]
}
const formElements = ref<FormElement[]>([])

const configFormRules = ref<FormRules>({
  name: {
    required: true,
    message: $t('common.deviceConfigName'),
    trigger: 'blur'
  },
  device_type: {
    required: true,
    message: $t('common.deviceAccessType'),
    trigger: 'change'
  },
  device_conn_type: {
    required: true,
    message: $t('common.deviceConnectionMethod'),
    trigger: 'change'
  }
})

const queryTemplate = ref({
  page: 1,
  page_size: 20,
  total: 0
})
// 下拉项 name 兼容字符串与渲染函数（首项“解绑物模型”用渲染函数保证 i18n 响应）。
const deviceTemplateOptions = ref<Array<{ id: string | number; name: string | ((option: unknown) => string) }>>([
  { name: () => $t('generate.unbind'), id: '' }
])

// 物模型下拉支持滚动分页，避免一次性加载全部物模型导致编辑页初始化过重。
const getDeviceTemplate = () => {
  deviceTemplate({ ...queryTemplate.value })
    .then(res => {
      const list = res.data?.list ?? []
      deviceTemplateOptions.value = deviceTemplateOptions.value.concat(list)
      queryTemplate.value.total = res.data?.total ?? queryTemplate.value.total
    })
    .catch(error => {
      console.error('Failed to get thing models:', error)
      message.error($t('generate.failedToLoadDeviceTemplates'))
    })
}

// 下拉滚动到底时继续加载物模型列表，并保留首项“解绑物模型”占位。
const deviceTemplateScroll = (e: Event) => {
  const currentTarget = e.currentTarget as HTMLElement
  if (currentTarget.scrollTop + currentTarget.offsetHeight >= currentTarget.scrollHeight) {
    if (deviceTemplateOptions.value.length + 1 <= queryTemplate.value.total) {
      queryTemplate.value.page += 1
      getDeviceTemplate()
    }
  }
}

const configFormRef = ref<FormInst>()

// 关闭页时同时重置校验状态和表单快照，避免返回后残留旧输入。
const handleClose = () => {
  configFormRef.value?.restoreValidation()
  configForm.value = defaultConfigForm()
  router.go(-1)
}

// 提交时将动态协议配置对象序列化成后端约定的 JSON 字符串。
const handleSubmit = async () => {
  await configFormRef?.value?.validate()

  loading.value = true

  const postData = { ...configForm.value }

  postData.protocol_config = JSON.stringify(protocol_config.value || {})

  if (!configId.value) {
    const res = await deviceConfigAdd(postData)

    loading.value = false

    if (!res.error) {
      handleClose()
    } else {
      message.error((res as any)?.message || $t('generate.addFailed'))
    }
  } else {
    const res = await deviceConfigEdit(postData).catch((error: AxiosError) => {
      message.error((error && 'message' in error && error.message) || $t('generate.editFailed'))
      return { error: true }
    })

    loading.value = false

    if (!res.error) {
      handleClose()
    } else {
      message.error((res as any)?.message || $t('generate.editFailed'))
    }
  }
}

// 编辑模式先拉详情，再把字符串或对象形态的 protocol_config 统一回填成对象。
const getConfig = async () => {
  try {
    const res = await deviceConfigInfo({ id: configId.value as string })
    if (!res.data) {
      protocol_config.value = {}
      return
    }
    configForm.value = { ...res.data }

    try {
      if (typeof res.data.protocol_config === 'string') {
        protocol_config.value = JSON.parse(res.data.protocol_config)
      } else if (res.data.protocol_config !== null && typeof res.data.protocol_config === 'object') {
        protocol_config.value = res.data.protocol_config
      } else {
        protocol_config.value = {}
      }
    } catch (e) {
      console.error('Failed to parse protocol_config:', e)
      message.error($t('generate.failedToParseProtocolConfig'))
      protocol_config.value = {}
    }
  } catch (error) {
    console.error('Failed to get device config info:', error)
    message.error($t('generate.failedToLoadConfig'))
  }
}

watch(
  () => configId.value,
  async newId => {
    if (newId) {
      modalTitle.value = 'common.edit'
    }
  }
)
const getProtocolList = async (deviceCode: string | number) => {
  const queryData = { device_type: deviceCode }
  const res = await deviceProtocolServiceList(queryData)
  if (res.data) {
    // 明确数组元素的类型
    typeOptions.value = [
      {
        type: 'group',
        label: $t('common.protocol'), // naive-ui group 使用 label
        key: 'protocol',
        children: (res.data.protocol || []).map((p: any) => ({
          label: p.name,
          value: p.service_identifier
        })) as SelectOption[]
      },
      {
        type: 'group',
        label: $t('common.service'), // naive-ui group 使用 label
        key: 'service',
        children: (res.data.service || []).map((s: any) => ({
          label: s.name,
          value: s.service_identifier
        })) as SelectOption[]
      }
    ]
  }
}

// 协议插件动态表单由后端返回，前端只做展示和输入承接。
const getConfigForm = async data => {
  formElements.value = []
  if (!data || !configForm?.value?.device_type) {
    return
  }

  const res = await protocolPluginConfigForm({
    device_type: configForm?.value?.device_type,
    protocol_type: data
  })
  if (res.error) {
    message.error((res as any)?.message || $t('generate.failedToLoadConfig'))
    return
  }
  formElements.value = Array.isArray(res.data) ? res.data : []
}

// 凭证类型会随接入协议切换而变化，避免用户保留无效凭证方案。
const getVoucherType = async (data: any) => {
  connectOptions.value = []
  const res = await deviceConfigVoucherType({
    device_type: configForm?.value?.device_type,
    protocol_type: data
  })
  if (res.data) {
    // 明确 map 返回类型
    connectOptions.value = Object.keys(res.data).map(key => {
      return { label: key, value: res.data[key] } as SelectOption
    })
  }
}

const choseProtocolType = async data => {
  configForm.value.voucher_type = null
  protocol_config.value = {}
  formElements.value = []
  await getVoucherType(data)
  await getConfigForm(data)
}

// 定义设备类型及其帮助信息的数组
const deviceTypes = ref([
  { value: '1', labelKey: 'generate.direct-connected-device', helpKey: 'generate.deviceTypeHelp.direct' },
  { value: '2', labelKey: 'generate.gateway', helpKey: 'generate.deviceTypeHelp.gateway' },
  { value: '3', labelKey: 'generate.gateway-sub-device', helpKey: 'generate.deviceTypeHelp.subDevice' }
])

function getTooltipText(i18nKey: string) {
  const raw = String($t(i18nKey) ?? '')
  return raw
    .replace(/\\n/g, '\n')
    .replace(/<br\s*\/?\s*>/gi, '\n')
    .replace(/n(?=\s*[●○])/g, '\n')
}

function getTooltipLines(i18nKey: string) {
  return getTooltipText(i18nKey)
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean)
}

function tooltipLineClass(line: string) {
  // Sub-bullets (○) should be indented under main bullets (●)
  if (/^○/.test(line)) return 'pl-6'
  return 'pl-0'
}

onMounted(async () => {
  if (configId.value) {
    modalTitle.value = 'common.edit'
    isEdit.value = true
    await getConfig()
  } else {
    isEdit.value = false
    modalTitle.value = 'generate.create'
  }
  getDeviceTemplate()
  if (process.env.NODE_ENV === 'development') {
    /* intentionally empty */
  }

  await getProtocolList(configForm?.value.device_type || '1')

  if (configForm.value.protocol_type) {
    await getVoucherType(configForm.value.protocol_type)
    await getConfigForm(configForm.value.protocol_type)
  }
})

// 新增：处理设备类型变更的函数
function handleDeviceTypeChange(newValue: string | number) {
  configFormRef.value?.restoreValidation()
  // 在 script 块中访问，类型检查通常更可靠
  if (!configForm.value) {
    // 可以保留检查以防万一
    console.error('configForm.value is unexpectedly null/undefined during device type change')
    return
  }
  protocol_config.value = {}
  configForm.value.voucher_type = null
  configForm.value.protocol_type = null
  formElements.value = []
  getProtocolList(newValue)
}

// const getPlatform = computed(() => {
//   const { proxy }: any = getCurrentInstance();
//   return proxy.getPlatform();
// });
</script>

<template>
  <div class="overflow-y-auto">
    <NCard :title="`${$t(modalTitle)}${$t('custom.devicePage.configTemplate')}`">
      <NForm ref="configFormRef" :model="configForm" :rules="configFormRules" label-placement="left" label-width="auto">
        <!-- 第一个文件中的原表单项 -->
        <NFormItem :label="$t('generate.device-configuration-name')" path="name" class="w-[600px]">
          <NInput v-model:value="configForm.name" :placeholder="$t('common.deviceConfigName')" />
        </NFormItem>
        <NFormItem class="w-[600px]" :label="$t('generate.select-device-function-template')" path="device_template_id">
          <NSelect
            v-model:value="configForm.device_template_id"
            :options="deviceTemplateOptions"
            filterable
            label-field="name"
            value-field="id"
            @scroll="deviceTemplateScroll"
          ></NSelect>
        </NFormItem>
        <NFormItem :label="$t('generate.device-access-type')" path="device_type">
          <n-radio-group
            v-model:value="configForm.device_type"
            name="device_type"
            :disabled="isEdit"
            @update:value="handleDeviceTypeChange"
          >
            <n-space>
              <!-- 使用 v-for 循环渲染 -->
              <div v-for="dtype in deviceTypes" :key="dtype.value" class="flex">
                <n-radio :value="dtype.value">{{ $t(dtype.labelKey) }}</n-radio>
                <NTooltip
                  trigger="hover"
                  :content-style="{
                    whiteSpace: 'pre-wrap',
                    textAlign: 'left',
                    maxWidth: '400px',
                    wordBreak: 'break-word'
                  }"
                >
                  <template #trigger>
                    <NIcon class="cursor-help ml-1 mr-4">
                      <HelpCircle class="text-6" />
                    </NIcon>
                  </template>
                  <div class="tp-tooltip">
                    <div
                      v-for="(line, idx) in getTooltipLines(dtype.helpKey)"
                      :key="idx"
                      :class="tooltipLineClass(line)"
                    >
                      {{ line }}
                    </div>
                  </div>
                </NTooltip>
              </div>
            </n-space>
          </n-radio-group>
        </NFormItem>

        <!-- 第二个文件中的新增表单项 -->
        <template v-if="configForm.device_type">
          <NFormItem class="w-[600px]" :label="$t('generate.choose-protocol-or-Service')" path="protocol_type">
            <NSelect
              v-model:value="configForm.protocol_type"
              :options="typeOptions"
              :placeholder="$t('generate.select-protocol-service')"
              @update:value="choseProtocolType"
            ></NSelect>
          </NFormItem>

          <NFormItem
            v-if="connectOptions.length > 0"
            v-show="configForm.device_type === '1' || configForm.device_type === '2'"
            class="w-[600px]"
            :label="$t('generate.authentication-type')"
            path="voucher_type"
          >
            <NSelect
              v-model:value="configForm.voucher_type"
              :options="connectOptions"
              :placeholder="$t('generate.select-authentication-type')"
            ></NSelect>
          </NFormItem>
        </template>
        <NFormItem v-if="configForm.device_type && formElements.length > 0">
          <FormInput v-model:protocol-config="protocol_config" :form-elements="formElements"></FormInput>
        </NFormItem>
        <NFlex justify="flex-start">
          <NButton type="primary" :loading="loading" @click="handleSubmit">
            {{ $t('page.login.common.confirm') }}
          </NButton>
        </NFlex>
      </NForm>
    </NCard>
  </div>
</template>

<style lang="scss" scoped>
.w-600 {
  width: 600px;
}
// Add style for cursor
.cursor-help {
  cursor: help;
}

.tp-tooltip > div + div {
  margin-top: 4px;
}
</style>
