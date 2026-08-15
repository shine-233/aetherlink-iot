<!--
  文件用途：PageTab 页签总装组件，对外提供统一的页签渲染入口。
  核心逻辑：根据 mode 切换 Chrome 风格或按钮风格页签，并统一注入插槽、关闭事件和主题 CSS 变量。
  主要逻辑：将 activeColor 转为 CSS 变量；过滤样式专用 props；默认在 suffix 插槽中渲染关闭按钮。
  关键注意事项：如果调用方自定义 suffix 插槽，会覆盖默认关闭按钮，需要自行处理关闭入口。
  重构建议：建议为 mode 切换、suffix 覆盖和关闭事件补充组件测试。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import type { PageTabMode, PageTabProps } from '../../types'
import { ACTIVE_COLOR, createTabCssVars } from './shared'
import ChromeTab from './chrome-tab.vue'
import ButtonTab from './button-tab.vue'
import SvgClose from './svg-close.vue'
import style from './index.module.css'

defineOptions({
  name: 'PageTab'
})

const props = withDefaults(defineProps<PageTabProps>(), {
  mode: 'chrome',
  commonClass: 'transition-all-300',
  activeColor: ACTIVE_COLOR,
  closable: true
})

interface Emits {
  (e: 'close'): void
}

const emit = defineEmits<Emits>()

type SlotFn = (props?: Record<string, unknown>) => any

type Slots = {
  /**
   * Slot
   *
   * The center content of the tab
   */
  default?: SlotFn
  /**
   * Slot
   *
   * The left content of the tab
   */
  prefix?: SlotFn
  /**
   * Slot
   *
   * The right content of the tab
   */
  suffix?: SlotFn
}

defineSlots<Slots>()

const activeTabComponent = computed(() => {
  const { mode, chromeClass, buttonClass } = props

  const tabComponentMap = {
    chrome: {
      component: ChromeTab,
      class: chromeClass
    },
    button: {
      component: ButtonTab,
      class: buttonClass
    }
  } satisfies Record<PageTabMode, { component: Component; class?: string }>

  return tabComponentMap[mode]
})

const cssVars = computed(() => createTabCssVars(props.activeColor))

const bindProps = computed(() => {
  const { chromeClass: _chromeCls, buttonClass: _btnCls, ...rest } = props

  return rest
})

function handleClose() {
  emit('close')
}
</script>

<template>
  <component :is="activeTabComponent.component" :class="activeTabComponent.class" :style="cssVars" v-bind="bindProps">
    <template #prefix>
      <slot name="prefix"></slot>
    </template>
    <slot></slot>
    <template #suffix>
      <slot name="suffix">
        <SvgClose v-if="closable" :class="[style['svg-close']]" @click="handleClose" />
      </slot>
    </template>
  </component>
</template>

<style scoped></style>
