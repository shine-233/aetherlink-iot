<!--
  文件用途：资源中心浏览与运营页（ROADMAP TP-5 / P1.6）——设备物模型与大屏看板统一市场。
  核心逻辑：
    1. 支持全部资源 / 设备物模型 / 大屏看板 多形态切换与行业分类过滤；
    2. 资源统一展示、卡片预览与一键应用到当前租户；
    3. 支持跨租户综合资源包（物模型+大屏看板）的打包导出与加密签名导入闸门：
       解析预检 → 只读双重预览（物模型/大屏待新建/待覆盖/阻断项）→ 覆盖项显式确认后提交。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  buildBundleImportPayload,
  canSubmitBundleImport,
  decideBundleImport,
  hasBundleSignature,
  nextConfirmOverwrite,
  parseBundleText,
  previewNameLists,
  summarizeBundleImportResults,
  type BundleImportDecision,
  type BundleImportSummary,
  type MarketBundlePayload,
  type MarketBundlePreview
} from './bundle-import-model'
import {
  downloadMarketBundle,
  getLocalTemplateList,
  getMarketBundle,
  getMarketCatalog,
  importMarketBundle,
  type MarketCatalogEntry
} from '@/service/api/market'
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

// ---------------------------------------------------------------------------
// 综合资源包导入闸门
// ---------------------------------------------------------------------------

const importVisible = ref(false)
const importBusy = ref(false)
const importBundle = ref<MarketBundlePayload | null>(null)
const importPreview = ref<MarketBundlePreview | null>(null)
const importErrorKey = ref<string>('')
const confirmOverwrite = ref(false)
const importFileKey = ref<string>('')
const importSummary = ref<BundleImportSummary | null>(null)

const importLists = computed(() => previewNameLists(importPreview.value))
const importDecision = computed<BundleImportDecision>(() => decideBundleImport(importBundle.value, importPreview.value))
const importCanSubmit = computed(() => canSubmitBundleImport(importDecision.value, confirmOverwrite.value))
const importSigned = computed(() => hasBundleSignature(importBundle.value))

function fileKeyOf(file: File): string {
  return `${file.name}|${file.size}|${file.lastModified}`
}

const DECISION_MESSAGE_KEYS: Record<BundleImportDecision, string> = {
  invalid: 'page.marketBrowse.importDecisionInvalid',
  empty: 'page.marketBrowse.importDecisionEmpty',
  unsigned: 'page.marketBrowse.importDecisionUnsigned',
  blocked: 'page.marketBrowse.importDecisionBlocked',
  'needs-confirm': 'page.marketBrowse.importDecisionNeedsConfirm',
  ready: 'page.marketBrowse.importDecisionReady'
}

const importDecisionMessage = computed(() => $t(DECISION_MESSAGE_KEYS[importDecision.value]))

async function handleImportFile(file: File) {
  importErrorKey.value = ''
  importPreview.value = null
  importSummary.value = null

  const nextKey = fileKeyOf(file)
  confirmOverwrite.value = nextConfirmOverwrite(importFileKey.value, nextKey)
  importFileKey.value = nextKey

  const text = await file.text()
  const parsed = parseBundleText(text)
  if (!parsed.ok) {
    importBundle.value = null
    importErrorKey.value = parsed.errorKey
    importVisible.value = true
    return
  }

  importBundle.value = parsed.bundle
  importVisible.value = true

  if (!hasBundleSignature(parsed.bundle)) return

  importBusy.value = true
  try {
    const { data, error } = await importMarketBundle(buildBundleImportPayload(parsed.bundle, { preview: true }))
    if (error) {
      importErrorKey.value = 'page.marketBrowse.importPreviewFailed'
      return
    }
    importPreview.value = (data?.preview ?? null) as MarketBundlePreview | null
  } finally {
    importBusy.value = false
  }
}

function handleImportFileEvent(options: { file: { file: File | null } }) {
  const file = options.file.file
  if (file) void handleImportFile(file)
}

async function submitImport() {
  if (!importBundle.value || !importCanSubmit.value) return

  importBusy.value = true
  try {
    const { data, error } = await importMarketBundle(
      buildBundleImportPayload(importBundle.value, { confirmOverwrite: confirmOverwrite.value })
    )
    if (error) {
      importErrorKey.value = 'page.marketBrowse.importFailed'
      return
    }
    importSummary.value = summarizeBundleImportResults(data?.results)
    importVisible.value = false
    await Promise.all([loadCatalog(), loadTemplates()])
  } finally {
    importBusy.value = false
  }
}

function closeImport() {
  importVisible.value = false
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
    <n-modal
      v-model:show="importVisible"
      preset="card"
      class="max-w-720px"
      :title="$t('page.marketBrowse.importTitle')"
      :bordered="false"
    >
      <n-space vertical size="medium">
        <n-alert v-if="importErrorKey" type="error" :show-icon="true">
          {{ $t(importErrorKey) }}
        </n-alert>

        <n-alert v-if="importSummary" type="success" :show-icon="true">
          {{
            $t('page.marketBrowse.importDone', {
              total: importSummary.total,
              created: importSummary.created,
              idempotent: importSummary.idempotent,
              rejected: importSummary.rejected
            })
          }}
        </n-alert>

        <div v-if="importBundle" class="flex items-center gap-2 text-13px">
          <span class="opacity-70">{{ $t('page.marketBrowse.importSignature') }}:</span>
          <n-tag size="small" :type="importSigned ? 'success' : 'warning'">
            {{ importSigned ? $t('page.marketBrowse.importSigned') : $t('page.marketBrowse.importUnsigned') }}
          </n-tag>
        </div>

        <n-alert v-if="importBundle" :type="importDecision === 'ready' ? 'success' : 'warning'" :show-icon="true">
          {{ importDecisionMessage }}
        </n-alert>

        <!-- 三类名单分开渲染：阻断项、覆盖项、新建项 -->
        <div v-if="importLists.create.length" class="text-13px">
          <div class="mb-1 font-600">{{ $t('page.marketBrowse.importCreate') }}</div>
          <ul class="ml-4 list-disc opacity-80">
            <li v-for="name in importLists.create" :key="`create-${name}`">{{ name }}</li>
          </ul>
        </div>

        <div v-if="importLists.overwrite.length" class="text-13px">
          <div class="mb-1 font-600">{{ $t('page.marketBrowse.importOverwrite') }}</div>
          <ul class="ml-4 list-disc opacity-80">
            <li v-for="name in importLists.overwrite" :key="`overwrite-${name}`">{{ name }}</li>
          </ul>
        </div>

        <div v-if="importLists.blocking.length" class="text-13px">
          <div class="mb-1 font-600">{{ $t('page.marketBrowse.importBlocking') }}</div>
          <ul class="ml-4 list-disc opacity-80">
            <li v-for="name in importLists.blocking" :key="`blocking-${name}`">{{ name }}</li>
          </ul>
        </div>

        <n-checkbox v-if="importDecision === 'needs-confirm'" v-model:checked="confirmOverwrite">
          {{ $t('page.marketBrowse.importConfirmOverwrite') }}
        </n-checkbox>
      </n-space>

      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button size="small" @click="closeImport">{{ $t('common.cancel') }}</n-button>
          <n-button
            type="primary"
            size="small"
            :disabled="!importCanSubmit"
            :loading="importBusy"
            @click="submitImport"
          >
            {{ $t('page.marketBrowse.importSubmit') }}
          </n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>
