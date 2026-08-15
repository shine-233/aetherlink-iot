<script setup lang="ts">
import { $t } from '@/locales'

defineProps<{
  firstDevice: any
  firstDeviceAccessGuide: any
  firstDeviceSimulation: any
  firstDeviceOnboardingGuard: any
  operationChecklist: any[]
}>()

const emit = defineEmits<{
  copyConnectionSummary: []
}>()
</script>

<template>
  <div class="flex items-center justify-between gap-8px">
    <div class="flex min-w-0 flex-wrap items-center gap-8px">
      <div class="font-600">{{ $t('custom.home.firstDevice.connection.title') }}</div>
      <n-button
        size="tiny"
        secondary
        :disabled="!firstDevice"
        @click="emit('copyConnectionSummary')"
      >
        {{ $t('custom.home.firstDevice.connection.copySummary') }}
      </n-button>
    </div>
    <n-tag
      size="small"
      round
      :bordered="false"
      :type="firstDeviceOnboardingGuard.canCopyCommand ? 'success' : 'warning'"
    >
      {{
        firstDeviceOnboardingGuard.canCopyCommand
          ? $t('custom.home.firstDevice.connection.copyable')
          : $t('custom.home.firstDevice.connection.pending')
      }}
    </n-tag>
  </div>
  <div class="mt-8px grid gap-6px">
    <div class="first-device-access-row">
      <span>{{ $t('custom.home.firstDevice.connection.protocol') }}</span>
      <strong>{{ firstDeviceAccessGuide?.protocol || 'MQTT' }}</strong>
    </div>
    <div class="first-device-access-row">
      <span>{{ $t('custom.home.firstDevice.connection.endpoint') }}</span>
      <strong>
        {{
          firstDeviceAccessGuide?.endpoint ||
          `${firstDeviceSimulation?.server}:${firstDeviceSimulation?.port}`
        }}
      </strong>
    </div>
    <div class="first-device-access-row">
      <span>{{ $t('custom.home.firstDevice.connection.reportEntry') }}</span>
      <strong>
        {{
          firstDeviceAccessGuide?.endpointKind === 'http'
            ? firstDeviceAccessGuide.endpoint
            : firstDeviceAccessGuide?.reportTopic || firstDeviceSimulation?.topic || 'devices/telemetry'
        }}
      </strong>
    </div>
    <div class="first-device-access-row">
      <span>{{ $t('custom.home.firstDevice.connection.controlEntry') }}</span>
      <strong>
        {{
          firstDeviceAccessGuide?.controlTopic || $t('custom.home.firstDevice.connection.openReadyCheckToView')
        }}
      </strong>
    </div>
  </div>
  <div class="first-device-protocol-helper">
    <div class="min-w-0">
      <strong>{{ $t('custom.home.firstDevice.connection.helperTitle') }}</strong>
      <small>{{ $t('custom.home.firstDevice.connection.helperDesc') }}</small>
    </div>
    <div class="first-device-protocol-helper__grid">
      <div>
        <span>{{ $t('custom.home.firstDevice.connection.realDevice') }}</span>
        <small>
          {{
            firstDeviceAccessGuide?.endpointKind === 'http'
              ? 'HTTP Endpoint + Token/Password + JSON payload'
              : $t('custom.home.firstDevice.connection.mqttHint')
          }}
        </small>
      </div>
      <div>
        <span>{{ $t('custom.home.firstDevice.connection.tryItFirst') }}</span>
        <small>{{ $t('custom.home.firstDevice.connection.tryItFirstDesc') }}</small>
      </div>
    </div>
  </div>

  <div class="mt-10px grid gap-6px">
    <div
      v-for="item in operationChecklist"
      :key="item.key"
      class="first-device-operation-check"
    >
      <div class="min-w-0">
        <strong>{{ item.label }}</strong>
        <small>{{ item.detail }}</small>
      </div>
      <n-tag size="small" round :bordered="false" :type="item.ok ? 'success' : 'warning'">
        {{
          item.ok
            ? $t('custom.home.firstDevice.connection.ready')
            : $t('custom.home.firstDevice.connection.processing')
        }}
      </n-tag>
    </div>
  </div>
</template>

<style scoped>
.first-device-protocol-helper {
  display: grid;
  gap: 8px;
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #eff6ff;
}

.first-device-protocol-helper strong,
.first-device-protocol-helper span,
.first-device-protocol-helper small {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-protocol-helper strong {
  color: #0f172a;
  font-size: 12px;
}

.first-device-protocol-helper span {
  color: #1d4ed8;
  font-size: 11px;
  font-weight: 700;
}

.first-device-protocol-helper small {
  margin-top: 3px;
  color: #475569;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-protocol-helper__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.first-device-operation-check {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #f8fafc;
}

.first-device-operation-check strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 12px;
}

.first-device-operation-check small {
  display: block;
  margin-top: 3px;
  color: #64748b;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-access-row strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
}

.first-device-access-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  min-width: 0;
  font-size: 12px;
}

.first-device-access-row span {
  color: #64748b;
}

@media (max-width: 900px) {
  .first-device-protocol-helper__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
