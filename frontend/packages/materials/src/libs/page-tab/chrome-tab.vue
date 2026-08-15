<!--
  文件用途：Chrome 风格 PageTab 子组件，提供类似浏览器标签页的外观。
  核心逻辑：组合 SVG 背景、内容插槽、关闭区域和右侧分割线。
  主要逻辑：根据 active、darkMode 切换 CSS Modules 状态类，由背景组件绘制页签轮廓。
  关键注意事项：该组件带有负右边距和绝对定位背景，修改布局时需同步检查相邻页签重叠效果。
  重构建议：建议用视觉回归或快照测试覆盖 active/dark/相邻页签组合。
-->
<script setup lang="ts">
import type { PageTabProps } from '../../types'
import ChromeTabBg from './chrome-tab-bg.vue'
import style from './index.module.css'

defineOptions({
  name: 'ChromeTab'
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
    class=":soy: relative inline-flex cursor-pointer items-center justify-center gap-16px whitespace-nowrap px-24px py-6px -mr-18px"
    :class="[
      style['chrome-tab'],
      { [style['chrome-tab_dark']]: darkMode },
      { [style['chrome-tab_active']]: active },
      { [style['chrome-tab_active_dark']]: active && darkMode }
    ]"
  >
    <div class=":soy: pointer-events-none absolute left-0 top-0 h-full w-full -z-1" :class="[style['chrome-tab__bg']]">
      <ChromeTabBg />
    </div>
    <slot name="prefix"></slot>
    <slot></slot>
    <slot name="suffix"></slot>
    <div class=":soy: absolute right-7px h-16px w-1px bg-#1f2225" :class="[style['chrome-tab-divider']]"></div>
  </div>
</template>

<style scoped></style>
