<!--
文件用途：Integration（TB-45 统一集成实体）管理页——连接器实例纳管与转换器绑定。
核心逻辑：
1. 集成实例分页列表（名称搜索 + connector_type/enabled 过滤），行内启停开关；
   分页/加载态/过期请求收口在 useListPage，行内启停为原地改写行字段（深响应行）；
2. 新建/编辑表单拆到 modules/integration-form-modal.vue：名称、连接器类型（opcua/snmp/plugin）、
   上下行转换器绑定（UPLINK/DOWNLINK 各自过滤）、绑定设备多选（落 config JSONB 的 device_ids 数组）
   与高级配置 JSON；绑定选项由页面在打开弹窗时加载并注入，保持“打开即刷新选项”的旧行为；
3. 删除走二次确认；启停/删除失败不静默（请求层统一报错提示）。
-->
<script setup lang="tsx">
import { onMounted, ref } from 'vue'
import { NButton, NPopconfirm, NSpace, NSwitch, NTag } from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import {
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
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import IntegrationFormModal from './modules/integration-form-modal.vue'

type QueryFormModel = {
  search: string
  connector_type: IntegrationConnectorType | null
  // NSelect 选项值只接受 string/number，启停过滤用 'true'/'false' 字符串承载布尔语义。
  enabled: 'true' | 'false' | null
}

// 列表查询主入口：分页、加载态与过期请求丢弃交给 useListPage。
const {
  query: filter,
  rows: integrations,
  loading,
  pagination,
  load: fetchIntegrations,
  setPage
} = useListPage<IntegrationItem, QueryFormModel>({
  initialQuery: () => ({ search: '', connector_type: null, enabled: null }),
  deepRows: true,
  fetcher: async (params) => {
    const response = await getIntegrationsList({
      page: params.page,
      page_size: params.page_size,
      search: params.search || undefined,
      connector_type: params.connector_type || undefined,
      enabled: params.enabled === null ? undefined : params.enabled === 'true'
    })
    return fromFlatResponse<IntegrationItem>(response)
  }
})

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

const handleSearch = () => {
  // 搜索固定回第一页，与旧 handleSearch 行为一致。
  void setPage(1)
}

const connectorTagType = (t: string) => {
  if (t === 'opcua') return 'success' as const
  if (t === 'snmp') return 'info' as const
  return 'warning' as const
}

// 转换器与设备选项：转换器按 UPLINK/DOWNLINK 拆分，设备选择器供 device_ids 绑定。
// 弹窗打开时刷新，供表单选择器使用；列表列内的转换器名映射复用同一份选项。
const uplinkOptions = ref<SelectOption[]>([])
const downlinkOptions = ref<SelectOption[]>([])
const deviceOptions = ref<SelectOption[]>([])

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
  uplinkOptions.value = toOption((converters.data?.list ?? []).filter((item) => item.type === 'UPLINK'), 'id', 'name')
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

const handleToggle = async (row: IntegrationItem, value: boolean) => {
  const { error } = await updateIntegration({ id: row.id, enabled: value })
  if (!error) {
    row.enabled = value
    window.$message?.success($t('page.integration.toggleSuccess'))
  }
}

const handleDelete = async (row: IntegrationItem) => {
  const { error } = await deleteIntegration(row.id)
  if (!error) {
    window.$message?.success($t('page.integration.deleteSuccess'))
    await fetchIntegrations()
  }
}

// ---- 表单弹窗编排 ----
const modalVisible = ref(false)
const editingRow = ref<IntegrationItem | null>(null)

const openCreate = () => {
  editingRow.value = null
  modalVisible.value = true
  void loadBindingOptions()
}

const openEdit = (row: IntegrationItem) => {
  editingRow.value = row
  modalVisible.value = true
  void loadBindingOptions()
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
            v-model:value="filter.search"
            :placeholder="$t('page.integration.searchPlaceholder')"
            clearable
            style="width: 200px"
            @keyup.enter="handleSearch"
          />
          <NSelect
            v-model:value="filter.connector_type"
            :options="connectorOptions"
            :placeholder="$t('page.integration.connectorType')"
            clearable
            style="width: 140px"
          />
          <NSelect
            v-model:value="filter.enabled"
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
        :pagination="pagination"
        :scroll-x="1100"
        :row-key="(row: IntegrationItem) => row.id"
      />
    </NCard>

    <IntegrationFormModal
      v-model:show="modalVisible"
      :integration="editingRow"
      :uplink-options="uplinkOptions"
      :downlink-options="downlinkOptions"
      :device-options="deviceOptions"
      @success="fetchIntegrations"
    />
  </div>
</template>

<style scoped></style>
