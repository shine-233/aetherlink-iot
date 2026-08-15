<script setup lang="ts">
import { $t } from '@/locales'
import type { ReadyCheckEvidenceCard } from './device-access-guide-state'

defineProps<{
  evidenceCards: ReadyCheckEvidenceCard[]
}>()

const emit = defineEmits<{
  run: [card: ReadyCheckEvidenceCard]
}>()

const evidenceCardSummaryText = (card: ReadyCheckEvidenceCard) => {
  return card.summaryKey ? $t(card.summaryKey) : card.summary
}

const evidenceCardNextActionTexts = (card: ReadyCheckEvidenceCard) => {
  return [...(card.nextActionKeys || []).map((key) => $t(key)), ...card.nextActions]
}

const evidenceCardActionKey = (card: ReadyCheckEvidenceCard) => {
  if (card.key === 'twin') return 'custom.device_details.readyCheckOpenTwin'
  return card.status === 'next'
    ? 'custom.device_details.readyCheckOpenCommandCenter'
    : 'custom.device_details.readyCheckOpenCommand'
}

const statusTagType = (status: string) => {
  if (status === 'ready') return 'success'
  if (status === 'attention') return 'warning'
  return 'info'
}
</script>

<template>
  <section class="ready-check-evidence" data-testid="device-ready-check-evidence">
    <div v-for="card in evidenceCards" :key="card.key" class="ready-check-evidence-card">
      <div class="ready-check-card__head">
        <NTag :type="statusTagType(card.status)" size="small">
          {{ $t(card.statusKey) }}
        </NTag>
        <h3>{{ $t(card.titleKey) }}</h3>
      </div>
      <p>{{ evidenceCardSummaryText(card) }}</p>
      <div class="ready-check-evidence-metrics">
        <div v-for="metric in card.metrics" :key="metric.key">
          <span>{{ $t(metric.labelKey) }}</span>
          <strong :class="`ready-check-evidence-metric--${metric.tone}`">{{ metric.value }}</strong>
        </div>
      </div>
      <small class="ready-check-evidence-boundary">{{ $t(card.boundaryKey) }}</small>
      <div v-if="evidenceCardNextActionTexts(card).length" class="ready-check-evidence-next">
        <span>{{ $t('custom.device_details.accessGuideDiagnosticNextActions') }}</span>
        <strong>{{ evidenceCardNextActionTexts(card).join('; ') }}</strong>
      </div>
      <NButton size="small" secondary type="primary" @click="emit('run', card)">
        {{ $t(evidenceCardActionKey(card)) }}
      </NButton>
    </div>
  </section>
</template>

<style scoped>
.ready-check-evidence {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.ready-check-evidence-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  padding: 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.ready-check-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ready-check-card__head h3 {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
}

.ready-check-evidence-card p {
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-evidence-boundary {
  border-radius: 6px;
  background: #f8fafc;
  padding: 8px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.ready-check-evidence-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.ready-check-evidence-metrics > div {
  min-width: 0;
  border-radius: 6px;
  background: #f8fafc;
  padding: 8px;
}

.ready-check-evidence-metrics span,
.ready-check-evidence-next span {
  display: block;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

.ready-check-evidence-metrics strong,
.ready-check-evidence-next strong {
  display: block;
  margin-top: 3px;
  color: #0f172a;
  overflow-wrap: anywhere;
}

.ready-check-evidence-metric--success {
  color: #166534 !important;
}

.ready-check-evidence-metric--warning {
  color: #b45309 !important;
}

.ready-check-evidence-metric--danger {
  color: #b91c1c !important;
}

.ready-check-evidence-next {
  border-top: 1px solid #e5e7eb;
  padding-top: 8px;
}

@media (max-width: 900px) {
  .ready-check-evidence {
    grid-template-columns: 1fr;
  }
}
</style>
