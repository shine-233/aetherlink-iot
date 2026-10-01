<!--
  Map canvas with its grid fallback, fallback marker and selected-device caption.
  Owns the AMap instance: it initialises once and re-centres whenever the selected position changes.
-->
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { $t } from '@/locales'
import { useEquipmentAmap } from './useEquipmentAmap'

const props = defineProps<{
  /** Selected device position as [lng, lat]; null clears the marker. */
  position: [number, number] | null
  locationText?: string
  deviceLabel: string
  markerStyle: { left: string; top: string }
}>()

const { mapDomRef, mapLoading, mapReady, mapError, initMap, updateMarker } = useEquipmentAmap()

onMounted(() => {
  void initMap(props.position)
})

watch(
  () => props.position,
  position => {
    if (!mapReady.value) {
      void initMap(position)
      return
    }
    updateMarker(position)
  }
)
</script>

<template>
  <div class="device-map-surface">
    <div ref="mapDomRef" class="device-map-sdk" :class="{ visible: mapReady && !mapError }"></div>
    <div v-if="!mapReady || mapError || !position" class="device-map-grid"></div>
    <div
      v-if="deviceLabel && (!mapReady || mapError || !position)"
      class="device-map-marker"
      :style="markerStyle"
    >
      <span></span>
    </div>
    <div class="device-map-status">
      <strong>{{ deviceLabel || $t('rdi.map.selectDevice') }}</strong>
      <small>
        {{ mapError ? $t('rdi.map.mapUnavailable') : locationText || $t('rdi.map.locationMissing') }}
      </small>
    </div>
    <NSpin v-if="mapLoading" class="device-map-loading" :show="mapLoading" />
  </div>
</template>

<style scoped>
.device-map-surface {
  position: relative;
  min-height: 320px;
  overflow: hidden;
  background: #eef3f8;
}

.device-map-sdk {
  position: absolute;
  inset: 0;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.device-map-sdk.visible {
  opacity: 1;
}

.device-map-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(69, 90, 120, 0.12) 1px, transparent 1px),
    linear-gradient(90deg, rgba(69, 90, 120, 0.12) 1px, transparent 1px);
  background-size: 36px 36px;
}

.device-map-marker {
  position: absolute;
  transform: translate(-50%, -50%);
}

.device-map-marker span {
  display: block;
  width: 16px;
  height: 16px;
  border: 3px solid #ffffff;
  border-radius: 999px;
  background: #d03050;
  box-shadow: 0 2px 10px rgba(28, 35, 45, 0.35);
}

.device-map-status {
  position: absolute;
  left: 16px;
  bottom: 16px;
  display: flex;
  max-width: min(520px, calc(100% - 32px));
  flex-direction: column;
  gap: 4px;
  border: 1px solid rgba(69, 90, 120, 0.16);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  padding: 10px 12px;
}

.device-map-status small {
  color: #667085;
}

.device-map-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(238, 243, 248, 0.56);
}
</style>
