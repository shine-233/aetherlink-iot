<!-- 规则链列表页（ROADMAP B2）：分页表格 + 启用开关 + 新建/编辑/删除入口 -->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import { NButton, NPopconfirm, NSwitch } from 'naive-ui'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { ruleChainDelete, ruleChainList, ruleChainUpdate, type RuleChainRow } from '@/service/api'
import { $t } from '@/locales'

const router = useRouter()

const tableData = ref<RuleChainRow[]>([])
const loading = ref(false)

const query = reactive({
  keyword: '',
  page: 1,
  page_size: 20
})

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  itemCount: 0,
  onChange: (page: number) => {
    pagination.page = page
    query.page = page
    getTableData()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    query.page_size = pageSize
    getTableData()
  }
})

async function getTableData() {
  loading.value = true
  try {
    const { data, error } = await ruleChainList({ ...query })
    if (!error) {
      tableData.value = data?.list || []
      pagination.itemCount = data?.total || 0
    }
  } finally {
    loading.value = false
  }
}

function formatTime(value?: string) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '--'
}

async function toggleEnabled(row: RuleChainRow, enabled: boolean) {
  const graph = typeof row.graph === 'string' ? row.graph : JSON.stringify(row.graph ?? {})
  const { error } = await ruleChainUpdate({
    id: row.id,
    name: row.name,
    description: row.description ?? null,
    enabled,
    graph
  })
  if (!error) {
    window.$message?.success($t('common.operationSuccess'))
    getTableData()
  }
}

function openEditor(row?: RuleChainRow) {
  if (row) {
    router.push({ path: '/automation/rule-chain/edit', query: { id: row.id } })
  } else {
    router.push({ path: '/automation/rule-chain/edit' })
  }
}

async function handleDelete(row: RuleChainRow) {
  const { error } = await ruleChainDelete(row.id)
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'))
    getTableData()
  }
}

const columns: DataTableColumns<RuleChainRow> = [
  {
    title: () => $t('custom.rule_chain.name'),
    key: 'name',
    minWidth: 160
  },
  {
    title: () => $t('custom.rule_chain.description'),
    key: 'description',
    ellipsis: { tooltip: true },
    render: row => row.description || '--'
  },
  {
    title: () => $t('custom.rule_chain.enabled'),
    key: 'enabled',
    width: 90,
    render: row =>
      h(NSwitch, {
        value: row.enabled,
        onUpdateValue: (value: boolean) => toggleEnabled(row, value)
      })
  },
  {
    title: () => $t('custom.device_details.shadowCreatedAt'),
    key: 'updated_at',
    width: 170,
    render: row => formatTime(row.updated_at || row.created_at)
  },
  {
    title: () => $t('common.actions'),
    key: 'actions',
    width: 190,
    render: row =>
      h('div', { class: 'flex gap-2' }, [
        h(
          NButton,
          { size: 'small', onClick: () => openEditor(row) },
          { default: () => $t('common.edit') }
        ),
        h(
          NPopconfirm,
          { onPositiveClick: () => handleDelete(row) },
          {
            trigger: () =>
              h(
                NButton,
                { size: 'small', quaternary: true, type: 'error' },
                { default: () => $t('common.delete') }
              ),
            default: () => $t('custom.rule_chain.deleteConfirm')
          }
        )
      ])
  }
]

onMounted(getTableData)
</script>

<template>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-[#101014]">
    <n-card :bordered="false" class="rounded-8px" :title="$t('custom.rule_chain.title')">
      <template #header-extra>
        <n-button type="primary" @click="openEditor()">
          {{ $t('custom.rule_chain.create') }}
        </n-button>
      </template>

      <div class="mb-3 flex gap-3">
        <n-input
          v-model:value="query.keyword"
          :placeholder="$t('custom.rule_chain.searchPlaceholder')"
          style="width: 240px"
          clearable
          @clear="getTableData"
          @keyup.enter="getTableData"
        />
        <n-button secondary @click="getTableData">
          {{ $t('common.search') }}
        </n-button>
      </div>

      <n-data-table
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :pagination="pagination"
        :bordered="false"
        remote
        size="small"
      />
    </n-card>
  </div>
</template>
