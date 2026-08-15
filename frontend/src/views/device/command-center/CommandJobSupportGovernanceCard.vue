<script setup lang="ts">
import type { CommandJobGovernanceSummaryCard } from './commandCenterJobView'

defineProps<{
  governanceSummary: CommandJobGovernanceSummaryCard
}>()
</script>

<template>
  <div class="command-support-governance">
    <div class="command-support-governance__head">
      <div>
        <strong>{{ $t('custom.commandCenter.governanceSummaryTitle') }}</strong>
        <span>{{ governanceSummary.title }}</span>
      </div>
      <NTag :type="governanceSummary.type" size="small">
        {{ governanceSummary.levelLabel }}
      </NTag>
    </div>
    <NAlert :type="governanceSummary.type" :show-icon="false">
      {{ governanceSummary.summary }}
    </NAlert>
    <div class="command-support-governance__grid">
      <div v-for="item in governanceSummary.items" :key="item.key" class="command-support-governance__item">
        <NTag :type="item.type" size="small">{{ item.stateLabel }}</NTag>
        <div>
          <strong>{{ item.label }}</strong>
          <span>{{ item.value }}</span>
          <small>{{ item.detail }}</small>
        </div>
      </div>
    </div>
    <NAlert type="info" :show-icon="false">
      {{ governanceSummary.nextAction }}
    </NAlert>
  </div>
</template>

<style scoped>
.command-support-governance {
  display: grid;
  gap: 10px;
  width: 100%;
  padding: 10px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #ffffff;
}

.command-support-governance__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-support-governance__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-support-governance__head strong {
  color: #166534;
  font-size: 13px;
}

.command-support-governance__head span {
  overflow-wrap: anywhere;
  color: #15803d;
  font-size: 12px;
}

.command-support-governance__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-support-governance__item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 8px;
  border: 1px solid #dcfce7;
  border-radius: 8px;
  background: #f8fafc;
}

.command-support-governance__item > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.command-support-governance__item strong,
.command-support-governance__item span,
.command-support-governance__item small {
  overflow-wrap: anywhere;
}

.command-support-governance__item strong {
  color: #0f172a;
  font-size: 12px;
}

.command-support-governance__item span,
.command-support-governance__item small {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .command-support-governance__grid {
    grid-template-columns: 1fr;
  }
}
</style>
