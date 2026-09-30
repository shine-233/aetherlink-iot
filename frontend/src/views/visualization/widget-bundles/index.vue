<!--
文件用途：部件库管理页（widget_bundles，ROADMAP TB-04）——可视化域的部件库 CRUD 工作台。
核心逻辑：
1. 部件库列表分页检索（按名称/描述模糊搜索），列表状态（分页/加载/过期请求丢弃）收口在 useListPage；
2. 部件库创建/编辑表单拆到 modules/widget-bundle-form-modal.vue（含部件定义 JSON 本地校验）；
3. 内置四部件（gauge/chart/valve/twin3d）预览与一键种子导入拆到 modules/widget-builtin-drawer.vue。
关键注意事项：部件定义会被画布渲染与命令下发直接消费，JSON 必须是对象数组且逐项含 type/version/schema/capabilities；
  后端 service 层复用 ValidateWidgetDefinition 做最终校验，前端校验只做快速反馈。
-->
<script setup lang="tsx">
import { onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable, NInput, NPopconfirm, NSpace, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  deleteWidgetBundle,
  getBuiltinWidgetBundle,
  getWidgetBundlesList,
  type WidgetBundleExport,
  type WidgetBundleItem
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import { widgetCountOf } from './modules/widget-bundle'
import WidgetBundleBuiltinDrawer from './modules/widget-builtin-drawer.vue'
import WidgetBundleFormModal from './modules/widget-bundle-form-modal.vue'

const message = useMessage()

interface ListQuery {
  search: string
}

// 列表查询主入口：分页、加载态与过期请求丢弃交给 useListPage。
const {
  query: listFilter,
  rows: bundles,
  loading,
  pagination,
  rowKey,
  load: fetchBundles,
  search: runSearch
} = useListPage<WidgetBundleItem, ListQuery>({
  initialQuery: () => ({ search: '' }),
  fetcher: async (params) => {
    const response = await getWidgetBundlesList({
      page: params.page,
      page_size: params.page_size,
      search: params.search || undefined
    })
    return fromFlatResponse<WidgetBundleItem>(response)
  }
})

const handleSearch = () => {
  void runSearch()
}

const modalVisible = ref(false)
const editingRow = ref<WidgetBundleItem | null>(null)

const openCreate = () => {
  editingRow.value = null
  modalVisible.value = true
}

const openEdit = (row: WidgetBundleItem) => {
  editingRow.value = row
  modalVisible.value = true
}

const handleSaved = () => {
  void fetchBundles()
}

const builtinVisible = ref(false)
const builtinExport = ref<WidgetBundleExport | null>(null)

const openBuiltin = async () => {
  const { data, error } = await getBuiltinWidgetBundle()
  if (!error && data) {
    builtinExport.value = data
    builtinVisible.value = true
  }
}

const handleSeeded = () => {
  void fetchBundles()
}

const handleDelete = async (row: WidgetBundleItem) => {
  const { error } = await deleteWidgetBundle(row.id)
  if (!error) {
    message.success($t('page.widgetBundle.deleteSuccess'))
    await fetchBundles()
  }
}

onMounted(() => {
  void fetchBundles()
})

const columns: DataTableColumns<WidgetBundleItem> = [
  { title: $t('page.widgetBundle.name'), key: 'name', minWidth: 160 },
  { title: $t('page.widgetBundle.version'), key: 'version', width: 90 },
  {
    title: $t('page.widgetBundle.typeKey'),
    key: 'type_key',
    width: 110,
    render: (row) => row.type_key || '-'
  },
  {
    title: $t('page.widgetBundle.widgetCount'),
    key: 'widget_count',
    width: 110,
    render: (row) => widgetCountOf(row.widgets ?? '[]')
  },
  {
    title: $t('page.widgetBundle.description'),
    key: 'description',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row) => row.description || '-'
  },
  {
    title: $t('page.widgetBundle.createdAt'),
    key: 'created_at',
    width: 170,
    render: (row) => (row.created_at ? formatDateTime(row.created_at) : '-')
  },
  {
    title: $t('page.widgetBundle.actions'),
    key: 'actions',
    width: 180,
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
            default: () => $t('page.widgetBundle.deleteConfirm')
          }}
        </NPopconfirm>
      </NSpace>
    )
  }
]
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.visualization_widget-bundles')" :bordered="false">
      <template #header-extra>
        <NSpace>
          <NInput
            v-model:value="listFilter.search"
            :placeholder="$t('page.widgetBundle.searchPlaceholder')"
            clearable
            style="width: 220px"
            @keyup.enter="handleSearch"
          />
          <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
          <NButton @click="openBuiltin">{{ $t('page.widgetBundle.builtinPreview') }}</NButton>
          <NButton type="primary" @click="openCreate">{{ $t('page.widgetBundle.create') }}</NButton>
        </NSpace>
      </template>

      <NDataTable
        remote
        :columns="columns"
        :data="bundles"
        :loading="loading"
        :pagination="pagination"
        :scroll-x="900"
        :row-key="rowKey"
      />
    </NCard>

    <WidgetBundleFormModal v-model:show="modalVisible" :editing="editingRow" @saved="handleSaved" />

    <WidgetBundleBuiltinDrawer v-model:show="builtinVisible" :builtin-export="builtinExport" @seeded="handleSeeded" />
  </div>
</template>

<style scoped></style>
