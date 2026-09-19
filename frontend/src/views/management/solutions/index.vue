<!--
文件用途：解决方案模板引擎（ROADMAP TB-19）管理控制台。
核心逻辑：
1. 方案列表：分页展示行业方案与引用资源数量；
2. 方案创建：以有序资源引用清单（物模型模板 / 看板模板）组装行业方案，创建即校验引用可用；
3. 一键安装：逐项走资源中心应用管道，逐项展示 applied/failed 与目标实例 ID，安装流水可回查；
4. 方案删除：只删引用清单，不删除已安装的实例。
-->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPagination,
  NSelect,
  NSpace,
  NTag,
  useDialog
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  createIndustrySolution,
  deleteIndustrySolution,
  getIndustrySolution,
  installIndustrySolution,
  listIndustrySolutions
} from '@/service/api/solution'
import type {
  IndustrySolutionItem,
  SolutionInstallRow,
  SolutionInstallResponse,
  SolutionResourceRef
} from '@/service/api/solution'

const dialog = useDialog()

const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const solutions = ref<IndustrySolutionItem[]>([])

const createVisible = ref(false)
const createSaving = ref(false)
const createForm = reactive({
  name: '',
  description: '',
  resources: [{ resource_type: 'device_template', resource_id: '', target_name: '' }] as SolutionResourceRef[]
})

const detailVisible = ref(false)
const detailLoading = ref(false)
const detailSolution = ref<IndustrySolutionItem | null>(null)
const detailInstalls = ref<SolutionInstallRow[]>([])

const installRunning = ref(false)
const installResult = ref<SolutionInstallResponse | null>(null)

const resourceTypeOptions = [
  { label: '物模型模板', value: 'device_template' },
  { label: '看板模板', value: 'board_template' },
  { label: '规则链', value: 'rule_chain' }
]

const columns: DataTableColumns<IndustrySolutionItem> = [
  { title: '方案名称', key: 'name', minWidth: 160 },
  { title: '描述', key: 'description', minWidth: 180, ellipsis: { tooltip: true } },
  {
    title: '引用资源数',
    key: 'resources',
    width: 100,
    render: row => String((row.resources || []).length)
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: row =>
      h(
        NTag,
        { type: row.status === 'active' ? 'success' : 'default', size: 'small' },
        { default: () => (row.status === 'active' ? '启用' : '停用') }
      )
  },
  {
    title: '操作',
    key: 'actions',
    width: 220,
    render: row =>
      h(NSpace, { size: 8 }, {
        default: () => [
          h(
            NButton,
            { size: 'small', type: 'primary', onClick: () => openInstall(row) },
            { default: () => '一键安装' }
          ),
          h(
            NButton,
            { size: 'small', onClick: () => openDetail(row) },
            { default: () => '详情' }
          ),
          h(
            NButton,
            { size: 'small', type: 'error', secondary: true, onClick: () => confirmDelete(row) },
            { default: () => '删除' }
          )
        ]
      })
  }
]

async function fetchSolutions() {
  loading.value = true
  try {
    const { data, error } = await listIndustrySolutions({ page: page.value, page_size: pageSize.value })
    if (error || !data) return
    solutions.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  createForm.name = ''
  createForm.description = ''
  createForm.resources = [{ resource_type: 'device_template', resource_id: '', target_name: '' }]
  createVisible.value = true
}

function addResourceRow() {
  createForm.resources.push({ resource_type: 'device_template', resource_id: '', target_name: '' })
}

function removeResourceRow(index: number) {
  createForm.resources.splice(index, 1)
}

async function submitCreate() {
  const name = createForm.name.trim()
  if (!name) {
    window.$message?.error('请输入方案名称')
    return
  }
  const usable = createForm.resources.filter(row => row.resource_id.trim())
  if (!usable.length) {
    window.$message?.error('请至少填写一条有效的资源引用')
    return
  }
  createSaving.value = true
  try {
    const { error, data } = await createIndustrySolution({
      name,
      description: createForm.description.trim() || undefined,
      resources: usable.map(row => ({
        resource_type: row.resource_type,
        resource_id: row.resource_id.trim(),
        target_name: (row.target_name || '').trim() || undefined
      }))
    })
    if (error || !data) return
    window.$message?.success('方案已创建')
    createVisible.value = false
    await fetchSolutions()
  } finally {
    createSaving.value = false
  }
}

async function openDetail(row: IndustrySolutionItem) {
  detailVisible.value = true
  detailLoading.value = true
  detailSolution.value = row
  detailInstalls.value = []
  try {
    const { data, error } = await getIndustrySolution(row.id)
    if (error || !data) return
    detailInstalls.value = data.installs || []
  } finally {
    detailLoading.value = false
  }
}

async function openInstall(row: IndustrySolutionItem) {
  installResult.value = null
  dialog.warning({
    title: '一键安装',
    content: `将按顺序安装方案「${row.name}」的全部资源，每次安装都会实例化一套新资产。确认继续？`,
    positiveText: '开始安装',
    negativeText: '取消',
    onPositiveClick: async () => {
      installRunning.value = true
      try {
        const { data, error } = await installIndustrySolution(row.id)
        if (error || !data) return
        installResult.value = data
        detailSolution.value = row
        detailVisible.value = true
        const detail = await getIndustrySolution(row.id)
        if (!detail.error && detail.data) {
          detailInstalls.value = detail.data.installs || []
        }
      } finally {
        installRunning.value = false
      }
    }
  })
}

function confirmDelete(row: IndustrySolutionItem) {
  dialog.warning({
    title: '删除方案',
    content: `确定删除方案「${row.name}」？已安装的实例不受影响。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const { error } = await deleteIndustrySolution(row.id)
      if (error) return
      window.$message?.success('方案已删除')
      await fetchSolutions()
    }
  })
}

function onPageChange(next: number) {
  page.value = next
  fetchSolutions()
}

function onPageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  fetchSolutions()
}

onMounted(() => {
  fetchSolutions()
})
</script>

<template>
  <NCard title="解决方案模板" class="h-full" :bordered="false">
    <template #header-extra>
      <NButton type="primary" @click="openCreate">新建方案</NButton>
    </template>

    <NAlert type="info" :show-icon="false" class="mb-4">
      方案是有序资源引用清单（物模型模板 / 看板模板）。一键安装按顺序实例化一套新资产，安装流水逐项可追溯。
    </NAlert>

    <NDataTable
      :columns="columns"
      :data="solutions"
      :loading="loading"
      :row-key="(row: IndustrySolutionItem) => row.id"
    />

    <NSpace justify="end" class="mt-4">
      <NPagination
        :page="page"
        :page-size="pageSize"
        :item-count="total"
        :page-sizes="[10, 20, 30, 40, 50]"
        show-size-picker
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </NSpace>

    <NModal v-model:show="createVisible" preset="card" class="max-w-640px" title="新建方案">
      <NForm label-placement="top">
        <NFormItem label="方案名称" required>
          <NInput v-model:value="createForm.name" maxlength="128" placeholder="例如：水务行业监测方案" />
        </NFormItem>
        <NFormItem label="描述">
          <NInput v-model:value="createForm.description" type="textarea" maxlength="512" :autosize="{ minRows: 2, maxRows: 4 }" />
        </NFormItem>
        <div class="mb-2 flex items-center justify-between">
          <span class="font-500">资源引用清单（按安装顺序）</span>
          <NButton size="small" @click="addResourceRow">添加一项</NButton>
        </div>
        <NSpace v-for="(_, index) in createForm.resources" :key="index" :size="8" class="mb-2" align="center">
          <NSelect
            v-model:value="createForm.resources[index].resource_type"
            :options="resourceTypeOptions"
            class="w-150px"
          />
          <NInput
            v-model:value="createForm.resources[index].resource_id"
            placeholder="资源 ID"
            class="flex-1"
          />
          <NInput
            v-model:value="createForm.resources[index].target_name"
            placeholder="实例名称（可选）"
            class="w-200px"
          />
          <NButton
            size="small"
            type="error"
            secondary
            :disabled="createForm.resources.length <= 1"
            @click="removeResourceRow(index)"
          >
            移除
          </NButton>
        </NSpace>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="createVisible = false">取消</NButton>
          <NButton type="primary" :loading="createSaving" @click="submitCreate">创建方案</NButton>
        </NSpace>
      </template>
    </NModal>

    <NModal v-model:show="detailVisible" preset="card" class="max-w-720px" :title="`方案详情：${detailSolution?.name || ''}`">
      <NAlert
        v-if="installResult"
        :type="installResult.failed > 0 ? 'warning' : 'success'"
        :show-icon="false"
        class="mb-3"
      >
        安装完成：共 {{ installResult.total }} 项，成功 {{ installResult.applied }} 项，失败 {{ installResult.failed }} 项
      </NAlert>
      <div v-if="installResult" class="mb-4">
        <div class="mb-1 font-500">本次安装结果</div>
        <div v-for="item in installResult.items" :key="item.item_index" class="mb-1">
          <NTag :type="item.status === 'applied' ? 'success' : 'error'" size="small">
            {{ item.status === 'applied' ? '已应用' : '失败' }}
          </NTag>
          {{ item.resource_type }} → {{ item.target_id || item.error }}
        </div>
      </div>
      <div class="mb-1 font-500">安装流水（最近在前）</div>
      <NEmpty v-if="!detailInstalls.length" description="暂无安装记录" />
      <div v-for="rowItem in detailInstalls" :key="rowItem.id" class="mb-1">
        <NTag :type="rowItem.status === 'applied' ? 'success' : 'error'" size="small">
          {{ rowItem.status === 'applied' ? '已应用' : '失败' }}
        </NTag>
        {{ rowItem.solution_name }} #{{ rowItem.item_index }} {{ rowItem.resource_type }}
        <template v-if="rowItem.target_id">→ {{ rowItem.target_id }}</template>
        <template v-if="rowItem.error">（{{ rowItem.error }}）</template>
      </div>
    </NModal>
  </NCard>
</template>
