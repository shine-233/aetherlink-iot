<!--
  文件用途：实体版本差异对比弹窗（TB-25，从 index.vue 拆出）。
  核心逻辑：打开时拉取同一实体的全部版本作为候选目标（page_size 取 DAL 上限 500 一次拉全），
  选择目标后请求两份快照的 JSON 语义 diff（点号路径列表），左右两栏展示快照原文 + 变更列表。
  查询上下文（entity_type/entity_id）由页面传入，弹窗自身不做列表分页。
-->
<script setup lang="ts">
import { computed, h, ref, shallowRef, watch } from 'vue'
import { NTag } from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { entityVersionDiff, entityVersionList, type EntityVersion, type EntityVersionDiffChange } from '@/service/api'
import type { EntityVersionDiffResult, EntityVersionEntityType } from '@/service/api'
import { $t } from '@/locales'

defineOptions({ name: 'VersionCompareModal' })

const props = defineProps<{
  show: boolean
  /** 对比源版本；null 时弹窗不展示。 */
  source: EntityVersion | null
  entityType: EntityVersionEntityType | string
  entityId: string
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const compareLoading = ref(false)
const candidateLoading = ref(false)
const compareTargetId = ref<string | null>(null)
// 候选列表（最多 500 条）与 diff 结果只做整体替换，用 shallowRef 免去逐字段深度代理。
const compareCandidates = shallowRef<SelectOption[]>([])
const diffResult = shallowRef<EntityVersionDiffResult | null>(null)
const sourceSnapshotPretty = ref('')
const targetSnapshotPretty = ref('')

const compareSourceLabel = computed(() => (props.source ? `v${props.source.version_number}` : '--'))

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
    ellipsis: { tooltip: true },
    render: (row) => formatDiffValue(row.old_value)
  },
  {
    title: () => $t('custom.entityVersion.compareNewValue'),
    key: 'new_value',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row) => formatDiffValue(row.new_value)
  }
])

/** 打开弹窗：重置目标与 diff，并拉取候选版本列表。 */
async function loadCandidates(row: EntityVersion) {
  candidateLoading.value = true
  try {
    const { data, error } = await entityVersionList({
      entity_type: props.entityType,
      entity_id: props.entityId,
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

async function loadDiff(targetId: string) {
  if (!props.source) return
  compareLoading.value = true
  try {
    const { data, error } = await entityVersionDiff(props.source.id, targetId)
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

function handleCompareTargetChange(targetId: string | null) {
  if (!targetId || !props.source) {
    diffResult.value = null
    return
  }
  void loadDiff(targetId)
}

watch(
  () => props.show,
  (show) => {
    if (!show || !props.source) return
    compareTargetId.value = null
    diffResult.value = null
    sourceSnapshotPretty.value = prettySnapshot(props.source.snapshot)
    targetSnapshotPretty.value = ''
    void loadCandidates(props.source)
  }
)
</script>

<template>
  <n-modal
    :show="props.show"
    preset="card"
    style="width: 1100px"
    :title="$t('custom.entityVersion.compareTitle')"
    @update:show="emit('update:show', $event)"
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
        <n-empty v-if="diffResult.total === 0" class="py-6" :description="$t('custom.entityVersion.compareNone')" />
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
            :row-key="(row: EntityVersionDiffChange) => row.path"
            virtual-scroll
            :min-row-height="38"
          />
        </template>
      </template>
    </n-spin>
  </n-modal>
</template>

<style scoped></style>
