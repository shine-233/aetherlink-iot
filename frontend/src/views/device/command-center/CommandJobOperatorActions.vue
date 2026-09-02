<script setup lang="ts">
import { computed } from 'vue'
import type { CommandJobResultActions, CommandJobResultViewModel } from './commandCenterJobResultViewModel'

const props = defineProps<{
  jobResult: CommandJobResultViewModel
  jobActions: CommandJobResultActions
}>()

const state = computed(() => props.jobResult)
const actions = computed(() => props.jobActions)

const runOperatorPrimaryAction = () => {
  const action = state.value.jobOperatorNextAction?.primaryAction
  if (action === 'refresh') actions.value.refreshCommandJob()
  else if (action === 'retry') actions.value.retryCommandJob()
  else if (action === 'copy-retryable') actions.value.copyRetryableDeviceIds()
  else if (action === 'preview-support') actions.value.loadCommandJobSupportBundle()
  else if (action === 'copy-link') actions.value.copyCommandJobLink()
}

const reviewOperatorRows = () => {
  const statusFilter = state.value.jobOperatorNextAction?.reviewRowsStatusFilter
  if (statusFilter) actions.value.reviewCommandJobRows(statusFilter)
}
</script>

<template>
  <NAlert
    v-if="state.jobOperatorNextAction"
    :type="state.jobOperatorNextAction.type"
    :show-icon="false"
    class="command-job-operator-action"
  >
    <div class="command-job-operator-action__body">
      <div class="command-job-operator-action__copy">
        <strong>{{ state.jobOperatorNextAction.title }}</strong>
        <span>{{ state.jobOperatorNextAction.description }}</span>
        <em>{{ state.jobOperatorNextAction.evidence }}</em>
      </div>
      <NButton
        v-if="state.jobOperatorNextAction.primaryAction !== 'none'"
        size="small"
        secondary
        type="primary"
        :loading="state.jobActionLoading || state.supportBundleLoading"
        @click="runOperatorPrimaryAction"
      >
        {{ state.jobOperatorNextAction.primaryActionLabel }}
      </NButton>
      <NButton
        v-if="state.jobOperatorNextAction.reviewRowsStatusFilter"
        size="small"
        secondary
        :loading="state.commandJobRowsLoading"
        @click="reviewOperatorRows"
      >
        {{ $t('custom.commandCenter.reviewAffectedRows') }}
      </NButton>
    </div>
  </NAlert>

  <div class="command-job-troubleshooting">
    <div class="command-job-troubleshooting__head">
      <strong>{{ $t('custom.commandCenter.troubleshootingTitle') }}</strong>
      <span>{{ $t('custom.commandCenter.troubleshootingDesc') }}</span>
    </div>
    <div class="command-job-troubleshooting__grid">
      <NAlert
        v-for="row in state.jobTroubleshootingRows"
        :key="row.key"
        :type="row.type"
        :show-icon="false"
        class="command-job-troubleshooting__item"
      >
        <strong>{{ row.label }}</strong>
        <span>{{ row.value }}</span>
        <NButton
          v-if="row.reviewRowsStatusFilter"
          size="tiny"
          secondary
          :loading="state.commandJobRowsLoading"
          @click="actions.reviewCommandJobRows(row.reviewRowsStatusFilter)"
        >
          {{ $t('custom.commandCenter.reviewAffectedRows') }}
        </NButton>
      </NAlert>
    </div>
  </div>

  <div class="command-job-next-steps">
    <strong>{{ $t('custom.commandCenter.postSubmitTitle') }}</strong>
    <ul>
      <li v-for="item in state.postSubmitChecklist" :key="item">{{ $t(item) }}</li>
    </ul>
  </div>

  <div v-if="state.jobActionConsequenceRows.length" class="command-job-action-impact">
    <div class="command-job-action-impact__head">
      <strong>{{ $t('custom.commandCenter.actionConsequenceTitle') }}</strong>
      <span>{{ $t('custom.commandCenter.actionConsequenceDesc') }}</span>
    </div>
    <div class="command-job-action-impact__grid">
      <NAlert
        v-for="row in state.jobActionConsequenceRows"
        :key="row.key"
        :type="row.type"
        :show-icon="false"
        class="command-job-action-impact__item"
      >
        <strong>{{ row.label }}</strong>
        <span>{{ row.value }}</span>
        <NButton
          v-if="row.reviewRowsStatusFilter"
          size="tiny"
          secondary
          :loading="state.commandJobRowsLoading"
          @click="actions.reviewCommandJobRows(row.reviewRowsStatusFilter)"
        >
          {{ $t('custom.commandCenter.reviewAffectedRows') }}
        </NButton>
      </NAlert>
    </div>
  </div>

  <NSpace>
    <NButton size="small" secondary :loading="state.jobActionLoading" @click="actions.refreshCommandJob">
      {{ $t('custom.commandCenter.refreshJob') }}
    </NButton>
    <NTag v-if="state.jobAutoRefreshActive && !state.jobAutoRefreshDeferred" size="small" type="success">
      {{ $t('custom.commandCenter.autoRefreshActive') }}
    </NTag>
    <NTag v-if="state.jobAutoRefreshDeferred" size="small" type="warning">
      {{ $t('custom.commandCenter.autoRefreshDeferred') }}
    </NTag>
    <NButton size="small" secondary @click="actions.copyCommandJobLink">
      {{ $t('custom.commandCenter.copyJobLink') }}
    </NButton>
    <NButton size="small" secondary @click="actions.copyCommandJobHandoffSummary">
      {{ $t('custom.commandCenter.copyHandoffSummary') }}
    </NButton>
    <NButton size="small" secondary :loading="state.supportBundleLoading" @click="actions.loadCommandJobSupportBundle">
      {{ $t('custom.commandCenter.previewSupportBundle') }}
    </NButton>
    <NButton size="small" secondary :loading="state.supportBundleLoading" @click="actions.copyCommandJobSupportBundle">
      {{ $t('custom.commandCenter.copySupportBundle') }}
    </NButton>
    <NButton
      size="small"
      secondary
      :loading="state.supportBundleLoading"
      @click="actions.downloadCommandJobSupportBundle"
    >
      {{ $t('custom.commandCenter.downloadSupportBundle') }}
    </NButton>
    <NButton
      size="small"
      secondary
      type="warning"
      :loading="state.jobActionLoading"
      :disabled="!state.submitResult?.can_cancel"
      @click="actions.cancelCommandJob"
    >
      {{ $t('custom.commandCenter.cancelJob') }}
    </NButton>
    <NButton
      size="small"
      secondary
      type="primary"
      :loading="state.jobActionLoading"
      :disabled="!state.canRetryCurrentCommandJob"
      @click="actions.retryCommandJob"
    >
      {{ $t('custom.commandCenter.retryFailedJob') }}
    </NButton>
    <NButton
      size="small"
      secondary
      :disabled="state.retryableFailedRows.length === 0"
      @click="actions.copyRetryableDeviceIds"
    >
      {{ $t('custom.commandCenter.copyFailedDeviceIds') }}
    </NButton>
  </NSpace>
</template>

<style scoped>
.command-job-operator-action :deep(.n-alert-body__content) {
  min-width: 0;
}

.command-job-operator-action__body {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  min-width: 0;
}

.command-job-operator-action__copy {
  flex: 1 1 320px;
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-job-operator-action__copy strong,
.command-job-operator-action__copy span,
.command-job-operator-action__copy em {
  overflow-wrap: anywhere;
}

.command-job-operator-action__copy strong {
  color: #0f172a;
  font-size: 14px;
}

.command-job-operator-action__copy span,
.command-job-operator-action__copy em {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.command-job-operator-action__copy em {
  font-style: normal;
}

.command-job-troubleshooting {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #fecaca;
  border-radius: 8px;
  background: #fff1f2;
}

.command-job-troubleshooting__head {
  display: grid;
  gap: 4px;
}

.command-job-troubleshooting__head strong {
  color: #991b1b;
  font-size: 14px;
}

.command-job-troubleshooting__head span {
  color: #7f1d1d;
  font-size: 12px;
}

.command-job-troubleshooting__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-job-troubleshooting__item :deep(.n-alert-body__content) {
  display: grid;
  gap: 4px;
}

.command-job-troubleshooting__item strong {
  overflow-wrap: anywhere;
  font-size: 13px;
}

.command-job-troubleshooting__item span {
  color: #475569;
  font-size: 12px;
}

.command-job-next-steps {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background: #fff7ed;
}

.command-job-next-steps strong {
  color: #9a3412;
  font-size: 14px;
}

.command-job-next-steps ul {
  margin: 0;
  padding-left: 18px;
}

.command-job-action-impact {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  background: #f0f9ff;
}

.command-job-action-impact__head {
  display: grid;
  gap: 4px;
}

.command-job-action-impact__head strong {
  color: #0c4a6e;
  font-size: 14px;
}

.command-job-action-impact__head span {
  color: #075985;
  font-size: 12px;
  line-height: 1.5;
}

.command-job-action-impact__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.command-job-action-impact__item :deep(.n-alert-body__content) {
  display: grid;
  gap: 4px;
}

.command-job-action-impact__item strong,
.command-job-action-impact__item span {
  overflow-wrap: anywhere;
}

.command-job-action-impact__item strong {
  font-size: 13px;
}

.command-job-action-impact__item span {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .command-job-operator-action__body {
    align-items: flex-start;
    flex-direction: column;
  }

  .command-job-troubleshooting__grid {
    grid-template-columns: 1fr;
  }

  .command-job-action-impact__grid {
    grid-template-columns: 1fr;
  }
}
</style>
