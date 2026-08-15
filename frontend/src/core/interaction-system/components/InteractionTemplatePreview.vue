<!--
  文件用途：展示单个交互模板的详情、统计信息和预览效果。
  核心逻辑：读取模板配置生成事件与动作摘要，并支持预览、导出和关闭操作。
  关键注意事项：模板结构变更时需要同步更新统计、动作展示和导出格式。
  重构建议：可把模板统计计算与预览执行逻辑拆成纯函数，提升可测试性。
-->
<template>
  <div class="interaction-template-preview">
    <!-- 模板信息 -->
    <div class="template-info">
      <div class="template-header">
        <n-icon :color="template.color" size="32">
          <component :is="template.icon" />
        </n-icon>
        <div class="template-details">
          <h3 class="template-title">{{ template.name }}</h3>
          <p class="template-description">{{ template.description }}</p>
        </div>
      </div>

      <!-- 模板统计 -->
      <div class="template-stats">
        <n-space size="large">
          <div class="stat-item">
            <n-text strong>{{ template.config.length }}</n-text>
            <n-text depth="3" style="font-size: 12px">{{ t('interaction.template.interactionCount') }}</n-text>
          </div>
          <div class="stat-item">
            <n-text strong>{{ getTotalActionsCount() }}</n-text>
            <n-text depth="3" style="font-size: 12px">{{ t('interaction.template.actionCount') }}</n-text>
          </div>
          <div class="stat-item">
            <n-text strong>{{ getUniqueEventsCount() }}</n-text>
            <n-text depth="3" style="font-size: 12px">{{ t('interaction.template.eventTypeCount') }}</n-text>
          </div>
        </n-space>
      </div>
    </div>

    <!-- 配置详情 -->
    <div class="config-details">
      <h4 class="section-title">{{ t('interaction.template.configDetails') }}</h4>

      <div class="config-list">
        <n-card v-for="(config, index) in template.config" :key="`config-${index}`" size="small" class="config-card">
          <template #header>
            <div class="config-header">
              <n-space align="center">
                <n-tag :type="getEventTagType(config.event)" size="small" round>
                  {{ getEventDisplayName(config.event) }}
                </n-tag>
                <span class="config-name">
                  {{ config.name || t('interaction.template.configIndex', { index: index + 1 }) }}
                </span>
                <n-tag v-if="config.priority" size="tiny" type="info">
                  {{ t('interaction.template.priorityLabel', { priority: config.priority }) }}
                </n-tag>
              </n-space>

              <n-switch :value="config.enabled" size="small" disabled />
            </div>
          </template>

          <!-- 响应动作列表 -->
          <div class="responses-list">
            <div
              v-for="(response, responseIndex) in config.responses"
              :key="`response-${responseIndex}`"
              class="response-item"
            >
              <div class="response-main">
                <n-tag size="tiny" type="info">
                  {{ getActionDisplayName(response.action) }}
                </n-tag>
                <span class="response-value">{{ formatResponseValue(response) }}</span>
              </div>

              <div v-if="response.duration || response.delay || response.easing" class="response-meta">
                <n-text depth="3" style="font-size: 11px">
                  <span v-if="response.delay">
                    {{ t('interaction.template.delayLabel', { delay: response.delay }) }}
                  </span>
                  <span v-if="response.delay && (response.duration || response.easing)">·</span>
                  <span v-if="response.duration">
                    {{ t('interaction.template.durationLabel', { duration: response.duration }) }}
                  </span>
                  <span v-if="response.duration && response.easing">·</span>
                  <span v-if="response.easing">{{ response.easing }}</span>
                </n-text>
              </div>
            </div>
          </div>
        </n-card>
      </div>
    </div>

    <!-- 预览演示 -->
    <div class="preview-stage">
      <h4 class="section-title">{{ t('interaction.template.effectPreview') }}</h4>

      <div class="preview-canvas">
        <div
          ref="previewElement"
          class="preview-element"
          :style="previewRuntimeStyles"
          tabindex="0"
          @click="handlePreviewEvent('click')"
          @mouseenter="handlePreviewEvent('hover')"
          @mouseleave="resetPreviewElement"
          @focus="handlePreviewEvent('focus')"
          @blur="handlePreviewEvent('blur')"
        >
          <n-icon size="20">
            <FlashOutline />
          </n-icon>
          <span>{{ previewElementText }}</span>
        </div>
      </div>

      <div class="preview-controls">
        <n-space justify="center" size="small">
          <n-button size="small" @click="resetPreviewElement">
            <template #icon>
              <n-icon><RefreshOutline /></n-icon>
            </template>
            {{ t('interaction.reset') }}
          </n-button>

          <n-button size="small" type="primary" @click="runAllPreviewInteractions">
            <template #icon>
              <n-icon><PlayOutline /></n-icon>
            </template>
            {{ t('interaction.template.previewAll') }}
          </n-button>
        </n-space>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="template-actions">
      <n-space justify="space-between">
        <n-button @click="$emit('close')">{{ t('interaction.cancel') }}</n-button>

        <n-space size="small">
          <n-button @click="exportTemplate">
            <template #icon>
              <n-icon><DownloadOutline /></n-icon>
            </template>
            {{ t('interaction.template.export') }}
          </n-button>

          <n-button type="primary" @click="selectTemplate">
            <template #icon>
              <n-icon><CheckmarkOutline /></n-icon>
            </template>
            {{ t('interaction.template.selectTemplate') }}
          </n-button>
        </n-space>
      </n-space>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 交互模板预览组件
 * 展示模板的详细信息和效果预览
 */

import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NIcon, NText, NSpace, NCard, NTag, NSwitch, NButton, useMessage } from 'naive-ui'
import { FlashOutline, RefreshOutline, PlayOutline, DownloadOutline, CheckmarkOutline } from '@vicons/ionicons5'

import type {
  InteractionConfig,
  InteractionEventType,
  InteractionActionType,
  InteractionResponse
} from './interactionPreviewTypes'

interface InteractionTemplate {
  id: string
  name: string
  description: string
  category: string
  icon: any
  color: string
  config: InteractionConfig[]
  tags?: string[]
}

interface Props {
  template: InteractionTemplate
}

interface Emits {
  (e: 'close'): void
  (e: 'select', template: InteractionTemplate): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const message = useMessage()
const { t } = useI18n()

// 响应式状态
const previewElement = ref<HTMLElement>()
const previewElementText = ref('')
const previewRuntimeStyles = ref<Record<string, string>>({})

// 初始化文本
onMounted(() => {
  previewElementText.value = t('interaction.template.previewTarget')
})
const originalPreviewStyles = ref<any>({})

// 计算属性
const getTotalActionsCount = () => {
  return props.template.config.reduce((total, config) => total + config.responses.length, 0)
}

const getUniqueEventsCount = () => {
  const events = new Set(props.template.config.map(config => config.event))
  return events.size
}

// 工具方法
const getEventTagType = (event: InteractionEventType) => {
  const typeMap = {
    click: 'success',
    hover: 'info',
    focus: 'warning',
    blur: 'default',
    custom: 'error'
  }
  return typeMap[event] || 'default'
}

const getEventDisplayName = (event: InteractionEventType) => {
  const nameMap = {
    click: t('interaction.events.click'),
    hover: t('interaction.events.hover'),
    focus: t('interaction.events.focus'),
    blur: t('interaction.events.blur'),
    custom: t('interaction.events.custom')
  }
  return nameMap[event] || event
}

const getActionDisplayName = (action: InteractionActionType) => {
  const nameMap = {
    changeBackgroundColor: t('interaction.actions.changeBackgroundColor'),
    changeTextColor: t('interaction.actions.changeTextColor'),
    changeBorderColor: t('interaction.actions.changeBorderColor'),
    changeSize: t('interaction.actions.changeSize'),
    changeOpacity: t('interaction.actions.changeOpacity'),
    changeTransform: t('interaction.actions.changeTransform'),
    changeVisibility: t('interaction.actions.changeVisibility'),
    changeContent: t('interaction.actions.changeContent'),
    triggerAnimation: t('interaction.actions.triggerAnimation'),
    custom: t('interaction.actions.custom')
  }
  return nameMap[action] || action
}

const formatResponseValue = (response: InteractionResponse) => {
  const { action, value } = response

  switch (action) {
    case 'changeBackgroundColor':
    case 'changeTextColor':
    case 'changeBorderColor':
      return value
    case 'changeSize':
      if (typeof value === 'object' && value) {
        return `${value.width || '?'}×${value.height || '?'}`
      }
      return String(value)
    case 'changeOpacity':
      return `${Math.round((value as number) * 100)}%`
    case 'changeTransform':
      return String(value)
    case 'changeVisibility':
      return value === 'visible' ? t('interaction.visibility.visible') : t('interaction.visibility.hidden')
    case 'changeContent':
      return String(value).substring(0, 20) + (String(value).length > 20 ? '...' : '')
    case 'triggerAnimation':
      return String(value)
    case 'custom':
      try {
        return JSON.stringify(value).substring(0, 30) + '...'
      } catch {
        return String(value)
      }
    default:
      return String(value)
  }
}

// 预览相关方法
const handlePreviewEvent = (eventType: InteractionEventType) => {
  const matchingConfigs = props.template.config.filter(config => config.event === eventType && config.enabled)

  if (matchingConfigs.length === 0) return

  // 按优先级排序
  matchingConfigs.sort((a, b) => (b.priority || 0) - (a.priority || 0))

  matchingConfigs.forEach(config => {
    config.responses.forEach(response => {
      const delay = response.delay || 0
      setTimeout(() => {
        executePreviewResponse(response)
      }, delay)
    })
  })
}

const executePreviewResponse = (response: InteractionResponse) => {
  if (!previewElement.value) return

  const element = previewElement.value
  const { action, value, duration = 300, easing = 'ease' } = response

  // 设置过渡效果
  element.style.transition = `all ${duration}ms ${easing}`

  const setPreviewStyle = (property: string, styleValue: unknown) => {
    const normalizedValue = String(styleValue)
    const cssProperty = property.replace(/[A-Z]/g, match => `-${match.toLowerCase()}`)
    element.style.setProperty(cssProperty, normalizedValue)
    previewRuntimeStyles.value = {
      ...previewRuntimeStyles.value,
      [property]: normalizedValue
    }
  }

  switch (action) {
    case 'changeBackgroundColor':
      setPreviewStyle('backgroundColor', value)
      break
    case 'changeTextColor':
      setPreviewStyle('color', value)
      break
    case 'changeBorderColor':
      setPreviewStyle('borderStyle', 'solid')
      setPreviewStyle('borderTopStyle', 'solid')
      setPreviewStyle('borderRightStyle', 'solid')
      setPreviewStyle('borderBottomStyle', 'solid')
      setPreviewStyle('borderLeftStyle', 'solid')
      setPreviewStyle('borderColor', value)
      setPreviewStyle('borderTopColor', value)
      setPreviewStyle('borderRightColor', value)
      setPreviewStyle('borderBottomColor', value)
      setPreviewStyle('borderLeftColor', value)
      break
    case 'changeSize':
      if (typeof value === 'object' && value) {
        if (value.width) setPreviewStyle('width', `${value.width}px`)
        if (value.height) setPreviewStyle('height', `${value.height}px`)
      }
      break
    case 'changeOpacity':
      setPreviewStyle('opacity', value)
      break
    case 'changeTransform':
      setPreviewStyle('transform', value)
      break
    case 'changeVisibility':
      setPreviewStyle('visibility', value)
      break
    case 'changeContent':
      previewElementText.value = String(value)
      break
    case 'triggerAnimation':
      // 移除之前的动画
      element.style.animation = ''
      // 强制重排
      element.offsetHeight
      // 应用新动画
      setPreviewStyle('animation', `${value} ${duration}ms ${easing}`)
      break
    case 'custom':
      // 自定义动作，尝试应用为样式对象
      if (typeof value === 'object' && value) {
        for (const [property, styleValue] of Object.entries(value)) {
          setPreviewStyle(property, styleValue)
        }
      }
      break
  }
}

const resetPreviewElement = () => {
  if (!previewElement.value) return

  const element = previewElement.value

  // 重置所有样式
  element.style.cssText = ''
  element.className = 'preview-element'
  previewRuntimeStyles.value = {}
  previewElementText.value = t('interaction.template.previewTarget')

  // 恢复原始样式
  Object.assign(element.style, originalPreviewStyles.value)
}

const runAllPreviewInteractions = () => {
  // 依次触发所有事件类型
  const eventTypes: InteractionEventType[] = ['click', 'hover', 'focus', 'blur', 'custom']
  let delay = 0

  eventTypes.forEach(eventType => {
    const hasEvent = props.template.config.some(config => config.event === eventType && config.enabled)
    if (hasEvent) {
      setTimeout(() => {
        handlePreviewEvent(eventType)
      }, delay)
      delay += 1000 // 每个事件间隔1秒
    }
  })

  // 最后重置
  setTimeout(() => {
    resetPreviewElement()
  }, delay + 1000)
}

// 操作方法
const selectTemplate = () => {
  emit('select', props.template)
  message.success(t('interaction.messages.templateApplied'))
}

const exportTemplate = () => {
  try {
    const templateData = {
      name: props.template.name,
      description: props.template.description,
      category: props.template.category,
      config: props.template.config,
      tags: props.template.tags || [],
      exportedAt: new Date().toISOString()
    }

    const jsonString = JSON.stringify(templateData, null, 2)
    const blob = new Blob([jsonString], { type: 'application/json' })
    const url = URL.createObjectURL(blob)

    const link = document.createElement('a')
    link.href = url
    link.download = `${props.template.name}-template.json`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)

    message.success(t('interaction.messages.templateExported'))
  } catch (error) {
    message.error(t('interaction.messages.exportFailed'))
  }
}

// 生命周期
onMounted(() => {
  if (previewElement.value) {
    // 保存原始样式
    const computedStyles = window.getComputedStyle(previewElement.value)
    originalPreviewStyles.value = {
      backgroundColor: computedStyles.backgroundColor,
      color: computedStyles.color,
      borderColor: computedStyles.borderColor,
      opacity: computedStyles.opacity,
      transform: computedStyles.transform,
      visibility: computedStyles.visibility,
      width: computedStyles.width,
      height: computedStyles.height
    }
  }
})
</script>

<style scoped>
.interaction-template-preview {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-height: 70vh;
  overflow-y: auto;
}

.template-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.template-header {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.template-details {
  flex: 1;
}

.template-title {
  margin: 0 0 8px 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-color);
}

.template-description {
  margin: 0;
  color: var(--text-color-2);
  line-height: 1.5;
}

.template-stats {
  padding: 16px;
  background: var(--body-color);
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.config-details {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
}

.config-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 200px;
  overflow-y: auto;
}

.config-card {
  border: 1px solid var(--border-color);
}

.config-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.config-name {
  font-weight: 500;
  color: var(--text-color);
  font-size: 13px;
}

.responses-list {
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

.response-main {
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

.response-meta {
  text-align: right;
}

.preview-stage {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.preview-canvas {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 120px;
  padding: 20px;
  background: var(--body-color);
  border: 2px dashed var(--border-color);
  border-radius: 8px;
}

.preview-element {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 120px;
  height: 60px;
  padding: 12px;
  background: var(--card-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.3s ease;
  outline: none;
  font-size: 13px;
  color: var(--text-color);
}

.preview-element:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.preview-element:focus {
  outline: 2px solid var(--primary-color);
  outline-offset: 2px;
}

.preview-element:active {
  transform: translateY(0);
}

.preview-controls {
  display: flex;
  justify-content: center;
}

.template-actions {
  padding-top: 16px;
  border-top: 1px solid var(--border-color);
}

/* 响应式调整 */
@media (max-width: 768px) {
  .template-header {
    flex-direction: column;
    align-items: center;
    text-align: center;
  }

  .template-stats {
    padding: 12px;
  }

  .stat-item {
    font-size: 12px;
  }

  .preview-element {
    width: 100px;
    height: 50px;
    font-size: 12px;
  }
}

/* 滚动条样式 */
.interaction-template-preview::-webkit-scrollbar,
.config-list::-webkit-scrollbar {
  width: 6px;
}

.interaction-template-preview::-webkit-scrollbar-track,
.config-list::-webkit-scrollbar-track {
  background: var(--body-color);
  border-radius: 3px;
}

.interaction-template-preview::-webkit-scrollbar-thumb,
.config-list::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 3px;
}

.interaction-template-preview::-webkit-scrollbar-thumb:hover,
.config-list::-webkit-scrollbar-thumb:hover {
  background: var(--text-color-3);
}

/* 动画效果 */
.config-card {
  animation: slideIn 0.3s ease-out;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}
</style>
