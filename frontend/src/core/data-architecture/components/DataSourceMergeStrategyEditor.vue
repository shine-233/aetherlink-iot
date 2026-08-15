<!--
  文件用途: 数据源合并策略编辑器。
  核心逻辑: 配置 object、array、script、conditional 等多数据项合并方式并维护选中项状态。
  关键注意事项: 合并策略会被持久化，脚本文本和选中索引需要避免破坏已有配置。
  重构建议: 拆分策略表单、脚本编辑和预览校验，降低单组件复杂度。
-->
<script setup lang="ts">
/**
 * 数据源合并策略编辑器
 * 用于配置数据源内多个数据项的合并方式
 */

import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewMergeStrategy } from './merge-strategy-preview'

// 合并策略类型定义
interface MergeStrategy {
  type: 'object' | 'array' | 'script' | 'condition'
  script?: string
  description?: string
}

// Props 接口定义
interface Props {
  /** 数据源ID */
  dataSourceId: string
  /** 当前合并策略 */
  modelValue?: MergeStrategy
  /** 数据项数量（用于显示预期效果） */
  dataItemCount?: number
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: () => ({ type: 'object' }),
  dataItemCount: 1
})

const emit = defineEmits<{
  'update:modelValue': [value: MergeStrategy]
}>()

// 国际化
const { t } = useI18n()

// 响应式数据
const currentStrategy = ref<MergeStrategy>({ ...props.modelValue })
const showCustomScript = ref(false)
const previewResult = ref<unknown>(null)
const previewError = ref('')
const isPreviewing = ref(false)
let previewRequestId = 0

const DEFAULT_TELEMETRY_MERGE_SCRIPT = `// 合并设备遥测、在线状态和告警来源
const telemetry = Object.assign(
  {},
  ...items.filter(item => item && typeof item === 'object' && !Array.isArray(item))
)

return {
  device_id: telemetry.device_id || telemetry.deviceId || 'replace_with_device_id',
  temperature: telemetry.temperature,
  humidity: telemetry.humidity,
  rssi: telemetry.rssi,
  online: telemetry.online,
  alarm_count: telemetry.alarm_count || 0,
  latest_at: telemetry.latest_at || telemetry.timestamp || Date.now()
}`

// 预制合并策略选项
const mergeStrategyOptions = [
  {
    value: 'object',
    label: '对象浅合并',
    description: 'Object.assign({}, item1, item2, ...) - 将所有数据项的属性合并到一个对象中',
    snippet: '{ ...item1, ...item2, ...item3 }',
    icon: '🔗'
  },
  {
    value: 'array',
    label: '组成数组',
    description: '[item1, item2, item3] - 将所有数据项组成一个数组',
    snippet: '[telemetryRow, statusRow, alarmRow]',
    icon: '📝'
  },
  {
    value: 'condition',
    label: '条件选择',
    description: '根据预设条件选择合适的数据项',
    snippet: '选择第一个可用 / 选择最大数据集',
    icon: '⚖️'
  },
  {
    value: 'script',
    label: '自定义脚本',
    description: '使用自定义JavaScript脚本处理合并逻辑',
    snippet: 'return items.filter(...).map(...)',
    icon: '⚙️'
  }
]

// 条件选择策略选项
const conditionStrategyOptions = [
  {
    value: 'first-available',
    label: '选择第一个可用',
    script: 'return items.find(item => item !== null && item !== undefined) || {}'
  },
  {
    value: 'largest-dataset',
    label: '选择最大数据集',
    script: `return items.reduce((largest, current) => {
  const currentSize = Array.isArray(current) ? current.length : Object.keys(current || {}).length
  const largestSize = Array.isArray(largest) ? largest.length : Object.keys(largest || {}).length
  return currentSize > largestSize ? current : largest
}, {})`
  },
  {
    value: 'merge-arrays',
    label: '数组展开合并',
    script: `return items.reduce((result, item) => {
  if (Array.isArray(item)) {
    return [...result, ...item]
  } else if (item) {
    return [...result, item]
  }
  return result
}, [])`
  }
]

// 计算属性
const isScriptStrategy = computed(() => currentStrategy.value.type === 'script' || showCustomScript.value)

const previewText = computed(() => {
  const count = props.dataItemCount
  if (count <= 1) {
    return '单个数据项，无需合并'
  }

  switch (currentStrategy.value.type) {
    case 'object':
      return `合并 ${count} 个数据项的属性到一个对象`
    case 'array':
      return `将 ${count} 个数据项组成数组`
    case 'condition':
      return `根据条件从 ${count} 个数据项中选择`
    case 'script':
      return `使用自定义脚本处理 ${count} 个数据项`
    default:
      return ''
  }
})

// 监听变化并通知父组件
watch(
  currentStrategy,
  newValue => {
    emit('update:modelValue', { ...newValue })
  },
  { deep: true }
)

// 选择预制合并策略
const selectMergeStrategy = (strategyType: string) => {
  currentStrategy.value.type = strategyType as any

  if (strategyType === 'script') {
    showCustomScript.value = true
    currentStrategy.value.script = currentStrategy.value.script || DEFAULT_TELEMETRY_MERGE_SCRIPT
  } else {
    showCustomScript.value = false
    currentStrategy.value.script = undefined
  }
}

// 选择条件策略
const selectConditionStrategy = (option: any) => {
  currentStrategy.value.type = 'script'
  currentStrategy.value.script = option.script
  currentStrategy.value.description = option.label
}

// 使用设备运维数据预览合并结果，避免泛用演示数据误导配置。
const previewItems = [
  { device_id: 'replace_with_device_id', temperature: 25, humidity: 60 },
  { online: true, rssi: -52, latest_at: '2026-07-07T08:00:00.000Z' },
  { alarm_count: 0, proof_stage: 'latest-telemetry-visible' }
]

// 预览使用与运行时一致的本地脚本引擎，不在浏览器宿主上下文动态求值。
const previewMergeResult = async () => {
  const requestId = ++previewRequestId
  isPreviewing.value = true
  previewError.value = ''

  const result = await previewMergeStrategy(previewItems, {
    type: currentStrategy.value.type,
    ...(currentStrategy.value.type === 'script' ? { script: currentStrategy.value.script } : {})
  })

  // 用户连续修改脚本时，仅保留最后一次预览结果。
  if (requestId !== previewRequestId) return

  isPreviewing.value = false
  if (result.success) {
    previewResult.value = result.data
  } else {
    previewResult.value = null
    previewError.value = result.error || '合并预览执行失败'
  }
}
</script>

<template>
  <div class="data-source-merge-strategy-editor">
    <!-- 标题和说明 -->
    <div class="strategy-header">
      <n-space align="center" justify="space-between">
        <div>
          <n-text strong>{{ dataSourceId }} 合并策略</n-text>
          <n-text depth="3" style="margin-left: 8px">
            {{ previewText }}
          </n-text>
        </div>
        <n-tag v-if="dataItemCount > 1" type="info" size="small">{{ dataItemCount }} 个数据项</n-tag>
      </n-space>
    </div>

    <!-- 合并策略选择 -->
    <n-card size="small" style="margin-top: 16px">
      <template #header>
        <n-space align="center">
          <span>⚙️</span>
          <span>合并策略选择</span>
        </n-space>
      </template>

      <n-space vertical size="large">
        <!-- 策略选项 -->
        <n-radio-group :value="currentStrategy.type" size="large" @update:value="selectMergeStrategy">
          <n-space vertical size="medium">
            <div
              v-for="option in mergeStrategyOptions"
              :key="option.value"
              class="strategy-option"
              :class="{ active: currentStrategy.type === option.value }"
            >
              <n-radio :value="option.value" size="large">
                <n-space align="center">
                  <span class="strategy-icon">{{ option.icon }}</span>
                  <div class="strategy-content">
                    <div class="strategy-title">{{ option.label }}</div>
                    <div class="strategy-description">{{ option.description }}</div>
                    <n-code class="strategy-snippet">{{ option.snippet }}</n-code>
                  </div>
                </n-space>
              </n-radio>
            </div>
          </n-space>
        </n-radio-group>

        <!-- 条件选择策略详细配置 -->
        <div v-if="currentStrategy.type === 'condition'" class="condition-strategies">
          <n-divider>条件选择详细配置</n-divider>
          <n-space vertical>
            <div v-for="option in conditionStrategyOptions" :key="option.value" class="condition-option">
              <n-button
                secondary
                type="primary"
                style="width: 100%; text-align: left"
                @click="selectConditionStrategy(option)"
              >
                <n-space justify="space-between" style="width: 100%">
                  <span>{{ option.label }}</span>
                  <span>→</span>
                </n-space>
              </n-button>
            </div>
          </n-space>
        </div>

        <!-- 自定义脚本编辑 -->
        <div v-if="isScriptStrategy" class="custom-script-section">
          <n-divider>自定义脚本编辑</n-divider>

          <n-space vertical>
            <n-alert type="info" :show-icon="false">
              <template #icon><span>💡</span></template>
              <div>
                <strong>脚本说明</strong>
                <ul style="margin: 8px 0; padding-left: 20px">
                  <li>
                    <code>items</code>
                    : 数据项数组，包含所有处理后的数据项
                  </li>
                  <li>
                    <code>return</code>
                    : 返回合并后的最终数据
                  </li>
                  <li>使用本地 script-engine 安全执行；网络、存储和宿主全局默认阻断</li>
                </ul>
              </div>
            </n-alert>

            <n-input
              v-model:value="currentStrategy.script"
              type="textarea"
              :placeholder="DEFAULT_TELEMETRY_MERGE_SCRIPT"
              :rows="8"
              style="font-family: 'Consolas', 'Monaco', monospace"
            />

            <!-- 脚本预览 -->
            <n-card size="small" title="🔍 预览效果">
              <n-alert v-if="previewError" type="error" :show-icon="false" style="margin-bottom: 8px">
                {{ previewError }}
              </n-alert>
              <n-code
                :code="JSON.stringify(previewResult, null, 2)"
                language="json"
                style="max-height: 200px; overflow-y: auto"
              />
            </n-card>
          </n-space>
        </div>
      </n-space>
    </n-card>

    <!-- 操作按钮 -->
    <n-space justify="end" style="margin-top: 16px">
      <n-button :loading="isPreviewing" @click="previewMergeResult">🔍 预览效果</n-button>
      <n-button type="primary" @click="$emit('update:modelValue', currentStrategy)">✅ 确认策略</n-button>
    </n-space>
  </div>
</template>

<style scoped>
.data-source-merge-strategy-editor {
  padding: 16px;
}

.strategy-header {
  padding: 12px 16px;
  background: var(--card-color);
  border-radius: var(--border-radius);
  border: 1px solid var(--border-color);
}

.strategy-option {
  padding: 16px;
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius);
  transition: all 0.3s ease;
  cursor: pointer;
}

.strategy-option:hover {
  border-color: var(--primary-color);
  background: var(--primary-color-hover);
}

.strategy-option.active {
  border-color: var(--primary-color);
  background: var(--primary-color-pressed);
}

.strategy-icon {
  font-size: 24px;
  margin-right: 12px;
}

.strategy-content {
  flex: 1;
}

.strategy-title {
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--text-color);
}

.strategy-description {
  font-size: 13px;
  color: var(--text-color-2);
  margin-bottom: 8px;
  line-height: 1.4;
}

.strategy-snippet {
  font-size: 12px;
  background: var(--code-color);
  padding: 4px 8px;
  border-radius: 4px;
  display: inline-block;
}

.condition-strategies {
  background: var(--body-color);
  padding: 16px;
  border-radius: var(--border-radius);
  border: 1px solid var(--border-color);
}

.condition-option {
  margin-bottom: 8px;
}

.custom-script-section {
  background: var(--body-color);
  padding: 16px;
  border-radius: var(--border-radius);
  border: 1px solid var(--border-color);
}
</style>
