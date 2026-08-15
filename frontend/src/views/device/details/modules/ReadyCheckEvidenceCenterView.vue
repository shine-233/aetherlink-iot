<script setup lang="ts">
import { $t } from '@/locales'
import type { ReadyCheckEvidenceCard } from './device-access-guide-state'
import type { ReadyCheckDeepLink } from './ready-check-deep-links'
import type { ReadyCheckSupportBackendStep, ReadyCheckSupportEvidenceCenterItem } from './ready-check-support-bundle'
import ReadyCheckEvidenceCardsView from './ReadyCheckEvidenceCardsView.vue'
import ReadyCheckEvidenceLinksView from './ReadyCheckEvidenceLinksView.vue'

defineProps<{
  loading: boolean
  readySummary: string
  latestTelemetryText: string
  nextActions: string[]
  evidenceCenterItems: ReadyCheckSupportEvidenceCenterItem[]
  evidenceCards: ReadyCheckEvidenceCard[]
  backendNextSteps: ReadyCheckSupportBackendStep[]
  deepLinks: ReadyCheckDeepLink[]
}>()

const emit = defineEmits<{
  refresh: []
  copySupportBundle: []
  downloadSupportBundle: []
  openDeepLink: [link: ReadyCheckDeepLink]
  copyDeepLink: [link: ReadyCheckDeepLink]
  copyAllDeepLinks: []
  runEvidenceCard: [card: ReadyCheckEvidenceCard]
}>()

</script>

<template>
  <section class="ready-check-diagnostics" data-testid="device-ready-check-diagnostics">
    <div class="ready-check-diagnostics__main">
      <span>{{ $t('custom.device_details.accessGuideReadyCheck') }}</span>
      <strong>{{ loading ? $t('common.loading') : readySummary }}</strong>
    </div>
    <div class="ready-check-diagnostics__side">
      <div>
        <span>{{ $t('custom.device_details.accessGuideLatestTelemetry') }}</span>
        <strong>{{ latestTelemetryText }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.device_details.accessGuideDiagnosticNextActions') }}</span>
        <strong>{{ nextActions.length ? nextActions.join('; ') : $t('custom.device_details.accessGuideDiagnosticUnknown') }}</strong>
      </div>
    </div>
    <NButton
      size="small"
      secondary
      :loading="loading"
      data-testid="device-ready-check-refresh"
      @click="emit('refresh')"
    >
      {{ $t('custom.device_details.accessGuideDiagnosticRefresh') }}
    </NButton>
    <NButton
      size="small"
      secondary
      type="primary"
      data-testid="device-ready-check-support-bundle"
      @click="emit('copySupportBundle')"
    >
      {{ $t('custom.commandCenter.copySupportBundle') }}
    </NButton>
    <NButton
      size="small"
      secondary
      data-testid="device-ready-check-download-support-bundle"
      @click="emit('downloadSupportBundle')"
    >
      {{ $t('custom.device_details.readyCheckDownloadSupportBundle') }}
    </NButton>
  </section>

  <section class="ready-check-evidence-center" data-testid="device-ready-check-evidence-center">
    <div class="ready-check-evidence-center__head">
      <h3>{{ $t('custom.device_details.readyCheckEvidenceCenterTitle') }}</h3>
      <p>{{ $t('custom.device_details.readyCheckEvidenceCenterDesc') }}</p>
    </div>
    <div class="ready-check-evidence-center__grid">
      <div v-for="item in evidenceCenterItems" :key="item.key" class="ready-check-evidence-center__item">
        <span>{{ $t(item.labelKey) }}</span>
        <strong>{{ item.value }}</strong>
        <p>{{ item.detail }}</p>
      </div>
    </div>
  </section>

  <ReadyCheckEvidenceLinksView
    :links="deepLinks"
    @open="emit('openDeepLink', $event)"
    @copy="emit('copyDeepLink', $event)"
    @copy-all="emit('copyAllDeepLinks')"
  />

  <section v-if="backendNextSteps.length" class="ready-check-backend-steps" data-testid="device-ready-check-backend-steps">
    <div class="ready-check-evidence-center__head">
      <h3>{{ $t('custom.device_details.readyCheckEvidenceNextStepsTitle') }}</h3>
      <p>{{ $t('custom.device_details.readyCheckEvidenceNextStepsDesc') }}</p>
    </div>
    <div class="ready-check-backend-steps__list">
      <div v-for="step in backendNextSteps" :key="step.key" class="ready-check-backend-steps__item">
        <NTag size="small" type="info">{{ step.status }}</NTag>
        <div>
          <strong>{{ step.title }}</strong>
          <p>{{ step.description }}</p>
        </div>
      </div>
    </div>
  </section>

  <ReadyCheckEvidenceCardsView
    :evidence-cards="evidenceCards"
    @run="emit('runEvidenceCard', $event)"
  />
</template>

<style scoped>
.ready-check-diagnostics,
.ready-check-evidence-center,
.ready-check-backend-steps {
  min-width: 0;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.ready-check-diagnostics {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.8fr) auto;
  align-items: start;
  gap: 12px;
  padding: 14px;
  border-left: 4px solid #18a058;
}

.ready-check-diagnostics span {
  color: #64748b;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-diagnostics strong {
  overflow: hidden;
  color: #0f172a;
  text-overflow: ellipsis;
  overflow-wrap: anywhere;
}

.ready-check-diagnostics__main,
.ready-check-diagnostics__side > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.ready-check-diagnostics__side {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  min-width: 0;
}

.ready-check-evidence-center,
.ready-check-backend-steps {
  display: grid;
  gap: 12px;
  padding: 14px;
}

.ready-check-evidence-center__head {
  display: grid;
  gap: 4px;
}

.ready-check-evidence-center__head h3 {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
}

.ready-check-evidence-center__head p {
  margin: 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-evidence-center__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.ready-check-evidence-center__item {
  min-width: 0;
  border-radius: 6px;
  background: #f8fafc;
  padding: 10px;
}

.ready-check-evidence-center__item span {
  display: block;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

.ready-check-evidence-center__item strong {
  display: block;
  margin-top: 3px;
  color: #0f172a;
  overflow-wrap: anywhere;
}

.ready-check-evidence-center__item p {
  margin: 5px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.ready-check-backend-steps__list {
  display: grid;
  gap: 8px;
}

.ready-check-backend-steps__item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: 10px;
  border-radius: 6px;
  background: #f8fafc;
  padding: 10px;
}

.ready-check-backend-steps__item strong {
  color: #0f172a;
  font-size: 13px;
}

.ready-check-backend-steps__item p {
  margin: 3px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .ready-check-diagnostics,
  .ready-check-diagnostics__side,
  .ready-check-evidence-center__grid {
    grid-template-columns: 1fr;
  }
}
</style>
