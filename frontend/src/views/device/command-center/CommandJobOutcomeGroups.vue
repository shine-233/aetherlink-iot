<script setup lang="ts">
import type { CommandJobOutcomeGroup } from './commandCenterJobView'

defineProps<{
  groups: CommandJobOutcomeGroup[]
}>()

const emit = defineEmits<{
  openDeviceDiagnosis: [deviceId: string]
}>()
</script>

<template>
  <div class="command-job-outcomes">
    <NAlert
      v-for="group in groups"
      :key="group.key"
      :type="group.type"
      :show-icon="false"
      class="command-job-outcome"
    >
      <div class="command-job-outcome__head">
        <strong>{{ group.title }}</strong>
        <NTag size="small" type="info">{{ group.count }}</NTag>
      </div>
      <span>{{ group.description }}</span>
      <div v-if="group.rows.length" class="command-job-outcome__devices">
        <div v-for="row in group.rows" :key="row.key" class="command-job-outcome__device">
          <strong>{{ row.device }}</strong>
          <span>{{ row.status }}</span>
          <span>{{ row.readiness }}</span>
          <span>{{ row.reason }}</span>
          <div class="command-job-outcome__action">
            <em>{{ row.action }}</em>
            <NButton
              v-if="row.deviceId"
              size="tiny"
              secondary
              type="primary"
              @click="emit('openDeviceDiagnosis', row.deviceId)"
            >
              {{ $t('custom.commandCenter.openDeviceDiagnosis') }}
            </NButton>
          </div>
        </div>
      </div>
    </NAlert>
  </div>
</template>

<style scoped>
.command-job-outcomes {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-job-outcome :deep(.n-alert-body__content) {
  display: grid;
  gap: 8px;
}

.command-job-outcome__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.command-job-outcome__head strong {
  overflow-wrap: anywhere;
  font-size: 13px;
}

.command-job-outcome span {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.command-job-outcome__devices {
  display: grid;
  gap: 6px;
}

.command-job-outcome__device {
  display: grid;
  grid-template-columns: minmax(90px, 1.2fr) minmax(70px, 0.8fr) minmax(90px, 1fr) minmax(90px, 1fr) minmax(120px, 1.6fr);
  gap: 8px;
  align-items: start;
  padding: 8px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #fff;
}

.command-job-outcome__device strong,
.command-job-outcome__device span,
.command-job-outcome__device em {
  overflow-wrap: anywhere;
  font-size: 12px;
}

.command-job-outcome__device em {
  color: #0f172a;
  font-style: normal;
}

.command-job-outcome__action {
  display: grid;
  gap: 6px;
  align-items: start;
}

@media (max-width: 900px) {
  .command-job-outcomes,
  .command-job-outcome__device {
    grid-template-columns: 1fr;
  }
}
</style>
