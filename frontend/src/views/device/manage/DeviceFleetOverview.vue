<script setup lang="ts">
import type { FleetTargetPresetKey } from './device-fleet-target-presets'
import type { FleetSelectionSummary } from './device-fleet-operations'

const props = defineProps<{
  currentPageSummary: FleetSelectionSummary
  targetPreviewTotal?: number | null
  selectedDeviceCount: number
  activePreset: FleetTargetPresetKey
}>()

const emit = defineEmits<{
  applyPreset: [preset: FleetTargetPresetKey]
  showSelectedSummary: []
  exportCurrentPage: []
}>()

const metricItems = [
  {
    key: 'online',
    labelKey: 'custom.devicePage.selectedDeviceSummaryOnline',
    icon: 'mdi:wifi-check',
    tone: 'success',
    preset: 'online' as FleetTargetPresetKey
  },
  {
    key: 'offline',
    labelKey: 'custom.devicePage.selectedDeviceSummaryOffline',
    icon: 'mdi:wifi-off',
    tone: 'neutral',
    preset: 'offline' as FleetTargetPresetKey
  },
  {
    key: 'alarmed',
    labelKey: 'custom.devicePage.selectedDeviceSummaryAlarmed',
    icon: 'mdi:bell-alert-outline',
    tone: 'warning',
    preset: 'alarmed' as FleetTargetPresetKey
  },
  {
    key: 'missingVersion',
    labelKey: 'custom.devicePage.selectedDeviceSummaryMissingVersion',
    icon: 'mdi:package-variant-closed-remove',
    tone: 'danger',
    // No preset filter maps to "missing version"; keep the field present so the
    // template's `item.preset && ...` guard type-checks across every metric item.
    preset: undefined as FleetTargetPresetKey | undefined
  }
] as const

const getMetricValue = (key: (typeof metricItems)[number]['key']) => props.currentPageSummary[key]
</script>

<template>
  <section class="device-fleet-overview" data-testid="device-fleet-overview">
    <div class="device-fleet-overview__header">
      <div>
        <p class="device-fleet-overview__eyebrow">{{ $t('custom.devicePage.fleetTargetPresets') }}</p>
        <h2 class="device-fleet-overview__title">{{ $t('custom.devicePage.fleetTargetPreviewCount') }}</h2>
      </div>
      <div class="device-fleet-overview__actions">
        <NButton size="small" secondary @click="emit('exportCurrentPage')">
          <template #icon>
            <SvgIcon icon="mdi:download-outline" />
          </template>
          {{ $t('custom.devicePage.fleetExportCurrentPage') }}
        </NButton>
        <NButton size="small" type="primary" secondary @click="emit('showSelectedSummary')">
          <template #icon>
            <SvgIcon icon="mdi:checkbox-marked-circle-outline" />
          </template>
          {{ $t('custom.devicePage.selectedDeviceSummaryTitle') }}
        </NButton>
      </div>
    </div>

    <div class="device-fleet-overview__grid">
      <button
        class="device-fleet-overview__total"
        :class="{ 'is-active': props.activePreset === 'all' }"
        type="button"
        @click="emit('applyPreset', 'all')"
      >
        <span class="device-fleet-overview__total-label">{{ $t('custom.devicePage.fleetTargetPreviewCount') }}</span>
        <strong>{{ props.targetPreviewTotal ?? props.currentPageSummary.total }}</strong>
        <small>{{ $t('custom.devicePage.fleetCurrentPageCount') }} {{ props.currentPageSummary.total }}</small>
      </button>

      <button
        v-for="item in metricItems"
        :key="item.key"
        class="device-fleet-overview__metric"
        :class="[`tone-${item.tone}`, { 'is-active': item.preset && props.activePreset === item.preset }]"
        type="button"
        :disabled="!item.preset"
        @click="item.preset && emit('applyPreset', item.preset)"
      >
        <span class="device-fleet-overview__metric-icon">
          <SvgIcon :icon="item.icon" />
        </span>
        <span class="device-fleet-overview__metric-body">
          <span class="device-fleet-overview__metric-label">{{ $t(item.labelKey) }}</span>
          <strong>{{ getMetricValue(item.key) }}</strong>
        </span>
      </button>
    </div>

    <div class="device-fleet-overview__footer">
      <span>{{ $t('custom.devicePage.fleetCurrentPageOnly') }}</span>
      <NTag size="small" :bordered="false" type="info">
        {{ $t('custom.devicePage.selectedDeviceSummaryTotal') }} {{ props.selectedDeviceCount }}
      </NTag>
    </div>
  </section>
</template>

<style scoped>
.device-fleet-overview {
  position: relative;
  overflow: hidden;
  margin-bottom: 12px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background:
    radial-gradient(circle at 8% 0%, rgba(59, 130, 246, 0.16), transparent 30%),
    radial-gradient(circle at 92% 8%, rgba(20, 184, 166, 0.14), transparent 28%),
    linear-gradient(135deg, #f8fbff 0%, #f1f7ff 58%, #ffffff 100%);
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.08);
}

.device-fleet-overview::after {
  position: absolute;
  right: -72px;
  bottom: -86px;
  width: 210px;
  height: 210px;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.16), rgba(34, 197, 94, 0.14));
  content: '';
  pointer-events: none;
}

.device-fleet-overview__header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.device-fleet-overview__eyebrow {
  margin: 0 0 4px;
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  line-height: 1.3;
  text-transform: uppercase;
}

.device-fleet-overview__title {
  margin: 0;
  color: #0f172a;
  font-size: 21px;
  font-weight: 700;
  line-height: 1.2;
}

.device-fleet-overview__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.device-fleet-overview__grid {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(160px, 1.3fr) repeat(4, minmax(120px, 1fr));
  gap: 10px;
  margin-top: 14px;
}

.device-fleet-overview__total,
.device-fleet-overview__metric {
  min-width: 0;
  min-height: 82px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.05);
  text-align: left;
  backdrop-filter: blur(10px);
  cursor: pointer;
  transition:
    background-color 0.16s ease,
    border-color 0.16s ease,
    box-shadow 0.16s ease,
    transform 0.16s ease;
}

.device-fleet-overview__total:hover,
.device-fleet-overview__metric:not(:disabled):hover {
  border-color: #93c5fd;
  background: #fff;
  box-shadow: 0 16px 32px rgb(15 23 42 / 10%);
  transform: translateY(-1px);
}

.device-fleet-overview__total.is-active,
.device-fleet-overview__metric.is-active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow:
    0 0 0 2px rgb(37 99 235 / 14%),
    0 16px 34px rgb(37 99 235 / 10%);
}

.device-fleet-overview__total {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 12px 14px;
  background: linear-gradient(135deg, #1d4ed8 0%, #0f766e 100%);
}

.device-fleet-overview__total-label,
.device-fleet-overview__metric-label {
  overflow: hidden;
  color: #64748b;
  font-size: 12px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-fleet-overview__total strong,
.device-fleet-overview__metric strong {
  display: block;
  margin-top: 3px;
  color: #0f172a;
  font-size: 24px;
  font-weight: 760;
  line-height: 1.15;
}

.device-fleet-overview__total small {
  margin-top: 6px;
  color: rgba(255, 255, 255, 0.78);
  font-size: 12px;
  line-height: 1.35;
}

.device-fleet-overview__total .device-fleet-overview__total-label,
.device-fleet-overview__total strong {
  color: #fff;
}

.device-fleet-overview__metric {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
}

.device-fleet-overview__metric:disabled {
  cursor: default;
}

.device-fleet-overview__metric-icon {
  display: flex;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: #eef2f7;
  color: #344054;
  font-size: 20px;
}

.device-fleet-overview__metric-body {
  min-width: 0;
}

.tone-success .device-fleet-overview__metric-icon {
  background: #dcfce7;
  color: #15803d;
}

.tone-warning .device-fleet-overview__metric-icon {
  background: #fef3c7;
  color: #b45309;
}

.tone-danger .device-fleet-overview__metric-icon {
  background: #fee2e2;
  color: #b91c1c;
}

.device-fleet-overview__footer {
  position: relative;
  z-index: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 10px;
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 1100px) {
  .device-fleet-overview__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 680px) {
  .device-fleet-overview__header {
    display: block;
  }

  .device-fleet-overview__actions {
    justify-content: flex-start;
    margin-top: 10px;
  }

  .device-fleet-overview__grid {
    grid-template-columns: 1fr;
  }
}
</style>
