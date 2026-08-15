<!--
  文件用途: 设备 ID 选择器组件。
  核心逻辑: 选择单个设备并生成 deviceId 参数。
  关键注意事项: 生成参数结构必须与 HTTP 参数编辑器和设备参数组类型一致。
  重构建议: 复用统一设备选择查询逻辑，避免各选择器重复维护设备加载状态。
-->
<script setup lang="ts">
/**
 * DeviceIdSelector - 设备ID选择器（简单模式）
 * 提供最基础的设备选择功能，只生成deviceId参数
 */

import { ref, computed, onMounted } from 'vue'
import { NSelect, NSpace, NText, NIcon, NButton, NAlert } from 'naive-ui'
import { PhonePortraitOutline as DeviceIcon } from '@vicons/ionicons5'
import type { DeviceInfo } from '@/core/data-architecture/types/device-parameter-group'
import type { SelectOption } from 'naive-ui'
import { getDeviceSourceList } from '@/service/api'

interface Props {
  /** 预选择的设备（编辑模式下使用） */
  preSelectedDevice?: DeviceInfo
  /** 是否为编辑模式 */
  editMode?: boolean
}

interface Emits {
  (e: 'deviceSelected', device: DeviceInfo): void
  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

type ApiDevice = Record<string, unknown>

const deviceList = ref<DeviceInfo[]>(props.preSelectedDevice ? [props.preSelectedDevice] : [])
const isLoadingDevices = ref(false)

// 当前选择的设备ID
const selectedDeviceId = ref<string>(props.preSelectedDevice?.deviceId || '')

const extractArray = (value: unknown): unknown[] => {
  if (Array.isArray(value)) return value
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>
    return extractArray(record.data ?? record.list ?? record.records ?? record.items)
  }
  return []
}

const normalizeDevice = (device: unknown): DeviceInfo | null => {
  if (!device || typeof device !== 'object') return null

  const record = device as ApiDevice
  const deviceId = String(record.deviceId ?? record.device_id ?? record.id ?? record.value ?? '')
  if (!deviceId) return null

  const deviceName = String(record.deviceName ?? record.device_name ?? record.name ?? record.label ?? deviceId)
  const deviceType = String(record.deviceType ?? record.device_type ?? record.type ?? record.product_name ?? '')
  const deviceModel = record.deviceModel ?? record.device_model ?? record.model

  return {
    deviceId,
    deviceName,
    deviceType,
    ...(deviceModel ? { deviceModel: String(deviceModel) } : {})
  }
}

const mergeDevices = (devices: DeviceInfo[]) => {
  const byId = new Map<string, DeviceInfo>()

  for (const device of deviceList.value) {
    byId.set(device.deviceId, device)
  }
  for (const device of devices) {
    byId.set(device.deviceId, device)
  }
  if (props.preSelectedDevice) {
    byId.set(props.preSelectedDevice.deviceId, byId.get(props.preSelectedDevice.deviceId) ?? props.preSelectedDevice)
  }

  deviceList.value = Array.from(byId.values())
}

const loadDeviceOptions = async () => {
  try {
    isLoadingDevices.value = true
    const response = await getDeviceSourceList({})
    const devices = extractArray(response)
      .map(normalizeDevice)
      .filter((device): device is DeviceInfo => Boolean(device))
    mergeDevices(devices)
  } catch {
    mergeDevices([])
  } finally {
    isLoadingDevices.value = false
  }
}

// 转换为下拉选项格式
const deviceOptions = computed<SelectOption[]>(() => {
  return deviceList.value.map(device => ({
    label: device.deviceType ? `${device.deviceName} (${device.deviceType})` : device.deviceName,
    value: device.deviceId,
    device: device // 携带完整设备信息
  }))
})

// 当前选择的设备信息
const selectedDevice = computed<DeviceInfo | null>(() => {
  if (!selectedDeviceId.value) return null
  return deviceList.value.find(device => device.deviceId === selectedDeviceId.value) || null
})

// 是否可以确认选择
const canConfirm = computed(() => {
  return selectedDevice.value !== null
})

/**
 * 处理设备选择
 */
const handleDeviceChange = (deviceId: string) => {
  selectedDeviceId.value = deviceId
}

/**
 * 确认选择
 */
const confirmSelection = () => {
  if (!selectedDevice.value) return
  emit('deviceSelected', selectedDevice.value)
}

/**
 * 取消选择
 */
const cancelSelection = () => {
  emit('cancel')
}

onMounted(() => {
  loadDeviceOptions()
})
</script>

<template>
  <div class="device-id-selector">
    <!-- 选择器标题 -->
    <div class="selector-header">
      <n-space align="center">
        <n-icon size="20" color="#2080f0">
          <DeviceIcon />
        </n-icon>
        <n-text strong>{{ editMode ? '重新选择设备' : '选择设备' }}</n-text>
      </n-space>
      <n-text depth="3" style="font-size: 12px; margin-top: 4px">
        选择一个设备，将生成
        <strong>deviceId</strong>
        参数
      </n-text>
    </div>

    <!-- 设备选择器 -->
    <div class="device-selector">
      <n-select
        v-model:value="selectedDeviceId"
        :options="deviceOptions"
        placeholder="请选择设备..."
        :loading="isLoadingDevices"
        :disabled="isLoadingDevices && deviceOptions.length === 0"
        clearable
        filterable
        @update:value="handleDeviceChange"
      />
    </div>

    <!-- 选择预览 -->
    <div v-if="selectedDevice" class="selection-preview">
      <n-alert type="info" style="margin-top: 12px">
        <template #header>
          <n-space align="center">
            <n-icon size="16"><DeviceIcon /></n-icon>
            <span>已选择设备</span>
          </n-space>
        </template>

        <n-space vertical size="small">
          <n-space>
            <n-text strong>设备名称：</n-text>
            <n-text>{{ selectedDevice.deviceName }}</n-text>
          </n-space>
          <n-space>
            <n-text strong>设备类型：</n-text>
            <n-text>{{ selectedDevice.deviceType }}</n-text>
          </n-space>
          <n-space>
            <n-text strong>设备型号：</n-text>
            <n-text>{{ selectedDevice.deviceModel }}</n-text>
          </n-space>

          <div style="margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border-color)">
            <n-text depth="3" style="font-size: 12px">
              💡 将生成参数：
              <strong>deviceId = "{{ selectedDevice.deviceId }}"</strong>
            </n-text>
          </div>
        </n-space>
      </n-alert>
    </div>

    <!-- 操作按钮 -->
    <div class="selector-actions">
      <n-space justify="end">
        <n-button @click="cancelSelection">取消</n-button>
        <n-button type="primary" :disabled="!canConfirm" @click="confirmSelection">
          {{ editMode ? '更新参数' : '生成参数' }}
        </n-button>
      </n-space>
    </div>
  </div>
</template>

<style scoped>
.device-id-selector {
  padding: 20px;
  min-height: 300px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.selector-header {
  margin-bottom: 8px;
}

.device-selector {
  flex: 1;
}

.selection-preview {
  margin: 12px 0;
}

.selector-actions {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}
</style>
