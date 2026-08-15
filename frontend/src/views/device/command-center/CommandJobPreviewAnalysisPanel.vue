<script setup lang="ts">
import { computed, ref } from 'vue'
import { $t } from '@/locales'
import { buildCommandJobGovernanceSummaryCard } from './commandCenterJobView'
import { filterCommandJobPreviewRowsByImpactGroup } from './commandCenterState'
import type { CommandJobEligibilityImpactFilterKey } from './commandCenterState'

const previewTableRowLimit = 50

const props = defineProps<{
  activeJobWarnings: string[]
  commandJobEligibilityImpactPreview: any
  commandJobPreviewActionPlan: any
  filteredFleetEligibilityPreview: any
  previewColumns: any[]
  previewResult: any
  previewTokenShort: string
  routeDecisionSummary: any
}>()

const emit = defineEmits<{
  copyEligibilityImpactSummary: []
}>()

const previewImpactFilter = ref<CommandJobEligibilityImpactFilterKey>('all')

const previewRows = computed(() => {
  const rows = props.previewResult?.rows
  if (!Array.isArray(rows)) return []
  return rows
})

const previewRowsForImpactFilter = computed(() =>
  filterCommandJobPreviewRowsByImpactGroup(previewRows.value, previewImpactFilter.value)
)

const previewRowsForDisplay = computed(() => previewRowsForImpactFilter.value.slice(0, previewTableRowLimit))

const activePreviewImpactFilterLabel = computed(() => {
  if (previewImpactFilter.value === 'all') return ''
  const group = props.commandJobEligibilityImpactPreview?.groups?.find((item: any) => item.key === previewImpactFilter.value)
  return group ? $t(group.labelKey) : ''
})

const previewTotalRows = computed(() => previewRowsForImpactFilter.value.length)
const hiddenPreviewRows = computed(() => Math.max(0, previewTotalRows.value - previewRowsForDisplay.value.length))

const previewGovernanceSummaryCard = computed(() =>
  buildCommandJobGovernanceSummaryCard(props.previewResult?.governance_summary, $t)
)

const formatFilteredFleetPreviewMessage = (t: (key: string) => string, preview: any) => {
  if (!preview) return ''
  return t(preview.messageKey)
    .replace('{shown}', String(preview.shownCount))
    .replace('{requested}', String(preview.requestedCount))
    .replace('{matched}', String(preview.totalMatched))
    .replace('{eligible}', String(preview.subsetEligibleCount))
    .replace('{blocked}', String(preview.subsetBlockedCount))
    .replace('{immediate}', String(preview.immediateCount))
    .replace('{jobs}', String(preview.jobsCount))
    .replace('{pathBlocked}', String(preview.blockedPathCount))
    .replace('{telemetry}', String(preview.telemetryCount))
}
</script>

<template>
  <div v-if="commandJobEligibilityImpactPreview" class="command-impact-preview">
    <div class="command-impact-preview__head">
      <div>
        <strong>{{ $t('custom.commandCenter.impactPreviewTitle') }}</strong>
        <span>
          {{
            $t('custom.commandCenter.impactPreviewDesc')
              .replace('{shown}', String(commandJobEligibilityImpactPreview.shownCount))
              .replace('{requested}', String(commandJobEligibilityImpactPreview.requestedCount))
          }}
        </span>
      </div>
      <NSpace :size="[8, 8]" align="center">
        <NButton size="small" secondary @click="emit('copyEligibilityImpactSummary')">
          {{ $t('custom.commandCenter.copyImpactPreview') }}
        </NButton>
        <NTag :type="commandJobEligibilityImpactPreview.coverageType" size="small">
          {{ $t(commandJobEligibilityImpactPreview.coverageLabelKey) }}
        </NTag>
      </NSpace>
    </div>
    <div class="command-impact-preview__grid">
      <div
        v-for="group in commandJobEligibilityImpactPreview.groups"
        :key="group.key"
        class="command-impact-preview__group"
      >
        <div class="command-impact-preview__group-head">
          <div>
            <span>{{ $t(group.labelKey) }}</span>
            <small>{{ $t(group.descriptionKey) }}</small>
          </div>
          <NSpace :size="[6, 6]" align="center">
            <NButton
              size="tiny"
              secondary
              :disabled="group.count === 0"
              @click="previewImpactFilter = group.key"
            >
              {{ $t('custom.commandCenter.impactPreviewShowRows') }}
            </NButton>
            <NTag :type="group.type" size="small">{{ group.count }}</NTag>
          </NSpace>
        </div>
        <div v-if="group.representativeRows.length" class="command-impact-preview__representatives">
          <div
            v-for="representative in group.representativeRows"
            :key="representative.key"
            class="command-impact-preview__representative"
          >
            <strong>{{ representative.device }}</strong>
            <span>{{ representative.reason }}</span>
            <small>{{ representative.advice }}</small>
          </div>
        </div>
      </div>
    </div>
    <NAlert type="info" :show-icon="false">
      {{ commandJobEligibilityImpactPreview.nextAction }}
    </NAlert>
  </div>

  <div v-if="previewGovernanceSummaryCard" class="command-preview-governance">
    <div class="command-preview-governance__head">
      <div>
        <strong>{{ $t('custom.commandCenter.governanceSummaryTitle') }}</strong>
        <span>{{ previewGovernanceSummaryCard.title }}</span>
      </div>
      <NTag :type="previewGovernanceSummaryCard.type" size="small">
        {{ previewGovernanceSummaryCard.levelLabel }}
      </NTag>
    </div>
    <NAlert :type="previewGovernanceSummaryCard.type" :show-icon="false">
      {{ previewGovernanceSummaryCard.summary }}
    </NAlert>
    <div class="command-preview-governance__grid">
      <div
        v-for="item in previewGovernanceSummaryCard.items"
        :key="item.key"
        class="command-preview-governance__item"
      >
        <NTag :type="item.type" size="small">
          {{ item.stateLabel }}
        </NTag>
        <div>
          <strong>{{ item.label }}</strong>
          <span>{{ item.value }}</span>
          <small>{{ item.detail }}</small>
        </div>
      </div>
    </div>
    <NAlert type="info" :show-icon="false">
      {{ previewGovernanceSummaryCard.nextAction }}
    </NAlert>
  </div>

  <div v-if="commandJobPreviewActionPlan" class="command-preview-plan">
    <div class="command-preview-plan__head">
      <div>
        <strong>{{ $t('custom.commandCenter.previewPlanTitle') }}</strong>
        <span>{{ $t('custom.commandCenter.previewPlanDesc') }}</span>
      </div>
      <NTag type="info" size="small">{{ $t('custom.commandCenter.nextAction') }}</NTag>
    </div>
    <div class="command-preview-plan__grid">
      <div v-for="card in commandJobPreviewActionPlan.cards" :key="card.key" class="command-preview-plan__card">
        <span>{{ $t(card.labelKey) }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </div>
    <NAlert type="info" :show-icon="false">
      {{ commandJobPreviewActionPlan.nextAction }}
    </NAlert>
    <div v-if="commandJobPreviewActionPlan.blockers.length" class="command-preview-plan__blockers">
      <strong>{{ $t('custom.commandCenter.previewPlanTopBlockers') }}</strong>
      <div
        v-for="blocker in commandJobPreviewActionPlan.blockers"
        :key="`${blocker.reason}-${blocker.advice || ''}`"
        class="command-preview-plan__blocker"
      >
        <NTag type="warning" size="small">
          {{ $t('custom.commandCenter.previewPlanBlockedCount').replace('{count}', String(blocker.count)) }}
        </NTag>
        <div>
          <span>{{ blocker.reason }}</span>
          <small v-if="blocker.advice">{{ blocker.advice }}</small>
        </div>
      </div>
    </div>
  </div>

  <NAlert
    v-if="filteredFleetEligibilityPreview"
    :type="filteredFleetEligibilityPreview.alertType"
    :show-icon="false"
  >
    {{ formatFilteredFleetPreviewMessage($t, filteredFleetEligibilityPreview) }}
  </NAlert>
  <NAlert v-for="warning in activeJobWarnings" :key="warning" type="warning" :show-icon="false">
    {{ warning }}
  </NAlert>
  <NAlert v-if="previewResult" type="info" :show-icon="false">
    {{
      $t('custom.commandCenter.previewSummary')
        .replace('{requested}', String(previewResult.requested_count))
        .replace('{eligible}', String(previewResult.eligible_count))
        .replace('{blocked}', String(previewResult.blocked_count))
    }}
  </NAlert>
  <NAlert v-if="previewResult" type="success" :show-icon="false">
    {{
      $t('custom.commandCenter.routeDecisionSummary')
        .replace('{immediate}', String(routeDecisionSummary.immediate))
        .replace('{jobs}', String(routeDecisionSummary.jobs))
        .replace('{blocked}', String(routeDecisionSummary.blocked))
        .replace('{telemetry}', String(routeDecisionSummary.telemetry))
    }}
  </NAlert>
  <NAlert v-if="previewResult" type="info" :show-icon="false">
    {{ $t('custom.commandCenter.previewTokenSummary').replace('{token}', previewTokenShort) }}
  </NAlert>
  <NAlert v-if="previewResult && activePreviewImpactFilterLabel" type="info" :show-icon="false">
    <NSpace justify="space-between" align="center">
      <span>
        {{
          $t('custom.commandCenter.impactPreviewActiveFilter').replace('{group}', activePreviewImpactFilterLabel)
        }}
      </span>
      <NButton size="tiny" secondary @click="previewImpactFilter = 'all'">
        {{ $t('custom.commandCenter.impactPreviewShowAllRows') }}
      </NButton>
    </NSpace>
  </NAlert>
  <NAlert v-if="previewResult && hiddenPreviewRows > 0" type="warning" :show-icon="false">
    {{
      $t('custom.commandCenter.previewDisplayLimit')
        .replace('{shown}', String(previewRowsForDisplay.length))
        .replace('{total}', String(previewTotalRows))
        .replace('{hidden}', String(hiddenPreviewRows))
    }}
  </NAlert>
  <NDataTable
    v-if="previewResult"
    size="small"
    :columns="previewColumns"
    :data="previewRowsForDisplay"
    :pagination="false"
    :bordered="false"
  />
</template>

<style scoped>
.command-impact-preview {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #c7d2fe;
  border-radius: 8px;
  background: #eef2ff;
}

.command-impact-preview__head,
.command-impact-preview__group-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-impact-preview__head > div,
.command-impact-preview__group-head > div,
.command-impact-preview__representative {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-impact-preview__head strong {
  color: #312e81;
  font-size: 14px;
}

.command-impact-preview__head span,
.command-impact-preview__group-head small,
.command-impact-preview__representative span,
.command-impact-preview__representative small {
  overflow-wrap: anywhere;
  color: #4338ca;
  font-size: 12px;
  line-height: 1.45;
}

.command-impact-preview__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-impact-preview__group {
  display: grid;
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #ddd6fe;
  border-radius: 8px;
  background: #ffffff;
}

.command-impact-preview__group-head span {
  color: #0f172a;
  font-size: 13px;
  font-weight: 600;
}

.command-impact-preview__representatives {
  display: grid;
  gap: 6px;
}

.command-impact-preview__representative {
  padding: 8px;
  border-radius: 6px;
  background: #f8fafc;
}

.command-impact-preview__representative strong {
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 12px;
}

.command-preview-governance {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #f0fdf4;
}

.command-preview-governance__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-preview-governance__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-preview-governance__head strong {
  color: #14532d;
  font-size: 14px;
}

.command-preview-governance__head span {
  overflow-wrap: anywhere;
  color: #166534;
  font-size: 12px;
  line-height: 1.5;
}

.command-preview-governance__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-preview-governance__item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #dcfce7;
  border-radius: 8px;
  background: #ffffff;
}

.command-preview-governance__item > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.command-preview-governance__item strong,
.command-preview-governance__item span,
.command-preview-governance__item small {
  overflow-wrap: anywhere;
}

.command-preview-governance__item strong {
  color: #0f172a;
  font-size: 12px;
}

.command-preview-governance__item span,
.command-preview-governance__item small {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.command-preview-plan {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  background: #f0f9ff;
}

.command-preview-plan__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-preview-plan__head > div,
.command-preview-plan__blockers,
.command-preview-plan__blocker > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-preview-plan__head strong,
.command-preview-plan__blockers strong {
  color: #0c4a6e;
  font-size: 14px;
}

.command-preview-plan__head span,
.command-preview-plan__blocker span,
.command-preview-plan__blocker small {
  overflow-wrap: anywhere;
  color: #075985;
  font-size: 12px;
  line-height: 1.45;
}

.command-preview-plan__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.command-preview-plan__card {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #e0f2fe;
  border-radius: 8px;
  background: #ffffff;
}

.command-preview-plan__card span {
  color: #64748b;
  font-size: 12px;
}

.command-preview-plan__card strong {
  color: #0f172a;
  font-size: 18px;
}

.command-preview-plan__blocker {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: flex-start;
  gap: 8px;
  padding: 8px;
  border: 1px solid #fde68a;
  border-radius: 8px;
  background: #fffbeb;
}

@media (max-width: 900px) {
  .command-preview-plan__grid,
  .command-impact-preview__grid {
    grid-template-columns: 1fr;
  }
}
</style>
