<script setup lang="ts">
defineProps<{
  statusLabel: string
  progressPercent: number
  progressSummary: string
}>()
</script>

<template>
  <div class="command-job-progress-card">
    <NTag type="info" size="small">{{ statusLabel }}</NTag>
    <NProgress type="line" :percentage="progressPercent" :show-indicator="false" />
    <span>{{ progressSummary }}</span>
  </div>
</template>

<style scoped>
.command-job-progress-card {
  display: grid;
  grid-template-columns: auto minmax(120px, 1fr) minmax(0, auto);
  gap: 8px;
  align-items: center;
}

.command-job-progress-card span {
  color: #334155;
  font-size: 12px;
}
</style>
