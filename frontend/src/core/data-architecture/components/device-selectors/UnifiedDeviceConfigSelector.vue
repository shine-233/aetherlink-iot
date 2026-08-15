<!--
  文件用途: 统一设备配置选择器。
  核心逻辑: 用一个表单管理设备 ID、指标、属性等参数选择，避免重复参数。
  关键注意事项: 每种参数类型只能存在一个实例，去重规则影响最终 HTTP 配置。
  重构建议: 拆分参数类型视图和去重/合并策略，并补充冲突处理测试。
-->
<script setup lang="ts">
/**
 * UnifiedDeviceConfigSelector - 统一设备配置选择器
 *
 * 设计原则：
 * - 每种参数类型只能存在一个实例（deviceId、metric等）
 * - 增量式配置：用户可以逐步添加参数，不会产生冲突
 * - 修改模式：再次选择就是修改现有配置
 * - 可用性：只展示当前能生成真实参数的设备字段
 */

import { ref, computed, watch, onMounted } from 'vue'
import { NCard, NSpace, NText, NIcon, NButton, NSelect, NCheckbox, NAlert, NDivider } from 'naive-ui'
import { PhonePortraitOutline as DeviceIcon, BarChartOutline as MetricIcon } from '@vicons/ionicons5'
import type { DeviceInfo, DeviceMetric } from '@/core/data-architecture/types/device-parameter-group'
import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import type { SelectOption } from 'naive-ui'
import { getDeviceMetricList, getDeviceSourceList } from '@/service/api'
import {
  applyRestoredDeviceConfig,
  buildPreviewParameters,
  canGenerateDeviceParameters,
  createDefaultDeviceConfig,
  generateDeviceParameters,
  mergeSelectionOptions,
  readExistingDeviceConfig,
  reconcileDeviceSelectionChange,
  reconcileMetricInclusion,
  selectDeviceById,
  selectMetricByKey,
  type DeviceConfig
} from './deviceConfigSelectionModel'
import { extractArray, normalizeDevice, normalizeMetrics } from './deviceConfigOptionNormalizers'

interface Props {
  /** 当前已有的参数列表（用于检测现有配置） */
  existingParameters?: EnhancedParameter[]
  /** 是否为编辑模式 */
  editMode?: boolean
}

interface Emits {
  (e: 'parametersGenerated', parameters: EnhancedParameter[]): void
  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// 配置状态
const config = ref<DeviceConfig>(createDefaultDeviceConfig())

const deviceList = ref<DeviceInfo[]>([])
const metricList = ref<DeviceMetric[]>([])
const isLoadingDevices = ref(false)
const isLoadingMetrics = ref(false)

const mergeDeviceOptions = (devices: DeviceInfo[]) => {
  deviceList.value = mergeSelectionOptions(
    deviceList.value,
    devices,
    config.value.selectedDevice,
    (device) => device.deviceId
  )
}

const mergeMetricOptions = (metrics: DeviceMetric[]) => {
  metricList.value = mergeSelectionOptions(
    metricList.value,
    metrics,
    config.value.selectedMetric,
    (metric) => metric.metricKey
  )
}

const loadDeviceOptions = async () => {
  try {
    isLoadingDevices.value = true
    const response = await getDeviceSourceList({})
    const devices = extractArray(response)
      .map(normalizeDevice)
      .filter((device): device is DeviceInfo => Boolean(device))
    mergeDeviceOptions(devices)
  } catch {
    mergeDeviceOptions([])
  } finally {
    isLoadingDevices.value = false
  }
}

const loadMetricOptions = async (deviceId: string) => {
  if (!deviceId) return

  try {
    isLoadingMetrics.value = true
    const response = await getDeviceMetricList(deviceId)
    mergeMetricOptions(normalizeMetrics(response))
  } catch {
    mergeMetricOptions([])
  } finally {
    isLoadingMetrics.value = false
  }
}

// 设备选项
const deviceOptions = computed<SelectOption[]>(() => {
  return deviceList.value.map((device) => ({
    label: device.deviceType ? `${device.deviceName} (${device.deviceType})` : device.deviceName,
    value: device.deviceId,
    device: device
  }))
})

// 可用指标选项
const availableMetrics = computed<DeviceMetric[]>(() => {
  if (!config.value.selectedDevice) return []
  return metricList.value
})

const metricOptions = computed<SelectOption[]>(() => {
  return availableMetrics.value.map((metric) => ({
    label: `${metric.metricLabel}${metric.unit ? ` (${metric.unit})` : ''}`,
    value: metric.metricKey,
    metric: metric
  }))
})

// 预览生成的参数
const previewParameters = computed(() => buildPreviewParameters(config.value))

// 是否可以生成参数
const canGenerate = computed(() => canGenerateDeviceParameters(config.value))

const syncMetricsForSelectedDevice = (newDeviceId: string | undefined, oldDeviceId: string | undefined) => {
  const selectionChange = reconcileDeviceSelectionChange(config.value, newDeviceId, oldDeviceId)

  if (selectionChange.shouldClearMetricOptions) {
    // 设备变化时，重置指标选择
    metricList.value = []
  }

  if (selectionChange.deviceIdToLoad) {
    loadMetricOptions(selectionChange.deviceIdToLoad)
  }
}

/**
 * 监听设备变化，重置相关选择
 */
watch(() => config.value.selectedDevice?.deviceId, syncMetricsForSelectedDevice)

/**
 * 监听指标开关，自动处理指标选择
 */
watch(
  () => config.value.includeMetric,
  (includeMetric) => {
    reconcileMetricInclusion(config.value, includeMetric)
  }
)

/**
 * 处理设备选择
 */
const handleDeviceChange = (deviceId: string | null) => {
  selectDeviceById(config.value, deviceList.value, deviceId)
}

/**
 * 处理指标选择
 */
const handleMetricChange = (metricKey: string | null) => {
  selectMetricByKey(config.value, availableMetrics.value, metricKey)
}

/**
 * 初始化编辑模式（从现有参数中恢复配置）
 */
const initEditMode = () => {
  if (!props.existingParameters) return

  const restoredOptions = applyRestoredDeviceConfig(config.value, readExistingDeviceConfig(props.existingParameters))
  if (restoredOptions.device) mergeDeviceOptions([restoredOptions.device])
  if (restoredOptions.metric) mergeMetricOptions([restoredOptions.metric])
}

/**
 * 生成参数
 */
const generateParameters = () => {
  if (!canGenerate.value) return

  emit('parametersGenerated', generateDeviceParameters(config.value))
}

/**
 * 取消选择
 */
const cancel = () => {
  emit('cancel')
}

onMounted(async () => {
  if (props.editMode) {
    initEditMode()
  }

  await loadDeviceOptions()
  if (config.value.selectedDevice) {
    await loadMetricOptions(config.value.selectedDevice.deviceId)
  }
})
</script>

<template>
  <div class="unified-device-config-selector">
    <!-- 标题 -->
    <div class="selector-header">
      <n-space align="center">
        <n-icon size="20" color="#2080f0">
          <DeviceIcon />
        </n-icon>
        <n-text strong>{{ editMode ? '修改设备配置' : '设备参数配置' }}</n-text>
      </n-space>
      <n-text depth="3" style="font-size: 12px; margin-top: 4px">选择设备和需要的参数类型，避免重复参数问题</n-text>
    </div>

    <n-card :bordered="false" class="config-card">
      <!-- 设备选择 -->
      <div class="config-section">
        <n-space align="center" style="margin-bottom: 12px">
          <n-icon size="16"><DeviceIcon /></n-icon>
          <n-text strong>选择设备</n-text>
          <n-text type="error" style="font-size: 12px">*</n-text>
        </n-space>

        <n-select
          :value="config.selectedDevice?.deviceId"
          :options="deviceOptions"
          placeholder="请选择设备..."
          :loading="isLoadingDevices"
          :disabled="isLoadingDevices && deviceOptions.length === 0"
          clearable
          filterable
          @update:value="handleDeviceChange"
        />
      </div>

      <n-divider style="margin: 20px 0" />

      <!-- 参数类型选择 -->
      <div class="config-section">
        <n-text strong style="margin-bottom: 12px; display: block">选择需要的参数类型</n-text>

        <n-space vertical size="large">
          <!-- 设备ID参数 -->
          <div class="param-type-option">
            <n-space align="center">
              <n-checkbox v-model:checked="config.includeDeviceId" :disabled="!config.selectedDevice" />
              <n-icon size="16" color="#2080f0">
                <DeviceIcon />
              </n-icon>
              <div class="param-type-info">
                <n-text strong>设备ID参数</n-text>
                <n-text depth="3" style="font-size: 12px; display: block">生成 deviceId 参数，用于标识具体设备</n-text>
              </div>
              <n-tag v-if="config.includeDeviceId && config.selectedDevice" size="small" type="success">
                {{ config.selectedDevice.deviceId }}
              </n-tag>
            </n-space>
          </div>

          <!-- 指标参数 -->
          <div class="param-type-option">
            <n-space align="center">
              <n-checkbox
                v-model:checked="config.includeMetric"
                :disabled="!config.selectedDevice || availableMetrics.length === 0"
              />
              <n-icon size="16" color="#18a058">
                <MetricIcon />
              </n-icon>
              <div class="param-type-info">
                <n-text strong>指标参数</n-text>
                <n-text depth="3" style="font-size: 12px; display: block">生成 metric 参数，用于指定监控指标</n-text>
              </div>
              <n-tag v-if="config.includeMetric && config.selectedMetric" size="small" type="info">
                {{ config.selectedMetric.metricKey }}
              </n-tag>
            </n-space>

            <!-- 指标选择 -->
            <div v-if="config.includeMetric && config.selectedDevice" style="margin-left: 32px; margin-top: 8px">
              <n-select
                :value="config.selectedMetric?.metricKey"
                :options="metricOptions"
                placeholder="选择指标..."
                size="small"
                :loading="isLoadingMetrics"
                :disabled="isLoadingMetrics || availableMetrics.length === 0"
                @update:value="handleMetricChange"
              />
            </div>
          </div>

        </n-space>
      </div>

      <!-- 参数预览 -->
      <div v-if="previewParameters.length > 0" class="preview-section">
        <n-divider style="margin: 20px 0" />

        <n-alert type="info">
          <template #header>
            <span>生成参数预览</span>
          </template>

          <div class="param-preview">
            <div v-for="param in previewParameters" :key="param.key" class="param-preview-item">
              <strong>{{ param.key }}</strong>
              = "{{ param.value }}"
              <n-tag size="small" style="margin-left: 8px">{{ param.type }}</n-tag>
            </div>
          </div>
        </n-alert>
      </div>
    </n-card>

    <!-- 操作按钮 -->
    <div class="selector-actions">
      <n-space justify="end">
        <n-button @click="cancel">取消</n-button>
        <n-button type="primary" :disabled="!canGenerate" @click="generateParameters">
          {{ editMode ? '更新参数' : '生成参数' }} ({{ previewParameters.length }})
        </n-button>
      </n-space>
    </div>
  </div>
</template>

<style scoped>
.unified-device-config-selector {
  padding: 20px;
  min-height: 500px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.selector-header {
  margin-bottom: 8px;
}

.config-card {
  flex: 1;
  background: var(--card-color);
}

.config-section {
  margin-bottom: 20px;
}

.param-type-option {
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  transition: all 0.3s ease;
}

.param-type-option:hover {
  border-color: var(--primary-color-suppl);
  background: var(--hover-color);
}

.param-type-info {
  flex: 1;
  margin-left: 8px;
}

.preview-section {
  margin-top: 16px;
}

.param-preview {
  font-family: monospace;
  font-size: 12px;
  line-height: 1.6;
}

.param-preview-item {
  padding: 4px 8px;
  margin: 4px 0;
  background: var(--code-color);
  border-radius: 4px;
  display: flex;
  align-items: center;
}

.selector-actions {
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}
</style>
