<!--
  文件用途：按钮风格 PageTab 子组件，负责紧凑边框页签的结构和状态类名。
  核心逻辑：渲染 prefix、default、suffix 三个插槽，并根据 active、darkMode 切换 CSS Modules 样式。
  主要逻辑：组件只负责展示，不处理关闭事件；关闭按钮由上层 PageTab 默认插槽注入。
  关键注意事项：新增 props 时需同步检查 PageTabProps，避免子组件接收无意义字段。
  重构建议：建议为 active/dark/prefix/suffix 组合补充轻量组件测试。
-->
<script setup lang="ts">
import type { PageTabProps } from '../../types'
import style from './index.module.css'

defineOptions({
  name: 'ButtonTab'
})

defineProps<PageTabProps>()

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
</script>

<template>
  <div
    class=":soy: relative inline-flex cursor-pointer items-center justify-center gap-12px whitespace-nowrap border-1px rounded-4px border-solid px-12px py-4px"
    :class="[
      style['button-tab'],
      { [style['button-tab_dark']]: darkMode },
      { [style['button-tab_active']]: active },
      { [style['button-tab_active_dark']]: active && darkMode }
    ]"
  >
    <slot name="prefix"></slot>
    <slot></slot>
    <slot name="suffix"></slot>
  </div>
</template>

<style scoped></style>
