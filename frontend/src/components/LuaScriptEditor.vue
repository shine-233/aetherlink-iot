<!--
  Lightweight script editor compatible with the previous script-editor wrapper API.
  It keeps the external value/options/exposed-method contract while using the
  existing CodeMirror dependency instead of loading Monaco at runtime.
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    value?: string
    height?: number | string
    language?: string
    options?: Record<string, any>
  }>(),
  {
    value: '',
    height: 300,
    language: 'lua',
    options: () => ({})
  }
)

const emit = defineEmits<{
  'update:value': [val: string]
}>()

const editorHostRef = ref<HTMLElement | null>(null)

// 项目未安装 Lua language extension；使用 CodeMirror 纯文本模式，
// 不以 JavaScript 高亮冒充 Lua 语义，也不增加额外部署依赖。
const CodeMirror = defineAsyncComponent(() => import('vue-codemirror6').then(module => module.default))

const editorValue = computed({
  get: () => props.value,
  set: (value: string) => emit('update:value', value)
})

const editorHeight = computed(() => {
  const h = props.height
  return typeof h === 'number' ? `${h}px` : /^\d+$/.test(String(h)) ? `${h}px` : String(h)
})

const editorStyle = computed(() => ({
  height: editorHeight.value,
  fontSize: `${props.options?.fontSize ?? 14}px`,
  lineHeight: `${props.options?.lineHeight ?? 20}px`,
  fontFamily: props.options?.fontFamily ?? 'Consolas, "Courier New", monospace'
}))

const wordWrapClass = computed(() => (props.options?.wordWrap === 'off' ? 'is-nowrap' : 'is-wrap'))

const focusEditor = () => {
  const editor = editorHostRef.value?.querySelector<HTMLElement>('.cm-content')
  editor?.focus()
}

defineExpose({
  getAction(_id: string) {
    // 保留旧编辑器查询接口，但明确表示本地纯文本模式不支持格式化等 Monaco action。
    return undefined
  },
  getValue() {
    return props.value ?? ''
  },
  setValue(val: string) {
    emit('update:value', val)
  },
  focus() {
    focusEditor()
  }
})
</script>

<template>
  <div ref="editorHostRef" class="monaco-lua-editor" :class="wordWrapClass">
    <CodeMirror
      v-model="editorValue"
      basic
      :style="editorStyle"
      :disabled="Boolean(options?.readOnly)"
    />
  </div>
</template>

<style scoped>
.monaco-lua-editor {
  width: 100%;
  min-height: v-bind(editorHeight);
  text-align: left;
}

.monaco-lua-editor :deep(.cm-editor) {
  height: v-bind(editorHeight);
  border: none;
  font-family: inherit;
  background: #fff;
}

.monaco-lua-editor :deep(.cm-scroller) {
  font-family: inherit;
}

.monaco-lua-editor :deep(.cm-content) {
  min-height: v-bind(editorHeight);
}

.monaco-lua-editor.is-wrap :deep(.cm-line) {
  white-space: pre-wrap;
  word-break: break-word;
}

.monaco-lua-editor.is-nowrap :deep(.cm-line) {
  white-space: pre;
}
</style>
