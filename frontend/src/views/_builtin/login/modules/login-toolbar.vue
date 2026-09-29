<!--
  文件用途：登录卡片右上角工具栏（主题切换 + 语言循环切换）。
  核心逻辑：两个按钮共享同一套按钮样式；图标为内联 SVG（避免登录页额外拉取图标集）。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

const props = defineProps<{
  darkMode: boolean
  borderColor: string
  localeLabel: string
}>()

const emit = defineEmits<{
  (event: 'toggle-theme'): void
  (event: 'cycle-locale'): void
}>()

const buttonStyle = computed(() => ({
  background: props.darkMode ? '#374151' : '#f9fafb',
  border: `1px solid ${props.borderColor}`,
  color: props.darkMode ? '#d1d5db' : '#6b7280'
}))

const MOON_PATH = 'M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z'
const SUN_PATH =
  'M12 2.25a.75.75 0 01.75.75v2.25a.75.75 0 01-1.5 0V3a.75.75 0 01.75-.75zM7.5 12a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM18.894 6.166a.75.75 0 00-1.06-1.06l-1.591 1.59a.75.75 0 101.06 1.061l1.591-1.59zM21.75 12a.75.75 0 01-.75.75h-2.25a.75.75 0 010-1.5H21a.75.75 0 01.75.75zM17.834 18.894a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 10-1.061 1.06l1.59 1.591zM12 18a.75.75 0 01.75.75V21a.75.75 0 01-1.5 0v-2.25A.75.75 0 0112 18zM7.758 17.303a.75.75 0 00-1.061-1.06l-1.591 1.59a.75.75 0 001.06 1.061l1.591-1.59zM6 12a.75.75 0 01-.75.75H3a.75.75 0 010-1.5h2.25A.75.75 0 016 12zM6.697 7.757a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 00-1.061 1.06l1.59 1.591z'
const LOCALE_PATH =
  'M12.87 15.07l-2.54-2.51.03-.03c1.74-1.94 2.98-4.17 3.71-6.53H17V4h-7V2H8v2H1v1.99h11.17C11.5 7.92 10.44 9.75 9 11.35 8.07 10.32 7.3 9.19 6.69 8h-2c.73 1.63 1.73 3.17 2.98 4.56l-5.09 5.02L4 19l5-5 3.11 3.11.76-2.04zM18.5 10h-2L12 22h2l1.12-3h4.75L21 22h2l-4.5-12zm-2.62 7l1.62-4.33L19.12 17h-3.24z'
const toolbarButtonClass =
  'flex items-center gap-1 px-2 py-1.5 text-xs rounded-lg border transition-all duration-200 hover:scale-105'
</script>

<template>
  <div class="flex justify-end gap-2 mb-4">
    <button
      type="button"
      :class="toolbarButtonClass"
      :style="buttonStyle"
      :aria-label="$t('icon.themeSchema')"
      @click="emit('toggle-theme')"
    >
      <svg class="w-3 h-3 fill-current" viewBox="0 0 24 24" aria-hidden="true">
        <path :d="darkMode ? MOON_PATH : SUN_PATH" />
      </svg>
    </button>
    <button type="button" :class="toolbarButtonClass" :style="buttonStyle" @click="emit('cycle-locale')">
      <svg class="w-3 h-3 fill-current" viewBox="0 0 24 24" aria-hidden="true">
        <path :d="LOCALE_PATH" />
      </svg>
      <span>{{ localeLabel }}</span>
    </button>
  </div>
</template>
