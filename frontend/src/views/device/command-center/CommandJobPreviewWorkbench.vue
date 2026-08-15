<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { $t } from '@/locales'
import type { CommandCenterSavedCommandTemplate } from './useCommandCenterCommandTemplates'
import CommandPayloadAssistant from './CommandPayloadAssistant.vue'
import CommandTemplateLibrary from './CommandTemplateLibrary.vue'
const CommandJobPreviewAnalysisPanel = defineAsyncComponent(() => import('./CommandJobPreviewAnalysisPanel.vue'))

interface Props {
  activeJobWarnings: string[]
  canPreviewCommandJobNow: boolean
  canSubmitCommandJobNow: boolean
  commandIdentify: string
  commandJobEligibilityImpactPreview: any
  commandJobError: string
  commandJobPreviewActionPlan: any
  commandJobReadiness: any
  commandJobReadinessTagType: 'default' | 'error' | 'info' | 'primary' | 'success' | 'warning'
  commandSubmitDisabledHint: string
  commandValue: string
  commandTemplateName: string
  filterExecutionCapSummary: string
  filteredFleetEligibilityPreview: any
  hasCommandJobScope: boolean
  isDeviceFilterScope: boolean
  jobRequirements: string[]
  maxDevices: number | null
  previewColumns: any[]
  previewExplanationRows: any[]
  previewLoading: boolean
  previewResult: any
  previewTokenShort: string
  routeDecisionSummary: any
  savedCommandTemplates: CommandCenterSavedCommandTemplate[]
  scopeSafetyDescription: string
  scopeSafetyMeta: string
  scopeSafetyTag: string
  scopeSafetyTagType: 'default' | 'error' | 'info' | 'success' | 'warning'
  showPreviewRecoveryAction: boolean
  submitLoading: boolean
  scheduledAt: number | null
  timeoutSeconds: number | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:commandIdentify': [value: string]
  'update:commandValue': [value: string]
  'update:commandTemplateName': [value: string]
  'update:scheduledAt': [value: number | null]
  'update:timeoutSeconds': [value: number]
  'update:maxDevices': [value: number]
  applyBuiltInCommandTemplate: [template: { identify: string; value: string; timeoutSeconds: number }]
  applySavedCommandTemplate: [template: CommandCenterSavedCommandTemplate]
  copySavedCommandTemplate: [template: CommandCenterSavedCommandTemplate]
  copySavedCommandTemplates: []
  copyEligibilityImpactSummary: []
  deleteSavedCommandTemplate: [templateId: string]
  importSavedCommandTemplates: [raw: string]
  openOtaJobs: []
  previewCommandJob: []
  saveCommandTemplate: []
  submitCommandJob: []
}>()

</script>

<template>
  <ul>
    <li v-for="item in jobRequirements" :key="item">{{ $t(item) }}</li>
  </ul>

  <CommandTemplateLibrary
    :command-identify="commandIdentify"
    :command-template-name="commandTemplateName"
    :has-command-job-scope="hasCommandJobScope"
    :is-device-filter-scope="isDeviceFilterScope"
    :saved-command-templates="savedCommandTemplates"
    @update:command-template-name="emit('update:commandTemplateName', $event)"
    @apply-built-in-command-template="emit('applyBuiltInCommandTemplate', $event)"
    @apply-saved-command-template="emit('applySavedCommandTemplate', $event)"
    @copy-saved-command-template="emit('copySavedCommandTemplate', $event)"
    @copy-saved-command-templates="emit('copySavedCommandTemplates')"
    @delete-saved-command-template="emit('deleteSavedCommandTemplate', $event)"
    @import-saved-command-templates="emit('importSavedCommandTemplates', $event)"
    @save-command-template="emit('saveCommandTemplate')"
  />

  <div class="command-job-form">
    <NInput
      :value="commandIdentify"
      :placeholder="$t('custom.commandCenter.commandIdentifier')"
      @update:value="emit('update:commandIdentify', $event)"
    />
    <NInput
      :value="commandValue"
      type="textarea"
      :rows="3"
      :placeholder="$t('custom.commandCenter.commandValue')"
      @update:value="emit('update:commandValue', $event)"
    />
    <CommandPayloadAssistant
      :command-value="commandValue"
      @update:command-value="emit('update:commandValue', $event)"
    />
    <div class="command-timeout-control">
      <span>{{ $t('custom.commandCenter.timeoutSeconds') }}</span>
      <NInputNumber
        :value="timeoutSeconds"
        :min="1"
        :max="3600"
        class="w-160px"
        @update:value="emit('update:timeoutSeconds', $event ?? 1)"
      />
    </div>
    <div class="command-schedule-control">
      <div>
        <span>{{ $t('custom.commandCenter.scheduledAt') }}</span>
        <small>{{ $t('custom.commandCenter.scheduleImmediateHint') }}</small>
      </div>
      <NDatePicker
        :value="scheduledAt"
        type="datetime"
        clearable
        class="command-schedule-picker"
        @update:value="emit('update:scheduledAt', $event)"
      />
    </div>
    <div v-if="isDeviceFilterScope" class="command-cap-control">
      <div>
        <span>{{ $t('custom.commandCenter.maxDevices') }}</span>
        <small>{{ filterExecutionCapSummary }}</small>
      </div>
      <NInputNumber
        :value="maxDevices"
        :min="1"
        :max="1000"
        class="w-160px"
        @update:value="emit('update:maxDevices', $event ?? 1)"
      />
    </div>
    <div class="command-scope-safety">
      <div>
        <strong>{{ $t('custom.commandCenter.scopeSafetyTitle') }}</strong>
        <span>{{ scopeSafetyDescription }}</span>
        <small>{{ scopeSafetyMeta }}</small>
      </div>
      <NTag :type="scopeSafetyTagType" size="small">{{ scopeSafetyTag }}</NTag>
    </div>
    <NSpace>
      <NButton
        type="primary"
        :loading="previewLoading"
        :disabled="!canPreviewCommandJobNow"
        @click="emit('previewCommandJob')"
      >
        {{ $t('custom.commandCenter.previewCommandJob') }}
      </NButton>
      <NButton
        type="success"
        :loading="submitLoading"
        :disabled="!canSubmitCommandJobNow"
        @click="emit('submitCommandJob')"
      >
        {{ $t('custom.commandCenter.submitEligibleDevices') }}
      </NButton>
      <NButton secondary :disabled="!hasCommandJobScope" @click="emit('openOtaJobs')">
        {{ $t('custom.commandCenter.openOtaJobs') }}
      </NButton>
    </NSpace>
  </div>

  <div
    v-if="hasCommandJobScope"
    class="command-readiness-card"
    :class="`command-readiness-card--${commandJobReadiness.customerRiskLevel}`"
  >
    <div class="command-readiness-card__head">
      <strong>{{ $t('custom.commandCenter.previewExplainSubmitGate') }}</strong>
      <NTag :type="commandJobReadinessTagType" size="small">
        {{
          commandJobReadiness.canSubmit
            ? $t('custom.commandCenter.previewExplainSubmitUnlocked')
            : $t('custom.commandCenter.previewExplainSubmitLocked')
        }}
      </NTag>
    </div>
    <div class="command-readiness-card__body">
      <span>
        {{
          commandJobReadiness.blockingReason ||
          $t('custom.commandCenter.previewTokenSummary').replace('{token}', previewTokenShort)
        }}
      </span>
      <strong>{{ $t('custom.commandCenter.nextAction') }}: {{ commandJobReadiness.requiredNextAction }}</strong>
    </div>
  </div>

  <NAlert v-if="commandJobError" type="error" :show-icon="false">
    {{ commandJobError }}
  </NAlert>
  <NAlert v-if="!canSubmitCommandJobNow && commandSubmitDisabledHint" type="warning" :show-icon="false">
    <div class="command-preview-recovery">
      <span>{{ commandSubmitDisabledHint || $t('custom.commandCenter.previewBeforeSubmit') }}</span>
      <NButton
        v-if="showPreviewRecoveryAction"
        size="small"
        secondary
        type="warning"
        :loading="previewLoading"
        @click="emit('previewCommandJob')"
      >
        {{ $t('custom.commandCenter.previewCommandJob') }}
      </NButton>
    </div>
  </NAlert>

  <div v-if="hasCommandJobScope" class="command-preview-explanation">
    <div class="command-preview-explanation__head">
      <strong>{{ $t('custom.commandCenter.previewExplainTitle') }}</strong>
      <span>{{ $t('custom.commandCenter.previewExplainDesc') }}</span>
    </div>
    <div class="command-preview-explanation__grid">
      <div v-for="row in previewExplanationRows" :key="row.label" class="command-preview-explanation__item">
        <span>{{ row.label }}</span>
        <strong>{{ row.value }}</strong>
      </div>
    </div>
  </div>
  <CommandJobPreviewAnalysisPanel
    v-if="
      commandJobEligibilityImpactPreview ||
      commandJobPreviewActionPlan ||
      filteredFleetEligibilityPreview ||
      activeJobWarnings.length ||
      previewResult
    "
    :active-job-warnings="activeJobWarnings"
    :command-job-eligibility-impact-preview="commandJobEligibilityImpactPreview"
    :command-job-preview-action-plan="commandJobPreviewActionPlan"
    :filtered-fleet-eligibility-preview="filteredFleetEligibilityPreview"
    :preview-columns="previewColumns"
    :preview-result="previewResult"
    :preview-token-short="previewTokenShort"
    :route-decision-summary="routeDecisionSummary"
    @copy-eligibility-impact-summary="emit('copyEligibilityImpactSummary')"
  />
</template>

<style scoped>
.command-job-form {
  display: grid;
  gap: 12px;
}

.command-timeout-control {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #475569;
  font-size: 13px;
}

.command-schedule-control {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #c7d2fe;
  border-radius: 8px;
  background: #eef2ff;
  color: #475569;
  font-size: 13px;
}

.command-schedule-control > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-schedule-control span {
  color: #312e81;
  font-weight: 600;
}

.command-schedule-control small {
  color: #64748b;
  line-height: 1.5;
}

.command-schedule-picker {
  width: min(280px, 100%);
}

.command-cap-control {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  background: #f0f9ff;
  color: #475569;
  font-size: 13px;
}

.command-cap-control > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-cap-control span {
  color: #0f172a;
  font-weight: 600;
}

.command-cap-control small {
  color: #64748b;
  line-height: 1.5;
}

.command-scope-safety {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border: 1px solid #facc15;
  border-radius: 10px;
  background: linear-gradient(135deg, #fefce8 0%, #fff7ed 100%);
}

.command-scope-safety > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-scope-safety strong {
  color: #854d0e;
  font-size: 13px;
}

.command-scope-safety span,
.command-scope-safety small {
  color: #713f12;
  font-size: 12px;
  line-height: 1.45;
}

.command-readiness-card {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid #fecdd3;
  border-radius: 8px;
  background: #fff1f2;
}

.command-readiness-card--ready {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.command-readiness-card--warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.command-readiness-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.command-readiness-card__head strong {
  color: #0f172a;
  font-size: 14px;
}

.command-readiness-card__body {
  display: grid;
  gap: 4px;
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.command-readiness-card__body strong {
  color: #0f172a;
}

.command-preview-recovery {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.command-preview-recovery span {
  min-width: 0;
  overflow-wrap: anywhere;
}

.command-preview-explanation {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #f0fdf4;
}

.command-preview-explanation__head {
  display: grid;
  gap: 4px;
}

.command-preview-explanation__head strong {
  color: #14532d;
  font-size: 14px;
}

.command-preview-explanation__head span {
  color: #166534;
  font-size: 12px;
}

.command-preview-explanation__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.command-preview-explanation__item {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #dcfce7;
  border-radius: 8px;
  background: #ffffff;
}

.command-preview-explanation__item span {
  color: #64748b;
  font-size: 12px;
}

.command-preview-explanation__item strong {
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 13px;
}

@media (max-width: 900px) {
  .command-preview-recovery,
  .command-scope-safety,
  .command-schedule-control,
  .command-cap-control {
    flex-direction: column;
  }

  .command-preview-explanation__grid {
    grid-template-columns: 1fr;
  }
}
</style>
