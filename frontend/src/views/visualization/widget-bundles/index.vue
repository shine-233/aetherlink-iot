<!--
文件用途：部件库管理页（widget_bundles，ROADMAP TB-04）——可视化域的部件库 CRUD 工作台。
核心逻辑：
1. 部件库列表分页检索（按名称/描述模糊搜索）；
2. 部件库创建/编辑表单：名称、版本、行业分类与部件定义 JSON 编辑器（提交前本地校验 JSON 数组结构）；
3. 内置四部件（gauge/chart/valve/twin3d）一键种子导入（幂等）与内置定义查看。
关键注意事项：部件定义会被画布渲染与命令下发直接消费，JSON 必须是对象数组且逐项含 type/version/schema/capabilities；
  后端 service 层复用 ValidateWidgetDefinition 做最终校验，前端校验只做快速反馈。
-->
<script setup lang="tsx">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'
import {
  createWidgetBundle,
  deleteWidgetBundle,
  getBuiltinWidgetBundle,
  getWidgetBundlesList,
  seedBuiltinWidgetBundle,
  updateWidgetBundle,
  type WidgetBundleExport,
  type WidgetBundleItem
} from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const message = useMessage()
const { loading, startLoading, endLoading } = useLoading(false)
const bundles = ref<WidgetBundleItem[]>([])
const searchValue = ref('')
const pagination = reactive({ page: 1, pageSize: 10, itemCount: 0 })

const modalVisible = ref(false)
const submitting = ref(false)
const editingId = ref('')
const formRef = ref<FormInst | null>(null)
const formModel = reactive({
  name: '',
  version: '1.0.0',
  type_key: '',
  description: '',
  widgets: '[]'
})

const formRules: FormRules = {
  name: { required: true, message: $t('page.widgetBundle.nameRequired'), trigger: 'blur' }
}

/** 部件定义数量：解析失败按 0 展示，交由编辑器兜底校验。 */
const widgetCountOf = (raw: string) => {
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.length : 0
  } catch {
    return 0
  }
}

/** 提交前本地校验：必须是对象数组且逐项含 type/version。 */
const validateWidgetsJson = (raw: string): string | null => {
  const trimmed = raw.trim()
  if (!trimmed) return $t('page.widgetBundle.widgetsRequired')
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return $t('page.widgetBundle.widgetsInvalidJson')
  }
  if (!Array.isArray(parsed)) return $t('page.widgetBundle.widgetsNotArray')
  for (let i = 0; i < parsed.length; i += 1) {
    const item = parsed[i] as Record<string, unknown> | null
    if (!item || typeof item !== 'object') {
      return `${$t('page.widgetBundle.widgetsItemInvalid')} #${i}`
    }
    if (!item.type || !item.version) {
      return `${$t('page.widgetBundle.widgetsItemInvalid')} #${i}`
    }
  }
  return null
}

const builtinVisible = ref(false)
const builtinExport = ref<WidgetBundleExport | null>(null)

const builtinWidgetCount = computed(() =>
  builtinExport.value ? widgetCountOf(builtinExport.value.widgets) : 0
)

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

const fetchBundles = async () => {
  startLoading()
  try {
    const { data, error } = await getWidgetBundlesList({
      page: pagination.page,
      page_size: pagination.pageSize,
      search: searchValue.value || undefined
    })
    if (!error && data) {
      bundles.value = data.list ?? []
      pagination.itemCount = data.total ?? 0
    }
  } finally {
    endLoading()
  }
}

const handleSearch = () => {
  pagination.page = 1
  void fetchBundles()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void fetchBundles()
}

const openCreate = () => {
  editingId.value = ''
  Object.assign(formModel, {
    name: '',
    version: '1.0.0',
    type_key: '',
    description: '',
    widgets: '[]'
  })
  modalVisible.value = true
}

const openEdit = (row: WidgetBundleItem) => {
  editingId.value = row.id
  Object.assign(formModel, {
    name: row.name,
    version: row.version || '1.0.0',
    type_key: row.type_key || '',
    description: row.description || '',
    widgets: row.widgets || '[]'
  })
  modalVisible.value = true
}

const openBuiltin = async () => {
  const { data, error } = await getBuiltinWidgetBundle()
  if (!error && data) {
    builtinExport.value = data
    builtinVisible.value = true
  }
}

const handleSeed = async () => {
  submitting.value = true
  try {
    const { data, error } = await seedBuiltinWidgetBundle()
    if (!error && data) {
      if (data.idempotent) {
        message.info($t('page.widgetBundle.seedIdempotent'))
      } else {
        message.success($t('page.widgetBundle.seedSuccess'))
      }
      builtinVisible.value = false
      await fetchBundles()
    }
  } finally {
    submitting.value = false
  }
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  const widgetsError = validateWidgetsJson(formModel.widgets)
  if (widgetsError) {
    message.error(widgetsError)
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: formModel.name,
      version: formModel.version || undefined,
      type_key: formModel.type_key || undefined,
      description: formModel.description || undefined,
      widgets: formModel.widgets
    }
    const { error } = editingId.value
      ? await updateWidgetBundle({ id: editingId.value, ...payload })
      : await createWidgetBundle(payload)
    if (!error) {
      message.success($t('page.widgetBundle.saveSuccess'))
      modalVisible.value = false
      await fetchBundles()
    }
  } finally {
    submitting.value = false
  }
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
</script>

<template>
  <div class="min-h-500px">
    <NCard :title="$t('route.visualization_widget-bundles')" :bordered="false">
      <template #header-extra>
        <NSpace>
          <NInput
            v-model:value="searchValue"
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
        :pagination="{ ...pagination, onChange: handlePageChange, 'onUpdate:pageSize': (size: number) => (pagination.pageSize = size) }"
        :scroll-x="900"
        :row-key="(row: WidgetBundleItem) => row.id"
      />
    </NCard>

    <NModal
      v-model:show="modalVisible"
      preset="card"
      :title="editingId ? $t('page.widgetBundle.editTitle') : $t('page.widgetBundle.createTitle')"
      class="w-720px"
    >
      <NForm ref="formRef" :model="formModel" :rules="formRules" label-placement="left" label-width="110">
        <NFormItem :label="$t('page.widgetBundle.name')" path="name">
          <NInput v-model:value="formModel.name" :placeholder="$t('page.widgetBundle.namePlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.widgetBundle.version')" path="version">
          <NInput v-model:value="formModel.version" placeholder="1.0.0" />
        </NFormItem>
        <NFormItem :label="$t('page.widgetBundle.typeKey')" path="type_key">
          <NInput v-model:value="formModel.type_key" :placeholder="$t('page.widgetBundle.typeKeyPlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.widgetBundle.description')" path="description">
          <NInput v-model:value="formModel.description" type="textarea" :rows="2" />
        </NFormItem>
        <NFormItem :label="$t('page.widgetBundle.widgets')" path="widgets">
          <NInput
            v-model:value="formModel.widgets"
            type="textarea"
            :rows="12"
            class="font-mono"
            :placeholder="$t('page.widgetBundle.widgetsPlaceholder')"
          />
        </NFormItem>
        <div class="mb-8px text-12px text-gray-400">{{ $t('page.widgetBundle.widgetsHint') }}</div>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="modalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="submitting" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NDrawer v-model:show="builtinVisible" :width="520">
      <NDrawerContent :title="$t('page.widgetBundle.builtinPreview')" closable>
        <NSpace vertical>
          <NSpace>
            <NTag type="info">{{ builtinExport?.kind }}</NTag>
            <NTag>{{ builtinExport?.name }}</NTag>
            <NTag>v{{ builtinExport?.version }}</NTag>
            <NTag type="success">{{ $t('page.widgetBundle.widgetCount') }}: {{ builtinWidgetCount }}</NTag>
          </NSpace>
          <div v-if="builtinExport?.description" class="text-13px text-gray-500">{{ builtinExport.description }}</div>
          <NInput
            :value="builtinExport?.widgets ?? ''"
            type="textarea"
            :rows="16"
            readonly
            class="font-mono"
          />
          <div class="text-12px text-gray-400">{{ $t('page.widgetBundle.seedHint') }}</div>
        </NSpace>
        <template #footer>
          <NSpace justify="end">
            <NButton @click="builtinVisible = false">{{ $t('common.cancel') }}</NButton>
            <NButton type="primary" :loading="submitting" @click="handleSeed">
              {{ $t('page.widgetBundle.seedAction') }}
            </NButton>
          </NSpace>
        </template>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped></style>
