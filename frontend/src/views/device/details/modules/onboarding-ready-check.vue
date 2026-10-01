<!--
  文件用途: 设备 Ready Check（上线就绪检查）tab。
  核心逻辑: 状态、派生与动作全部由 useOnboardingReadyCheck 提供（纯规则在 ready-check-view-model.ts）；
  本文件只负责编排 hero、采集告警、证据中心（视口懒挂载）与动作面板。
-->
<script setup lang="ts">
import { defineAsyncComponent, ref } from 'vue'
import { useViewportDeferredMount } from '@/hooks/common/useViewportDeferredMount'
import ReadyCheckActionPanel from './ReadyCheckActionPanel.vue'
import ReadyCheckHero from './ReadyCheckHero.vue'
import { useOnboardingReadyCheck } from './use-onboarding-ready-check'

const ReadyCheckEvidenceCenterView = defineAsyncComponent(() => import('./ReadyCheckEvidenceCenterView.vue'))

const props = defineProps<{
  id: string
  online?: number
  deviceData?: Record<string, any>
}>()

const {
  diagnosticsLoading,
  recommendedCommandLoading,
  recommendedCommandDraft,
  collectionFailures,
  deviceName,
  deviceNumber,
  isOnline,
  source,
  readySummary,
  latestTelemetryText,
  nextActions,
  evidenceCards,
  evidenceCenterItems,
  evidenceDeepLinks,
  backendNextSteps,
  collectionFailureSummary,
  readyCheckDiagnosticSummary,
  steps,
  primaryReadyAction,
  primaryReadyActionSummary,
  showFirstDeviceReadyHandoff,
  refreshDiagnostics,
  copyReadyCheckDiagnosticSummary,
  downloadReadyCheckDiagnosticSummary,
  openEvidenceDeepLink,
  copyEvidenceDeepLink,
  copyAllEvidenceDeepLinks,
  openCommandCenter,
  openFirstDeviceHomeProof,
  openFirstDeviceAutomation,
  openFirstDeviceDashboard,
  runEvidenceCardAction,
  runReadyCheckStep
} = useOnboardingReadyCheck(props)

const evidenceCenterViewportRef = ref<HTMLElement | null>(null)
const { shouldMount: shouldMountEvidenceCenter, mountNow: mountEvidenceCenterNow } = useViewportDeferredMount(
  evidenceCenterViewportRef,
  { rootMargin: '420px 0px', fallbackDelay: 650 }
)
</script>

<template>
  <div class="ready-check" data-testid="device-ready-check">
    <ReadyCheckHero
      :device-name="deviceName"
      :device-number="deviceNumber"
      :is-online="isOnline"
      :source="source"
      :primary-title-key="primaryReadyAction.titleKey"
    />

    <NAlert
      v-if="collectionFailures.length"
      type="warning"
      :show-icon="false"
      class="ready-check-collection-warning"
      data-testid="device-ready-check-collection-warning"
    >
      <div class="ready-check-collection-warning__copy">
        <strong>{{ $t('custom.device_details.readyCheckCollectionWarningTitle') }}</strong>
        <span>{{ collectionFailureSummary }}</span>
      </div>
      <NSpace size="small">
        <NButton size="small" secondary type="primary" :loading="diagnosticsLoading" @click="refreshDiagnostics">
          {{ $t('custom.device_details.accessGuideDiagnosticRefresh') }}
        </NButton>
        <NButton size="small" secondary @click="downloadReadyCheckDiagnosticSummary">
          {{ $t('custom.device_details.readyCheckDownloadSupportBundle') }}
        </NButton>
      </NSpace>
    </NAlert>

    <div ref="evidenceCenterViewportRef">
      <ReadyCheckEvidenceCenterView
        v-if="shouldMountEvidenceCenter"
        :loading="diagnosticsLoading"
        :ready-summary="readySummary"
        :latest-telemetry-text="latestTelemetryText"
        :next-actions="nextActions"
        :evidence-center-items="evidenceCenterItems"
        :evidence-cards="evidenceCards"
        :backend-next-steps="backendNextSteps"
        :deep-links="evidenceDeepLinks"
        @refresh="refreshDiagnostics"
        @copy-support-bundle="copyReadyCheckDiagnosticSummary"
        @download-support-bundle="downloadReadyCheckDiagnosticSummary"
        @open-deep-link="openEvidenceDeepLink"
        @copy-deep-link="copyEvidenceDeepLink"
        @copy-all-deep-links="copyAllEvidenceDeepLinks"
        @run-evidence-card="runEvidenceCardAction"
      />
      <div v-else class="ready-check-deferred-placeholder">
        <div class="ready-check-deferred-placeholder__copy">
          <span>{{ $t('custom.device_details.readyCheckEvidenceBoundary') }}</span>
          <strong>{{ $t('custom.device_details.readyCheckEvidenceCenterDeferredTitle') }}</strong>
          <p>{{ $t('custom.device_details.readyCheckEvidenceCenterDeferredDesc') }}</p>
        </div>
        <NButton
          size="small"
          secondary
          type="primary"
          data-testid="device-ready-check-load-evidence-center"
          @click="mountEvidenceCenterNow"
        >
          {{ $t('custom.device_details.readyCheckLoadEvidenceCenter') }}
        </NButton>
      </div>
    </div>

    <ReadyCheckActionPanel
      :primary-action="primaryReadyAction"
      :primary-action-summary="primaryReadyActionSummary"
      :recommended-command-loading="recommendedCommandLoading"
      :recommended-command-draft="recommendedCommandDraft"
      :show-first-device-ready-handoff="showFirstDeviceReadyHandoff"
      :steps="steps"
      @run-primary-action="primaryReadyAction.action"
      @open-command-center="openCommandCenter"
      @open-first-device-home-proof="openFirstDeviceHomeProof"
      @open-first-device-automation="openFirstDeviceAutomation"
      @open-first-device-dashboard="openFirstDeviceDashboard"
      @run-step="runReadyCheckStep"
    />
  </div>
</template>

<style scoped>
.ready-check {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.ready-check-collection-warning :deep(.n-alert-body__content) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.ready-check-collection-warning__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-collection-warning__copy strong {
  color: rgb(var(--warning-800-color));
  font-size: var(--font-size-base);
}

.ready-check-collection-warning__copy span {
  color: rgb(var(--warning-800-color));
  font-size: var(--font-size-secondary);
  line-height: 1.5;
}

.ready-check-deferred-placeholder {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  border: 1px dashed var(--border-color);
  border-radius: 16px;
  background: linear-gradient(135deg, var(--action-color) 0%, rgb(var(--primary-color) / 0.06) 100%);
  padding: 18px 20px;
}

.ready-check-deferred-placeholder__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-deferred-placeholder__copy span {
  color: rgb(var(--primary-color));
  font-size: var(--font-size-caption);
  font-weight: 700;
}

.ready-check-deferred-placeholder__copy strong {
  color: var(--text-color-1);
  font-size: 15px;
}

.ready-check-deferred-placeholder__copy p {
  margin: 0;
  color: var(--text-color-2);
  font-size: var(--font-size-secondary);
  line-height: 1.6;
}

@media (max-width: 900px) {
  .ready-check-collection-warning :deep(.n-alert-body__content) {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
