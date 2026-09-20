<script setup lang="ts">
import { computed, ref } from 'vue'
import { resolveMapData } from './data'
import type { LocalViewerFields, MapWidgetConfig } from './types'

const props = defineProps<{
  config: MapWidgetConfig
  fields: LocalViewerFields
}>()

const mapData = computed(() => resolveMapData(props.config, props.fields))

const zoomLevel = ref(props.config.zoom ?? 1)
const showTooltip = ref(true)

function zoomIn() {
  if (zoomLevel.value < 4) zoomLevel.value += 0.5
}

function zoomOut() {
  if (zoomLevel.value > 0.5) zoomLevel.value -= 0.5
}

function resetZoom() {
  zoomLevel.value = props.config.zoom ?? 1
}

const markerX = computed(() => {
  if (!mapData.value.available || mapData.value.lng === undefined) return 50
  return ((mapData.value.lng + 180) / 360) * 100
})

const markerY = computed(() => {
  if (!mapData.value.available || mapData.value.lat === undefined) return 50
  return ((90 - mapData.value.lat) / 180) * 100
})
</script>

<template>
  <div class="local-map-widget" role="region" aria-label="Local dashboard map">
    <div v-if="!mapData.available" class="local-map-unavailable">
      <span>{{ mapData.fallback ?? 'No GPS coordinates available' }}</span>
    </div>
    <div v-else class="local-map-container">
      <div v-if="mapData.title" class="local-map-title">
        {{ mapData.title }}
      </div>

      <!-- Controls -->
      <div class="local-map-controls">
        <button class="map-ctrl-btn" title="Zoom In" @click="zoomIn">+</button>
        <button class="map-ctrl-btn" title="Zoom Out" @click="zoomOut">-</button>
        <button class="map-ctrl-btn" title="Reset Center" @click="resetZoom">&#x21bb;</button>
      </div>

      <!-- Map Surface -->
      <div
        class="local-map-viewport"
        :style="{
          transform: `scale(${zoomLevel})`,
          transformOrigin: `${markerX}% ${markerY}%`
        }"
      >
        <svg class="local-map-svg" viewBox="0 0 1000 500" preserveAspectRatio="none">
          <defs>
            <pattern id="grid" width="50" height="50" patternUnits="userSpaceOnUse">
              <path d="M 50 0 L 0 0 0 50" fill="none" stroke="rgba(128, 128, 128, 0.15)" stroke-width="1" />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />
          <!-- Equator & Prime Meridian -->
          <line
            x1="0"
            y1="250"
            x2="1000"
            y2="250"
            stroke="rgba(128, 128, 128, 0.25)"
            stroke-width="1.5"
            stroke-dasharray="4,4"
          />
          <line
            x1="500"
            y1="0"
            x2="500"
            y2="500"
            stroke="rgba(128, 128, 128, 0.25)"
            stroke-width="1.5"
            stroke-dasharray="4,4"
          />
        </svg>

        <!-- Marker Pin -->
        <div
          class="local-map-marker"
          :style="{ left: `${markerX}%`, top: `${markerY}%` }"
          @click="showTooltip = !showTooltip"
        >
          <div class="marker-pulse" />
          <div class="marker-pin" />
          <div v-if="showTooltip" class="marker-tooltip">
            <div class="tooltip-title">{{ config.title || 'Device Location' }}</div>
            <div class="tooltip-coords">Lat: {{ mapData.lat?.toFixed(4) }}, Lng: {{ mapData.lng?.toFixed(4) }}</div>
            <div v-if="config.entityId" class="tooltip-extra">ID: {{ config.entityId }}</div>
          </div>
        </div>
      </div>

      <div class="local-map-footer">
        <span class="status-indicator online" />
        <span>GPS: {{ mapData.lat?.toFixed(4) }}, {{ mapData.lng?.toFixed(4) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.local-map-widget {
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  min-height: 160px;
  position: relative;
  overflow: hidden;
  background: var(--n-color, #1e293b);
  color: var(--n-text-color, #f8fafc);
  font-family: inherit;
}

.local-map-unavailable {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #8c8c8c;
  font-size: 13px;
}

.local-map-container {
  width: 100%;
  height: 100%;
  position: relative;
  display: flex;
  flex-direction: column;
}

.local-map-title {
  position: absolute;
  top: 8px;
  left: 12px;
  z-index: 10;
  font-size: 13px;
  font-weight: 600;
  background: rgba(15, 23, 42, 0.6);
  padding: 2px 8px;
  border-radius: 4px;
  backdrop-filter: blur(4px);
}

.local-map-controls {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.map-ctrl-btn {
  width: 26px;
  height: 26px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(15, 23, 42, 0.7);
  color: #fff;
  font-size: 14px;
  font-weight: bold;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.map-ctrl-btn:hover {
  background: rgba(59, 130, 246, 0.8);
}

.local-map-viewport {
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  transition: transform 0.25s ease-out;
}

.local-map-svg {
  width: 100%;
  height: 100%;
  position: absolute;
  top: 0;
  left: 0;
}

.local-map-marker {
  position: absolute;
  width: 16px;
  height: 16px;
  margin-left: -8px;
  margin-top: -8px;
  cursor: pointer;
  z-index: 5;
}

.marker-pin {
  width: 12px;
  height: 12px;
  background: #10b981;
  border: 2px solid #ffffff;
  border-radius: 50%;
  position: relative;
  z-index: 2;
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.8);
}

.marker-pulse {
  position: absolute;
  top: -4px;
  left: -4px;
  width: 24px;
  height: 24px;
  background: rgba(16, 185, 129, 0.4);
  border-radius: 50%;
  animation: pulse-ring 1.8s cubic-bezier(0.215, 0.61, 0.355, 1) infinite;
}

@keyframes pulse-ring {
  0% {
    transform: scale(0.5);
    opacity: 0.8;
  }
  100% {
    transform: scale(2.2);
    opacity: 0;
  }
}

.marker-tooltip {
  position: absolute;
  bottom: 22px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(15, 23, 42, 0.9);
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 11px;
  white-space: nowrap;
  pointer-events: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
  z-index: 20;
}

.tooltip-title {
  font-weight: 600;
  color: #38bdf8;
  margin-bottom: 2px;
}

.tooltip-coords {
  color: #cbd5e1;
}

.tooltip-extra {
  color: #94a3b8;
  font-size: 10px;
}

.local-map-footer {
  position: absolute;
  bottom: 6px;
  left: 10px;
  z-index: 10;
  font-size: 11px;
  background: rgba(15, 23, 42, 0.6);
  padding: 2px 8px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.status-indicator.online {
  background: #10b981;
}
</style>
