<!--
文件用途：模板市场浏览页（PHASE-D-D10）——行业分类 tab + 模板卡片 + 按行业打包下载 + 导入。
核心逻辑：本地租户模板库按 type_key 归类展示；打包导出走 market/bundle（base64 信封）；
导入复用既有 device/template/import 契约（同租户同名同版本幂等）。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  downloadMarketBundle,
  getLocalTemplateList,
  getMarketBundle,
  getMarketCatalog,
  importDeviceTemplate,
  type MarketCatalogEntry
} from '@/service/api/market'
import { $t } from '@/locales'

defineOptions({ name: 'MarketBrowse' })

interface TemplateRow {
  id: string
  name: string
  version?: string
  author?: string
  description?: string
  type_key?: string
  download_count?: number
}

const catalog = ref<MarketCatalogEntry[]>([])
const activeType = ref<string>('')
const templates = ref<TemplateRow[]>([])
const loading = ref(false)

const tabs = computed(() => [
  { key: '', label: $t('page.marketBrowse.allTypes'), count: catalog.value.reduce((sum, item) => sum + Number(item.template_count), 0) },
  ...catalog.value.map(item => ({
    key: item.type_key || '',
    label: item.type_key || $t('page.marketBrowse.uncategorized'),
    count: Number(item.template_count)
  }))
])

const filtered = computed(() =>
  templates.value.filter(row => (activeType.value ? (row.type_key || '') === activeType.value : true))
)

async function loadCatalog() {
  const { data, error } = await getMarketCatalog()
  if (!error && Array.isArray(data)) catalog.value = data
}

async function loadTemplates() {
  loading.value = true
  try {
    // 本地模板分页接口（后端 type_key 过滤已支持）；浏览页取大页一次拉全。
    const params: { page: number; page_size: number; type_key?: string } = { page: 1, page_size: 200 }
    if (activeType.value) params.type_key = activeType.value
    const { data, error } = await getLocalTemplateList(params)
    if (!error && data) {
      templates.value = ((data as { list?: TemplateRow[] }).list || []) as TemplateRow[]
    }
  } finally {
    loading.value = false
  }
}

async function handleDownloadBundle(typeKey: string) {
  const { data, error } = await getMarketBundle(typeKey)
  if (error || !data) return
  downloadMarketBundle(data as { file_name: string; content_base64: string })
  window.$message?.success($t('page.marketBrowse.downloadStarted'))
}

async function handleImportFile(file: File) {
  const text = await file.text()
  const { error } = await importDeviceTemplate(JSON.parse(text))
  if (!error) {
    window.$message?.success($t('common.operationSuccess'))
    await Promise.all([loadCatalog(), loadTemplates()])
  }
}

function handleImportFileEvent(options: { file: { file: File | null } }) {
  const file = options.file.file
  if (file) void handleImportFile(file)
}

onMounted(() => {
  void loadCatalog()
  void loadTemplates()
})
</script>

<template>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-[#101014]">
    <n-card :bordered="false" class="rounded-8px">
      <template #header>
        <div class="flex items-center gap-3">
          <span>{{ $t('page.marketBrowse.title') }}</span>
          <n-upload :show-file-list="false" accept="application/json,.json" @change="handleImportFileEvent">
            <n-button size="small">{{ $t('page.marketBrowse.import') }}</n-button>
          </n-upload>
        </div>
      </template>

      <n-tabs v-model:value="activeType" type="segment" class="mb-4">
        <n-tab v-for="tab in tabs" :key="tab.key || '__all__'" :name="tab.key">
          {{ tab.label }} ({{ tab.count }})
        </n-tab>
      </n-tabs>

      <div class="mb-3 flex justify-end">
        <n-button type="primary" size="small" :loading="loading" @click="handleDownloadBundle(activeType)">
          {{ $t('page.marketBrowse.downloadBundle') }}
        </n-button>
      </div>

      <n-empty v-if="!filtered.length" :description="$t('page.marketBrowse.empty')" />
      <div v-else class="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
        <n-card v-for="row in filtered" :key="row.id" size="small" :bordered="true" class="rounded-8px">
          <div class="flex items-center justify-between">
            <span class="font-600">{{ row.name }}</span>
            <n-tag size="small" type="info">{{ row.version || '-' }}</n-tag>
          </div>
          <div class="mt-1 text-12px opacity-70">{{ row.description || row.author || '—' }}</div>
          <div class="mt-2 flex items-center justify-between text-12px">
            <n-tag size="small">{{ row.type_key || $t('page.marketBrowse.uncategorized') }}</n-tag>
            <span class="opacity-70">{{ row.download_count ?? 0 }} ↓</span>
          </div>
        </n-card>
      </div>
    </n-card>
  </div>
</template>
