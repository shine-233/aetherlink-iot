<!--
  资产层级页（ROADMAP C2）
  左侧资产树（GET /asset/tree，租户作用域内递归）+ 右侧当前节点子资产列表（GET /asset/list）。
  后端已完成租户作用域（self∪子孙）与成环拒绝校验，本页只负责呈现与错误出口展示。
-->
<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import type { DataTableColumns, FormInst, FormRules, TreeOption } from 'naive-ui'
import {
  NButton,
  NEmpty,
  NInput,
  NPopconfirm,
  NSelect,
  NTag,
  useMessage
} from 'naive-ui'
import {
  assetCreate,
  assetDelete,
  assetList,
  assetTree,
  assetUpdate,
  type Asset,
  type AssetTreeNode
} from '@/service/api'
import { $t } from '@/locales'

defineOptions({ name: 'DeviceAsset' })

const message = useMessage()

const ASSET_TYPES = ['device', 'group', 'space', 'custom'] as const

function typeLabel(value: string) {
  const map: Record<string, () => string> = {
    device: () => $t('custom.asset.typeDevice'),
    group: () => $t('custom.asset.typeGroup'),
    space: () => $t('custom.asset.typeSpace'),
    custom: () => $t('custom.asset.typeCustom')
  }
  return (map[value] || map.custom)()
}

// ---------- 资产树 ----------
const treeLoading = ref(false)
const treeData = ref<TreeOption[]>([])

/** 把后端 AssetTreeNode 递归转换为 n-tree 的 { label, key, children } 结构。 */
function toTreeOptions(nodes: AssetTreeNode[]): TreeOption[] {
  return (nodes || []).map(node => ({
    key: node.id,
    label: node.name,
    children: node.children && node.children.length > 0 ? toTreeOptions(node.children) : undefined
  }))
}

async function loadTree() {
  treeLoading.value = true
  try {
    const { data, error } = await assetTree()
    if (!error) {
      treeData.value = toTreeOptions((data as AssetTreeNode[]) || [])
    }
  } finally {
    treeLoading.value = false
  }
}

// ---------- 列表 ----------
const tableData = ref<Asset[]>([])
const listLoading = ref(false)
const selectedParentId = ref('')

const query = reactive({
  keyword: '',
  page: 1,
  page_size: 10
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
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
    query.page = 1
    getTableData()
  }
})

async function getTableData() {
  listLoading.value = true
  try {
    const { data, error } = await assetList({
      parent_id: selectedParentId.value,
      keyword: query.keyword,
      page: query.page,
      page_size: query.page_size
    })
    if (!error) {
      tableData.value = (data as any)?.list || []
      pagination.itemCount = (data as any)?.total || 0
    }
  } finally {
    listLoading.value = false
  }
}

function handleSelectNode(keys: Array<string | number>) {
  const next = keys.length > 0 ? String(keys[0]) : ''
  selectedParentId.value = next
  query.page = 1
  pagination.page = 1
  getTableData()
}

// ---------- 新建/编辑 ----------
const showModal = ref(false)
const submitting = ref(false)
const editingId = ref('')
const formRef = ref<FormInst | null>(null)

const formData = reactive({
  parent_id: '',
  name: '',
  asset_type: 'device' as string,
  meta: ''
})

const rules: FormRules = {
  name: {
    required: true,
    message: () => $t('custom.asset.nameRequired'),
    trigger: ['input', 'blur']
  }
}

const modalTitle = computed(() =>
  editingId.value ? $t('custom.asset.editTitle') : $t('custom.asset.createTitle')
)

function openCreate(childOfSelected: boolean) {
  editingId.value = ''
  formData.parent_id = childOfSelected ? selectedParentId.value : ''
  formData.name = ''
  formData.asset_type = 'device'
  formData.meta = ''
  showModal.value = true
}

function openEdit(row: Asset) {
  editingId.value = row.id
  formData.parent_id = row.parent_id || ''
  formData.name = row.name
  formData.asset_type = row.asset_type || 'device'
  formData.meta = row.meta || ''
  showModal.value = true
}

type MetaResult = { ok: true; value: string | null } | { ok: false }

/** meta 允许留空；填写时必须是合法 JSON，否则不提交（后端按 jsonb 存储，脏数据会污染读路径）。 */
function normalizeMeta(raw: string): MetaResult {
  const text = (raw || '').trim()
  if (!text) return { ok: true, value: null }
  try {
    JSON.parse(text)
  } catch {
    message.error($t('custom.asset.metaInvalid'))
    return { ok: false }
  }
  return { ok: true, value: text }
}

async function submit() {
  await formRef.value?.validate()
  const meta = normalizeMeta(formData.meta)
  if (!meta.ok) return

  submitting.value = true
  try {
    const payload = {
      parent_id: formData.parent_id,
      name: formData.name.trim(),
      asset_type: formData.asset_type,
      meta: meta.value
    }
    const { error } = editingId.value
      ? await assetUpdate({ ...payload, id: editingId.value })
      : await assetCreate(payload)
    if (!error) {
      message.success($t('common.operationSuccess'))
      showModal.value = false
      loadTree()
      getTableData()
    }
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: Asset) {
  const { error } = await assetDelete(row.id)
  if (!error) {
    message.success($t('common.deleteSuccess'))
    loadTree()
    getTableData()
  }
}

const typeOptions = computed(() =>
  ASSET_TYPES.map(value => ({ label: typeLabel(value), value }))
)

const columns = computed<DataTableColumns<Asset>>(() => [
  {
    title: () => $t('custom.asset.name'),
    key: 'name',
    minWidth: 180
  },
  {
    title: () => $t('custom.asset.type'),
    key: 'asset_type',
    width: 110,
    render: row => h(NTag, { size: 'small', bordered: false }, { default: () => typeLabel(row.asset_type) })
  },
  {
    title: () => $t('custom.asset.tenantId'),
    key: 'tenant_id',
    width: 200,
    ellipsis: { tooltip: true }
  },
  {
    title: () => $t('custom.asset.createdAt'),
    key: 'created_at',
    width: 180,
    render: row => row.created_at?.replace('T', ' ').slice(0, 19) || '--'
  },
  {
    title: () => $t('common.actions'),
    key: 'actions',
    width: 170,
    render: row =>
      h('div', { class: 'flex gap-2' }, [
        h(
          NButton,
          { size: 'small', onClick: () => openEdit(row) },
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
            default: () => $t('custom.asset.deleteConfirm')
          }
        )
      ])
  }
])

onMounted(() => {
  loadTree()
  getTableData()
})
</script>

<template>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-[#101014]">
    <div class="flex flex-col gap-4 lg:flex-row">
      <!-- 资产树 -->
      <n-card :bordered="false" class="rounded-8px lg:w-320px" :title="$t('custom.asset.treeTitle')">
        <template #header-extra>
          <n-button size="small" secondary @click="loadTree">{{ $t('common.refresh') }}</n-button>
        </template>
        <n-spin :show="treeLoading">
          <n-empty v-if="!treeLoading && treeData.length === 0" :description="$t('custom.asset.treeEmpty')" />
          <n-tree
            v-else
            :data="treeData"
            block-line
            selectable
            :selected-keys="selectedParentId ? [selectedParentId] : []"
            @update:selected-keys="handleSelectNode"
          />
        </n-spin>
      </n-card>

      <!-- 资产列表 -->
      <n-card :bordered="false" class="flex-1 rounded-8px" :title="$t('custom.asset.listTitle')">
        <template #header-extra>
          <n-button size="small" tertiary :disabled="!selectedParentId" @click="openCreate(true)">
            {{ $t('custom.asset.createChild') }}
          </n-button>
          <n-button type="primary" size="small" @click="openCreate(false)">
            {{ $t('custom.asset.createRoot') }}
          </n-button>
        </template>

        <div class="mb-3 flex gap-3">
          <n-input
            v-model:value="query.keyword"
            :placeholder="$t('custom.asset.searchPlaceholder')"
            style="width: 240px"
            clearable
            @clear="getTableData"
            @keyup.enter="getTableData"
          />
          <n-button secondary @click="getTableData">{{ $t('common.search') }}</n-button>
        </div>

        <n-data-table
          :columns="columns"
          :data="tableData"
          :loading="listLoading"
          :pagination="pagination"
          :bordered="false"
          remote
          size="small"
        />
      </n-card>
    </div>

    <!-- 新建/编辑弹窗 -->
    <n-modal v-model:show="showModal" preset="dialog" :title="modalTitle" style="width: 520px">
      <n-form ref="formRef" :model="formData" :rules="rules" label-placement="top" class="mt-4">
        <n-form-item :label="$t('custom.asset.name')" path="name">
          <n-input v-model:value="formData.name" :placeholder="$t('custom.asset.nameRequired')" />
        </n-form-item>
        <n-form-item :label="$t('custom.asset.type')" path="asset_type">
          <n-select v-model:value="formData.asset_type" :options="typeOptions" />
        </n-form-item>
        <n-form-item :label="$t('custom.asset.parent')" path="parent_id">
          <n-input
            v-model:value="formData.parent_id"
            :placeholder="$t('custom.asset.parentRoot')"
            clearable
          />
        </n-form-item>
        <n-form-item :label="$t('custom.asset.meta')" path="meta">
          <n-input
            v-model:value="formData.meta"
            type="textarea"
            :rows="3"
            :placeholder="$t('custom.asset.metaPlaceholder')"
          />
        </n-form-item>
      </n-form>
      <template #action>
        <n-button @click="showModal = false">{{ $t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="submitting" @click="submit">{{ $t('common.confirm') }}</n-button>
      </template>
    </n-modal>
  </div>
</template>
