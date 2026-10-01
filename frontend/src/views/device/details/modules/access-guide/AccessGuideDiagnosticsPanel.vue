<!--
  文件用途: 接入诊断助手：分诊结论、下一步快捷动作与诊断指标列表。
  核心逻辑: DeviceAccessGuide 拆分出的展示分区；复制与调试动作通过 emit 交回 DeviceAccessGuide 统一转发。
-->
<script setup lang="ts">
import type { DeviceDebugStatus } from '@/service/api/device'
import type { DeviceAccessGuideState } from '../device-access-guide-state'
import type { buildDeviceAccessGuideTriageView } from '../device-access-guide-triage-view'

defineProps<{
  accessGuide: DeviceAccessGuideState
  triageView: ReturnType<typeof buildDeviceAccessGuideTriageView>
  debugStatus?: DeviceDebugStatus
  debugLoading?: boolean
  debugActionLoading?: boolean
}>()

const emit = defineEmits<{
  copy: [text: unknown]
  openSupportSummary: []
  enableDebug: []
  refreshDebugEvidence: []
  openReadyCheck: []
  openTwinEvidence: []
}>()
</script>

<template>
  <div>
    <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideDiagnostics') }}</div>
    <div class="access-guide-diagnostic-hero" :class="`access-guide-diagnostic-hero--${triageView.tone}`">
      <div class="access-guide-diagnostic-hero-main">
        <span class="access-guide-label">{{ $t('custom.device_details.accessGuideDiagnosticAssistant') }}</span>
        <strong>{{ triageView.summary }}</strong>
        <p>{{ triageView.nextAction }}</p>
        <div class="access-guide-diagnostic-hero-actions">
          <NButton size="small" secondary type="primary" @click="emit('copy', accessGuide.endpoint)">
            {{ $t('custom.device_details.accessGuideNextStepCopyEndpoint') }}
          </NButton>
          <NButton
            size="small"
            secondary
            type="primary"
            data-testid="device-access-guide-support-bundle"
            @click="emit('openSupportSummary')"
          >
            {{ $t('custom.commandCenter.copySupportBundle') }}
          </NButton>
          <NButton
            size="small"
            secondary
            :disabled="!triageView.primaryTestCommand"
            @click="emit('copy', triageView.primaryTestCommand)"
          >
            {{ $t('custom.device_details.accessGuideNextStepCopyTestCommand') }}
          </NButton>
          <NButton
            v-if="!debugStatus?.enabled"
            size="small"
            secondary
            :loading="debugActionLoading"
            @click="emit('enableDebug')"
          >
            {{ $t('custom.device_details.accessGuideDebugEnable30m') }}
          </NButton>
          <NButton v-else size="small" secondary :loading="debugLoading" @click="emit('refreshDebugEvidence')">
            {{ $t('custom.device_details.accessGuideDebugRefresh') }}
          </NButton>
          <NButton
            size="small"
            secondary
            type="success"
            data-testid="device-access-guide-run-ready-check"
            @click="emit('openReadyCheck')"
          >
            {{ $t('custom.device_details.accessGuideNextStepRunReadyCheck') }}
          </NButton>
          <NButton
            size="small"
            secondary
            type="info"
            data-testid="device-access-guide-open-twin"
            @click="emit('openTwinEvidence')"
          >
            {{ $t('custom.device_details.accessGuideNextStepOpenTwin') }}
          </NButton>
        </div>
      </div>
      <div class="access-guide-diagnostic-hero-side">
        <span>
          {{ $t('custom.device_details.accessGuideReadyCheck') }}:
          <strong>{{ triageView.ready }}</strong>
        </span>
        <span>
          {{ $t('custom.device_details.accessGuideLatestTelemetry') }}:
          <strong>{{ triageView.telemetry }}</strong>
        </span>
        <span>
          {{ $t('custom.device_details.accessGuideDiagnosticCurrentIssue') }}:
          <strong>{{ triageView.issue }}</strong>
        </span>
        <span>
          {{ $t('custom.device_details.accessGuideDiagnosticPartial') }}:
          <strong>{{ triageView.completeness }}</strong>
        </span>
        <span>
          {{ $t('custom.device_details.accessGuideDebugEvidence') }}:
          <strong>{{ triageView.latestDebugEvidence }}</strong>
        </span>
        <NButton size="small" secondary :loading="debugLoading" @click="emit('refreshDebugEvidence')">
          {{ $t('custom.device_details.accessGuideDiagnosticRefresh') }}
        </NButton>
      </div>
    </div>
    <div class="access-guide-diagnostics">
      <div
        v-for="item in accessGuide.diagnostics"
        :key="item.labelKey"
        class="access-guide-diagnostic"
        :class="`access-guide-diagnostic--${item.tone}`"
      >
        <span class="access-guide-label">{{ $t(item.labelKey) }}</span>
        <strong>{{ item.valueKey ? $t(item.valueKey) : item.value }}</strong>
      </div>
    </div>
  </div>
</template>

<style scoped src="./access-guide-shared.css"></style>

<style scoped>
.access-guide-diagnostics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}

.access-guide-diagnostic-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(220px, 320px);
  gap: 14px;
  margin-bottom: 12px;
  padding: 14px;
  border: 1px solid var(--border-color);
  border-left-width: 4px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--card-color) 0%, var(--action-color) 100%);
}

.access-guide-diagnostic-hero strong {
  overflow-wrap: anywhere;
}

.access-guide-diagnostic-hero p {
  margin: 8px 0 0;
  color: var(--text-color-2);
  line-height: 1.5;
}

.access-guide-diagnostic-hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.access-guide-diagnostic-hero-side {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.access-guide-diagnostic-hero-side strong {
  display: block;
  margin-top: 2px;
  color: var(--text-color-1);
}

.access-guide-diagnostic-hero--success {
  border-left-color: rgb(var(--success-color));
}

.access-guide-diagnostic-hero--warning {
  border-left-color: rgb(var(--warning-color));
}

.access-guide-diagnostic-hero--danger {
  border-left-color: rgb(var(--error-color));
}

.access-guide-diagnostic-hero--neutral {
  border-left-color: var(--text-color-3);
}

.access-guide-diagnostic {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-left-width: 3px;
  border-radius: 6px;
  background: var(--card-color);
}

.access-guide-diagnostic strong {
  display: block;
  overflow-wrap: anywhere;
}

.access-guide-diagnostic--success {
  border-left-color: rgb(var(--success-color));
}

.access-guide-diagnostic--warning {
  border-left-color: rgb(var(--warning-color));
}

.access-guide-diagnostic--danger {
  border-left-color: rgb(var(--error-color));
}

.access-guide-diagnostic--neutral {
  border-left-color: var(--text-color-3);
}

@media (max-width: 720px) {
  .access-guide-diagnostic-hero {
    grid-template-columns: 1fr;
  }
}
</style>
