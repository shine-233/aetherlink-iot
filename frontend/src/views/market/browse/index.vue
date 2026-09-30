<!--
  文件用途：资源中心浏览与运营页（ROADMAP TP-5 / P1.6）——设备物模型与大屏看板统一市场。
  核心逻辑：
    1. 支持全部资源 / 设备物模型 / 大屏看板 多形态切换与行业分类过滤；
    2. 资源统一展示、卡片预览与一键应用到当前租户；
    3. 跨租户综合资源包的导入闸门（解析预检 → 只读双重预览 → 覆盖确认 → 提交）
       由子组件 modules/bundle-import-modal.vue 承担，本页只负责触发与刷新。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import BundleImportModal from './modules/bundle-import-modal.vue'
import { downloadMarketBundle, getLocalTemplateList, getMarketBundle, getMarketCatalog, type MarketCatalogEntry } from '@/service/api/market'
import { applyResource, getResourceCenterCatalog, getResourceCenterList } from '@/service/api/resource-center'
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
  resource_type?: 'device_template' | 'board_template' | string
  vis_type?: string
}

const catalog = ref<MarketCatalogEntry[]>([])
const activeType = ref<string>('')
const activeResourceType = ref<'all' | 'device_template' | 'board_template'>('all')
const templates = ref<TemplateRow[]>([])
const loading = ref(false)

const importModal = ref<InstanceType<typeof BundleImportModal> | null>(null)

const tabs = computed(() => [
  {
    key: '',
    label: $t('page.marketBrowse.allTypes'),
    count: catalog.value.reduce((sum, item) => sum + Number(item.template_count), 0)
  },
  ...catalog.value.map((item) => ({
    key: item.type_key || '',
    label: item.type_key || $t('page.marketBrowse.uncategorized'),
    count: Number(item.template_count)
  }))
])

const filtered = computed(() =>
  templates.value.filter((row) => {
    const matchType = activeType.value ? (row.type_key || '') === activeType.value : true
    const matchResource =
      activeResourceType.value === 'all' || !row.resource_type ? true : row.resource_type === activeResourceType.value
    return matchType && matchResource
  })
)

async function loadCatalog() {
  try {
    const { data, error } = await getResourceCenterCatalog()
    if (!error && Array.isArray(data) && data.length > 0) {
      catalog.value = data.map((item) => ({
        type_key: item.type_key,
        template_count: item.total_count,
        download_count: item.download_count
      }))
      return
    }
  } catch (_err) {
    // fallback to market catalog
  }

  const { data, error } = await getMarketCatalog()
  if (!error && Array.isArray(data)) catalog.value = data as MarketCatalogEntry[]
}

async function loadTemplates() {
  loading.value = true
  try {
    try {
      const { data, error } = await getResourceCenterList({
        page: 1,
        page_size: 200,
        resource_type: activeResourceType.value === 'all' ? undefined : activeResourceType.value,
        type_key: activeType.value || undefined
      })
      if (!error && data && Array.isArray(data.list) && data.list.length > 0) {
        templates.value = data.list.map((r) => ({
          id: r.id,
          name: r.name,
          version: r.version,
          author: r.author,
          description: r.description,
          type_key: r.type_key,
          download_count: r.download_count,
          resource_type: r.resource_type,
          vis_type: r.vis_type
        }))
        return
      }
    } catch (_err) {
      // fallback to local template list
    }

    const { data, error } = await getLocalTemplateList({ page: 1, page_size: 200 })
    if (!error && data) {
      const payload = data as { list?: TemplateRow[] }
      templates.value = Array.isArray(payload.list) ? payload.list : []
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

async function handleApply(row: TemplateRow) {
  try {
    const rType = (row.resource_type as 'device_template' | 'board_template') || 'device_template'
    const { error } = await applyResource({
      resource_type: rType,
      resource_id: row.id
    })
    if (!error) {
      window.$message?.success($t('page.marketBrowse.applySuccess'))
      await loadTemplates()
    }
  } catch (err: any) {
    window.$message?.error(err?.message || 'Apply failed')
  }
}

/**
 * 导入闸门由子组件持有状态；本页只做入口转发。
 * 保留同名方法是为了不破坏既有测试与外部调用契约（__tests__/index.test.ts 直接驱动它）。
 */
async function handleImportFile(file: File) {
  await importModal.value?.open(file)
}

function handleImportFileEvent(options: { file: { file: File | null } }) {
  const file = options.file.file
  if (file) void handleImportFile(file)
}

async function reloadAfterImport() {
  await Promise.all([loadCatalog(), loadTemplates()])
}

onMounted(() => {
  void loadCatalog()
  void loadTemplates()
})

defineExpose({ handleImportFile })
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

      <!-- 资源形态切换（全部 / 设备物模型 / 大屏看板） -->
      <n-tabs v-model:value="activeResourceType" type="line" class="mb-3" @update:value="loadTemplates">
        <n-tab name="all">{{ $t('page.marketBrowse.allResources') }}</n-tab>
        <n-tab name="device_template">{{ $t('page.marketBrowse.deviceTemplates') }}</n-tab>
        <n-tab name="board_template">{{ $t('page.marketBrowse.boardTemplates') }}</n-tab>
      </n-tabs>

      <!-- 行业分类切换 -->
      <n-tabs v-model:value="activeType" type="segment" class="mb-4" @update:value="loadTemplates">
        <n-tab v-for="tab in tabs" :key="tab.key || '__all__'" :name="tab.key">{{ tab.label }} ({{ tab.count }})</n-tab>
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
            <div class="flex items-center gap-2">
              <span class="font-600">{{ row.name }}</span>
              <n-tag size="tiny" :type="row.resource_type === 'board_template' ? 'warning' : 'success'">
                {{
                  row.resource_type === 'board_template'
                    ? $t('page.marketBrowse.boardTemplates')
                    : $t('page.marketBrowse.deviceTemplates')
                }}
              </n-tag>
            </div>
            <n-tag size="small" type="info">{{ row.version || '-' }}</n-tag>
          </div>
          <div class="mt-1 text-12px opacity-70">{{ row.description || row.author || '—' }}</div>
          <div class="mt-2 flex items-center justify-between text-12px">
            <n-tag size="small">{{ row.type_key || $t('page.marketBrowse.uncategorized') }}</n-tag>
            <div class="flex items-center gap-3">
              <span class="opacity-70">{{ row.download_count ?? 0 }} ↓</span>
              <n-button text type="primary" size="tiny" @click="handleApply(row)">
                {{ $t('page.marketBrowse.apply') }}
              </n-button>
            </div>
          </div>
        </n-card>
      </div>
    </n-card>

    <!-- 导入闸门：解析预检 → 只读预览 → 覆盖确认 → 提交 -->
    <BundleImportModal ref="importModal" @imported="reloadAfterImport" />
  </div>
</template>
