<!--
  文件用途：展示交互配置的预览视图，帮助用户模拟执行交互效果。
  核心逻辑：根据当前交互列表执行单项或全部预览，并维护预览状态、日志和重置流程。
  关键注意事项：预览语义需要与向导、模板选择器和持久化交互协议保持一致。
  重构建议：可将动作执行和日志收集抽为 composable，便于组件与测试复用。
-->
<template>
  <div class="interaction-preview">
    <!-- 预览控制区 -->
    <div class="preview-controls">
      <n-space justify="space-between" align="center">
        <div class="preview-info">
          <n-text strong>{{ t('interaction.preview.title') }}</n-text>
          <n-text depth="3" style="margin-left: 8px">
            {{ interactions.length }} {{ t('interaction.preview.interactionCount') }}
          </n-text>
        </div>

        <n-space size="small">
          <n-button size="small" :disabled="!hasActiveInteractions" @click="resetPreview">
            <template #icon>
              <n-icon><RefreshOutline /></n-icon>
            </template>
            {{ t('interaction.reset') }}
          </n-button>

          <n-button size="small" type="primary" :disabled="!hasActiveInteractions" @click="runAllInteractions">
            <template #icon>
              <n-icon><PlayOutline /></n-icon>
            </template>
            {{ t('interaction.preview.runAll') }}
          </n-button>
        </n-space>
      </n-space>
    </div>

    <!-- 预览画布 -->
    <div class="preview-canvas">
      <div
        ref="previewElement"
        class="preview-element"
        :style="previewElementStyles"
        tabindex="0"
        @click="handleEvent('click')"
        @mouseenter="handleEvent('hover')"
        @mouseleave="handleEvent('hover', false)"
        @focus="handleEvent('focus')"
        @blur="handleEvent('blur')"
      >
        <div class="element-content">
          <n-icon class="element-icon" size="24">
            <FlashOutline />
          </n-icon>
          <div class="element-text">{{ currentContent }}</div>
          <div class="element-subtitle">{{ t('interaction.preview.testClickHere') }}</div>
        </div>
      </div>
    </div>

    <!-- 交互列表 -->
    <div class="interactions-list">
      <h4 class="list-title">{{ t('interaction.preview.configList') }}</h4>

      <div class="interactions-grid">
        <n-card
          v-for="(interaction, index) in interactions"
          :key="`preview-interaction-${index}`"
          size="small"
          class="interaction-item"
          :class="{
            disabled: !interaction.enabled,
            active: isInteractionActive(interaction)
          }"
        >
          <template #header>
            <div class="interaction-header">
              <n-space align="center">
                <n-tag :type="getEventTagType(interaction.event)" size="small" round>
                  {{ getEventDisplayName(interaction.event) }}
                </n-tag>
                <span class="interaction-name">
                  {{ interaction.name || t('interaction.template.configIndex', { index: index + 1 }) }}
                </span>
              </n-space>

              <n-space size="small">
                <n-switch
                  :value="interaction.enabled"
                  size="small"
                  @update:value="value => toggleInteraction(index, value)"
                />
                <n-button
                  size="tiny"
                  type="primary"
                  :disabled="!interaction.enabled"
                  @click="testSingleInteraction(interaction)"
                >
                  <template #icon>
                    <n-icon><PlayCircleOutline /></n-icon>
                  </template>
                  {{ t('interaction.preview.test') }}
                </n-button>
              </n-space>
            </div>
          </template>

          <!-- 响应动作列表 -->
          <div class="responses-preview">
            <div
              v-for="(response, responseIndex) in interaction.responses"
              :key="`response-${responseIndex}`"
              class="response-item"
            >
              <div class="response-info">
                <n-tag size="tiny" type="info">
                  {{ getActionDisplayName(response.action) }}
                </n-tag>
                <span class="response-value">
                  {{ formatResponseValue(response) }}
                </span>
              </div>

              <div v-if="response.duration || response.delay" class="response-timing">
                <n-text depth="3" style="font-size: 11px">
                  <span v-if="response.delay">
                    {{ t('interaction.template.delayLabel', { delay: response.delay }) }}
                  </span>
                  <span v-if="response.delay && response.duration">·</span>
                  <span v-if="response.duration">
                    {{ t('interaction.template.durationLabel', { duration: response.duration }) }}
                  </span>
                </n-text>
              </div>
            </div>
          </div>
        </n-card>
      </div>
    </div>

    <!-- 执行日志 -->
    <div class="execution-log">
      <div class="log-header">
        <span class="log-title">{{ t('interaction.preview.executionLog') }}</span>
        <n-button size="tiny" quaternary :disabled="executionLog.length === 0" @click="clearLog">
          {{ t('interaction.clear') }}
        </n-button>
      </div>

      <div class="log-content">
        <div v-for="(log, index) in executionLog" :key="`log-${index}`" class="log-entry" :class="log.type">
          <span class="log-time">{{ formatTime(log.timestamp) }}</span>
          <span class="log-message">{{ log.message }}</span>
        </div>

        <div v-if="executionLog.length === 0" class="log-empty">
          <n-text depth="3">{{ t('interaction.preview.noExecutionRecords') }}</n-text>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 交互预览组件
 * 提供实时的交互效果预览和测试功能
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NSpace, NText, NButton, NIcon, NCard, NTag, NSwitch, useMessage } from 'naive-ui'
import { PlayOutline, RefreshOutline, FlashOutline, PlayCircleOutline } from '@vicons/ionicons5'

import type {
  InteractionConfig,
  InteractionEventType,
  InteractionResponse
} from './interactionPreviewTypes'
import {
  applyInteractionPreviewResponse,
  formatInteractionPreviewTime,
  formatInteractionResponseValue,
  getEnabledInteractionsByEvent,
  getInteractionActionDisplayName,
  getInteractionEventDisplayName,
  getInteractionEventTagType,
  type PreviewLogType
} from './interactionPreviewHelpers'

interface Props {
  interactions: InteractionConfig[]
  componentId?: string
}

interface Emits {
  (e: 'close'): void
}

interface LogEntry {
  type: PreviewLogType
  message: string
  timestamp: Date
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const message = useMessage()
const { t } = useI18n()

// 响应式状态
const previewElement = ref<HTMLElement>()
const currentContent = ref('')

// 初始化内容
onMounted(() => {
  currentContent.value = t('interaction.editor.previewElement')
})
const originalStyles = ref<any>({})
const runtimeStyles = ref<Record<string, string>>({})
const activeInteractions = ref<Set<number>>(new Set())
const executionLog = ref<LogEntry[]>([])
const isHovering = ref(false)

// 计算属性
const hasActiveInteractions = computed(() => {
  return props.interactions.some(interaction => interaction.enabled)
})

const previewElementStyles = computed(() => {
  return {
    transition: 'all 0.3s ease',
    cursor: 'pointer',
    userSelect: 'none',
    outline: 'none',
    ...originalStyles.value,
    ...runtimeStyles.value
  }
})

// 工具方法
const getEventTagType = (event: InteractionEventType) => {
  return getInteractionEventTagType(event)
}

const getEventDisplayName = (event: InteractionEventType) => {
  return getInteractionEventDisplayName(event, t)
}

const getActionDisplayName = (action: InteractionResponse['action']) => {
  return getInteractionActionDisplayName(action, t)
}

const formatResponseValue = (response: InteractionResponse) => {
  return formatInteractionResponseValue(response, t)
}

const formatTime = (date: Date) => {
  return formatInteractionPreviewTime(date)
}

const isInteractionActive = (interaction: InteractionConfig) => {
  const index = props.interactions.indexOf(interaction)
  return activeInteractions.value.has(index)
}

// 事件处理
const handleEvent = (eventType: InteractionEventType, isActive = true) => {
  if (eventType === 'hover') {
    isHovering.value = isActive
    if (isActive) {
      executeInteractionsByEvent('hover')
    } else {
      // 悬停结束，可以添加恢复逻辑
      addLog('info', t('interaction.preview.hoverEnd'))
    }
  } else {
    executeInteractionsByEvent(eventType)
  }
}

const executeInteractionsByEvent = (eventType: InteractionEventType) => {
  const matchingInteractions = getEnabledInteractionsByEvent(props.interactions, eventType)

  if (matchingInteractions.length === 0) {
    addLog('warning', t('interaction.preview.noEnabledInteractions', { eventType: getEventDisplayName(eventType) }))
    return
  }

  addLog(
    'info',
    t('interaction.preview.triggerEvent', {
      eventType: getEventDisplayName(eventType),
      count: matchingInteractions.length
    })
  )

  matchingInteractions.forEach(({ interaction, index }) => {
    executeInteraction(interaction, index)
  })
}

const executeInteraction = (interaction: InteractionConfig, index: number) => {
  activeInteractions.value.add(index)

  addLog(
    'success',
    t('interaction.preview.startExecution', {
      name: interaction.name || t('interaction.template.configIndex', { index: index + 1 })
    })
  )

  interaction.responses.forEach((response, responseIndex) => {
    const delay = response.delay || 0

    setTimeout(() => {
      try {
        executeResponse(response)
        addLog(
          'success',
          t('interaction.preview.executeAction', {
            action: getActionDisplayName(response.action),
            value: formatResponseValue(response)
          })
        )
      } catch (error) {
        addLog('error', t('interaction.preview.actionFailed', { action: getActionDisplayName(response.action), error }))
      }
    }, delay)
  })

  // 标记交互完成
  setTimeout(
    () => {
      activeInteractions.value.delete(index)
    },
    Math.max(...interaction.responses.map(r => (r.delay || 0) + (r.duration || 300)))
  )
}

const executeResponse = (response: InteractionResponse) => {
  if (!previewElement.value) return

  const element = previewElement.value

  const setRuntimeStyle = (property: string, styleValue: unknown) => {
    const normalizedValue = String(styleValue)
    const cssProperty = property.replace(/[A-Z]/g, match => `-${match.toLowerCase()}`)
    element.style.setProperty(cssProperty, normalizedValue)
    runtimeStyles.value = {
      ...runtimeStyles.value,
      [property]: normalizedValue
    }
  }

  applyInteractionPreviewResponse(element, response, {
    setRuntimeStyle,
    setContent: value => {
      currentContent.value = value
    }
  })
}

const testSingleInteraction = (interaction: InteractionConfig) => {
  const index = props.interactions.indexOf(interaction)
  executeInteraction(interaction, index)
}

const runAllInteractions = () => {
  addLog('info', t('interaction.preview.startExecutingAll'))

  // 模拟触发所有事件类型
  const eventTypes: InteractionEventType[] = ['click', 'hover', 'focus', 'blur', 'custom']

  eventTypes.forEach(eventType => {
    const hasEvent = props.interactions.some(i => i.event === eventType && i.enabled)
    if (hasEvent) {
      executeInteractionsByEvent(eventType)
    }
  })
}

const resetPreview = () => {
  if (!previewElement.value) return

  const element = previewElement.value

  // 重置所有样式
  element.style.cssText = ''
  element.className = 'preview-element'
  runtimeStyles.value = {}
  currentContent.value = t('interaction.editor.previewElement')

  // 清空活动状态
  activeInteractions.value.clear()

  addLog('info', t('interaction.messages.previewReset'))
  message.success(t('interaction.messages.previewReset'))
}

const toggleInteraction = (index: number, enabled: boolean) => {
  props.interactions[index].enabled = enabled
  const status = enabled ? t('interaction.events.enabled') : t('interaction.events.disabled')
  const name = props.interactions[index].name || t('interaction.template.configIndex', { index: index + 1 })
  addLog('info', t('interaction.preview.interactionToggled', { status, name }))
}

const addLog = (type: LogEntry['type'], message: string) => {
  executionLog.value.unshift({
    type,
    message,
    timestamp: new Date()
  })

  // 限制日志数量
  if (executionLog.value.length > 100) {
    executionLog.value = executionLog.value.slice(0, 100)
  }
}

const clearLog = () => {
  executionLog.value = []
  addLog('info', t('interaction.messages.logCleared'))
}

// 生命周期
onMounted(() => {
  if (previewElement.value) {
    // 保存原始样式
    const computedStyles = window.getComputedStyle(previewElement.value)
    originalStyles.value = {
      backgroundColor: computedStyles.backgroundColor,
      color: computedStyles.color,
      borderColor: computedStyles.borderColor,
      opacity: computedStyles.opacity,
      transform: computedStyles.transform,
      visibility: computedStyles.visibility
    }
  }

  addLog('info', t('interaction.preview.previewStarted'))
})

onUnmounted(() => {
  activeInteractions.value.clear()
})
</script>

<style scoped>
.interaction-preview {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 600px;
}

.preview-controls {
  padding: 16px;
  background: var(--card-color);
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.preview-canvas {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
  padding: 32px;
  background: var(--body-color);
  border: 2px dashed var(--border-color);
  border-radius: 12px;
  position: relative;
}

.preview-element {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 200px;
  height: 120px;
  padding: 20px;
  background: var(--card-color);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  cursor: pointer;
}

.element-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
}

.element-icon {
  color: var(--primary-color);
}

.element-text {
  font-weight: 600;
  color: var(--text-color);
  font-size: 14px;
}

.element-subtitle {
  font-size: 12px;
  color: var(--text-color-3);
}

.interactions-list {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.list-title {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
}

.interactions-grid {
  flex: 1;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
}

.interaction-item {
  transition: all 0.3s ease;
  border: 1px solid var(--border-color);
}

.interaction-item.active {
  border-color: var(--primary-color);
  background: var(--primary-color-suppl);
  box-shadow: 0 2px 8px var(--primary-color-hover);
}

.interaction-item.disabled {
  opacity: 0.6;
  filter: grayscale(0.3);
}

.interaction-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.interaction-name {
  font-weight: 500;
  color: var(--text-color);
  font-size: 13px;
}

.responses-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.response-item {
  padding: 8px;
  background: var(--body-color);
  border-radius: 4px;
  border: 1px solid var(--border-color);
}

.response-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.response-value {
  font-size: 12px;
  color: var(--text-color-2);
  font-family: Monaco, Consolas, monospace;
}

.response-timing {
  text-align: right;
}

.execution-log {
  display: flex;
  flex-direction: column;
  height: 150px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}

.log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--card-color);
  border-bottom: 1px solid var(--border-color);
}

.log-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
}

.log-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  background: var(--body-color);
  font-family: Monaco, Consolas, monospace;
  font-size: 11px;
}

.log-entry {
  display: flex;
  gap: 8px;
  padding: 2px 0;
  word-break: break-all;
}

.log-entry.info {
  color: var(--text-color-2);
}

.log-entry.success {
  color: var(--success-color);
}

.log-entry.warning {
  color: var(--warning-color);
}

.log-entry.error {
  color: var(--error-color);
}

.log-time {
  flex-shrink: 0;
  width: 60px;
  color: var(--text-color-3);
}

.log-message {
  flex: 1;
}

.log-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

/* 响应式调整 */
@media (max-width: 768px) {
  .interaction-preview {
    height: auto;
  }

  .interactions-grid {
    grid-template-columns: 1fr;
  }

  .preview-canvas {
    min-height: 150px;
    padding: 20px;
  }

  .preview-element {
    width: 150px;
    height: 100px;
  }
}

/* 滚动条样式 */
.interactions-grid::-webkit-scrollbar,
.log-content::-webkit-scrollbar {
  width: 6px;
}

.interactions-grid::-webkit-scrollbar-track,
.log-content::-webkit-scrollbar-track {
  background: var(--body-color);
  border-radius: 3px;
}

.interactions-grid::-webkit-scrollbar-thumb,
.log-content::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 3px;
}

.interactions-grid::-webkit-scrollbar-thumb:hover,
.log-content::-webkit-scrollbar-thumb:hover {
  background: var(--text-color-3);
}

/* 动画效果 */
.interaction-item {
  animation: slideIn 0.3s ease-out;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* 设备状态预览的交互状态 */
.preview-element:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}

.preview-element:focus {
  outline: 2px solid var(--primary-color);
  outline-offset: 2px;
}

.preview-element:active {
  transform: translateY(0);
}
</style>
