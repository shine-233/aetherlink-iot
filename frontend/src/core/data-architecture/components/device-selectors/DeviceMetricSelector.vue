<!--
  文件用途: 设备指标选择器组件。
  核心逻辑: 选择设备和指标并生成 deviceId 与 metric 参数。
  关键注意事项: 指标名称和参数角色会影响模板请求参数绑定。
  重构建议: 将设备加载、指标加载和参数生成策略拆分。
-->
<script setup lang="ts">
/**
 * DeviceMetricSelector - 设备指标选择器（中等复杂）
 * 需要选择设备和指标，生成deviceId + metric两个参数
 */

import { ref, computed, watch, onMounted } from 'vue'
import { NSelect, NSpace, NText, NIcon, NButton, NAlert, NDivider } from 'naive-ui'
import { PhonePortraitOutline as DeviceIcon, BarChartOutline as MetricIcon } from '@vicons/ionicons5'
import type { DeviceInfo, DeviceMetric } from '@/core/data-architecture/types/device-parameter-group'
import type { SelectOption } from 'naive-ui'
import { getDeviceMetricList, getDeviceSourceList } from '@/service/api'

interface Props {
  /** 预选择的设备（编辑模式下使用） */
  preSelectedDevice?: DeviceInfo
  /** 预选择的指标（编辑模式下使用） */
  preSelectedMetric?: DeviceMetric
  /** 是否为编辑模式 */
  editMode?: boolean
}

interface Emits {
  (e: 'selectionCompleted', data: { device: DeviceInfo; metric: DeviceMetric }): void
  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

type ApiRecord = Record<string, unknown>
type MetricType = DeviceMetric['metricType']

// 当前选择状态
const selectedDeviceId = ref<string>(props.preSelectedDevice?.deviceId || '')
const selectedMetricKey = ref<string>(props.preSelectedMetric?.metricKey || '')
const deviceList = ref<DeviceInfo[]>(props.preSelectedDevice ? [props.preSelectedDevice] : [])
const metricList = ref<DeviceMetric[]>(props.preSelectedMetric ? [props.preSelectedMetric] : [])
const isLoadingDevices = ref(false)
const isLoadingMetrics = ref(false)

const extractArray = (value: unknown): unknown[] => {
  if (Array.isArray(value)) return value
  if (value && typeof value === 'object') {
    const record = value as ApiRecord
    return extractArray(record.data ?? record.list ?? record.records ?? record.items)
  }
  return []
}

const normalizeDevice = (device: unknown): DeviceInfo | null => {
  if (!device || typeof device !== 'object') return null

  const record = device as ApiRecord
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

const normalizeMetricType = (value: unknown): MetricType => {
  return value === 'number' || value === 'boolean' || value === 'json' || value === 'string' ? value : 'string'
}

const normalizeMetric = (metric: unknown): DeviceMetric | null => {
  if (!metric || typeof metric !== 'object') return null

  const record = metric as ApiRecord
  const metricKey = String(record.metricKey ?? record.key ?? record.data_identifier ?? record.id ?? record.value ?? '')
  if (!metricKey) return null

  const unit = record.unit ?? record.symbol
  const description = record.description ?? record.desc

  return {
    metricKey,
    metricLabel: String(record.metricLabel ?? record.label ?? record.name ?? metricKey),
    metricType: normalizeMetricType(record.metricType ?? record.data_type ?? record.type),
    ...(unit ? { unit: String(unit) } : {}),
    ...(description ? { description: String(description) } : {})
  }
}

const normalizeMetrics = (response: unknown): DeviceMetric[] => {
  const groupsOrMetrics = extractArray(response)
  const metrics: DeviceMetric[] = []

  for (const item of groupsOrMetrics) {
    const rawOptions: unknown = item && typeof item === 'object' ? (item as ApiRecord).options : undefined
    if (Array.isArray(rawOptions)) {
      metrics.push(
        ...rawOptions.map(normalizeMetric).filter((metric): metric is DeviceMetric => Boolean(metric))
      )
      continue
    }

    const metric = normalizeMetric(item)
    if (metric) metrics.push(metric)
  }

  return metrics
}

const mergeDevices = (devices: DeviceInfo[]) => {
  const byId = new Map<string, DeviceInfo>()

  for (const device of deviceList.value) byId.set(device.deviceId, device)
  for (const device of devices) byId.set(device.deviceId, device)
  if (props.preSelectedDevice) {
    byId.set(props.preSelectedDevice.deviceId, byId.get(props.preSelectedDevice.deviceId) ?? props.preSelectedDevice)
  }

  deviceList.value = Array.from(byId.values())
}

const mergeMetrics = (metrics: DeviceMetric[]) => {
  const byKey = new Map<string, DeviceMetric>()

  for (const metric of metricList.value) byKey.set(metric.metricKey, metric)
  for (const metric of metrics) byKey.set(metric.metricKey, metric)
  if (props.preSelectedMetric) {
    byKey.set(
      props.preSelectedMetric.metricKey,
      byKey.get(props.preSelectedMetric.metricKey) ?? props.preSelectedMetric
    )
  }

  metricList.value = Array.from(byKey.values())
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

const loadMetricOptions = async (deviceId: string) => {
  if (!deviceId) return

  try {
    isLoadingMetrics.value = true
    const response = await getDeviceMetricList(deviceId)
    mergeMetrics(normalizeMetrics(response))
  } catch {
    mergeMetrics([])
  } finally {
    isLoadingMetrics.value = false
  }
}

// 设备选项
const deviceOptions = computed<SelectOption[]>(() => {
  return deviceList.value.map(device => ({
    label: device.deviceType ? `${device.deviceName} (${device.deviceType})` : device.deviceName,
    value: device.deviceId,
    device: device
  }))
})

// 当前选择的设备
const selectedDevice = computed<DeviceInfo | null>(() => {
  if (!selectedDeviceId.value) return null
  return deviceList.value.find(device => device.deviceId === selectedDeviceId.value) || null
})

// 可用的指标选项（根据选择的设备动态变化）
const availableMetrics = computed<DeviceMetric[]>(() => {
  if (!selectedDevice.value) return []
  return metricList.value
})

// 指标选项
const metricOptions = computed<SelectOption[]>(() => {
  return availableMetrics.value.map(metric => ({
    label: `${metric.metricLabel}${metric.unit ? ` (${metric.unit})` : ''}`,
    value: metric.metricKey,
    metric: metric
  }))
})

// 当前选择的指标
const selectedMetric = computed<DeviceMetric | null>(() => {
  if (!selectedMetricKey.value) return null
  return availableMetrics.value.find(metric => metric.metricKey === selectedMetricKey.value) || null
})

// 是否可以确认选择
const canConfirm = computed(() => {
  return selectedDevice.value !== null && selectedMetric.value !== null
})

/**
 * 监听设备变化，重置指标选择
 */
watch(selectedDeviceId, (newDeviceId, oldDeviceId) => {
  if (newDeviceId !== oldDeviceId && oldDeviceId !== undefined) {
    selectedMetricKey.value = ''
    metricList.value = []
  }

  if (newDeviceId) {
    loadMetricOptions(newDeviceId)
  }
})

/**
 * 处理设备选择
 */
const handleDeviceChange = (deviceId: string) => {
  selectedDeviceId.value = deviceId
}

/**
 * 处理指标选择
 */
const handleMetricChange = (metricKey: string) => {
  selectedMetricKey.value = metricKey
}

/**
 * 确认选择
 */
const confirmSelection = () => {
  if (!selectedDevice.value || !selectedMetric.value) return
  emit('selectionCompleted', {
    device: selectedDevice.value,
    metric: selectedMetric.value
  })
}

/**
 * 取消选择
 */
const cancelSelection = () => {
  emit('cancel')
}

onMounted(() => {
  loadDeviceOptions()
  if (selectedDeviceId.value) {
    loadMetricOptions(selectedDeviceId.value)
  }
})
</script>

<template>
  <div class="device-metric-selector">
    <!-- 选择器标题 -->
    <div class="selector-header">
      <n-space align="center">
        <n-icon size="20" color="#2080f0">
          <MetricIcon />
        </n-icon>
        <n-text strong>{{ editMode ? '重新选择设备指标' : '选择设备指标' }}</n-text>
      </n-space>
      <n-text depth="3" style="font-size: 12px; margin-top: 4px">
        选择设备和指标，将生成
        <strong>deviceId</strong>
        +
        <strong>metric</strong>
        两个参数
      </n-text>
    </div>

    <!-- 设备选择 -->
    <div class="selection-step">
      <n-space align="center" style="margin-bottom: 8px">
        <n-icon size="16"><DeviceIcon /></n-icon>
        <n-text strong>第1步：选择设备</n-text>
      </n-space>

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

    <n-divider style="margin: 12px 0" />

    <!-- 指标选择 -->
    <div class="selection-step">
      <n-space align="center" style="margin-bottom: 8px">
        <n-icon size="16"><MetricIcon /></n-icon>
        <n-text strong>第2步：选择指标</n-text>
        <n-text v-if="!selectedDevice" depth="3" style="font-size: 12px">（请先选择设备）</n-text>
      </n-space>

      <n-select
        v-model:value="selectedMetricKey"
        :options="metricOptions"
        placeholder="请选择指标..."
        :loading="isLoadingMetrics"
        :disabled="!selectedDevice || isLoadingMetrics || metricOptions.length === 0"
        clearable
        @update:value="handleMetricChange"
      />
    </div>

    <!-- 选择预览 -->
    <div v-if="selectedDevice || selectedMetric" class="selection-preview">
      <n-alert type="info" style="margin-top: 16px">
        <template #header>
          <span>选择预览</span>
        </template>

        <n-space vertical size="small">
          <!-- 设备信息 -->
          <div v-if="selectedDevice">
            <n-space align="center" style="margin-bottom: 8px">
              <n-icon size="16"><DeviceIcon /></n-icon>
              <n-text strong>选择的设备：</n-text>
            </n-space>
            <div style="padding-left: 20px">
              <n-space vertical size="small">
                <n-space>
                  <n-text depth="3">设备名称：</n-text>
                  <n-text>{{ selectedDevice.deviceName }}</n-text>
                </n-space>
                <n-space>
                  <n-text depth="3">设备类型：</n-text>
                  <n-text>{{ selectedDevice.deviceType }}</n-text>
                </n-space>
              </n-space>
            </div>
          </div>

          <!-- 指标信息 -->
          <div v-if="selectedMetric">
            <n-space align="center" style="margin-bottom: 8px">
              <n-icon size="16"><MetricIcon /></n-icon>
              <n-text strong>选择的指标：</n-text>
            </n-space>
            <div style="padding-left: 20px">
              <n-space vertical size="small">
                <n-space>
                  <n-text depth="3">指标名称：</n-text>
                  <n-text>{{ selectedMetric.metricLabel }}</n-text>
                </n-space>
                <n-space>
                  <n-text depth="3">数据类型：</n-text>
                  <n-text>{{ selectedMetric.metricType }}</n-text>
                </n-space>
                <n-space v-if="selectedMetric.unit">
                  <n-text depth="3">单位：</n-text>
                  <n-text>{{ selectedMetric.unit }}</n-text>
                </n-space>
              </n-space>
            </div>
          </div>

          <!-- 生成参数预览 -->
          <div v-if="canConfirm" style="margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border-color)">
            <n-text depth="3" style="font-size: 12px">💡 将生成参数：</n-text>
            <div
              style="
                margin-top: 8px;
                padding: 8px;
                background: var(--code-color);
                border-radius: 4px;
                font-family: monospace;
                font-size: 12px;
              "
            >
              <div>
                <strong>deviceId</strong>
                = "{{ selectedDevice!.deviceId }}"
              </div>
              <div>
                <strong>metric</strong>
                = "{{ selectedMetric!.metricKey }}"
              </div>
            </div>
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
.device-metric-selector {
  padding: 20px;
  min-height: 400px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.selector-header {
  margin-bottom: 8px;
}

.selection-step {
  margin-bottom: 12px;
}

.selection-preview {
  margin: 16px 0;
}

.selector-actions {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}
</style>
