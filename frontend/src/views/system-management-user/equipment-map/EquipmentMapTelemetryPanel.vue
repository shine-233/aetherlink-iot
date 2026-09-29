<!--
  Telemetry panel for the device currently selected on the equipment map.
-->
<script setup lang="ts">
import { $t } from '@/locales'
import { formatTime, telemetryLabel, telemetryValue, type DeviceRecord, type MapTelemetry } from './equipment-map-model'

defineProps<{
  device: DeviceRecord | null
  telemetry: MapTelemetry | null
  loading: boolean
  requested: boolean
}>()

const emit = defineEmits<{ load: []; openDetails: [] }>()
</script>

<template>
  <div class="device-map-detail">
    <div class="device-map-detail-head">
      <div>
        <h3>{{ device?.name || device?.device_number || '-' }}</h3>
        <p>{{ device?.location || $t('rdi.map.locationMissing') }}</p>
      </div>
      <NSpace>
        <NButton :disabled="!device" :loading="loading" @click="emit('load')">
          {{ $t('rdi.map.loadTelemetry') }}
        </NButton>
        <NButton :disabled="!device" @click="emit('openDetails')">{{ $t('rdi.map.openDetails') }}</NButton>
      </NSpace>
    </div>

    <NSpin :show="loading">
      <div class="device-map-telemetry-summary">
        <span>{{ $t('rdi.map.lastPushTime') }}</span>
        <strong>{{ formatTime(telemetry?.last_push_time) }}</strong>
      </div>
      <div class="device-map-telemetry">
        <NEmpty
          v-if="!telemetry?.telemetry_data?.length"
          :description="requested ? $t('rdi.map.noTelemetry') : $t('rdi.map.telemetryNotLoaded')"
        >
          <template v-if="device && !requested" #extra>
            <NButton type="primary" size="small" @click="emit('load')">
              {{ $t('rdi.map.loadTelemetry') }}
            </NButton>
          </template>
        </NEmpty>
        <div v-for="item in telemetry?.telemetry_data || []" :key="item.key">
          <span>{{ telemetryLabel(item) }}</span>
          <strong>{{ telemetryValue(item) }}</strong>
        </div>
      </div>
    </NSpin>
  </div>
</template>

<style scoped>
.device-map-detail {
  border: 1px solid var(--n-border-color);
  border-width: 1px 0 0;
  border-radius: 0;
  background: var(--n-color);
  padding: 16px;
}

.device-map-detail h3 {
  margin: 0;
  font-size: 18px;
}

.device-map-detail p {
  margin: 4px 0 0;
  color: var(--n-text-color-3);
}

.device-map-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.device-map-telemetry-summary {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-top: 16px;
  border-top: 1px solid var(--n-border-color);
  padding-top: 12px;
}

.device-map-telemetry-summary span {
  color: var(--n-text-color-3);
  font-size: 13px;
}

.device-map-telemetry {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.device-map-telemetry div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  padding: 10px;
}

.device-map-telemetry span {
  color: var(--n-text-color-3);
  font-size: 13px;
}

.device-map-telemetry strong {
  overflow-wrap: anywhere;
}

@media (max-width: 900px) {
  .device-map-detail-head {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
