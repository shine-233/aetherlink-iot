<!--
文件用途：RDI 概览「系统快照」网格中的单台设备卡片（温度、能耗、开关量、固件、安装信息）。
核心逻辑：纯展示；点击整卡 emit open 由父级跳转设备详情。温度单位由父级传入。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { NTag } from 'naive-ui'
import { $t } from '@/locales'
import { formatSwitch, formatTemperature, type DeviceSnapshot, type TemperatureUnit } from './rdiOverviewState'
import { snapshotInstallationEntries, snapshotStatusLabel, snapshotStatusTagType } from './rdiSnapshotPresentation'

const props = defineProps<{
  device: DeviceSnapshot
  temperatureUnit: TemperatureUnit
  showTenant: boolean
}>()

defineEmits<{ open: [deviceId: string] }>()

const installation = computed(() => snapshotInstallationEntries(props.device))
const temp = (value: unknown) => formatTemperature(value, props.temperatureUnit)
const sw = (value: unknown) => formatSwitch(value, $t)
</script>

<template>
  <button type="button" class="snapshot-card" @click="$emit('open', device.id)">
    <div class="snapshot-head">
      <div class="snapshot-title">
        <strong>{{ device.name }}</strong>
        <span>{{ $t('rdi.overview.pid') }}: {{ device.pid }}</span>
      </div>
      <NTag :type="snapshotStatusTagType(device)">{{ snapshotStatusLabel(device) }}</NTag>
    </div>
    <div class="snapshot-values">
      <span>
        T1
        <strong>{{ temp(device.telemetry.temperature_1) }}</strong>
      </span>
      <span>
        T2
        <strong>{{ temp(device.telemetry.temperature_2) }}</strong>
      </span>
      <span>
        {{ $t('rdi.overview.energy') }}
        <strong>{{ device.telemetry.electricity_consumption ?? '--' }}</strong>
      </span>
    </div>
    <div class="snapshot-status">
      <span>SW1 {{ sw(device.telemetry.switch_1) }}</span>
      <span>SW2 {{ sw(device.telemetry.switch_2) }}</span>
      <span>DO {{ sw(device.telemetry.dry_contact_output) }}</span>
    </div>
    <div class="snapshot-foot">{{ $t('rdi.overview.firmware') }}: {{ device.firmware }}</div>
    <div v-if="showTenant && device.tenantId !== '--'" class="snapshot-foot">
      {{ $t('rdi.overview.tenantScope') }}: {{ device.tenantId }}
    </div>
    <div v-if="installation.length" class="snapshot-installation">
      <span v-for="entry in installation" :key="entry.field">{{ $t(entry.labelKey) }} {{ entry.value }}</span>
    </div>
  </button>
</template>

<style scoped>
.snapshot-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 168px;
  padding: 14px;
  text-align: left;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  cursor: pointer;
}

.snapshot-card:hover,
.snapshot-card:focus-visible {
  border-color: #2563eb;
  box-shadow: 0 8px 22px rgb(15 23 42 / 8%);
}

.snapshot-head,
.snapshot-values,
.snapshot-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.snapshot-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.snapshot-title strong {
  overflow: hidden;
  color: #111827;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.snapshot-title span,
.snapshot-foot {
  color: #6b7280;
  font-size: 12px;
}

.snapshot-installation {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 8px;
  color: #4b5563;
  font-size: 12px;
}

.snapshot-installation span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.snapshot-values,
.snapshot-status {
  flex-wrap: wrap;
  color: #374151;
  font-size: 13px;
}

.snapshot-values span,
.snapshot-status span {
  min-width: 72px;
}

.snapshot-values strong {
  display: block;
  margin-top: 2px;
  color: #111827;
  font-size: 18px;
}
</style>
