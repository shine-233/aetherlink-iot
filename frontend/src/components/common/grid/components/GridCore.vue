<!--
  文件用途：适配 grid-layout-plus 的核心渲染组件，负责真实网格和网格项渲染。
  核心逻辑：把内部布局传给 GridLayout/GridItem，统一处理拖拽、缩放、断点和容器事件转发。
  关键注意事项：事件参数顺序和 v-model:layout 同步是上层组件契约，改动容易造成布局回写异常。
  重构建议：可把事件转发映射抽成独立函数，并为第三方组件升级保留适配测试。
-->
<!--
  Grid 核心组件
  负责网格布局的核心逻辑和渲染
-->
<template>
  <!-- 这里仍然通过局部副本承接 v-model，避免第三方组件直接改写上层 props。 -->
  <GridLayout
    v-model:layout="internalLayout"
    :col-num="config.colNum"
    :row-height="config.rowHeight"
    :is-draggable="!readonly && config.isDraggable && !config.staticGrid"
    :is-resizable="!readonly && config.isResizable && !config.staticGrid"
    :is-mirrored="config.isMirrored"
    :auto-size="config.autoSize"
    :vertical-compact="config.verticalCompact"
    :margin="config.margin"
    :use-css-transforms="config.useCssTransforms"
    :responsive="config.responsive"
    :breakpoints="config.breakpoints as Breakpoints"
    :cols="config.cols as Breakpoints"
    :prevent-collision="config.preventCollision"
    :use-style-cursor="config.useStyleCursor"
    :restore-on-drag="config.restoreOnDrag"
    @layout-created="handleLayoutCreated"
    @layout-before-mount="handleLayoutBeforeMount"
    @layout-mounted="handleLayoutMounted"
    @layout-updated="handleLayoutUpdated"
    @layout-ready="handleLayoutReady"
    @update:layout="handleLayoutChange"
    @breakpoint-changed="handleBreakpointChanged"
    @container-resized="handleContainerResized"
    @item-resize="handleItemResize"
    @item-resized="handleItemResized"
    @item-move="handleItemMove"
    @item-moved="handleItemMoved"
  >
    <GridItem
      v-for="item in internalLayout"
      :key="item.i"
      :x="item.x"
      :y="item.y"
      :w="item.w"
      :h="item.h"
      :i="item.i"
      :min-w="item.minW"
      :min-h="item.minH"
      :max-w="item.maxW"
      :max-h="item.maxH"
      :is-draggable="!readonly && item.isDraggable !== false && !item.static"
      :is-resizable="!readonly && item.isResizable !== false && !item.static"
      :static="item.static"
      :drag-ignore-from="item.dragIgnoreFrom"
      :drag-allow-from="item.dragAllowFrom"
      :resize-ignore-from="item.resizeIgnoreFrom"
      :preserve-aspect-ratio="item.preserveAspectRatio"
      :drag-option="item.dragOption"
      :resize-option="item.resizeOption"
      @resize="(i, newH, newW, newHPx, newWPx) => handleItemResize(i, newH, newW, newHPx, newWPx)"
      @resized="(i, newH, newW, newHPx, newWPx) => handleItemResized(i, newH, newW, newHPx, newWPx)"
      @move="(i, newX, newY) => handleItemMove(i, newX, newY)"
      @moved="(i, newX, newY) => handleItemMoved(i, newX, newY)"
      @container-resized="(i, newH, newW, newHPx, newWPx) => handleItemContainerResized(i, newH, newW, newHPx, newWPx)"
    >
      <GridItemContent :item="item" :readonly="readonly" :show-title="showTitle" :content-padding="contentPadding">
        <template #default="{ item: slotItem }">
          <slot :item="slotItem">
            <!-- 回退到默认内容 -->
          </slot>
        </template>
      </GridItemContent>
    </GridItem>
  </GridLayout>
</template>

<script setup lang="ts">
/**
 * Grid 核心组件。
 * 这里只做三件事：桥接 grid-layout-plus、维护布局副本、把底层事件稳定地转发给上层。
 */

import { shallowRef, watch } from 'vue'
import { GridLayout, GridItem } from 'grid-layout-plus'
import type { Breakpoints } from 'grid-layout-plus'
import GridItemContent from './GridItemContent.vue'
import type { GridLayoutPlusConfig, GridLayoutPlusItem, GridLayoutPlusEmits } from '../gridLayoutPlusTypes'

interface Props {
  /** 网格布局数据 */
  layout: GridLayoutPlusItem[]
  /** 网格配置 */
  config: GridLayoutPlusConfig
  /** 是否只读模式 */
  readonly?: boolean
  /** 是否显示网格项标题 */
  showTitle?: boolean
  /** 是否保留默认内容内边距 */
  contentPadding?: boolean
}

interface Emits extends GridLayoutPlusEmits {}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  showTitle: false,
  contentPadding: true
})

const emit = defineEmits<Emits>()

// 内部布局状态与 props.layout 解耦，避免第三方组件在拖拽过程中直接回写父层数据。
const internalLayout = shallowRef<GridLayoutPlusItem[]>([...props.layout])

// 上层替换 layout 时，这里同步刷新局部副本；保留深监听以兼容原地更新场景。
watch(
  () => props.layout,
  (newLayout) => {
    internalLayout.value = [...newLayout]
  },
  { deep: true }
)

// 以下处理函数只负责“原样转发”事件，不在这里叠加业务逻辑，便于定位布局问题。
// item 级事件继续原样透传，调用方依赖当前参数顺序做状态同步。
const handleLayoutCreated = (event: any) => {
  emit('layout-created', event)
}

const handleLayoutBeforeMount = (event: any) => {
  emit('layout-before-mount', event)
}

const handleLayoutMounted = (event: any) => {
  emit('layout-mounted', event)
}

const handleLayoutUpdated = (event: any) => {
  emit('layout-updated', event)
}

const handleLayoutReady = (event: any) => {
  emit('layout-ready', event)
}

const handleLayoutChange = (newLayout: GridLayoutPlusItem[]) => {
  // update:layout 是与第三方组件保持同步的第一落点，先更新副本，再通知上层。
  internalLayout.value = newLayout
  emit('layout-change', newLayout)
}

const handleBreakpointChanged = (breakpoint: string, layout: GridLayoutPlusItem[]) => {
  emit('breakpoint-changed', breakpoint, layout)
}

const handleContainerResized = (width: number, height: number, cols: number) => {
  emit('container-resized', width, height, cols)
}

const handleItemResize = (i: string, newH: number, newW: number, newHPx: number, newWPx: number) => {
  emit('item-resize', i, newH, newW, newHPx, newWPx)
}

const handleItemResized = (i: string, newH: number, newW: number, newHPx: number, newWPx: number) => {
  emit('item-resized', i, newH, newW, newHPx, newWPx)
}

const handleItemMove = (i: string, newX: number, newY: number) => {
  emit('item-move', i, newX, newY)
}

const handleItemMoved = (i: string, newX: number, newY: number) => {
  emit('item-moved', i, newX, newY)
}

const handleItemContainerResized = (i: string, newH: number, newW: number, newHPx: number, newWPx: number) => {
  emit('item-container-resized', i, newH, newW, newHPx, newWPx)
}

// 暴露内部布局，供外层容器在必要时读取当前拖拽后的即时状态。
defineExpose({
  internalLayout
})
</script>

<style scoped>
/* Grid 核心样式将继承父组件的样式 */
</style>
