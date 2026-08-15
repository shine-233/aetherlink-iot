<!--
  文件用途: 组件级轮询配置组件。
  核心逻辑: 维护轮询启停、间隔和立即执行等表单状态，并向父级同步配置。
  关键注意事项: 轮询配置会触发组件全部数据源，默认值变化可能放大请求频率。
  重构建议: 将表单状态、校验规则和执行提示拆分为可复用组合函数。
-->
<script setup lang="ts">
/**
 * ComponentPollingConfig - 组件级别轮询配置
 * 统一管理组件的轮询配置，执行时会触发组件的所有数据源
 */

import { reactive, computed, watch, onMounted, ref, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/store/modules/theme'

// 轮询配置接口
interface PollingConfig {
  /** 是否启用轮询 */
  enabled: boolean
  /** 轮询间隔（毫秒） */
  interval: number
  /** 是否立即执行首次轮询 */
  immediate: boolean
}

// Props接口
interface Props {
  /** 组件ID */
  componentId: string
  /** 组件名称 */
  componentName?: string
  /** 是否为预览模式 - 仅用于UI状态显示 */
  previewMode?: boolean
  /** 初始轮询配置 */
  initialConfig?: PollingConfig | null
}

// Emits接口
interface Emits {
  (e: 'configChange', config: PollingConfig): void
}

const props = withDefaults(defineProps<Props>(), {
  componentName: '组件',
  previewMode: false,
  initialConfig: null
})

const emit = defineEmits<Emits>()
const { t } = useI18n()
const themeStore = useThemeStore()

/**
 * 防止配置循环更新的标志
 */
const isInternalUpdate = ref(false)

/**
 * 本地轮询配置状态
 */
const pollingConfig = reactive<PollingConfig>({
  enabled: props.initialConfig?.enabled || false,
  interval: props.initialConfig?.interval || 30000,
  // 🔥 修复：正确处理 immediate 属性的默认值
  immediate: props.initialConfig?.immediate !== undefined ? props.initialConfig.immediate : true
})

/**
 * 间隔选项
 */
const intervalOptions = [
  { label: '5秒', value: 5000 },
  { label: '10秒', value: 10000 },
  { label: '30秒', value: 30000 },
  { label: '1分钟', value: 60000 },
  { label: '5分钟', value: 300000 },
  { label: '10分钟', value: 600000 }
]

/**
 * 轮询状态显示文本
 */
const statusText = computed(() => {
  if (!props.previewMode && pollingConfig.enabled) {
    return '已配置（预览模式执行）'
  }

  if (pollingConfig.enabled) {
    return '已配置'
  }

  return '未配置'
})

/**
 * 状态显示类型
 */
const statusType = computed(() => {
  if (pollingConfig.enabled) {
    return props.previewMode ? 'success' : 'info'
  }
  return 'default'
})

/**
 * 处理配置变化
 */
const handleConfigChange = () => {
  // 🔥 防止内部更新时触发事件
  if (isInternalUpdate.value) {
    return
  }

  // 发射配置变化事件，由父组件处理保存
  emit('configChange', { ...pollingConfig })
}

/**
 * 监听配置变化
 */
watch(() => pollingConfig.enabled, handleConfigChange)
watch(() => pollingConfig.interval, handleConfigChange)
watch(() => pollingConfig.immediate, handleConfigChange)

/**
 * 监听初始配置变化（用于恢复已保存的配置）
 */
watch(
  () => props.initialConfig,
  newConfig => {
    if (newConfig) {
      // 🔥 设置内部更新标志，防止触发配置变化事件
      isInternalUpdate.value = true

      pollingConfig.enabled = newConfig.enabled || false
      pollingConfig.interval = newConfig.interval || 30000
      // 🔥 修复：正确处理 immediate 属性，允许为 false
      pollingConfig.immediate = newConfig.immediate !== undefined ? newConfig.immediate : true

      // 🔥 延迟重置标志，确保所有响应式更新完成
      nextTick(() => {
        isInternalUpdate.value = false
      })
    }
  },
  { deep: true, immediate: true }
)

/**
 * 组件挂载时初始化调试
 */
onMounted(() => {})
</script>

<template>
  <div class="component-polling-config">
    <!-- 紧凑的轮询配置行 -->
    <div class="polling-row">
      <div class="polling-left">
        <n-text class="polling-title">组件轮询配置</n-text>
        <n-tag :type="statusType" size="small">
          {{ statusText }}
        </n-tag>
      </div>

      <div class="polling-right">
        <n-switch v-model:value="pollingConfig.enabled" size="small" />
      </div>
    </div>

    <!-- 轮询详细配置（折叠显示） -->
    <div v-if="pollingConfig.enabled" class="polling-details">
      <div class="detail-item">
        <span class="detail-label">轮询间隔</span>
        <n-select v-model:value="pollingConfig.interval" :options="intervalOptions" size="small" style="width: 80px" />
      </div>

      <div class="detail-item">
        <span class="detail-label">立即执行</span>
        <n-switch v-model:value="pollingConfig.immediate" size="small" />
      </div>

      <div class="detail-note">
        <n-text depth="3" size="small">轮询仅在预览模式下执行，将刷新组件的所有数据源</n-text>
      </div>
    </div>
  </div>
</template>

<style scoped>
.component-polling-config {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--card-color);
  margin-bottom: 12px;
  font-size: 13px;
}

/* 主要配置行 - 水平布局 */
.polling-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  gap: 12px;
}

.polling-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.polling-title {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.polling-right {
  flex-shrink: 0;
}

/* 详细配置区域 */
.polling-details {
  border-top: 1px solid var(--border-color);
  padding: 8px 12px 10px;
  background: var(--body-color);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.detail-label {
  font-size: 12px;
  color: var(--text-color-2);
  white-space: nowrap;
}

.detail-note {
  padding-top: 6px;
  border-top: 1px dashed var(--divider-color);
  margin-top: 2px;
}

/* 响应主题变化 */
[data-theme='dark'] .component-polling-config {
  background: var(--card-color);
  border-color: var(--border-color);
}

[data-theme='dark'] .polling-details {
  background: var(--body-color);
  border-top-color: var(--border-color);
}
</style>
