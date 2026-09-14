<!--
  文件用途：模板市场浏览页（ROADMAP P1.6）——本地模板库浏览 + 按行业打包导出 + 打包导入闸门。
  核心逻辑：浏览/导出沿用旧能力；导入走 market/bundle/import 的三段式流程：
            解析预检 → 只读预览（create/overwrite/blocking）→ 覆盖项显式确认后提交。
  关键注意事项：
    1. **不得回退到 /device/template/import**。那是无签名的单模板回放端点，
       会绕过 VerifyMarketBundle 与 confirm_overwrite 闸门；本页曾有一条
       handleImportFile 直接调它，等于把整条 P1.6 门禁做成摆设。
       所有导入必须经 importMarketBundle。
    2. 前端不验签，也不得假装验签——只做"包里有没有签名三字段"的预检，
       真正的验签由后端 fail closed 执行（见 bundle-import-model.ts 注释 1）。
    3. 阻断项不可被确认绕过；覆盖项必须显式确认，且换文件时确认态必须重置。
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
  if (!error && Array.isArray(data)) catalog.value = data as MarketCatalogEntry[]
}

async function loadTemplates() {
  loading.value = true
  try {
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

// ---------------------------------------------------------------------------
// 打包导入闸门
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
const importDecision = computed<BundleImportDecision>(() =>
  decideBundleImport(importBundle.value, importPreview.value)
)
const importCanSubmit = computed(() => canSubmitBundleImport(importDecision.value, confirmOverwrite.value))
const importSigned = computed(() => hasBundleSignature(importBundle.value))

/** 文件指纹：同名同大小但内容不同的极端情况由后端预览结果兜住。 */
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

  // 未签名的包后端一定会拒，没必要多打一次往返；决策会显示 unsigned。
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

        <!-- 三类名单必须分开渲染：阻断项混进覆盖项会让用户以为勾一下就能过 -->
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
