<!--
  文件用途: Twin Lite 修复清单：对齐 / 漂移 / 缺少上报 计数卡片 + 证据包下载、复制摘要、一键用上报值修复。
  核心逻辑: 纯展示组件，动作通过 emit 交回 TwinLiteCard（useTwinLite）。
-->
<script setup lang="ts">
import type { TwinLiteState } from './twin-lite-normalizer'

defineProps<{
  alertType: 'warning' | 'info' | 'success'
  headline: string
  summary: TwinLiteState['summary']
  canRepair: boolean
}>()

defineEmits<{
  downloadEvidence: []
  copySummary: []
  repair: []
}>()
</script>

<template>
  <n-alert :type="alertType" class="mb-3" :show-icon="false">
    <n-space vertical :size="12">
      <div class="flex flex-col gap-10px md:flex-row md:items-center md:justify-between">
        <div class="min-w-0">
          <div class="font-600">{{ $t('custom.device_details.twinRepairChecklistTitle') }}</div>
          <div class="mt-4px text-12px line-height-18px text-gray-500">
            {{ headline }}
          </div>
        </div>
        <div class="flex flex-wrap gap-8px">
          <n-button
            size="small"
            secondary
            data-testid="device-twin-download-evidence-bundle"
            @click="$emit('downloadEvidence')"
          >
            {{ $t('custom.device_details.twinEvidenceBundleDownload') }}
          </n-button>
          <n-button size="small" secondary @click="$emit('copySummary')">
            {{ $t('custom.device_details.twinRepairCopySummary') }}
          </n-button>
          <n-button size="small" type="primary" secondary :disabled="!canRepair" @click="$emit('repair')">
            {{ $t('custom.device_details.twinRepairUseReportedAction') }}
          </n-button>
        </div>
      </div>
      <n-grid cols="1 640:3" :x-gap="12" :y-gap="8">
        <n-gi>
          <div class="twin-repair-card">
            <span>{{ $t('custom.device_details.twinRepairCardAligned') }}</span>
            <strong>{{ summary.matchedCount }}</strong>
          </div>
        </n-gi>
        <n-gi>
          <div class="twin-repair-card twin-repair-card--warning">
            <span>{{ $t('custom.device_details.twinRepairCardDelta') }}</span>
            <strong>{{ summary.deltaCount }}</strong>
          </div>
        </n-gi>
        <n-gi>
          <div class="twin-repair-card twin-repair-card--info">
            <span>{{ $t('custom.device_details.twinRepairCardMissingReported') }}</span>
            <strong>{{ summary.unavailableCount }}</strong>
          </div>
        </n-gi>
      </n-grid>
    </n-space>
  </n-alert>
</template>

<style scoped>
.twin-repair-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #f0fdf4;
}

.twin-repair-card--warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.twin-repair-card--info {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.twin-repair-card span {
  color: #64748b;
  font-size: 12px;
}

.twin-repair-card strong {
  color: #0f172a;
  font-size: 18px;
}
</style>
