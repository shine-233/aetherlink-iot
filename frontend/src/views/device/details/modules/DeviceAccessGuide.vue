<!--
  文件用途: 设备接入指南（join tab 与添加设备向导共用）。
  核心逻辑: 计算分诊视图与首要阻塞卡片，编排快速开始、凭证网格、诊断面板、调试证据、测试代码与支持包预览；
  各分区在 ./access-guide/ 下，复制动作统一经 emit('copy') 交给宿主（宿主负责未保存凭证拦截与 toast）。
  关键注意事项: 凭证脱敏态（credentialsMasked）下密码与快速开始凭证步骤不提供复制入口。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { NAlert, NButton, NCard, NCode, NModal, NScrollbar } from 'naive-ui'
import { $t } from '@/locales'
import type { DeviceAccessGuideState } from './device-access-guide-state'
import type { DeviceDebugLogEntry, DeviceDebugStatus } from '@/service/api/device'
import {
  buildDeviceAccessGuideAccessPacket,
  buildDeviceAccessGuideSupportSummary,
  buildDeviceAccessGuideTriageView
} from './device-access-guide-triage-view'
import { downloadJsonWithFeedback } from '../shared/detail-feedback'
import ConnectionProofSteps from './ConnectionProofSteps.vue'
import DeviceMqttDebugWorkbench from './DeviceMqttDebugWorkbench.vue'
import AccessGuideCredentialGrid from './access-guide/AccessGuideCredentialGrid.vue'
import AccessGuideDiagnosticsPanel from './access-guide/AccessGuideDiagnosticsPanel.vue'
import AccessGuideDebugEvidence from './access-guide/AccessGuideDebugEvidence.vue'
import AccessGuideTestCodePanel from './access-guide/AccessGuideTestCodePanel.vue'

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

const triageInput = () => ({
  accessGuide: props.accessGuide,
  debugStatus: props.debugStatus,
  debugLogs: props.debugLogs,
  t: $t
})

const triageView = computed(() => buildDeviceAccessGuideTriageView(triageInput()))

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

const supportSummaryPreview = ref('')
const supportSummaryPreviewVisible = ref(false)

const buildSupportSummary = () =>
  buildDeviceAccessGuideSupportSummary({ ...triageInput(), triageView: triageView.value })

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

const downloadAccessPacket = () =>
  downloadJsonWithFeedback(
    () => buildDeviceAccessGuideAccessPacket({ ...triageInput(), triageView: triageView.value }),
    {
      fileName: accessPacketFileName(),
      successKey: 'custom.device_details.accessGuideDownloadSdkBundleSuccess',
      failureKey: 'custom.device_details.accessGuideDownloadSdkBundleFailed',
      failureLevel: 'warning'
    }
  )
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
            @click="
              firstBlockerCard.secondaryActionKind === 'refresh' ? emit('refreshDebugEvidence') : emit('enableDebug')
            "
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

      <AccessGuideCredentialGrid
        :access-guide="accessGuide"
        :credentials-masked="credentialsMasked"
        :password-display-visible="passwordDisplayVisible"
        @copy="emit('copy', $event)"
      />

      <AccessGuideDiagnosticsPanel
        :access-guide="accessGuide"
        :triage-view="triageView"
        :debug-status="debugStatus"
        :debug-loading="debugLoading"
        :debug-action-loading="debugActionLoading"
        @copy="emit('copy', $event)"
        @open-support-summary="openSupportSummaryPreview"
        @enable-debug="emit('enableDebug')"
        @refresh-debug-evidence="emit('refreshDebugEvidence')"
        @open-ready-check="emit('openReadyCheck')"
        @open-twin-evidence="emit('openTwinEvidence')"
      />

      <AccessGuideDebugEvidence
        :debug-status="debugStatus"
        :debug-logs="debugLogs"
        :debug-loading="debugLoading"
        :debug-action-loading="debugActionLoading"
        @enable-debug="emit('enableDebug')"
        @disable-debug="emit('disableDebug')"
        @refresh-debug-evidence="emit('refreshDebugEvidence')"
      />

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

    <AccessGuideTestCodePanel
      :access-guide="accessGuide"
      :connect-info="connectInfo"
      @copy="emit('copy', $event)"
      @download-access-packet="downloadAccessPacket"
    />

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
  border: 1px solid var(--border-color);
  border-left-width: 5px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgb(var(--info-color) / 0.05) 0%, var(--card-color) 100%);
}

.access-guide-blocker-card strong {
  display: block;
  color: var(--text-color-1);
  font-size: 18px;
  overflow-wrap: anywhere;
}

.access-guide-blocker-card p {
  margin: 8px 0 0;
  color: var(--text-color-2);
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.access-guide-blocker-card p span {
  color: var(--text-color-2);
  font-weight: 700;
}

.access-guide-blocker-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.access-guide-blocker-card--success {
  border-left-color: rgb(var(--success-color));
}

.access-guide-blocker-card--warning {
  border-left-color: rgb(var(--warning-color));
}

.access-guide-blocker-card--danger {
  border-left-color: rgb(var(--error-color));
}

.access-guide-blocker-card--neutral {
  border-left-color: var(--text-color-3);
}

.access-guide-label {
  display: block;
  margin-bottom: 6px;
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.access-guide-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
}

.access-guide-check {
  min-width: 0;
  padding: 10px 12px;
  border-left: 3px solid rgb(var(--success-color));
  background: rgb(var(--success-color) / 0.08);
}

.access-guide-check strong,
.access-guide-check span {
  display: block;
}

.access-guide-check span {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
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
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
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

@media (max-width: 720px) {
  .access-guide-blocker-card {
    grid-template-columns: 1fr;
  }

  .access-guide-blocker-actions {
    justify-content: flex-start;
  }
}
</style>
