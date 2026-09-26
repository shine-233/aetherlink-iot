<!--
文件用途：Integration（TB-45 统一集成实体）管理页——连接器实例纳管与转换器绑定。
核心逻辑：
1. 集成实例分页列表（名称搜索 + connector_type/enabled 过滤），行内启停开关；
2. 新建/编辑表单：名称、连接器类型（opcua/snmp/plugin）、上下行转换器绑定（UPLINK/DOWNLINK 各自过滤）、
   绑定设备多选（落 config JSONB 的 device_ids 数组，与采集器管线约定一致）与高级配置 JSON；
3. 删除走二次确认；启停/删除失败不静默（请求层统一报错提示）。
-->
<script setup lang="tsx">
import { onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules, SelectOption } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import {
  createIntegration,
  deleteIntegration,
  getIntegrationsList,
  updateIntegration,
  getDeviceListForSelect,
  type IntegrationConnectorType,
  type IntegrationItem
} from '@/service/api'
import { getDataConvertersList } from '@/service/api/data-converter'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const message = useMessage()
const { loading, startLoading, endLoading } = useLoading(false)
const integrations = ref<IntegrationItem[]>([])
const searchValue = ref('')
const filterConnector = ref<IntegrationConnectorType | null>(null)
// NSelect 选项值只接受 string/number，启停过滤用 'true'/'false' 字符串承载布尔语义。
const filterEnabled = ref<'true' | 'false' | null>(null)
const pagination = reactive({ page: 1, pageSize: 10, itemCount: 0 })

const modalVisible = ref(false)
const submitting = ref(false)
const editingId = ref('')
const formRef = ref<FormInst | null>(null)
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

// 连接器类型选项：与后端 CHECK 约束（130.sql）保持一致。
const connectorOptions: SelectOption[] = [
  { label: $t('page.integration.connector.opcua'), value: 'opcua' },
  { label: $t('page.integration.connector.snmp'), value: 'snmp' },
  { label: $t('page.integration.connector.plugin'), value: 'plugin' }
]

const enabledFilterOptions: SelectOption[] = [
  { label: $t('page.integration.enabledOn'), value: 'true' },
  { label: $t('page.integration.enabledOff'), value: 'false' }
]

// 转换器与设备选项：转换器按 UPLINK/DOWNLINK 拆分，设备选择器供 device_ids 绑定。
const uplinkOptions = ref<SelectOption[]>([])
const downlinkOptions = ref<SelectOption[]>([])
const deviceOptions = ref<SelectOption[]>([])

const connectorTagType = (t: string) => {
  if (t === 'opcua') return 'success' as const
  if (t === 'snmp') return 'info' as const
  return 'warning' as const
}

/** 绑定转换器 ID -> 展示名：选项未加载时回退为 ID，未绑定为“-”。 */
const converterLabel = (id: string | null | undefined) => {
  if (!id) return $t('page.integration.unbound')
  const option = [...uplinkOptions.value, ...downlinkOptions.value].find((item) => item.value === id)
  return (option?.label as string) || id
}

const deviceCount = (configJSON: string) => {
  try {
    const parsed = JSON.parse(configJSON || '{}') as { device_ids?: string[] }
    return parsed.device_ids?.length ?? 0
  } catch {
    return 0
  }
}

const columns: DataTableColumns<IntegrationItem> = [
  { title: $t('page.integration.name'), key: 'name', minWidth: 140 },
  {
    title: $t('page.integration.connectorType'),
    key: 'connector_type',
    width: 110,
    render: (row) => (
      <NTag size="small" type={connectorTagType(row.connector_type)}>
        {row.connector_type}
      </NTag>
    )
  },
  {
    title: $t('page.integration.converterUplink'),
    key: 'converter_uplink_id',
    minWidth: 150,
    render: (row) => converterLabel(row.converter_uplink_id)
  },
  {
    title: $t('page.integration.converterDownlink'),
    key: 'converter_downlink_id',
    minWidth: 150,
    render: (row) => converterLabel(row.converter_downlink_id)
  },
  {
    title: $t('page.integration.deviceCount'),
    key: 'device_count',
    width: 110,
    render: (row) => deviceCount(row.config)
  },
  {
    title: $t('page.integration.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) => (
      <NSwitch value={row.enabled} size="small" onUpdateValue={(value: boolean) => handleToggle(row, value)} />
    )
  },
  {
    title: $t('page.integration.createdAt'),
    key: 'created_at',
    width: 170,
    render: (row) => (row.created_at ? formatDateTime(row.created_at) : '-')
  },
  {
    title: $t('page.integration.actions'),
    key: 'actions',
    width: 170,
    render: (row) => (
      <NSpace>
        <NButton size="small" onClick={() => openEdit(row)}>
          {$t('common.edit')}
        </NButton>
        <NPopconfirm onPositiveClick={() => handleDelete(row)}>
          {{
            trigger: () => (
              <NButton size="small" type="error" ghost>
                {$t('common.delete')}
              </NButton>
            ),
            default: () => $t('page.integration.deleteConfirm')
          }}
        </NPopconfirm>
      </NSpace>
    )
  }
]

const fetchIntegrations = async () => {
  startLoading()
  try {
    const { data, error } = await getIntegrationsList({
      page: pagination.page,
      page_size: pagination.pageSize,
      search: searchValue.value || undefined,
      connector_type: filterConnector.value || undefined,
      enabled: filterEnabled.value === null ? undefined : filterEnabled.value === 'true'
    })
    if (!error && data) {
      integrations.value = data.list ?? []
      pagination.itemCount = data.total ?? 0
    }
  } finally {
    endLoading()
  }
}

const handleSearch = () => {
  pagination.page = 1
  void fetchIntegrations()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void fetchIntegrations()
}

const loadBindingOptions = async () => {
  const [converters, devices] = await Promise.all([
    getDataConvertersList({ page: 1, page_size: 200 }),
    getDeviceListForSelect({ page: '1', page_size: '200' })
  ])
  const toOption = (
    list: Array<{ id?: string; device_id?: string; name?: string; device_name?: string }>,
    key: 'id' | 'device_id',
    label: 'name' | 'device_name'
  ) =>
    (list ?? [])
      .filter((item) => item[key])
      .map((item) => ({ label: item[label] || String(item[key]), value: String(item[key]) }))
  uplinkOptions.value = toOption(
    (converters.data?.list ?? []).filter((item) => item.type === 'UPLINK'),
    'id',
    'name'
  )
  downlinkOptions.value = toOption(
    (converters.data?.list ?? []).filter((item) => item.type === 'DOWNLINK'),
    'id',
    'name'
  )
  deviceOptions.value = (devices.data?.list ?? []).map((item) => ({
    label: item.device_name || item.device_id,
    value: item.device_id
  }))
}

/** config JSONB 与表单互转：device_ids 由设备多选托管，其余键保留在 configExtra。 */
const configFromForm = () => {
  let extra: Record<string, unknown> = {}
  if (formModel.configExtra.trim() !== '') {
    extra = JSON.parse(formModel.configExtra) as Record<string, unknown>
  }
  extra.device_ids = formModel.deviceIds
  return JSON.stringify(extra)
}

const applyConfigToForm = (configJSON: string) => {
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

const openCreate = () => {
  editingId.value = ''
  Object.assign(formModel, {
    name: '',
    connector_type: 'opcua',
    converter_uplink_id: null,
    converter_downlink_id: null,
    deviceIds: [],
    configExtra: '',
    enabled: true
  })
  modalVisible.value = true
  void loadBindingOptions()
}

const openEdit = (row: IntegrationItem) => {
  editingId.value = row.id
  Object.assign(formModel, {
    name: row.name,
    connector_type: row.connector_type,
    converter_uplink_id: row.converter_uplink_id ?? null,
    converter_downlink_id: row.converter_downlink_id ?? null,
    enabled: row.enabled
  })
  applyConfigToForm(row.config)
  modalVisible.value = true
  void loadBindingOptions()
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
    const { error } = editingId.value
      ? await updateIntegration({ id: editingId.value, ...payload })
      : await createIntegration(payload)
    if (!error) {
      message.success($t('page.integration.saveSuccess'))
      modalVisible.value = false
      await fetchIntegrations()
    }
  } finally {
    submitting.value = false
  }
}

const handleToggle = async (row: IntegrationItem, value: boolean) => {
  const { error } = await updateIntegration({ id: row.id, enabled: value })
  if (!error) {
    row.enabled = value
    message.success($t('page.integration.toggleSuccess'))
  }
}

const handleDelete = async (row: IntegrationItem) => {
  const { error } = await deleteIntegration(row.id)
  if (!error) {
    message.success($t('page.integration.deleteSuccess'))
    await fetchIntegrations()
  }
}

onMounted(() => {
  void fetchIntegrations()
})
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.integration_list')" :bordered="false">
      <template #header-extra>
        <NSpace>
          <NInput
            v-model:value="searchValue"
            :placeholder="$t('page.integration.searchPlaceholder')"
            clearable
            style="width: 200px"
            @keyup.enter="handleSearch"
          />
          <NSelect
            v-model:value="filterConnector"
            :options="connectorOptions"
            :placeholder="$t('page.integration.connectorType')"
            clearable
            style="width: 140px"
          />
          <NSelect
            v-model:value="filterEnabled"
            :options="enabledFilterOptions"
            :placeholder="$t('page.integration.enabled')"
            clearable
            style="width: 120px"
          />
          <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
          <NButton type="primary" @click="openCreate">{{ $t('page.integration.create') }}</NButton>
        </NSpace>
      </template>

      <NDataTable
        remote
        :columns="columns"
        :data="integrations"
        :loading="loading"
        :pagination="{ ...pagination, onChange: handlePageChange, 'onUpdate:pageSize': (size: number) => (pagination.pageSize = size) }"
        :scroll-x="1100"
        :row-key="(row: IntegrationItem) => row.id"
      />
    </NCard>

    <NModal
      v-model:show="modalVisible"
      preset="card"
      :title="editingId ? $t('page.integration.editTitle') : $t('page.integration.createTitle')"
      class="w-640px"
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
            :options="uplinkOptions"
            :placeholder="$t('page.integration.unbound')"
            clearable
            filterable
          />
        </NFormItem>
        <NFormItem :label="$t('page.integration.converterDownlink')" path="converter_downlink_id">
          <NSelect
            v-model:value="formModel.converter_downlink_id"
            :options="downlinkOptions"
            :placeholder="$t('page.integration.unbound')"
            clearable
            filterable
          />
        </NFormItem>
        <NFormItem :label="$t('page.integration.devices')" path="deviceIds">
          <NSelect
            v-model:value="formModel.deviceIds"
            :options="deviceOptions"
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
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped></style>
