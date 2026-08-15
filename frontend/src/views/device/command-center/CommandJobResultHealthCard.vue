<script setup lang="ts">
import type { CommandJobProgressHealthCard } from './commandCenterJobView'

defineProps<{
  healthCard: CommandJobProgressHealthCard
}>()
</script>

<template>
  <div class="command-job-health-card">
    <div class="command-job-health-card__head">
      <strong>{{ $t('custom.commandCenter.progressHealthTitle') }}</strong>
      <NTag :type="healthCard.type" size="small">
        {{ healthCard.stateLabel }}
      </NTag>
    </div>
    <div class="command-job-health-card__grid">
      <div v-for="row in healthCard.rows" :key="row.label">
        <span>{{ row.label }}</span>
        <strong>{{ row.value }}</strong>
      </div>
    </div>
    <NAlert :type="healthCard.type" :show-icon="false">
      {{ healthCard.nextAction }}
    </NAlert>
  </div>
</template>

<style scoped>
.command-job-health-card {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #ddd6fe;
  border-radius: 8px;
  background: #f5f3ff;
}

.command-job-health-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.command-job-health-card__head strong {
  color: #312e81;
  font-size: 14px;
}

.command-job-health-card__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.command-job-health-card__grid > div {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #ede9fe;
  border-radius: 8px;
  background: #ffffff;
}

.command-job-health-card__grid span {
  color: #64748b;
  font-size: 12px;
}

.command-job-health-card__grid strong {
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 13px;
}

@media (max-width: 900px) {
  .command-job-health-card__grid {
    grid-template-columns: 1fr;
  }
}
</style>
