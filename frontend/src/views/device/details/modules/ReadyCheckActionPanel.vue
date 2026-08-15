<script setup lang="ts">
type ReadyCheckActionStatus = 'ready' | 'attention' | 'next'

type ReadyCheckPrimaryAction = {
  status: ReadyCheckActionStatus
  titleKey: string
  actionKey: string
}

type ReadyCheckCommandDraft = {
  identify: string
  label: string
}

type ReadyCheckStep = {
  key: string
  status: ReadyCheckActionStatus
  titleKey: string
  descKey: string
  actionKey: string
}

const props = defineProps<{
  primaryAction: ReadyCheckPrimaryAction
  primaryActionSummary: string
  recommendedCommandLoading: boolean
  recommendedCommandDraft: ReadyCheckCommandDraft | null
  showFirstDeviceReadyHandoff: boolean
  steps: ReadyCheckStep[]
}>()

const emit = defineEmits<{
  runPrimaryAction: []
  openCommandCenter: []
  openFirstDeviceHomeProof: []
  openFirstDeviceAutomation: []
  openFirstDeviceDashboard: []
  runStep: [key: string]
}>()

const statusLabelKey = (status: ReadyCheckActionStatus) => {
  if (status === 'ready') return 'custom.device_details.readyCheckStatusReady'
  if (status === 'attention') return 'custom.device_details.readyCheckStatusAttention'
  return 'custom.device_details.readyCheckStatusNext'
}

const statusTagType = (status: ReadyCheckActionStatus) => {
  if (status === 'ready') return 'success'
  if (status === 'attention') return 'warning'
  return 'info'
}

const handleStep = (key: string) => {
  emit('runStep', key)
}
</script>

<template>
  <NAlert
    :type="statusTagType(primaryAction.status)"
    :show-icon="false"
    class="ready-check-primary-action"
    data-testid="device-ready-check-primary-action"
  >
    <div class="ready-check-primary-action__copy">
      <span>
        {{
          $t(
            primaryAction.status === 'attention'
              ? 'custom.device_details.readyCheckPrimaryBlocker'
              : 'custom.device_details.readyCheckPrimaryNext'
          )
        }}
      </span>
      <strong>{{ $t(primaryAction.titleKey) }}</strong>
      <p>{{ primaryActionSummary }}</p>
    </div>
    <NButton type="primary" secondary @click="emit('runPrimaryAction')">
      {{ $t(primaryAction.actionKey) }}
    </NButton>
  </NAlert>

  <NAlert
    type="info"
    :show-icon="false"
    class="ready-check-command-draft"
    data-testid="device-ready-check-command-draft"
  >
    <div class="ready-check-command-draft__copy">
      <strong>{{ $t('custom.device_details.readyCheckCommandDraftTitle') }}</strong>
      <span v-if="recommendedCommandLoading">
        {{ $t('custom.device_details.readyCheckCommandDraftLoading') }}
      </span>
      <span v-else-if="recommendedCommandDraft">
        {{ $t('custom.device_details.readyCheckCommandDraftDesc', {
          identify: recommendedCommandDraft.identify,
          name: recommendedCommandDraft.label
        }) }}
      </span>
      <span v-else>
        {{ $t('custom.device_details.readyCheckCommandDraftEmpty') }}
      </span>
    </div>
    <NButton size="small" type="primary" secondary :loading="recommendedCommandLoading" @click="emit('openCommandCenter')">
      {{
        $t(
          recommendedCommandDraft
            ? 'custom.device_details.readyCheckOpenCommandCenterWithDraft'
            : 'custom.device_details.readyCheckOpenCommandCenter'
        )
      }}
    </NButton>
  </NAlert>

  <NAlert
    v-if="showFirstDeviceReadyHandoff"
    type="success"
    :show-icon="false"
    class="ready-check-first-device-handoff"
  >
    <div class="ready-check-first-device-handoff__copy">
      <strong>{{ $t('custom.device_details.readyCheckFirstDeviceNextTitle') }}</strong>
      <span>{{ $t('custom.device_details.readyCheckFirstDeviceNextDesc') }}</span>
    </div>
    <div class="ready-check-first-device-handoff__actions">
      <NButton size="small" type="primary" @click="emit('openFirstDeviceHomeProof')">
        {{ $t('custom.device_details.readyCheckBackHomeChart') }}
      </NButton>
      <NButton size="small" secondary type="primary" @click="emit('openFirstDeviceAutomation')">
        {{ $t('custom.automation.createFirstTelemetryRule') }}
      </NButton>
      <NButton size="small" secondary @click="emit('openFirstDeviceDashboard')">
        {{ $t('rdi.thingsvis.createHomepageDashboard') }}
      </NButton>
    </div>
  </NAlert>

  <div class="ready-check-grid" data-testid="device-ready-check-next-steps">
    <section v-for="step in steps" :key="step.key" class="ready-check-card">
      <div class="ready-check-card__head">
        <NTag :type="statusTagType(step.status)" size="small">
          {{ $t(statusLabelKey(step.status)) }}
        </NTag>
        <h3>{{ $t(step.titleKey) }}</h3>
      </div>
      <p>{{ $t(step.descKey) }}</p>
      <NButton size="small" secondary type="primary" @click="handleStep(step.key)">
        {{ $t(step.actionKey) }}
      </NButton>
    </section>
  </div>
</template>

<style scoped>
.ready-check-command-draft {
  border-color: #93c5fd;
  background: #eff6ff;
}

.ready-check-command-draft :deep(.n-alert-body__content) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.ready-check-first-device-handoff :deep(.n-alert-body__content) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.ready-check-primary-action__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-command-draft__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-first-device-handoff__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.ready-check-primary-action__copy span {
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.ready-check-primary-action__copy strong {
  color: #0f172a;
  font-size: 15px;
}

.ready-check-primary-action__copy p {
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-command-draft__copy strong {
  color: #1d4ed8;
  font-size: 15px;
}

.ready-check-command-draft__copy span {
  color: #1e3a8a;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-first-device-handoff__copy strong {
  color: #166534;
  font-size: 15px;
}

.ready-check-first-device-handoff__copy span {
  color: #166534;
  font-size: 13px;
  line-height: 1.5;
}

.ready-check-first-device-handoff__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.ready-check-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.ready-check-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  padding: 14px;
}

.ready-check-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ready-check-card h3 {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
}

.ready-check-card p {
  margin: 0;
}

@media (max-width: 900px) {
  .ready-check-grid {
    grid-template-columns: 1fr;
  }

  .ready-check-primary-action :deep(.n-alert-body__content),
  .ready-check-command-draft :deep(.n-alert-body__content) {
    align-items: flex-start;
    flex-direction: column;
  }

  .ready-check-first-device-handoff :deep(.n-alert-body__content) {
    align-items: flex-start;
    flex-direction: column;
  }

  .ready-check-first-device-handoff__actions {
    justify-content: flex-start;
  }
}
</style>
