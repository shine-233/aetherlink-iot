<!--
  Device twin tab wrapper.
  It keeps the customer-facing twin/shadow entry independent from the telemetry
  workbench while reusing the existing TwinLiteCard backend contract.
-->
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { $t } from '@/locales'
import TwinLiteCard from './telemetry/twin-lite/TwinLiteCard.vue'
import { useTelemetryRealtimeState } from './telemetry/useTelemetryRealtimeState'

const props = defineProps<{
  id: string
}>()

const { telemetryData, telemetryLoadError, telemetryLoadStatus, refreshTelemetry } = useTelemetryRealtimeState(
  () => props.id
)

onMounted(() => {
  refreshTelemetry()
})

watch(
  () => props.id,
  () => {
    refreshTelemetry()
  }
)
</script>

<template>
  <n-space vertical size="large">
    <n-alert type="info" :show-icon="true">
      {{ $t('custom.device_details.twinIntro') }}
    </n-alert>

    <n-alert v-if="telemetryLoadStatus === 'error'" type="warning" :show-icon="true">
      {{ $t('custom.device_details.telemetrySnapshotLoadFailed') }}
      <span v-if="telemetryLoadError">: {{ telemetryLoadError }}</span>
    </n-alert>

    <TwinLiteCard :id="props.id" :reported-telemetry="telemetryData" />
  </n-space>
</template>
