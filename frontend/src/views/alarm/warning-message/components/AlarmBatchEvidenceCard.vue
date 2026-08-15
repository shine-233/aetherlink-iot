<script setup lang="ts">
import type { AlarmBatchActionEvidence } from './alarm-configuration.helpers'

defineProps<{
  evidence: AlarmBatchActionEvidence
  actionLabel: string
}>()

const emit = defineEmits<{
  copy: []
  download: []
}>()
</script>

<template>
  <NCard embedded size="small" class="alarm-batch-evidence-card">
    <div class="alarm-batch-evidence-header">
      <div>
        <div class="alarm-batch-evidence-title">
          {{ $t('custom.alarmPage.batchActionEvidenceTitle') }}
        </div>
        <div class="alarm-batch-evidence-desc">
          {{ $t('custom.alarmPage.batchActionEvidenceDesc') }}
        </div>
      </div>
      <NFlex :size="8" align="center" wrap justify="end">
        <NTag :type="evidence.type" size="small">
          {{ evidence.summary }}
        </NTag>
        <NButton size="small" secondary @click="emit('copy')">
          {{ $t('custom.alarmPage.batchActionEvidenceCopy') }}
        </NButton>
        <NButton size="small" secondary data-testid="alarm-download-batch-evidence" @click="emit('download')">
          {{ $t('custom.alarmPage.downloadEvidenceBundle') }}
        </NButton>
      </NFlex>
    </div>
    <div class="alarm-batch-evidence-grid">
      <div>
        <span>{{ $t('custom.alarmPage.batchActionEvidenceAction') }}</span>
        <strong>{{ actionLabel }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.alarmPage.batchActionEvidenceGeneratedAt') }}</span>
        <strong>{{ evidence.generatedAt }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.alarmPage.batchActionEvidenceExpected') }}</span>
        <strong>{{ evidence.expectedCount }}</strong>
      </div>
      <div>
        <span>{{ $t('custom.alarmPage.batchActionEvidenceNote') }}</span>
        <strong>{{ evidence.note }}</strong>
      </div>
    </div>
    <div class="alarm-batch-evidence-detail">{{ evidence.detail }}</div>
    <div class="alarm-batch-evidence-failures">
      <strong>{{ $t('custom.alarmPage.batchActionFailedRows') }}</strong>
      <span v-if="evidence.failedItems.length === 0">
        {{ $t('custom.alarmPage.batchActionNoFailedRows') }}
      </span>
      <ul v-else>
        <li v-for="item in evidence.failedItems" :key="item">{{ item }}</li>
      </ul>
    </div>
  </NCard>
</template>

<style scoped lang="scss">
.alarm-batch-evidence-card {
  margin-bottom: 12px;
  border: 1px solid #cbd5e1;
  background: #f8fafc;
}

.alarm-batch-evidence-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.alarm-batch-evidence-title {
  color: #0f172a;
  font-weight: 700;
}

.alarm-batch-evidence-desc,
.alarm-batch-evidence-detail,
.alarm-batch-evidence-failures {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.alarm-batch-evidence-desc {
  margin-top: 4px;
}

.alarm-batch-evidence-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.alarm-batch-evidence-grid > div {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.alarm-batch-evidence-grid span {
  color: #64748b;
  font-size: 12px;
}

.alarm-batch-evidence-grid strong,
.alarm-batch-evidence-failures li {
  overflow-wrap: anywhere;
}

.alarm-batch-evidence-detail,
.alarm-batch-evidence-failures {
  margin-top: 10px;
}

.alarm-batch-evidence-failures {
  display: grid;
  gap: 4px;
}

.alarm-batch-evidence-failures ul {
  margin: 0;
  padding-left: 18px;
}

@media (max-width: 900px) {
  .alarm-batch-evidence-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .alarm-batch-evidence-header {
    flex-direction: column;
  }

  .alarm-batch-evidence-grid {
    grid-template-columns: 1fr;
  }
}
</style>
