<script setup lang="ts">
import { nextTick, ref } from 'vue'
import type { CommandJobResultActions, CommandJobResultViewModel } from './commandCenterJobResultViewModel'
import CommandJobRowsTable from './CommandJobRowsTable.vue'
import CommandJobSupportPreview from './CommandJobSupportPreview.vue'

defineProps<{
  jobResult: CommandJobResultViewModel
  jobActions: CommandJobResultActions
}>()

const rowsTableRef = ref<HTMLElement | null>(null)

const scrollRowsTableIntoView = async () => {
  await nextTick()
  rowsTableRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

defineExpose({ scrollRowsTableIntoView })
</script>

<template>
  <section class="command-job-result-section command-job-result-section--evidence">
    <div class="command-job-result-section__head">
      <strong>{{ $t('custom.commandCenter.submitEvidenceTimelineTitle') }}</strong>
      <span>{{ $t('custom.commandCenter.submitEvidenceTimelineDesc') }}</span>
    </div>

    <NDescriptions bordered :column="3" size="small">
      <NDescriptionsItem v-for="row in jobResult.jobStatusRows" :key="row.label" :label="row.label">
        {{ row.value }}
      </NDescriptionsItem>
    </NDescriptions>

    <div v-if="jobResult.jobStatusCountRows.length" class="command-job-status-counts">
      <NTag v-for="row in jobResult.jobStatusCountRows" :key="row.status" size="small" type="info">
        {{ row.label }}: {{ row.count }}
      </NTag>
    </div>

    <NAlert v-if="jobResult.submitResult?.audit_remark" type="warning" :show-icon="false">
      <strong>{{ $t('custom.commandCenter.auditRemarkTitle') }}</strong>
      <span>{{ jobResult.submitResult.audit_remark }}</span>
    </NAlert>

    <div v-if="jobResult.jobAuditSummaryCard" class="command-job-audit-card">
      <div class="command-job-audit-card__head">
        <div>
          <strong>{{ $t('custom.commandCenter.auditSummaryTitle') }}</strong>
          <span>{{ jobResult.jobAuditSummaryCard.nextAction }}</span>
        </div>
        <NTag size="small" type="info">{{ jobResult.jobAuditSummaryCard.latestLabel }}</NTag>
      </div>
      <div class="command-job-audit-card__grid">
        <div v-for="row in jobResult.jobAuditSummaryCard.rows" :key="row.label">
          <span>{{ row.label }}</span>
          <strong>{{ row.value }}</strong>
        </div>
      </div>
    </div>

    <div class="command-job-timeline">
      <div v-for="row in jobResult.jobTimelineRows" :key="row.key || row.label" class="command-job-timeline__item">
        <span>{{ row.label }}</span>
        <strong>{{ row.value }}</strong>
      </div>
    </div>

    <CommandJobSupportPreview
      v-if="jobResult.supportBundlePreview"
      :support-bundle-preview="jobResult.supportBundlePreview"
      @open-device-diagnosis="jobActions.openCommandJobDeviceDiagnosis"
    />

    <div ref="rowsTableRef" class="command-job-rows-table-anchor">
      <CommandJobRowsTable :job-result="jobResult" :job-actions="jobActions" />
    </div>
  </section>
</template>

<style scoped>
.command-job-result-section {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.command-job-result-section--evidence {
  border-color: #cbd5e1;
  background: #f8fafc;
}

.command-job-result-section__head {
  display: grid;
  gap: 4px;
}

.command-job-result-section__head strong {
  color: #0f172a;
  font-size: 14px;
}

.command-job-result-section__head span {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.command-job-status-counts,
.command-job-timeline {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.command-job-rows-table-anchor {
  display: grid;
  gap: 10px;
  scroll-margin-top: 16px;
}

.command-job-audit-card {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #c7d2fe;
  border-radius: 8px;
  background: #eef2ff;
}

.command-job-audit-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-job-audit-card__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-job-audit-card__head strong {
  color: #3730a3;
  font-size: 14px;
}

.command-job-audit-card__head span {
  overflow-wrap: anywhere;
  color: #4338ca;
  font-size: 12px;
  line-height: 1.5;
}

.command-job-audit-card__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.command-job-audit-card__grid > div {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #e0e7ff;
  border-radius: 8px;
  background: #ffffff;
}

.command-job-audit-card__grid span {
  color: #64748b;
  font-size: 12px;
}

.command-job-audit-card__grid strong {
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 13px;
}

.command-job-timeline__item {
  display: grid;
  gap: 4px;
  min-width: 140px;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.command-job-timeline__item span {
  color: #64748b;
  font-size: 12px;
}

.command-job-timeline__item strong {
  color: #0f172a;
  font-size: 13px;
}

@media (max-width: 900px) {
  .command-job-audit-card__grid {
    grid-template-columns: 1fr;
  }
}
</style>
