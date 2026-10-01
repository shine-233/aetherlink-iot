<!--
  文件用途：market/browse 的「跨租户综合资源包导入闸门」子组件（从 index.vue 抽出）。
  核心逻辑：解析预检 → 只读双重预览（待新建 / 待覆盖 / 阻断项）→ 覆盖项显式确认 → 提交。
  关键注意事项：
    1. 前端不验签，只做预检；未签名必须显式告知，而不是显示成可导入；
    2. 覆盖项必须由用户显式勾选确认后才允许提交；
    3. 闸门自身持有开关与状态，父页通过 ref 调 open(file) 驱动，成功提交后 emit('imported')
       让父页刷新目录与列表——父页因此不必知道闸门的内部状态。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
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
} from '../bundle-import-model'
import { importMarketBundle } from '@/service/api/market'
import { $t } from '@/locales'

const emit = defineEmits<{ imported: [] }>()

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

async function open(file: File) {
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
    emit('imported')
  } finally {
    importBusy.value = false
  }
}

function closeImport() {
  importVisible.value = false
}

defineExpose({ open })
</script>

<template>
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
</template>
