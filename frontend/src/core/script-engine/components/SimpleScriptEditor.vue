<!--
  文件用途：提供基于 CodeMirror 6 的轻量脚本编辑器组件。
  核心逻辑：接收 v-model 脚本内容，配置 JavaScript 编辑能力，并适配主题与占位提示。
  关键注意事项：组件只负责编辑体验，不应在这里加入脚本执行或业务副作用。
  重构建议：可抽出 CodeMirror 配置工厂，方便多个脚本编辑场景共享。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, defineComponent, h, onMounted, shallowRef } from 'vue'
import { useThemeStore } from '@/store/modules/theme'
import { useI18n } from 'vue-i18n'

interface Props {
  /** 脚本内容 */
  modelValue?: string
  /** 编辑器占位符 */
  placeholder?: string
  /** 是否显示模板选择 */
  showTemplates?: boolean
  /** 模板类别过滤 */
  templateCategory?: string
  /** 编辑器高度 */
  height?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  placeholder: '请输入JavaScript脚本...',
  showTemplates: true,
  height: '200px'
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

// 国际化集成
const { t } = useI18n()

// 主题系统集成
const themeStore = useThemeStore()

// 编辑器分块加载失败时保留基础输入能力，不让脚本配置页变成空白。
const PlainTextEditor = defineComponent({
  name: 'PlainTextEditor',
  inheritAttrs: false,
  props: {
    modelValue: { type: String, default: '' },
    placeholder: { type: String, default: '' }
  },
  emits: ['update:modelValue'],
  setup(fallbackProps, { attrs, emit: emitFallback }) {
    return () => h('textarea', {
      class: 'plain-text-editor',
      'data-editor-fallback': 'plain-text',
      value: fallbackProps.modelValue,
      placeholder: fallbackProps.placeholder,
      style: attrs.style,
      onInput: (event: Event) => {
        emitFallback('update:modelValue', (event.target as HTMLTextAreaElement).value)
      }
    })
  }
})

const CodeMirror = defineAsyncComponent({
  loader: () => import('vue-codemirror6').then(module => module.default),
  errorComponent: PlainTextEditor,
  delay: 0
})
const editorLanguage = shallowRef<unknown>()

onMounted(async () => {
  try {
    const { javascript } = await import('@codemirror/lang-javascript')
    editorLanguage.value = javascript()
  } catch {
    // JavaScript language extension is optional for editing; CodeMirror remains usable as plain text.
    editorLanguage.value = undefined
  }
})

type CodeTemplateSnippet = {
  labelKey: string
  code: string
}

// Built-in script snippets for common IoT data handling tasks.
const codeTemplateSnippets: Record<string, CodeTemplateSnippet[]> = {
  'data-generation': [
    {
      labelKey: 'script.template.generateRandomData',
      code: `return {
  device_id: 'replace_with_device_id',
  online: true,
  metrics: {
    temperature: Math.round((20 + Math.random() * 8) * 10) / 10,
    humidity: Math.round((45 + Math.random() * 20) * 10) / 10,
    rssi: -45 - Math.round(Math.random() * 15)
  },
  alarm_count: 0,
  timestamp: Date.now()
}`
    },
    {
      labelKey: 'script.template.generateTimeSeries',
      code: `return Array.from({ length: 10 }, (_, i) => ({
  time: Date.now() + i * 1000,
  device_id: 'replace_with_device_id',
  temperature: Math.round((24 + Math.sin(i / 2) * 2 + Math.random()) * 10) / 10,
  humidity: Math.round((55 + Math.random() * 5) * 10) / 10
}))`
    }
  ],
  'data-processing': [
    {
      labelKey: 'script.template.dataFiltering',
      code: `return data.filter(item =>
  item.online === false ||
  item.alarm_count > 0 ||
  item.metrics?.temperature > 35
)`
    },
    {
      labelKey: 'script.template.dataTransformation',
      code: `return data.map(item => ({
  device_id: item.device_id,
  online_text: item.online ? '在线' : '离线',
  temperature: item.metrics?.temperature,
  humidity: item.metrics?.humidity,
  alarm_count: item.alarm_count || 0,
  checked_at: Date.now()
}))`
    }
  ],
  'data-merger': [
    {
      labelKey: 'script.template.mergeAsObject',
      code: `return items.reduce((acc, item, index) => {
  const deviceId = item.device_id || \`device_\${index + 1}\`
  acc[deviceId] = item
  return acc
}, {})`
    },
    {
      labelKey: 'script.template.mergeAsArray',
      code: `return items.flat()`
    }
  ],
  'http-pre-request': [
    {
      labelKey: 'script.template.fillDeviceRequestParams',
      code: `const deviceId = data?.device_id || data?.deviceId || 'replace_with_device_id'

return {
  ...data,
  device_id: deviceId,
  include_latest: true,
  request_source: 'iot-console',
  requested_at: Date.now()
}`
    },
    {
      labelKey: 'script.template.buildLatestTelemetryQuery',
      code: `return {
  ...data,
  fields: ['temperature', 'humidity', 'rssi', 'online'],
  window: 'latest',
  limit: 1
}`
    }
  ]
}

const availableTemplateSnippets = computed(() => {
  if (props.templateCategory && codeTemplateSnippets[props.templateCategory]) {
    return codeTemplateSnippets[props.templateCategory]
  }
  return Object.values(codeTemplateSnippets).flat()
})

const templateOptions = computed(() =>
  availableTemplateSnippets.value.map(snippet => ({
    label: t(snippet.labelKey),
    value: snippet.code
  }))
)

/**
 * 应用选中的模板
 */
const applyTemplate = (templateCode: string) => {
  if (templateCode) {
    emit('update:modelValue', templateCode)
  }
}

// CodeMirror 6 配置
const editorValue = computed({
  get: () => props.modelValue,
  set: (value: string) => emit('update:modelValue', value)
})
</script>

<template>
  <div class="simple-script-editor">
    <!-- 模板选择器 -->
    <div v-if="showTemplates && templateOptions.length > 0" class="template-selector">
      <span class="mr-4">JavaScript 处理:</span>

      <n-select
        :options="templateOptions"
        :placeholder="t('script.selectTemplate')"
        size="small"
        style="width: 240px"
        clearable
        @update:value="applyTemplate"
      />
      <n-popover trigger="hover" placement="top">
        <template #trigger>
          <span class="help-icon">❓</span>
        </template>
        <div>
          <p>对数据进行自定义转换</p>
          <p>
            可用变量:
            <code>data</code>
            (输入数据)
          </p>
          <p>
            必须:
            <code>return</code>
            返回处理后的数据
          </p>
          <p>留空表示不处理</p>
        </div>
      </n-popover>
    </div>

    <!-- CodeMirror 6 编辑器 -->
    <div class="editor-container">
      <CodeMirror
        v-model="editorValue"
        basic
        :dark="themeStore.darkMode"
        :lang="editorLanguage"
        :placeholder="props.placeholder"
        :style="{ height: props.height }"
      />
    </div>
  </div>
</template>

<style scoped>
.simple-script-editor {
  width: 100%;
  display: flex;
  flex-direction: column;

  gap: 8px;
}

.template-selector {
  display: flex;
  align-items: center;
}

.editor-container {
  flex: 1;
  border: 1px solid var(--n-border-color);
  border-radius: var(--n-border-radius);
  overflow: hidden;
  transition: all 0.3s var(--n-bezier);
}

.editor-container:focus-within {
  border-color: var(--n-color-primary);
  box-shadow: 0 0 0 2px var(--n-color-primary-hover-opacity);
}

.editor-hint {
  font-size: 12px;
  color: var(--n-text-color-disabled);
  text-align: center;
}

/* CodeMirror 6 样式定制 */
.simple-script-editor :deep(.cm-editor) {
  border: none;
  border-radius: var(--n-border-radius);
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  background: transparent;
  height: 100%;
}

.simple-script-editor :deep(.cm-focused) {
  outline: none;
}

.simple-script-editor :deep(.cm-content) {
  min-height: v-bind(height);
  line-height: 1.6;
  caret-color: var(--n-color-primary);
  padding: 12px;
}

.simple-script-editor :deep(.cm-gutters) {
  background: var(--n-color-base);
  border-right: 1px solid var(--n-border-color);
  color: var(--n-text-color-disabled);
}

.simple-script-editor :deep(.cm-lineNumbers .cm-gutterElement) {
  color: var(--n-text-color-disabled);
  padding: 0 8px;
  font-size: 12px;
}

.simple-script-editor :deep(.cm-selectionBackground) {
  background: rgba(24, 160, 88, 0.2) !important;
}

.simple-script-editor :deep(.cm-activeLine) {
  background: var(--n-color-hover);
}

.simple-script-editor :deep(.cm-activeLineGutter) {
  background: var(--n-color-hover);
}

/* 滚动条样式 */
.simple-script-editor :deep(.cm-scroller::-webkit-scrollbar) {
  width: 6px;
  height: 6px;
}

.simple-script-editor :deep(.cm-scroller::-webkit-scrollbar-track) {
  background: var(--n-color-base);
}

.simple-script-editor :deep(.cm-scroller::-webkit-scrollbar-thumb) {
  background: var(--n-scrollbar-color);
  border-radius: 3px;
}

.simple-script-editor :deep(.cm-scroller::-webkit-scrollbar-thumb:hover) {
  background: var(--n-scrollbar-color-hover);
}

/* 占位符样式 */
.simple-script-editor :deep(.cm-placeholder) {
  color: var(--n-text-color-disabled);
  font-style: italic;
}

/* 语法高亮定制 */
.simple-script-editor :deep(.cm-editor.cm-focused .cm-selectionBackground) {
  background: rgba(24, 160, 88, 0.2) !important;
}

/* 响应主题变化 */
[data-theme='dark'] .simple-script-editor .editor-container {
  box-shadow: var(--n-box-shadow-1);
}

[data-theme='light'] .simple-script-editor .editor-container {
  background: var(--n-card-color);
}
</style>
