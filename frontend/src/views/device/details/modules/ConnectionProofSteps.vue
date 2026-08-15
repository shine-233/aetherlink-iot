<script setup lang="ts">
import { NButton } from 'naive-ui'
import { $t } from '@/locales'
import type { DeviceAccessGuideQuickstartStep } from './device-access-guide-state'

defineProps<{
  steps: DeviceAccessGuideQuickstartStep[]
  copyDisabled?: boolean
  debugEnabled?: boolean
  debugEvidence?: string
  evidenceLoading?: boolean
  readyState?: string
  telemetryState?: string
}>()

const emit = defineEmits<{
  copy: [text: unknown]
  refreshEvidence: []
  openReadyCheck: []
}>()
</script>

<template>
  <div class="connection-proof" data-testid="device-access-guide-quickstart-steps">
    <div class="connection-proof-steps">
      <div
        v-for="(step, index) in steps"
        :key="step.titleKey"
        class="connection-proof-step"
        :data-testid="`device-access-guide-quickstart-step-${index + 1}`"
      >
        <div class="connection-proof-step__index">{{ index + 1 }}</div>
        <div class="connection-proof-step__body">
          <strong>{{ $t(step.titleKey) }}</strong>
          <span>{{ $t(step.descriptionKey) }}</span>
        </div>
        <NButton
          v-if="step.copyText"
          size="small"
          secondary
          :disabled="copyDisabled"
          @click="emit('copy', step.copyText)"
        >
          {{ $t(step.copyLabelKey || 'generate.copy') }}
        </NButton>
        <NButton
          v-else-if="step.actionKey"
          size="small"
          secondary
          type="success"
          data-testid="device-access-guide-run-ready-check"
          @click="emit('openReadyCheck')"
        >
          {{ $t(step.actionKey) }}
        </NButton>
      </div>
    </div>

    <div class="connection-proof-evidence">
      <div class="connection-proof-evidence__item">
        <span>{{ $t('custom.device_details.accessGuideDiagnosticDebug') }}</span>
        <strong>
          {{
            debugEnabled
              ? $t('custom.device_details.accessGuideDiagnosticDebugOn')
              : $t('custom.device_details.accessGuideDiagnosticDebugOff')
          }}
        </strong>
      </div>
      <div class="connection-proof-evidence__item">
        <span>{{ $t('custom.device_details.accessGuideDebugEvidence') }}</span>
        <strong>{{ debugEvidence || '--' }}</strong>
      </div>
      <div class="connection-proof-evidence__item">
        <span>{{ $t('custom.device_details.accessGuideLatestTelemetry') }}</span>
        <strong>{{ telemetryState || '--' }}</strong>
      </div>
      <div class="connection-proof-evidence__item">
        <span>{{ $t('custom.device_details.accessGuideReadyCheck') }}</span>
        <strong>{{ readyState || '--' }}</strong>
      </div>
      <div class="connection-proof-evidence__actions">
        <NButton size="small" secondary :loading="evidenceLoading" @click="emit('refreshEvidence')">
          {{ $t('custom.device_details.accessGuideDebugRefresh') }}
        </NButton>
        <NButton
          size="small"
          secondary
          type="success"
          data-testid="device-access-guide-run-ready-check"
          @click="emit('openReadyCheck')"
        >
          {{ $t('custom.device_details.accessGuideNextStepRunReadyCheck') }}
        </NButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.connection-proof {
  margin-bottom: 18px;
}

.connection-proof-steps {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 10px;
  margin-bottom: 10px;
}

.connection-proof-step {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.connection-proof-step__index {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 999px;
  background: #2563eb;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.connection-proof-step__body {
  min-width: 0;
}

.connection-proof-step__body strong,
.connection-proof-step__body span {
  display: block;
}

.connection-proof-step__body span {
  margin-top: 4px;
  color: #555;
  font-size: 12px;
  line-height: 1.4;
}

.connection-proof-evidence {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-left: 4px solid #2563eb;
  border-radius: 10px;
  background: linear-gradient(135deg, #f8fbff 0%, #ffffff 100%);
}

.connection-proof-evidence__item {
  min-width: 0;
}

.connection-proof-evidence__item span,
.connection-proof-evidence__item strong {
  display: block;
}

.connection-proof-evidence__item span {
  margin-bottom: 4px;
  color: #64748b;
  font-size: 12px;
}

.connection-proof-evidence__item strong {
  color: #111827;
  overflow-wrap: anywhere;
}

.connection-proof-evidence__actions {
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  gap: 8px;
}

@media (max-width: 720px) {
  .connection-proof-step {
    grid-template-columns: 28px minmax(0, 1fr);
  }

  .connection-proof-step .n-button {
    grid-column: 2;
    justify-self: start;
  }

  .connection-proof-evidence__actions {
    grid-column: 1 / -1;
  }
}
</style>
