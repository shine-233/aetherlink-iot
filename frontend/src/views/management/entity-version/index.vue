<!--
  实体版本控制页（ROADMAP C7/TB-25）
  后端支持四类实体（board / rule_chain / device_config / calculated_field）：
  GET /entity_versions            —— 按 (entity_type, entity_id) 列版本历史
  POST /entity_versions           —— 为实体当前状态建快照（快照内容由后端读取，不接受前端传入）
  GET /entity_versions/:id        —— 版本详情（含完整快照）
  GET /entity_versions/:id/diff/:target_id —— 两份快照的 JSON 语义差异（点号路径列表）
  POST /entity_versions/:id/restore —— 恢复；dry_run=true 时只回显将写入的字段
-->
<script setup lang="ts">
import { computed, h, reactive, ref } from 'vue'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { NButton, NEmpty, NInput, NPopconfirm, NSelect, NTag, useMessage } from 'naive-ui'
import {
  entityVersionCreate,
  entityVersionDiff,
  entityVersionGet,
  entityVersionList,
  entityVersionRestore,
  type EntityVersion,
  type EntityVersionDiffChange,
  type EntityVersionDiffResult,
  type EntityVersionEntityType
} from '@/service/api'
import { $t } from '@/locales'

defineOptions({ name: 'ManagementEntityVersion' })

const message = useMessage()

/** 与后端 resolveEntityTable 白名单一一对应；改动需同步后端。 */
const ENTITY_TYPES: EntityVersionEntityType[] = ['board', 'rule_chain', 'device_config', 'calculated_field']

const typeOptions = computed<SelectOption[]>(() => ENTITY_TYPES.map((value) => ({ label: value, value })))

// ---------- 查询条件 ----------
const filter = reactive({
  entity_type: 'board' as EntityVersionEntityType | string,
  entity_id: ''
})

const hasQueried = ref(false)
const tableData = ref<EntityVersion[]>([])
const loading = ref(false)
const creating = ref(false)

const pagination = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  itemCount: 0,
  onChange: (page: number) => {
    pagination.page = page
    getTableData()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    getTableData()
  }
})

async function getTableData() {
  if (!filter.entity_id.trim()) {
    message.warning($t('custom.entityVersion.entityId'))
    return
  }
  loading.value = true
  try {
    const { data, error } = await entityVersionList({
      entity_type: filter.entity_type,
      entity_id: filter.entity_id.trim(),
      page: pagination.page,
      page_size: pagination.pageSize
    })
    if (!error) {
      hasQueried.value = true
      tableData.value = (data as any)?.list || []
      pagination.itemCount = (data as any)?.total || 0
    }
  } finally {
    loading.value = false
  }
}

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
      getTableData()
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

// ---------- 版本差异对比（TB-25：快照 JSON 语义 diff） ----------
const compareVisible = ref(false)
const compareLoading = ref(false)
const candidateLoading = ref(false)
const compareSource = ref<EntityVersion | null>(null)
const compareTargetId = ref<string | null>(null)
const compareCandidates = ref<SelectOption[]>([])
const diffResult = ref<EntityVersionDiffResult | null>(null)
const sourceSnapshotPretty = ref('')
const targetSnapshotPretty = ref('')

const compareSourceLabel = computed(() =>
  compareSource.value ? `v${compareSource.value.version_number}` : '--'
)

/** 快照文本格式化为缩进 JSON；解析失败按原文展示（快照本身由后端生成，一般不会失败）。 */
function prettySnapshot(raw?: string | null): string {
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function formatDiffValue(value: unknown): string {
  if (value === undefined || value === null) return '--'
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const KIND_META: Record<string, { type: 'success' | 'error' | 'warning'; labelKey: string }> = {
  added: { type: 'success', labelKey: 'custom.entityVersion.compareAdded' },
  removed: { type: 'error', labelKey: 'custom.entityVersion.compareRemoved' },
  modified: { type: 'warning', labelKey: 'custom.entityVersion.compareModified' }
}

const diffColumns = computed<DataTableColumns<EntityVersionDiffChange>>(() => [
  {
    title: () => $t('custom.entityVersion.comparePath'),
    key: 'path',
    minWidth: 200,
    ellipsis: { tooltip: true }
  },
  {
    title: () => $t('custom.entityVersion.compareKind'),
    key: 'kind',
    width: 110,
    render: (row) => {
      const meta = KIND_META[row.kind]
      return h(
        NTag,
        { size: 'small', type: meta?.type || 'default', bordered: false },
        { default: () => (meta ? $t(meta.labelKey) : row.kind) }
      )
    }
  },
  {
    title: () => $t('custom.entityVersion.compareOldValue'),
    key: 'old_value',
    minWidth: 160,
    render: (row) => formatDiffValue(row.old_value)
  },
  {
    title: () => $t('custom.entityVersion.compareNewValue'),
    key: 'new_value',
    minWidth: 160,
    render: (row) => formatDiffValue(row.new_value)
  }
])

/** 打开对比弹窗：候选目标为同一实体的全部其他版本（page_size 取 DAL 上限 500 一次拉全）。 */
async function openCompare(row: EntityVersion) {
  compareSource.value = row
  compareTargetId.value = null
  diffResult.value = null
  sourceSnapshotPretty.value = prettySnapshot(row.snapshot)
  targetSnapshotPretty.value = ''
  compareVisible.value = true

  candidateLoading.value = true
  try {
    const { data, error } = await entityVersionList({
      entity_type: filter.entity_type,
      entity_id: filter.entity_id.trim(),
      page: 1,
      page_size: 500
    })
    if (!error) {
      const list = ((data as any)?.list || []) as EntityVersion[]
      compareCandidates.value = list
        .filter((item) => item.id !== row.id)
        .map((item) => ({
          label: `v${item.version_number}${item.remark ? ` · ${item.remark}` : ''}`,
          value: item.id
        }))
    }
  } finally {
    candidateLoading.value = false
  }
}

function handleCompareTargetChange(targetId: string | null) {
  if (!targetId || !compareSource.value) {
    diffResult.value = null
    return
  }
  loadDiff(targetId)
}

async function loadDiff(targetId: string) {
  if (!compareSource.value) return
  compareLoading.value = true
  try {
    const { data, error } = await entityVersionDiff(compareSource.value.id, targetId)
    if (!error) {
      const payload = data as any
      diffResult.value = payload?.diff || null
      sourceSnapshotPretty.value = prettySnapshot(payload?.source?.snapshot)
      targetSnapshotPretty.value = prettySnapshot(payload?.target?.snapshot)
    }
  } finally {
    compareLoading.value = false
  }
}

// ---------- 恢复 ----------
async function handleRestore(row: EntityVersion) {
  // 先 dry_run 回显将写入的字段，确认后再真实恢复，避免误覆盖。
  const { data, error } = await entityVersionRestore(row.id, false)
  if (!error) {
    message.success($t('custom.entityVersion.restored'))
    getTableData()
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
    <n-modal
      v-model:show="compareVisible"
      preset="card"
      style="width: 1100px"
      :title="$t('custom.entityVersion.compareTitle')"
    >
      <n-spin :show="compareLoading">
        <div class="mb-3 flex flex-wrap items-center gap-3">
          <n-tag size="small" type="info" :bordered="false">
            {{ $t('custom.entityVersion.compareSource') }}: {{ compareSourceLabel }}
          </n-tag>
          <n-select
            v-model:value="compareTargetId"
            :options="compareCandidates"
            :placeholder="$t('custom.entityVersion.compareTargetPlaceholder')"
            :loading="candidateLoading"
            :disabled="compareCandidates.length === 0"
            clearable
            style="width: 320px"
            @update:value="handleCompareTargetChange"
          />
        </div>

        <n-empty
          v-if="compareCandidates.length === 0"
          class="py-6"
          :description="$t('custom.entityVersion.compareNoTarget')"
        />
        <template v-else-if="diffResult">
          <n-empty
            v-if="diffResult.total === 0"
            class="py-6"
            :description="$t('custom.entityVersion.compareNone')"
          />
          <template v-else>
            <div class="mb-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
              <div class="min-w-0">
                <div class="mb-1 text-sm font-bold">
                  {{ $t('custom.entityVersion.compareSource') }} ({{ compareSourceLabel }})
                </div>
                <n-code v-if="sourceSnapshotPretty" :code="sourceSnapshotPretty" language="json" word-wrap />
              </div>
              <div class="min-w-0">
                <div class="mb-1 text-sm font-bold">
                  {{ $t('custom.entityVersion.compareTarget') }}
                </div>
                <n-code v-if="targetSnapshotPretty" :code="targetSnapshotPretty" language="json" word-wrap />
              </div>
            </div>
            <div class="mb-1 text-sm font-bold">
              {{ $t('custom.entityVersion.compareChanges') }} ({{ diffResult.total }})
            </div>
            <n-data-table
              :columns="diffColumns"
              :data="diffResult.changes"
              :bordered="false"
              size="small"
              :max-height="320"
            />
          </template>
        </template>
      </n-spin>
    </n-modal>
  </div>
</template>
