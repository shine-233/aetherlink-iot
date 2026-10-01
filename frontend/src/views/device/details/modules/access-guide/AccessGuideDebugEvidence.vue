<!--
  文件用途: 设备调试证据区：调试开关状态、剩余时长与最近调试日志。
  核心逻辑: DeviceAccessGuide 拆分出的展示分区；复制与调试动作通过 emit 交回 DeviceAccessGuide 统一转发。
-->
<script setup lang="ts">
import type { DeviceDebugLogEntry, DeviceDebugStatus } from '@/service/api/device'
import {
  formatAccessGuideDebugLogMessage,
  formatAccessGuideDebugLogTitle,
  formatAccessGuideDebugTime
} from '../device-access-guide-triage-view'

defineProps<{
  debugStatus?: DeviceDebugStatus
  debugLogs?: DeviceDebugLogEntry[]
  debugLoading?: boolean
  debugActionLoading?: boolean
}>()

const emit = defineEmits<{
  enableDebug: []
  disableDebug: []
  refreshDebugEvidence: []
}>()
</script>

<template>
  <div class="access-guide-debug-section">
    <div class="access-guide-debug-header">
      <div>
        <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideDebugEvidence') }}</div>
        <div class="access-guide-debug-subtitle">
          {{ $t('custom.device_details.accessGuideDebugEvidenceHint') }}
        </div>
      </div>
      <div class="access-guide-debug-actions">
        <NButton size="small" secondary :loading="debugLoading" @click="emit('refreshDebugEvidence')">
          {{ $t('custom.device_details.accessGuideDebugRefresh') }}
        </NButton>
        <NButton
          v-if="debugStatus?.enabled"
          size="small"
          secondary
          type="warning"
          :loading="debugActionLoading"
          @click="emit('disableDebug')"
        >
          {{ $t('custom.device_details.accessGuideDebugDisable') }}
        </NButton>
        <NButton v-else size="small" type="primary" :loading="debugActionLoading" @click="emit('enableDebug')">
          {{ $t('custom.device_details.accessGuideDebugEnable30m') }}
        </NButton>
      </div>
    </div>
    <div class="access-guide-debug-status">
      <span>
        {{ $t('custom.device_details.accessGuideDebugStatus') }}:
        <strong>
          {{
            debugStatus?.enabled
              ? $t('custom.device_details.accessGuideDiagnosticDebugOn')
              : $t('custom.device_details.accessGuideDiagnosticDebugOff')
          }}
        </strong>
      </span>
      <span v-if="debugStatus?.enabled">
        {{ $t('custom.device_details.accessGuideDebugExpires') }}:
        <strong>{{ formatAccessGuideDebugTime(debugStatus.expire_at) }}</strong>
      </span>
      <span v-if="debugStatus?.enabled">
        {{ $t('custom.device_details.accessGuideDebugRemaining') }}:
        <strong>{{ debugStatus.remaining_seconds || 0 }}s</strong>
      </span>
    </div>
    <div class="access-guide-debug-logs">
      <NAlert v-if="!debugLogs?.length" type="warning" :show-icon="false">
        {{ $t('custom.device_details.accessGuideDebugNoLogs') }}
      </NAlert>
      <div v-for="(log, index) in debugLogs" :key="`${log.ts || index}-${index}`" class="access-guide-debug-log">
        <strong>{{ formatAccessGuideDebugLogTitle(log) }}</strong>
        <span>{{ formatAccessGuideDebugLogMessage(log, $t) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped src="./access-guide-shared.css"></style>

<style scoped>
.access-guide-debug-section {
  margin-bottom: 18px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: rgb(var(--info-color) / 0.05);
}

.access-guide-debug-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.access-guide-debug-subtitle {
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.access-guide-debug-actions,
.access-guide-debug-status,
.access-guide-debug-logs {
  display: flex;
  gap: 8px;
}

.access-guide-debug-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.access-guide-debug-status {
  flex-wrap: wrap;
  margin-bottom: 10px;
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.access-guide-debug-logs {
  flex-direction: column;
}

.access-guide-debug-log {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-left: 3px solid rgb(var(--info-color));
  border-radius: 6px;
  background: var(--card-color);
  font-size: var(--font-size-caption);
}

.access-guide-debug-log span {
  color: var(--text-color-3);
  overflow-wrap: anywhere;
}
</style>
