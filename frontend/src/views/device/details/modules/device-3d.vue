<!--
  文件用途：设备详情「3D 预览」标签页的薄包装。
  核心逻辑：复用 useTelemetryRealtimeState 拉取当前遥测快照，提取温度类 key 驱动 Device3DPanel 材质颜色；
  Device3DPanel 经父级 registry 的 defineAsyncComponent 懒加载，three/@tresjs 仅在本 tab 首次激活时下载
  （配合 vite vendor-three 手动分包）。
  关键注意事项：物模型 key 因产品而异，温度提取只做展示层启发式匹配，不得反向写入任何数据。
-->
<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { $t } from '@/locales'
import Device3DPanel from '@/components/device3d/Device3DPanel.vue'
import { useTelemetryRealtimeState } from './telemetry/useTelemetryRealtimeState'

const props = defineProps<{
  id: string
  online?: boolean
  deviceData?: any
}>()

const { telemetryData, telemetryLoadStatus, telemetryLoadError, refreshTelemetry } = useTelemetryRealtimeState(
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

// 温度启发式：先精确匹配 temperature，再退回包含 temp 的首个数值型遥测；都没有则交给面板默认值。
const temperature = computed<number | undefined>(() => {
  const items = telemetryData.value ?? []
  const exact = items.find(item => item.key?.toLowerCase() === 'temperature')
  const fuzzy = items.find(item => typeof item.value === 'number' && item.key?.toLowerCase().includes('temp'))
  const hit = exact ?? fuzzy
  return hit && typeof hit.value === 'number' ? hit.value : undefined
})

const deviceName = computed(() => props.deviceData?.name || props.deviceData?.device_name || props.id)
</script>

<template>
  <n-space vertical size="large">
    <n-alert type="info" :show-icon="true">{{ $t('custom.device_details.preview3dIntro') }}</n-alert>

    <n-alert v-if="telemetryLoadStatus === 'error'" type="warning" :show-icon="true">
      {{ $t('custom.device_details.telemetrySnapshotLoadFailed') }}
      <span v-if="telemetryLoadError">: {{ telemetryLoadError }}</span>
    </n-alert>

    <Device3DPanel :online="props.online" :temperature="temperature" :device-name="deviceName" />
  </n-space>
</template>
