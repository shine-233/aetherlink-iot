<script setup lang="ts">
import { computed, ref } from 'vue'
import type { CommandJobResultActions, CommandJobResultViewModel } from './commandCenterJobResultViewModel'
import CommandJobDeviceProgressTracks from './CommandJobDeviceProgressTracks.vue'
import CommandJobOperatorActions from './CommandJobOperatorActions.vue'
import CommandJobOutcomeGroups from './CommandJobOutcomeGroups.vue'
import CommandJobResultEvidenceSection from './CommandJobResultEvidenceSection.vue'
import CommandJobResultHandoffCard from './CommandJobResultHandoffCard.vue'
import CommandJobResultHealthCard from './CommandJobResultHealthCard.vue'
import CommandJobResultProgressCard from './CommandJobResultProgressCard.vue'
import CommandJobSupportExecutionCard from './CommandJobSupportExecutionCard.vue'
import CommandJobSupportGovernanceCard from './CommandJobSupportGovernanceCard.vue'

const props = defineProps<{
  jobResult: CommandJobResultViewModel
  jobActions: CommandJobResultActions
}>()

const state = computed(() => props.jobResult)
const evidenceSectionRef = ref<CommandJobResultEvidenceSectionHandle | null>(null)

interface CommandJobResultEvidenceSectionHandle {
  scrollRowsTableIntoView: () => Promise<void>
}

const operatorActions = computed<CommandJobResultActions>(() => ({
  ...props.jobActions,
  reviewCommandJobRows: async statusFilter => {
    await props.jobActions.reviewCommandJobRows(statusFilter)
    await evidenceSectionRef.value?.scrollRowsTableIntoView()
  }
}))

const actions = computed(() => props.jobActions)

</script>

<template>
  <template v-if="state.submitResult">
    <div class="command-job-result-flow">
      <section class="command-job-result-section command-job-result-section--overview">
        <div class="command-job-result-section__head">
          <strong>{{ $t('custom.commandCenter.submitOverviewTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.submitOverviewDesc') }}</span>
        </div>
        <NAlert type="success" :show-icon="false">
          {{
            $t('custom.commandCenter.submitSummary')
              .replace('{jobId}', state.submitResult.job_id)
              .replace('{submitted}', String(state.submitResult.submitted_count))
              .replace('{failed}', String(state.submitResult.failed_count))
          }}
        </NAlert>
        <NAlert type="info" :show-icon="false">
          {{ state.submitCapabilitySummary }}
        </NAlert>
        <NAlert :type="state.submitEvidenceAlertType" :show-icon="false">
          {{ state.submitEvidenceSummary }}
        </NAlert>
        <CommandJobSupportGovernanceCard
          v-if="state.jobGovernanceSummaryCard"
          :governance-summary="state.jobGovernanceSummaryCard"
        />
        <CommandJobResultHandoffCard
          v-if="state.jobHandoffSummary"
          :handoff-summary="state.jobHandoffSummary"
          @copy-handoff-summary="actions.copyCommandJobHandoffSummary"
          @copy-closeout-packet="actions.copyCommandJobCloseoutPacket"
        />
        <CommandJobResultProgressCard
          :status-label="state.jobStatusLabel"
          :progress-percent="state.jobProgressPercent"
          :progress-summary="state.jobProgressSummary"
        />
        <CommandJobResultHealthCard v-if="state.jobProgressHealthCard" :health-card="state.jobProgressHealthCard" />
        <CommandJobSupportExecutionCard
          v-if="state.jobExecutionSummaryCard"
          :execution-summary="state.jobExecutionSummaryCard"
        />
      </section>

      <section class="command-job-result-section command-job-result-section--next">
        <div class="command-job-result-section__head">
          <strong>{{ $t('custom.commandCenter.submitNextActionsTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.submitNextActionsDesc') }}</span>
        </div>
        <CommandJobOperatorActions :job-result="state" :job-actions="operatorActions" />
      </section>

      <section
        v-if="state.jobDeviceProgressTracks.length"
        class="command-job-result-section command-job-result-section--device-progress"
      >
        <div class="command-job-result-section__head">
          <strong>{{ $t('custom.commandCenter.deviceProgressTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.deviceProgressDesc') }}</span>
        </div>
        <CommandJobDeviceProgressTracks
          :tracks="state.jobDeviceProgressTracks"
          @open-device-diagnosis="actions.openCommandJobDeviceDiagnosis"
        />
      </section>

      <section v-if="state.jobOutcomeGroups.length" class="command-job-result-section command-job-result-section--outcomes">
        <div class="command-job-result-section__head">
          <strong>{{ $t('custom.commandCenter.outcomeTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.outcomeDesc') }}</span>
        </div>
        <CommandJobOutcomeGroups
          :groups="state.jobOutcomeGroups"
          @open-device-diagnosis="actions.openCommandJobDeviceDiagnosis"
        />
      </section>

      <CommandJobResultEvidenceSection ref="evidenceSectionRef" :job-result="state" :job-actions="actions" />
    </div>
  </template>
</template>

<style scoped>
.command-job-result-flow {
  display: grid;
  gap: 10px;
}

.command-job-result-section {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.command-job-result-section--overview {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.command-job-result-section--next {
  border-color: #fed7aa;
  background: #fff7ed;
}

.command-job-result-section--device-progress {
  border-color: #bae6fd;
  background: #f0f9ff;
}

.command-job-result-section--outcomes {
  border-color: #bbf7d0;
  background: #f0fdf4;
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

</style>
