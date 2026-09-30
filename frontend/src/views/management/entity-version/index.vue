<!--
  实体版本控制页（ROADMAP C7/TB-25）
  后端支持四类实体（board / rule_chain / device_config / calculated_field）：
  GET /entity_versions            —— 按 (entity_type, entity_id) 列版本历史
  POST /entity_versions           —— 为实体当前状态建快照（快照内容由后端读取，不接受前端传入）
  GET /entity_versions/:id        —— 版本详情（含完整快照）
  GET /entity_versions/:id/diff/:target_id —— 两份快照的 JSON 语义差异（点号路径列表）
  POST /entity_versions/:id/restore —— 恢复；dry_run=true 时只回显将写入的字段
  列表分页/加载态收口在 useListPage；版本差异对比弹窗拆到 modules/version-compare-modal.vue。
-->
<script setup lang="ts">
import { computed, h, ref } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NPopconfirm, NTag } from 'naive-ui'
import {
  entityVersionCreate,
  entityVersionGet,
  entityVersionList,
  entityVersionRestore,
  type EntityVersion,
  type EntityVersionEntityType
} from '@/service/api'
import { $t } from '@/locales'
import { useMessage } from 'naive-ui'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import VersionCompareModal from './modules/version-compare-modal.vue'

defineOptions({ name: 'ManagementEntityVersion' })

const message = useMessage()

/** 与后端 resolveEntityTable 白名单一一对应；改动需同步后端。 */
const ENTITY_TYPES: EntityVersionEntityType[] = ['board', 'rule_chain', 'device_config', 'calculated_field']

const typeOptions = computed(() => ENTITY_TYPES.map((value) => ({ label: value, value })))

// ---------- 查询条件与列表 ----------
const hasQueried = ref(false)
const creating = ref(false)

const { query: filter, rows: tableData, loading, pagination, load: getTableData } = useListPage<
  EntityVersion,
  { entity_type: EntityVersionEntityType | string; entity_id: string }
>({
  initialQuery: () => ({ entity_type: 'board', entity_id: '' }),
  pageSizes: [10, 20, 50],
  fetcher: async (params) => {
    // 查询契约要求显式 entity_id；缺失时提示且不发起请求。
    if (!params.entity_id.trim()) {
      message.warning($t('custom.entityVersion.entityId'))
      return null
    }
    const response = await entityVersionList({
      entity_type: params.entity_type,
      entity_id: params.entity_id.trim(),
      page: params.page,
      page_size: params.page_size
    })
    return fromFlatResponse<EntityVersion>(response)
  },
  onLoaded: () => {
    hasQueried.value = true
  }
})

async function handleCreateSnapshot() {
  if (!filter.entity_id.trim()) {
    message.warning($t('custom.entityVersion.entityId'))
    return
  }
  creating.value = true
  try {
    const { error } = await entityVersionCreate({
      entity_type: filter.entity_type,
      entity_id: filter.entity_id.trim()
    })
    if (!error) {
      message.success($t('common.operationSuccess'))
      await getTableData()
    }
  } finally {
    creating.value = false
  }
}

// ---------- 快照详情 ----------
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailRaw = ref('')

async function openDetail(row: EntityVersion) {
  detailVisible.value = true
  detailLoading.value = true
  detailRaw.value = ''
  try {
    const { data, error } = await entityVersionGet(row.id)
    if (!error) {
      detailRaw.value = JSON.stringify(data, null, 2)
    }
  } finally {
    detailLoading.value = false
  }
}

// ---------- 版本差异对比（TB-25：快照 JSON 语义 diff，弹窗内实现） ----------
const compareVisible = ref(false)
const compareSource = ref<EntityVersion | null>(null)

function openCompare(row: EntityVersion) {
  compareSource.value = row
  compareVisible.value = true
}

// ---------- 恢复 ----------
async function handleRestore(row: EntityVersion) {
  // 先 dry_run 回显将写入的字段，确认后再真实恢复，避免误覆盖。
  const { error } = await entityVersionRestore(row.id, false)
  if (!error) {
    message.success($t('custom.entityVersion.restored'))
    await getTableData()
  }
}

const columns = computed<DataTableColumns<EntityVersion>>(() => [
  {
    title: () => $t('custom.entityVersion.name'),
    key: 'version_number',
    width: 110,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => `v${row.version_number}` })
  },
  {
    title: () => $t('custom.entityVersion.entityType'),
    key: 'entity_type',
    width: 150
  },
  {
    title: () => $t('custom.entityVersion.entityId'),
    key: 'entity_id',
    minWidth: 200,
    ellipsis: { tooltip: true }
  },
  {
    title: () => $t('common.remark'),
    key: 'remark',
    minWidth: 160,
    render: (row) => row.remark || '--'
  },
  {
    title: () => $t('custom.asset.createdAt'),
    key: 'created_at',
    width: 180,
    render: (row) => row.created_at?.replace('T', ' ').slice(0, 19) || '--'
  },
  {
    title: () => $t('common.actions'),
    key: 'actions',
    width: 250,
    render: (row) =>
      h('div', { class: 'flex gap-2' }, [
        h(
          NButton,
          { size: 'small', secondary: true, onClick: () => openDetail(row) },
          { default: () => $t('custom.entityVersion.content') }
        ),
        h(
          NButton,
          { size: 'small', secondary: true, type: 'info', onClick: () => openCompare(row) },
          { default: () => $t('custom.entityVersion.compare') }
        ),
        h(
          NPopconfirm,
          { onPositiveClick: () => handleRestore(row) },
          {
            trigger: () =>
              h(NButton, { size: 'small', type: 'primary' }, { default: () => $t('custom.entityVersion.restore') }),
            default: () => $t('custom.entityVersion.restoreConfirm')
          }
        )
      ])
  }
])
</script>

<template>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-[#101014]">
    <n-card :bordered="false" class="rounded-8px" :title="$t('custom.entityVersion.title')">
      <div class="mb-3 flex flex-wrap items-center gap-3">
        <n-select
          v-model:value="filter.entity_type"
          :options="typeOptions"
          :placeholder="$t('custom.entityVersion.entityType')"
          style="width: 190px"
        />
        <n-input
          v-model:value="filter.entity_id"
          :placeholder="$t('custom.entityVersion.entityId')"
          style="width: 280px"
          clearable
          @keyup.enter="getTableData"
        />
        <n-button secondary @click="getTableData">{{ $t('common.search') }}</n-button>
        <n-button type="primary" :loading="creating" @click="handleCreateSnapshot">
          {{ $t('custom.entityVersion.create') }}
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

      <n-empty
        v-if="!loading && hasQueried && tableData.length === 0"
        class="py-6"
        :description="$t('custom.entityVersion.empty')"
      />
    </n-card>

    <!-- 快照详情 -->
    <n-modal
      v-model:show="detailVisible"
      preset="card"
      style="width: 720px"
      :title="$t('custom.entityVersion.content')"
    >
      <n-spin :show="detailLoading">
        <n-code v-if="detailRaw" :code="detailRaw" language="json" word-wrap />
        <n-empty v-else :description="$t('custom.entityVersion.empty')" />
      </n-spin>
    </n-modal>

    <!-- 版本差异对比：左右两栏快照 JSON + 变更列表 -->
    <VersionCompareModal
      v-model:show="compareVisible"
      :source="compareSource"
      :entity-type="filter.entity_type"
      :entity-id="filter.entity_id.trim()"
    />
  </div>
</template>
