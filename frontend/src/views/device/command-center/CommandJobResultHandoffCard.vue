<script setup lang="ts">
defineProps<{
  handoffSummary: string
}>()

const emit = defineEmits<{
  copyHandoffSummary: []
  copyCloseoutPacket: []
}>()
</script>

<template>
  <div class="command-job-handoff-card">
    <div>
      <strong>{{ $t('custom.commandCenter.handoffSummaryTitle') }}</strong>
      <span>{{ handoffSummary }}</span>
    </div>
    <NButton size="small" secondary @click="emit('copyHandoffSummary')">
      {{ $t('custom.commandCenter.copyHandoffSummary') }}
    </NButton>
    <NButton size="small" secondary @click="emit('copyCloseoutPacket')">
      {{ $t('custom.commandCenter.copyCloseoutPacket') }}
    </NButton>
  </div>
</template>

<style scoped>
.command-job-handoff-card {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: flex-start;
  padding: 12px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #f0fdf4;
}

.command-job-handoff-card > div {
  display: grid;
  flex: 1 1 280px;
  gap: 4px;
  min-width: 0;
}

.command-job-handoff-card strong {
  color: #14532d;
  font-size: 13px;
}

.command-job-handoff-card span {
  overflow-wrap: anywhere;
  color: #166534;
  font-size: 12px;
  line-height: 1.5;
}
</style>
