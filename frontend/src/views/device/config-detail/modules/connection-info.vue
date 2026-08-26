<!--
  设备配置详情页的连接信息面板。
  这里主要承载三类数据：
  1. 协议与凭据类型的只读展示
  2. 协议插件动态表单的回显与提交
  3. Topic 映射列表的查询、编辑、保存与删除

  维护约定：
  - 后端已保存的 protocol_config 可能是对象，也可能是 JSON 字符串，解析时要保持容错。
  - Topic 映射的字段名和后端接口保持一一对应，保存前再做一次 trim 和默认值收敛。
  - 这里只做展示和静态编辑，不在组件内引入额外的业务分支。
-->
<script setup lang="ts">
import { defineAsyncComponent, onMounted, ref, h, watch, computed } from 'vue'
import {
  deviceConfigEdit,
  protocolPluginConfigForm,
  getTopicMappingList,
  createTopicMapping,
  updateTopicMapping,
  deleteTopicMapping
} from '@/service/api/device'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import { NButton, NPopconfirm, NSpace, NDataTable, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import ConnectionAccessPackageCard from './ConnectionAccessPackageCard.vue'

const FormInput = defineAsyncComponent(() => import('./form.vue'))
const TopicMappingModal = defineAsyncComponent(() => import('./components/topic-mapping-modal.vue'))

type FormElementType = 'input' | 'table' | 'select'

interface Option {
  label: string
  value: number | string
}

// 动态表单校验规则描述，直接透传给协议插件表单渲染层。
interface Validate {
  message: string
  required?: boolean
  rules?: string
  type?: 'number' | 'string' | 'array' | 'boolean' | 'object'
}

// 协议插件返回的表单元数据。当前组件只负责承接结构，不负责解释更细的业务语义。
interface FormElement {
  type: FormElementType
  dataKey: string
  label: string
  options?: Option[]
  placeholder?: string
  validate?: Validate
  array?: FormElement[]
}

const formElements = ref<FormElement[]>([])

interface Emits {
  (e: 'upDateConfig'): void
}

const emit = defineEmits<Emits>()

interface Props {
  configInfo?: object | any
}

const props = withDefaults(defineProps<Props>(), {
  configInfo: null
})

// extendForm 承接配置详情里的基础连接字段；protocol_config 承接协议插件的动态配置内容。
const extendForm = ref({
  protocol_type: null,
  voucher_type: null
} as any)
const extendFormRules = ref({})
const protocol_config = ref<Record<string, unknown>>({})
const active = ref(false)

const parseProtocolConfig = (value: unknown): Record<string, unknown> => {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>

  try {
    const parsed = JSON.parse(String(value))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    // 后端保存的 protocol_config 可能存在历史脏数据，解析失败时返回空对象，避免阻断页面编辑。
    return {}
  }
}

// Topic 映射状态流：列表查询、弹窗编辑、保存后刷新表格。
interface TopicMapping {
  id?: string | number
  mapping_name: string
  direction: 'up' | 'down'
  description: string
  original_topic: string
  target_topic: string
  data_identifier?: string
  priority?: number
  enabled?: boolean
}

// 列表态、弹窗态和加载态拆开维护，避免 Topic 映射编辑和主表单提交互相干扰。
const topicMappingList = ref<TopicMapping[]>([])
const topicMappingModalVisible = ref(false)
const currentEditTopicMapping = ref<TopicMapping | null>(null)
const topicMappingLoading = ref(false)
const message = useMessage()
const { t } = useI18n()

const topicMappingColumns = computed<DataTableColumns<TopicMapping>>(() => [
  {
    title: t('generate.topicMapping.column.mappingName'),
    key: 'mapping_name',
    align: 'left'
  },
  {
    title: t('common.description'),
    key: 'description',
    align: 'left'
  },
  {
    title: t('generate.topicMapping.column.originalTopic'),
    key: 'original_topic',
    align: 'left'
  },
  {
    title: t('generate.topicMapping.column.targetTopic'),
    key: 'target_topic',
    align: 'left'
  },
  {
    title: t('generate.topicMapping.column.dataIdentifier'),
    key: 'data_identifier',
    align: 'left'
  },
  {
    title: t('common.actions'),
    key: 'actions',
    align: 'center',
    width: 150,
    render: (row) => {
      return h(
        NSpace,
        { justify: 'center' },
        {
          default: () => [
            h(
              NButton,
              {
                type: 'primary',
                size: 'small',
                text: true,
                onClick: () => handleEditTopicMapping(row)
              },
              { default: () => t('common.edit') }
            ),
            h(
              NPopconfirm,
              {
                onPositiveClick: () => handleDeleteTopicMapping(row)
              },
              {
                default: () => t('common.confirmDelete'),
                trigger: () =>
                  h(
                    NButton,
                    {
                      type: 'error',
                      size: 'small',
                      text: true
                    },
                    { default: () => t('common.delete') }
                  )
              }
            )
          ]
        }
      )
    }
  }
])

const sensitiveConfigKeyPattern = /(password|passwd|secret|token|credential|private|key|cert)/i

const maskSensitiveProtocolConfig = (value: unknown): unknown => {
  if (Array.isArray(value)) {
    return value.map((item) => maskSensitiveProtocolConfig(item))
  }
  if (!value || typeof value !== 'object') {
    return value
  }

  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).map(([key, item]) => [
      key,
      sensitiveConfigKeyPattern.test(key) ? '***' : maskSensitiveProtocolConfig(item)
    ])
  )
}

const stringifyForDisplay = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '-'
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return JSON.stringify(value)
}

const connectionSummaryRows = computed(() => [
  {
    label: t('generate.connectionWorkbench.protocol'),
    value: stringifyForDisplay(extendForm.value.protocol_type)
  },
  {
    label: t('generate.connectionWorkbench.authentication'),
    value: stringifyForDisplay(extendForm.value.voucher_type)
  },
  {
    label: t('generate.connectionWorkbench.configId'),
    value: stringifyForDisplay(props.configInfo?.id)
  },
  {
    label: t('generate.connectionWorkbench.topicMappings'),
    value: String(topicMappingList.value.length)
  }
])

const connectionChecklist = computed(() => [
  t('generate.connectionWorkbench.checkProtocol'),
  t('generate.connectionWorkbench.checkCredentials'),
  t('generate.connectionWorkbench.checkTopicMapping'),
  t('generate.connectionWorkbench.checkTelemetry'),
  t('generate.connectionWorkbench.checkCommandResponse')
])

const connectionAccessPackage = computed(() => ({
  device_config_id: props.configInfo?.id ?? '',
  device_config_name: props.configInfo?.name ?? '',
  device_type: props.configInfo?.device_type ?? '',
  protocol_type: extendForm.value.protocol_type ?? '',
  voucher_type: extendForm.value.voucher_type ?? '',
  protocol_config: maskSensitiveProtocolConfig(protocol_config.value),
  topic_mappings: topicMappingList.value.map((item) => ({
    name: item.mapping_name,
    direction: item.direction,
    device_topic: item.original_topic,
    system_topic: item.target_topic,
    data_identifier: item.data_identifier || ''
  })),
  checklist: connectionChecklist.value
}))

const connectionAccessPackageText = computed(() =>
  JSON.stringify(connectionAccessPackage.value, null, 2)
)

const copyConnectionAccessPackage = async () => {
  const copied = await writeClipboardText(connectionAccessPackageText.value)
  if (copied) {
    message.success(t('generate.connectionWorkbench.copyPackageSuccess'))
  } else {
    message.error(t('generate.connectionWorkbench.copyPackageFailed'))
  }
}

const copyMaskedProtocolConfig = async () => {
  const copied = await writeClipboardText(JSON.stringify(maskSensitiveProtocolConfig(protocol_config.value), null, 2))
  if (copied) {
    message.success(t('generate.connectionWorkbench.copyConfigSuccess'))
  } else {
    message.error(t('generate.connectionWorkbench.copyConfigFailed'))
  }
}

const handleEditTopicMapping = (row: TopicMapping) => {
  currentEditTopicMapping.value = { ...row }
  topicMappingModalVisible.value = true
}

const handleDeleteTopicMapping = async (row: TopicMapping) => {
  if (!row.id) return
  try {
    await deleteTopicMapping(row.id)
    await fetchTopicMappings()
  } catch {
    message.error(t('generate.topicMapping.message.deleteFailed'))
  }
}

const handleAddTopicMapping = () => {
  currentEditTopicMapping.value = null
  topicMappingModalVisible.value = true
}

// 后端字段命名和前端弹窗字段命名不同，这里统一做一次映射，避免模板中散落兼容逻辑。
const normalizeTopicMapping = (item: any): TopicMapping => ({
  id: item.id,
  mapping_name: item.name ?? '',
  direction: item.direction === 'up' ? 'up' : 'down',
  description: item.description ?? '',
  original_topic: item.source_topic ?? '',
  target_topic: item.target_topic ?? '',
  data_identifier: item.data_identifier ?? '',
  priority: item.priority ?? 0,
  enabled: item.enabled ?? true
})

// 保存前统一收敛字段：补 device_config_id、trim topic 文本，并兜底优先级和启用状态。
const buildTopicMappingPayload = (data: TopicMapping) => ({
  device_config_id: props.configInfo?.id,
  name: data.mapping_name?.trim(),
  direction: data.direction,
  source_topic: data.original_topic?.trim(),
  target_topic: data.target_topic?.trim(),
  data_identifier: data.data_identifier?.trim(),
  description: data.description,
  priority: data.priority ?? 0,
  enabled: data.enabled ?? true
})

// 列表查询链路：以当前设备配置 id 为唯一上下文，成功后统一归一化为表格可用结构。
const fetchTopicMappings = async () => {
  if (!props.configInfo?.id) {
    topicMappingList.value = []
    return
  }

  topicMappingLoading.value = true
  try {
    const res = await getTopicMappingList({
      device_config_id: props.configInfo.id
    })
    const list = res.data.list
    topicMappingList.value = list.map((item: any) => normalizeTopicMapping(item))
  } catch {
    message.error(t('generate.topicMapping.message.fetchFailed'))
  } finally {
    topicMappingLoading.value = false
  }
}

// Topic 映射保存链路：弹窗表单 -> payload 收敛 -> 按是否有 id 选择新增/更新 -> 成功后回刷列表。
const handleSaveTopicMapping = async (data: TopicMapping) => {
  if (!props.configInfo?.id) return

  const payload = buildTopicMappingPayload(data)
  try {
    if (data.id) {
      await updateTopicMapping(data.id, payload)
      message.success(t('generate.topicMapping.message.updateSuccess'))
    } else {
      await createTopicMapping(payload)
      message.success(t('generate.topicMapping.message.createSuccess'))
    }
    currentEditTopicMapping.value = null
    await fetchTopicMappings()
  } catch {
    message.error(t('generate.topicMapping.message.saveFailed'))
  }
}

// 主连接配置保存链路：同步协议/凭据类型和动态协议配置，再复用 deviceConfigEdit 落库。
const handleSubmit = async () => {
  const postData = props.configInfo
  postData.protocol_type = extendForm.value.protocol_type
  postData.voucher_type = extendForm.value.voucher_type
  postData.protocol_config = JSON.stringify(protocol_config.value)

  const res = await deviceConfigEdit(postData)
  if (!res.error) {
    emit('upDateConfig')
  }
}

const showTopicMapping = ref(true)
// 协议插件表单查询链路：根据 device_type + protocol_type 拉取元数据，并解析是否展示 Topic 映射区块。
const getConfigForm = async (protocolType: string | number | null | undefined) => {
  const res = await protocolPluginConfigForm({
    device_type: props.configInfo.device_type,
    protocol_type: protocolType
  })
  const elements: FormElement[] = res.data || []
  const metaIdx = elements.findIndex((e) => e.dataKey === '__topic_mapping__')
  if (metaIdx !== -1) {
    showTopicMapping.value = (elements[metaIdx] as any).default !== 'false'
    formElements.value = elements.filter((e) => e.dataKey !== '__topic_mapping__')
  } else {
    showTopicMapping.value = true
    formElements.value = elements
  }
}

// 初始化时先回填协议配置，再拉取协议插件表单和 Topic 映射列表，保证编辑态数据完整。
onMounted(async () => {
  if (props.configInfo.protocol_config) {
    protocol_config.value = parseProtocolConfig(props.configInfo.protocol_config)
  }

  extendForm.value = props.configInfo
  await getConfigForm(extendForm.value.protocol_type)
  await fetchTopicMappings()
})

// 配置详情切换时，仅按 id 重新拉取 Topic 映射，避免旧设备的数据残留在当前表格里。
watch(
  () => props.configInfo?.id,
  async (newId) => {
    if (newId) {
      await fetchTopicMappings()
    } else {
      topicMappingList.value = []
    }
  }
)
</script>

<template>
  <div class="connection-box">
    <div class="text-18px">{{ $t('generate.through-protocol-access') }}</div>
    <ConnectionAccessPackageCard
      :summary-rows="connectionSummaryRows"
      :checklist="connectionChecklist"
      @copy-package="copyConnectionAccessPackage"
      @copy-config="copyMaskedProtocolConfig"
    />
    <NForm class="mt-4" :model="extendForm" :rules="extendFormRules" label-placement="left" label-width="auto">
      <NFormItem :label="$t('generate.choose-protocol-or-Service')" path="protocol_type" class="w-300">
        <NSelect
          v-model:value="extendForm.protocol_type"
          :placeholder="$t('generate.select-protocol-service')"
          label-field="name"
          :disabled="true"
          value-field="service_identifier"
        ></NSelect>
      </NFormItem>
      <NFormItem
        v-show="configInfo.device_type !== '3'"
        :label="$t('generate.authentication-type')"
        path="voucher_type"
        class="w-300"
      >
        <NInput
          v-if="props.configInfo.device_type !== 1"
          v-model:value="extendForm.voucher_type"
          :disabled="true"
          :placeholder="$t('generate.select-authentication-type')"
        />
      </NFormItem>
      <NFormItem v-if="formElements?.length > 0">
        <FormInput v-model:protocol-config="protocol_config" :form-elements="formElements" :edit="true"></FormInput>
      </NFormItem>
      <!-- Topic 映射 -->
      <NFormItem v-if="showTopicMapping" class="topic-mapping-form-item">
        <div class="topic-mapping-section">
          <div class="topic-mapping-header">
            <div class="text-18px">{{ t('generate.topicMapping.sectionTitle') }}</div>
            <NButton type="primary" @click="handleAddTopicMapping">{{ t('common._add') }}</NButton>
          </div>
          <NDataTable
            :columns="topicMappingColumns"
            :data="topicMappingList"
            :bordered="false"
            :loading="topicMappingLoading"
            class="topic-mapping-table"
          >
            <template #empty>
              <n-empty :description="$t('common.noData')" />
            </template>
          </NDataTable>
        </div>
      </NFormItem>
      <NFormItem>
        <NButton type="primary" @click="handleSubmit">{{ $t('common.save') }}</NButton>
      </NFormItem>
      <NFlex justify="flex-end"></NFlex>
    </NForm>
    <n-drawer v-model:show="active" height="90%" placement="bottom">
      <n-drawer-content :title="$t('generate.form-configuration')">
        <FormInput v-model:protocol-config="protocol_config" :form-elements="formElements"></FormInput>
      </n-drawer-content>
    </n-drawer>
    <!-- Topic 映射弹窗 -->
    <TopicMappingModal
      v-model:visible="topicMappingModalVisible"
      :edit-data="currentEditTopicMapping"
      :device-config-id="props.configInfo?.id"
      @save="handleSaveTopicMapping"
    />
  </div>
</template>

<style scoped lang="scss">
.connection-box {
  .connection-title {
    font-size: 15px;
    font-weight: bold;
    margin-bottom: 20px;
  }

  .w-300 {
    width: 400px;
  }
}

.table-label {
  font-weight: bold;
  margin-bottom: 10px;
}

.table-content {
  margin-left: 20px;
}

.table-item {
  margin-bottom: 8px;
}

.topic-mapping-form-item {
  margin-top: 8px !important;
}

.topic-mapping-section {
  width: 100%;

  .topic-mapping-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;

    .topic-mapping-title {
      font-size: 15px;
      font-weight: bold;
    }
  }

  .topic-mapping-table {
    width: 100%;
  }
}
</style>
