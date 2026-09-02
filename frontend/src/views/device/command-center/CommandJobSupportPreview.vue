<script setup lang="ts">
import CommandJobSupportExecutionCard from './CommandJobSupportExecutionCard.vue'
import CommandJobSupportFailedDeviceCard from './CommandJobSupportFailedDeviceCard.vue'
import CommandJobSupportGovernanceCard from './CommandJobSupportGovernanceCard.vue'
import type { CommandJobSupportBundlePreview } from './commandCenterJobView'

defineProps<{
  supportBundlePreview: CommandJobSupportBundlePreview
}>()

const emit = defineEmits<{
  openDeviceDiagnosis: [deviceId: string]
}>()
</script>

<template>
  <div class="command-support-preview">
    <div class="command-support-preview__head">
      <strong>{{ $t('custom.commandCenter.supportBundlePreviewTitle') }}</strong>
      <span>{{ $t('custom.commandCenter.supportBundlePreviewDesc') }}</span>
    </div>
    <NDescriptions bordered :column="3" size="small">
      <NDescriptionsItem v-for="row in supportBundlePreview.summaryRows" :key="row.label" :label="row.label">
        {{ row.value }}
      </NDescriptionsItem>
    </NDescriptions>
    <div v-if="supportBundlePreview.nextActions.length" class="command-support-preview__section">
      <strong>{{ $t('custom.commandCenter.supportBundleNextActions') }}</strong>
      <NTag v-for="action in supportBundlePreview.nextActions" :key="action" size="small" type="info">
        {{ action }}
      </NTag>
    </div>
    <CommandJobSupportGovernanceCard
      v-if="supportBundlePreview.governanceSummary"
      :governance-summary="supportBundlePreview.governanceSummary"
    />
    <CommandJobSupportExecutionCard
      v-if="supportBundlePreview.executionSummary"
      :execution-summary="supportBundlePreview.executionSummary"
    />
    <div v-if="supportBundlePreview.failedDeviceEvidence.length" class="command-support-preview__section">
      <strong>{{ $t('custom.commandCenter.supportBundleFailedDevicesList') }}</strong>
      <CommandJobSupportFailedDeviceCard
        v-for="deviceEvidence in supportBundlePreview.failedDeviceEvidence"
        :key="deviceEvidence.key"
        :device-evidence="deviceEvidence"
        @open-device-diagnosis="emit('openDeviceDiagnosis', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.command-support-preview {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  background: #f0f9ff;
}

.command-support-preview__head {
  display: grid;
  gap: 4px;
}

.command-support-preview__head strong {
  color: #075985;
  font-size: 14px;
}

.command-support-preview__head span {
  color: #0369a1;
  font-size: 12px;
}

.command-support-preview__section {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.command-support-preview__section strong {
  width: 100%;
  color: #0f172a;
  font-size: 13px;
}

.command-support-preview__section span {
  padding: 4px 8px;
  border: 1px solid #e0f2fe;
  border-radius: 6px;
  background: #ffffff;
  color: #334155;
  font-size: 12px;
}
</style>
