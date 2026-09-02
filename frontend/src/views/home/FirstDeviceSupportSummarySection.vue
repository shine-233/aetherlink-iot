<script setup lang="ts">
import { computed, ref } from 'vue'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'

const props = defineProps<{
  getSummary: () => string
}>()

const previewVisible = ref(false)
const summary = ref('')
const ensureSummary = () => {
  summary.value = props.getSummary()
  return summary.value
}
const summaryLineCount = computed(() => summary.value.split(/\r?\n/).filter(Boolean).length)
const summarySizeLabel = computed(() =>
  summary.value
    ? $t('custom.home.firstDevice.support.sizeValue', {
        lines: summaryLineCount.value,
        chars: summary.value.length
      })
    : $t('custom.home.firstDevice.support.sizePending')
)

const openPreview = () => {
  ensureSummary()
  previewVisible.value = true
}

const copySummary = async () => {
  const copied = await writeClipboardText(ensureSummary())
  if (copied) {
    window.$message?.success($t('theme.configOperation.copySuccess'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

defineExpose({
  openPreview
})
</script>

<template>
  <div class="first-device-support-summary">
    <div class="flex flex-col gap-8px sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.common.stuckWhen') }}</div>
        <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.support.title') }}</div>
        <div class="mt-4px text-12px line-height-18px text-gray-500">
          {{ $t('custom.home.firstDevice.support.desc') }}
        </div>
        <div class="mt-6px text-11px text-gray-400">
          {{ $t('custom.home.firstDevice.support.sizeHint', { size: summarySizeLabel }) }}
        </div>
      </div>
      <div class="first-device-support-actions">
        <n-button size="tiny" type="primary" ghost @click="openPreview">
          {{ $t('custom.home.firstDevice.common.previewCopySupportSummary') }}
        </n-button>
        <n-button size="tiny" quaternary @click="copySummary">{{ $t('generate.copy') }}</n-button>
      </div>
    </div>

    <n-modal v-model:show="previewVisible" aria-label="dialog" preset="card" class="first-device-support-modal">
      <template #header>{{ $t('custom.commandCenter.supportBundlePreviewTitle') }}</template>
      <div class="first-device-support-modal-body">
        <n-alert type="info" :show-icon="false">
          {{ $t('custom.commandCenter.supportBundlePreviewDesc') }}
        </n-alert>
        <n-scrollbar class="first-device-support-modal-scroll">
          <n-code :code="summary" language="markdown" word-wrap />
        </n-scrollbar>
      </div>
      <template #footer>
        <div class="first-device-support-modal-footer">
          <n-button @click="previewVisible = false">{{ $t('common.cancel') }}</n-button>
          <n-button type="primary" @click="copySummary">{{ $t('generate.copy') }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.first-device-support-summary {
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #f8fafc;
}

.first-device-support-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.first-device-support-modal {
  max-width: 760px;
}

.first-device-support-modal-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.first-device-support-modal-scroll {
  max-height: 440px;
  padding: 10px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #0f172a;
}

.first-device-support-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
