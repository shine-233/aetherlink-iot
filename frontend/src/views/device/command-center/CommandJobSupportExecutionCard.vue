<script setup lang="ts">
import type { CommandJobExecutionSummaryCard } from './commandCenterJobView'

defineProps<{
  executionSummary: CommandJobExecutionSummaryCard
}>()
</script>

<template>
  <div class="command-support-execution">
    <div class="command-support-execution__head">
      <div>
        <strong>{{ $t('custom.commandCenter.executionSummaryTitle') }}</strong>
        <span>{{ executionSummary.pathLabel }}</span>
      </div>
      <NTag :type="executionSummary.type" size="small">
        {{ executionSummary.decisionLabel }}
      </NTag>
    </div>
    <NAlert :type="executionSummary.type" :show-icon="false">
      {{ executionSummary.nextAction }}
    </NAlert>
    <NAlert v-if="executionSummary.closeBlockers.length" type="warning" :show-icon="false">
      <strong>{{ $t('custom.commandCenter.closeReadinessBlocked') }}</strong>
      <ul>
        <li v-for="blocker in executionSummary.closeBlockers" :key="blocker">
          {{ blocker }}
        </li>
      </ul>
    </NAlert>
    <NAlert v-else-if="executionSummary.canClose" type="success" :show-icon="false">
      {{ $t('custom.commandCenter.closeReadinessReady') }}
    </NAlert>
    <div v-if="executionSummary.evidence.length" class="command-support-execution__evidence">
      <NTag v-for="item in executionSummary.evidence" :key="item" size="small" type="info">
        {{ item }}
      </NTag>
    </div>
    <div v-if="executionSummary.checklist.length" class="command-support-execution__checklist">
      <div v-for="item in executionSummary.checklist" :key="item.key" class="command-support-execution__check">
        <NTag :type="item.type" size="small">{{ item.stateLabel }}</NTag>
        <div>
          <strong>{{ item.label }}</strong>
          <span>{{ item.detail }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.command-support-execution {
  display: grid;
  gap: 10px;
  width: 100%;
  padding: 10px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #ffffff;
}

.command-support-execution__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-support-execution__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-support-execution__head strong {
  color: #1d4ed8;
  font-size: 13px;
}

.command-support-execution__head span {
  overflow-wrap: anywhere;
  color: #1e40af;
  font-size: 12px;
}

.command-support-execution__checklist {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-support-execution__evidence {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.command-support-execution__check {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 8px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fafc;
}

.command-support-execution__check > div {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.command-support-execution__check strong,
.command-support-execution__check span {
  overflow-wrap: anywhere;
  font-size: 12px;
}

.command-support-execution__check strong {
  color: #0f172a;
}

.command-support-execution__check span {
  color: #475569;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .command-support-execution__checklist {
    grid-template-columns: 1fr;
  }
}
</style>
