<script setup lang="ts">
import { computed, ref } from 'vue'
import { NAlert, NButton, NCard, NCode, NDescriptions, NDescriptionsItem, NModal, NScrollbar } from 'naive-ui'
import { $t } from '@/locales'
import type { DeviceAccessGuideState } from './device-access-guide-state'
import type { DeviceDebugLogEntry, DeviceDebugStatus } from '@/service/api/device'
import {
  buildDeviceAccessGuideAccessPacket,
  buildDeviceAccessGuideSupportSummary,
  buildDeviceAccessGuideTriageView,
  formatAccessGuideDebugLogMessage,
  formatAccessGuideDebugLogTitle,
  formatAccessGuideDebugTime
} from './device-access-guide-triage-view'
import ConnectionProofSteps from './ConnectionProofSteps.vue'
import DeviceMqttDebugWorkbench from './DeviceMqttDebugWorkbench.vue'

const props = defineProps<{
  deviceId?: string
  accessGuide: DeviceAccessGuideState
  connectInfo: Record<string, unknown>
  credentialsMasked?: boolean
  debugStatus?: DeviceDebugStatus
  debugLogs?: DeviceDebugLogEntry[]
  debugLoading?: boolean
  debugActionLoading?: boolean
  hasUnsavedCredentials?: boolean
}>()

const emit = defineEmits<{
  copy: [text: unknown]
  openReadyCheck: []
  openTwinEvidence: []
  enableDebug: []
  disableDebug: []
  refreshDebugEvidence: []
}>()

const triageView = computed(() =>
  buildDeviceAccessGuideTriageView({
    accessGuide: props.accessGuide,
    debugStatus: props.debugStatus,
    debugLogs: props.debugLogs,
    t: $t
  })
)

const firstBlockerCard = computed(() => {
  const view = triageView.value
  const isReady = view.tone === 'success'
  return {
    tone: view.tone,
    title: isReady
      ? $t('custom.device_details.accessGuideBlockerReadyTitle')
      : $t('custom.device_details.accessGuideBlockerTitle'),
    summary: view.summary,
    evidence: view.issue && view.issue !== '--' ? view.issue : view.latestDebugEvidence,
    nextAction: view.nextAction,
    primaryAction: isReady
      ? $t('custom.device_details.accessGuideViewTwinEvidence')
      : $t('custom.device_details.accessGuideRunReadyCheck'),
    secondaryAction: view.debugEnabled
      ? $t('custom.device_details.accessGuideRefreshDebugEvidence')
      : $t('custom.device_details.accessGuideEnableDebugThirtyMinutes'),
    secondaryActionKind: view.debugEnabled ? 'refresh' : 'enableDebug'
  }
})

const commandTestCodeVisible = ref(false)
const visibleCommands = computed(() =>
  commandTestCodeVisible.value ? props.accessGuide.commands : props.accessGuide.commands.slice(0, 1)
)
// 凭证已脱敏态（Phase 2a）：密码瓦片不再提供复制入口，展示固定占位；
// 快速开始的"使用这些凭证"步骤同样去掉复制按钮（凭证不可见即不可复制）。
const passwordDisplayVisible = computed(() => !props.credentialsMasked && Boolean(props.accessGuide.password))
const visibleQuickstartSteps = computed(() =>
  props.credentialsMasked
    ? props.accessGuide.quickstartSteps.map((step) =>
        step.titleKey === 'custom.device_details.accessGuideQuickstartCredential'
          ? { ...step, copyText: undefined, copyLabelKey: undefined }
          : step
      )
    : props.accessGuide.quickstartSteps
)
const hiddenCommandCount = computed(() => Math.max(props.accessGuide.commands.length - visibleCommands.value.length, 0))
const hiddenCommandCountText = computed(() =>
  String($t('custom.device_details.accessGuideHiddenTestCodeCount')).replace('{count}', String(hiddenCommandCount.value))
)
const supportSummaryPreview = ref('')
const supportSummaryPreviewVisible = ref(false)

const buildSupportSummary = () =>
  buildDeviceAccessGuideSupportSummary({
    accessGuide: props.accessGuide,
    triageView: triageView.value,
    debugStatus: props.debugStatus,
    debugLogs: props.debugLogs,
    t: $t
  })

const openSupportSummaryPreview = () => {
  supportSummaryPreview.value = buildSupportSummary()
  supportSummaryPreviewVisible.value = true
}

const copySupportSummary = () => {
  emit('copy', supportSummaryPreview.value || buildSupportSummary())
}

const accessPacketFileName = () => {
  const rawName = `${props.accessGuide.protocol || 'device'}-${props.accessGuide.endpointKind || 'access'}`
  const safeName = rawName.replace(/[^a-zA-Z0-9._-]/g, '_').replace(/^_+|_+$/g, '') || 'device'
  return `aetherlink-device-${safeName}-access-packet.json`
}

const buildAccessPacket = () =>
  buildDeviceAccessGuideAccessPacket({
    accessGuide: props.accessGuide,
    triageView: triageView.value,
    debugStatus: props.debugStatus,
    debugLogs: props.debugLogs,
    t: $t
  })

const downloadAccessPacket = () => {
  try {
    if (typeof window === 'undefined' || typeof document === 'undefined') {
      return
    }
    const blob = new Blob([JSON.stringify(buildAccessPacket(), null, 2)], { type: 'application/json' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = accessPacketFileName()
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    window.$message?.success($t('custom.device_details.accessGuideDownloadSdkBundleSuccess'))
  } catch {
    if (typeof window !== 'undefined') {
      window.$message?.warning($t('custom.device_details.accessGuideDownloadSdkBundleFailed'))
    }
  }
}
</script>

<template>
  <div data-testid="device-access-guide">
    <NCard class="mb-6 mt-6" data-testid="device-access-guide-quickstart">
      <NAlert type="info" class="mb-4" :show-icon="false">
        {{ $t('custom.device_details.accessGuideIntro') }}
      </NAlert>
      <NAlert v-if="hasUnsavedCredentials" type="warning" class="mb-4" :show-icon="false">
        {{ $t('custom.device_details.accessGuideUnsavedVoucherCopyBlocked') }}
      </NAlert>

      <div class="access-guide-blocker-card" :class="`access-guide-blocker-card--${firstBlockerCard.tone}`">
        <div class="access-guide-blocker-main">
          <span class="access-guide-label">{{ firstBlockerCard.title }}</span>
          <strong>{{ firstBlockerCard.summary }}</strong>
          <p>
            <span>{{ $t('custom.device_details.accessGuideEvidenceLabel') }}:</span>
            {{ firstBlockerCard.evidence || '--' }}
          </p>
          <p>
            <span>{{ $t('custom.device_details.accessGuideNextStepLabel') }}:</span>
            {{ firstBlockerCard.nextAction || '--' }}
          </p>
        </div>
        <div class="access-guide-blocker-actions">
          <NButton
            size="small"
            type="success"
            secondary
            data-testid="device-access-guide-first-blocker-ready-check"
            @click="firstBlockerCard.tone === 'success' ? emit('openTwinEvidence') : emit('openReadyCheck')"
          >
            {{ firstBlockerCard.primaryAction }}
          </NButton>
          <NButton
            size="small"
            secondary
            :loading="firstBlockerCard.secondaryActionKind === 'refresh' ? debugLoading : debugActionLoading"
            @click="firstBlockerCard.secondaryActionKind === 'refresh' ? emit('refreshDebugEvidence') : emit('enableDebug')"
          >
            {{ firstBlockerCard.secondaryAction }}
          </NButton>
          <NButton size="small" secondary @click="openSupportSummaryPreview">
            {{ $t('custom.commandCenter.copySupportBundle') }}
          </NButton>
        </div>
      </div>

      <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideQuickstartTitle') }}</div>
      <ConnectionProofSteps
        :steps="visibleQuickstartSteps"
        :copy-disabled="hasUnsavedCredentials"
        :debug-enabled="triageView.debugEnabled"
        :debug-evidence="triageView.latestDebugEvidence"
        :evidence-loading="debugLoading"
        :ready-state="triageView.ready"
        :telemetry-state="triageView.telemetry"
        @copy="emit('copy', $event)"
        @refresh-evidence="emit('refreshDebugEvidence')"
        @open-ready-check="emit('openReadyCheck')"
      />

      <div class="access-guide-grid">
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideProtocol') }}</span>
          <strong>{{ accessGuide.protocol }}</strong>
        </div>
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideAuthMode') }}</span>
          <strong>{{ accessGuide.authMode }}</strong>
        </div>
        <div class="access-guide-metric" data-testid="device-access-guide-endpoint">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideEndpoint') }}</span>
          <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.endpoint)">
            {{ accessGuide.endpoint }}
          </button>
        </div>
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideClientId') }}</span>
          <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.clientId)">
            {{ accessGuide.clientId }}
          </button>
        </div>
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideUsername') }}</span>
          <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.username)">
            {{ accessGuide.username }}
          </button>
        </div>
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuidePassword') }}</span>
          <!-- 脱敏态固定展示占位符，不提供明文/复制按钮（Phase 2a）。 -->
          <strong v-if="credentialsMasked">******</strong>
          <button
            v-else-if="passwordDisplayVisible"
            type="button"
            class="access-guide-copy"
            @click="emit('copy', accessGuide.password)"
          >
            {{ accessGuide.password }}
          </button>
          <strong v-else>{{ $t('custom.device_details.accessGuidePasswordEmpty') }}</strong>
        </div>
        <div v-if="accessGuide.endpointKind === 'mqtt'" class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideReportTopic') }}</span>
          <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.reportTopic)">
            {{ accessGuide.reportTopic }}
          </button>
        </div>
        <div v-if="accessGuide.endpointKind === 'mqtt'" class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideControlTopic') }}</span>
          <button type="button" class="access-guide-copy" @click="emit('copy', accessGuide.controlTopic)">
            {{ accessGuide.controlTopic }}
          </button>
        </div>
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideTls') }}</span>
          <strong>{{ $t(accessGuide.tlsHintKey) }}</strong>
        </div>
        <div class="access-guide-metric">
          <span class="access-guide-label">{{ $t('custom.device_details.accessGuideOnlineCheck') }}</span>
          <strong>{{ $t('custom.device_details.accessGuideOnlineHint') }}</strong>
        </div>
      </div>

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
              @click="openSupportSummaryPreview"
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

      <div class="access-guide-checks">
        <div v-for="check in accessGuide.checks" :key="check.titleKey" class="access-guide-check">
          <strong>{{ $t(check.titleKey) }}</strong>
          <span>{{ $t(check.descriptionKey) }}</span>
        </div>
      </div>

      <DeviceMqttDebugWorkbench
        v-if="deviceId && accessGuide.endpointKind === 'mqtt'"
        :device-id="deviceId"
        :default-subscribe-topic="accessGuide.reportTopic"
        :default-publish-topic="accessGuide.controlTopic"
      />

      <slot name="credential-form" />
    </NCard>

    <NScrollbar class="access-guide-scroll">
      <NCard class="mb-4" data-testid="device-access-guide-test-code">
        <div class="access-guide-section-heading">
          <div>
            <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideTestCode') }}</div>
            <div class="access-guide-section-subtitle">
              {{ $t('custom.device_details.accessGuideTestCodeHint') }}
            </div>
          </div>
          <div class="access-guide-section-actions">
            <NButton size="small" type="primary" secondary @click="emit('copy', accessGuide.sdkBundle)">
              {{ $t('custom.device_details.accessGuideCopySdkBundle') }}
            </NButton>
            <NButton
              size="small"
              secondary
              data-testid="device-access-guide-download-access-packet"
              @click="downloadAccessPacket"
            >
              {{ $t('custom.device_details.accessGuideDownloadSdkBundle') }}
            </NButton>
          </div>
        </div>
        <div class="access-guide-command-list">
          <div v-for="command in visibleCommands" :key="command.titleKey" class="access-guide-command">
            <div class="access-guide-command-header">
              <strong>{{ $t(command.titleKey) }}</strong>
              <NButton size="small" tertiary @click="emit('copy', command.code)">{{ $t('generate.copy') }}</NButton>
            </div>
            <NCode :code="command.code" :language="command.language" word-wrap />
          </div>
          <div v-if="hiddenCommandCount" class="access-guide-command-more">
            <span>{{ hiddenCommandCountText }}</span>
            <NButton size="small" secondary @click="commandTestCodeVisible = true">
              {{ $t('custom.device_details.accessGuideShowAllTestCode') }}
            </NButton>
          </div>
          <div v-else-if="accessGuide.commands.length > 1" class="access-guide-command-more">
            <NButton size="small" tertiary @click="commandTestCodeVisible = false">
              {{ $t('custom.device_details.accessGuideCollapseTestCode') }}
            </NButton>
          </div>
        </div>
      </NCard>

      <NCard>
        <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideRawInfo') }}</div>
        <NDescriptions :column="1">
          <NDescriptionsItem v-for="(value, key) in connectInfo" :key="key" :label="key">
            <button type="button" class="access-guide-copy" @click="emit('copy', value)">{{ value }}</button>
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>
    </NScrollbar>

    <NModal v-model:show="supportSummaryPreviewVisible" preset="card" class="access-guide-support-modal">
      <template #header>{{ $t('custom.commandCenter.supportBundlePreviewTitle') }}</template>
      <div class="access-guide-support-preview">
        <NAlert type="info" :show-icon="false">
          {{ $t('custom.commandCenter.supportBundlePreviewDesc') }}
        </NAlert>
        <NScrollbar class="access-guide-support-preview-scroll">
          <NCode :code="supportSummaryPreview" language="markdown" word-wrap />
        </NScrollbar>
      </div>
      <template #footer>
        <div class="access-guide-support-footer">
          <NButton @click="supportSummaryPreviewVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="copySupportSummary">{{ $t('generate.copy') }}</NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.access-guide-blocker-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  align-items: center;
  margin-bottom: 18px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-left-width: 5px;
  border-radius: 14px;
  background: linear-gradient(135deg, #f8fbff 0%, #ffffff 100%);
}

.access-guide-blocker-card strong {
  display: block;
  color: #111827;
  font-size: 18px;
  overflow-wrap: anywhere;
}

.access-guide-blocker-card p {
  margin: 8px 0 0;
  color: #475569;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.access-guide-blocker-card p span {
  color: #334155;
  font-weight: 700;
}

.access-guide-blocker-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.access-guide-blocker-card--success {
  border-left-color: #18a058;
}

.access-guide-blocker-card--warning {
  border-left-color: #f0a020;
}

.access-guide-blocker-card--danger {
  border-left-color: #d03050;
}

.access-guide-blocker-card--neutral {
  border-left-color: #6b7280;
}

.access-guide-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}

.access-guide-metric {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #fafafa;
}

.access-guide-label {
  display: block;
  margin-bottom: 6px;
  color: #666;
  font-size: 12px;
}

.access-guide-copy {
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  border: none;
  background: transparent;
  color: #2563eb;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.access-guide-copy:hover {
  text-decoration: underline;
}

.access-guide-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}

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
  border: 1px solid #e5e7eb;
  border-left-width: 4px;
  border-radius: 10px;
  background: linear-gradient(135deg, #ffffff 0%, #f8fafc 100%);
}

.access-guide-diagnostic-hero strong {
  overflow-wrap: anywhere;
}

.access-guide-diagnostic-hero p {
  margin: 8px 0 0;
  color: #475569;
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
  color: #555;
  font-size: 12px;
}

.access-guide-diagnostic-hero-side strong {
  display: block;
  margin-top: 2px;
  color: #111827;
}

.access-guide-diagnostic-hero--success {
  border-left-color: #18a058;
}

.access-guide-diagnostic-hero--warning {
  border-left-color: #f0a020;
}

.access-guide-diagnostic-hero--danger {
  border-left-color: #d03050;
}

.access-guide-diagnostic-hero--neutral {
  border-left-color: #6b7280;
}

.access-guide-diagnostic {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-left-width: 3px;
  border-radius: 6px;
  background: #fff;
}

.access-guide-diagnostic strong {
  display: block;
  overflow-wrap: anywhere;
}

.access-guide-diagnostic--success {
  border-left-color: #18a058;
}

.access-guide-diagnostic--warning {
  border-left-color: #f0a020;
}

.access-guide-diagnostic--danger {
  border-left-color: #d03050;
}

.access-guide-diagnostic--neutral {
  border-left-color: #6b7280;
}

.access-guide-debug-section {
  margin-bottom: 18px;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.access-guide-debug-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.access-guide-debug-subtitle {
  color: #555;
  font-size: 12px;
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
  color: #555;
  font-size: 12px;
}

.access-guide-debug-logs {
  flex-direction: column;
}

.access-guide-debug-log {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 8px 10px;
  border: 1px solid #e5e7eb;
  border-left: 3px solid #2563eb;
  border-radius: 6px;
  background: #fff;
  font-size: 12px;
}

.access-guide-debug-log span {
  color: #555;
  overflow-wrap: anywhere;
}

.access-guide-check {
  min-width: 0;
  padding: 10px 12px;
  border-left: 3px solid #18a058;
  background: #f6fffa;
}

.access-guide-check strong,
.access-guide-check span {
  display: block;
}

.access-guide-check span {
  margin-top: 4px;
  color: #555;
  font-size: 12px;
}

.access-guide-scroll {
  max-height: 560px;
}

.access-guide-support-modal {
  max-width: 760px;
}

.access-guide-support-preview {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.access-guide-support-preview-scroll {
  max-height: 440px;
  padding: 10px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #0f172a;
}

.access-guide-support-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.access-guide-section-title {
  margin-bottom: 12px;
  font-weight: 600;
}

.access-guide-section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.access-guide-section-heading .access-guide-section-title {
  margin-bottom: 4px;
}

.access-guide-section-subtitle {
  color: #555;
  font-size: 12px;
  line-height: 1.4;
}

.access-guide-section-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.access-guide-command-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.access-guide-command {
  min-width: 0;
}

.access-guide-command-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.access-guide-command-more {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px dashed #d4dde8;
  border-radius: 10px;
  color: #667085;
}

@media (max-width: 720px) {
  .access-guide-diagnostic-hero {
    grid-template-columns: 1fr;
  }

  .access-guide-section-heading,
  .access-guide-blocker-card {
    grid-template-columns: 1fr;
  }

  .access-guide-blocker-actions {
    justify-content: flex-start;
  }
}
</style>
