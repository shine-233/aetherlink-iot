<script setup lang="ts">
import type { CommandJobSupportFailedDeviceEvidence } from './commandCenterJobView'

defineProps<{
  deviceEvidence: CommandJobSupportFailedDeviceEvidence
}>()

const emit = defineEmits<{
  openDeviceDiagnosis: [deviceId: string]
}>()
</script>

<template>
  <div class="command-support-failed-device">
    <div v-if="deviceEvidence.diagnosticSummary" class="command-support-failed-device__diagnosis">
      <div class="command-support-failed-device__diagnosis-head">
        <NTag size="small" :type="deviceEvidence.diagnosticSummary.type">
          {{ $t('custom.commandCenter.supportBundleDiagnosis') }}
        </NTag>
        <strong>{{ deviceEvidence.diagnosticSummary.summary }}</strong>
      </div>
      <span class="command-support-failed-device__diagnosis-code">
        {{ deviceEvidence.diagnosticSummary.code }}
      </span>
      <div
        v-if="deviceEvidence.diagnosticSummary.evidence.length"
        class="command-support-failed-device__diagnosis-list"
      >
        <span>{{ $t('custom.commandCenter.supportBundleDiagnosisEvidence') }}</span>
        <NTag
          v-for="evidence in deviceEvidence.diagnosticSummary.evidence"
          :key="evidence"
          size="small"
          :bordered="false"
        >
          {{ evidence }}
        </NTag>
      </div>
      <div
        v-if="deviceEvidence.diagnosticSummary.nextActions.length"
        class="command-support-failed-device__diagnosis-list"
      >
        <span>{{ $t('custom.commandCenter.supportBundleDiagnosisNextAction') }}</span>
        <NTag
          v-for="action in deviceEvidence.diagnosticSummary.nextActions"
          :key="action"
          size="small"
          type="warning"
          :bordered="false"
        >
          {{ action }}
        </NTag>
      </div>
    </div>

    <div v-for="row in deviceEvidence.rows" :key="row.label" class="command-support-failed-device__row">
      <span>{{ row.label }}</span>
      <strong>{{ row.value }}</strong>
    </div>

    <div
      v-if="deviceEvidence.readyCheckUrl || deviceEvidence.jobDetailUrl || deviceEvidence.deviceId"
      class="command-support-failed-device__action"
    >
      <NButton
        v-if="deviceEvidence.readyCheckUrl"
        size="tiny"
        secondary
        type="primary"
        tag="a"
        :href="deviceEvidence.readyCheckUrl"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ $t('custom.commandCenter.openSupportReadyCheck') }}
      </NButton>
      <NButton
        v-if="deviceEvidence.jobDetailUrl"
        size="tiny"
        secondary
        tag="a"
        :href="deviceEvidence.jobDetailUrl"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ $t('custom.commandCenter.openSupportJobDetail') }}
      </NButton>
      <NButton
        v-if="deviceEvidence.deviceId"
        size="tiny"
        secondary
        :type="deviceEvidence.readyCheckUrl ? 'default' : 'primary'"
        @click="emit('openDeviceDiagnosis', deviceEvidence.deviceId)"
      >
        {{ $t('custom.commandCenter.openDeviceDiagnosis') }}
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.command-support-failed-device {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
  width: 100%;
  padding: 10px;
  border: 1px solid #e0f2fe;
  border-radius: 8px;
  background: #ffffff;
}

.command-support-failed-device__diagnosis {
  display: grid;
  grid-column: 1 / -1;
  gap: 8px;
  padding: 10px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background: linear-gradient(135deg, #fff7ed 0%, #ffffff 100%);
}

.command-support-failed-device__diagnosis-head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.command-support-failed-device__diagnosis-head strong {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #7c2d12;
  font-size: 13px;
}

.command-support-failed-device__diagnosis-code {
  padding: 0;
  border: 0;
  background: transparent;
  color: #9a3412;
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
}

.command-support-failed-device__diagnosis-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.command-support-failed-device__diagnosis-list > span {
  padding: 0;
  border: 0;
  background: transparent;
  color: #64748b;
  font-size: 12px;
}

.command-support-failed-device__row {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.command-support-failed-device__row span {
  padding: 0;
  border: 0;
  background: transparent;
  color: #64748b;
  font-size: 12px;
}

.command-support-failed-device__row strong {
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 13px;
}

.command-support-failed-device__action {
  display: flex;
  flex-wrap: wrap;
  grid-column: 1 / -1;
  gap: 6px;
  align-items: flex-end;
  justify-content: flex-start;
}

@media (max-width: 900px) {
  .command-support-failed-device {
    grid-template-columns: 1fr;
  }
}
</style>
