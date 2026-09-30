<!--
  Dependency boundary: this entry still depends on grid-layout-plus behavior; drag package cleanup needs caller-level evidence first.
  文件用途：提供增强版 Grid Layout Plus 对外组件，承载可拖拽、可缩放、响应式网格布局。
  核心逻辑：归一化 layout 与配置，连接 useGridLayoutPlus 状态，并把 GridCore 的生命周期和交互事件继续向外转发。
  关键注意事项：事件名、插槽参数和 readonly/static 行为是外部页面契约，改动前需同步检查调用方。
  重构建议：后续可把配置归一化、事件转发和导入导出逻辑拆成更薄的适配层，降低单组件维护成本。
-->
<template>
  <div
    class="grid-layout-plus-wrapper grid-background-base"
    :style="containerStyle"
    :class="[
      containerClass,
      {
        readonly: readonly,
        'dark-theme': isDarkTheme,
        'show-grid': showGrid && !readonly
      }
    ]"
  >
    <!-- 网格核心组件 -->
    <GridCore
      ref="gridCoreRef"
      :layout="normalizedLayout"
      :config="config"
      :readonly="readonly"
      :show-title="showTitle"
      :content-padding="contentPadding"
      @layout-created="handleLayoutCreated"
      @layout-before-mount="handleLayoutBeforeMount"
      @layout-mounted="handleLayoutMounted"
      @layout-updated="handleLayoutUpdated"
      @layout-ready="handleLayoutReady"
      @layout-change="handleLayoutChange"
      @breakpoint-changed="handleBreakpointChanged"
      @container-resized="handleContainerResized"
      @item-resize="handleItemResize"
      @item-resized="handleItemResized"
      @item-move="handleItemMove"
      @item-moved="handleItemMoved"
      @item-container-resized="handleItemContainerResized"
    >
      <template #default="{ item }">
        <slot :item="item">
          <!-- 默认内容会由 GridItemContent 处理 -->
        </slot>
      </template>
    </GridCore>

    <!-- 拖拽区域组件 -->
    <GridDropZone
      :readonly="readonly"
      :show-drop-zone="showDropZone"
      @drag-enter="handleDragEnter"
      @drag-over="handleDragOver"
      @drag-leave="handleDragLeave"
      @drop="handleDrop"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useThemeStore } from '@/store/modules/theme'
import { GridCore, GridDropZone } from './components'
import type {
  GridLayoutPlusConfig,
  GridLayoutPlusItem,
  GridLayoutPlusEmits,
  GridLayoutPlusProps
} from './gridLayoutPlusTypes'
import { normalizeLayout, withIdKey as applyIdKeyAlias } from './gridLayoutPlusIdKey'
import { useGridLayoutPlusConfig } from './useGridLayoutPlusConfig'
import { useGridLayoutPlusItems } from './useGridLayoutPlusItems'

// Props
interface Props extends GridLayoutPlusProps {
  /** 网格尺寸预设 */
  gridSize?: 'mini' | 'standard' | 'large' | 'mega' | 'extended' | 'custom'
  /** 自定义列数（当 gridSize 为 'custom' 时使用） */
  customColumns?: number
}

const props = withDefaults(defineProps<Props>(), {
  layout: () => [],
  readonly: false,
  showGrid: true,
  showDropZone: false,
  showTitle: false, // 默认不显示标题
  contentPadding: true,
  config: () => ({}),
  gridSize: 'standard', // 默认使用标准网格 (24列)
  customColumns: 50,
  /** 唯一键字段名，默认使用 'i'。允许外部数据结构重命名主键（如 'id'） */
  idKey: 'i'
})

// Emits
interface Emits extends GridLayoutPlusEmits {}

const emit = defineEmits<Emits>()

// Store
const themeStore = useThemeStore()

// 组件引用
const gridCoreRef = ref<InstanceType<typeof GridCore> | null>(null)

// 计算属性：根据 idKey 规范化布局，确保每个项都有 item.i
const normalizedLayout = computed<GridLayoutPlusItem[]>(() => normalizeLayout(props.layout || [], props.idKey || 'i'))

// Computed
const isDarkTheme = computed(() => themeStore.darkMode)
const containerStyle = computed(() => props.containerStyle || {})
const containerClass = computed(() => props.containerClass || '')

// 网格尺寸预设解析与校验（拆分到 useGridLayoutPlusConfig）
const { config, gridValidation } = useGridLayoutPlusConfig(props)

const withIdKey = (items: GridLayoutPlusItem[]): GridLayoutPlusItem[] => {
  return applyIdKeyAlias(items, props.idKey || 'i')
}

// 网格项增删改查与布局优化（拆分到 useGridLayoutPlusItems）
const { addItem, removeItem, updateItem, clearLayout, getItem, getAllItems, getLayout, optimizeLayoutForGridSize } =
  useGridLayoutPlusItems({
    gridCoreRef,
    config,
    idKey: () => props.idKey || 'i',
    onItemAdd: (item) => emit('item-add', item),
    onItemDelete: (itemId) => emit('item-delete', itemId),
    onItemUpdate: (itemId, updates) => emit('item-update', itemId, updates),
    onLayoutChange: (layout) => {
      emit('layout-change', layout)
      emit('update:layout', layout)
    }
  })

// 业务方法
const handleItemEdit = (item: GridLayoutPlusItem) => {
  emit('item-edit', withIdKey([item])[0])
}

const handleItemDelete = (item: GridLayoutPlusItem) => {
  // 通过 GridCore 组件处理删除逻辑
  const coreLayout = gridCoreRef.value?.internalLayout
  if (coreLayout) {
    const index = coreLayout.findIndex((i) => i.i === item.i)
    if (index > -1) {
      coreLayout.splice(index, 1)
      emit('item-delete', item.i)
    }
  }
}

const handleItemDataUpdate = (itemId: string, data: any) => {
  // 通过 GridCore 组件处理数据更新
  const coreLayout = gridCoreRef.value?.internalLayout
  if (coreLayout) {
    const item = coreLayout.find((i) => i.i === itemId)
    if (item) {
      item.data = { ...item.data, ...data }
      emit('item-data-update', itemId, data)
    }
  }
}

// Grid Layout Plus 事件处理
const handleLayoutCreated = (newLayout: GridLayoutPlusItem[]) => {
  // 统一对外布局协议：补齐 idKey 别名字段
  emit('layout-created', withIdKey(newLayout))
}

const handleLayoutBeforeMount = (newLayout: GridLayoutPlusItem[]) => {
  emit('layout-before-mount', withIdKey(newLayout))
}

const handleLayoutMounted = (newLayout: GridLayoutPlusItem[]) => {
  emit('layout-mounted', withIdKey(newLayout))
}

const handleLayoutUpdated = (newLayout: GridLayoutPlusItem[]) => {
  emit('layout-updated', withIdKey(newLayout))
}

const handleLayoutReady = (newLayout: GridLayoutPlusItem[]) => {
  emit('layout-ready', withIdKey(newLayout))
}

const handleLayoutChange = (newLayout: GridLayoutPlusItem[]) => {
  // 由 GridCore 组件处理布局变化，主组件只负责转发事件
  const patched = withIdKey(newLayout)
  emit('layout-change', patched)
  emit('update:layout', patched)
}

const handleBreakpointChanged = (newBreakpoint: string, newLayout: GridLayoutPlusItem[]) => {
  emit('breakpoint-changed', newBreakpoint, withIdKey(newLayout))
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

// 拖拽事件处理 - 委托给 GridDropZone 组件
const handleDragEnter = (e: DragEvent) => {
  emit('drag-enter', e)
}

const handleDragOver = (e: DragEvent) => {
  emit('drag-over', e)
}

const handleDragLeave = (e: DragEvent) => {
  emit('drag-leave', e)
}

const handleDrop = (e: DragEvent) => {
  const componentType = e.dataTransfer?.getData('text/plain')
  if (componentType) {
    addItem(componentType)
  }
  emit('drop', e)
}

// 布局数据监听已移至 GridCore 组件处理

// 暴露 API 方法给父组件
defineExpose({
  addItem,
  removeItem,
  updateItem,
  clearLayout,
  getItem,
  getAllItems,
  getLayout,
  // 🔥 新增：网格扩展相关API
  getGridInfo: () => ({
    colNum: config.value.colNum,
    gridSize: props.gridSize,
    validation: gridValidation.value
  }),
  optimizeLayoutForGridSize,
  getGridValidation: () => gridValidation.value,
  // 暴露子组件引用以便高级操作
  gridCore: gridCoreRef
})
</script>

<style scoped>
.grid-layout-plus-wrapper {
  position: relative;
  width: 100%;
  height: 100%; /* 🔧 恢复高度100%以支持栅格容器中的高度自适应 */
}

/* 网格项内容 */
.grid-item-content {
  height: 100%;
  /* 🔧 移除默认样式，避免与NodeWrapper base配置冲突 */
  background: transparent;
  border: none;
  border-radius: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  /* 🔧 移除默认阴影和过渡，由内部组件控制 */
  transition: none;
}

.dark-theme .grid-item-content {
  /* 🔧 移除暗主题默认样式，避免与NodeWrapper配置冲突 */
  background: transparent;
  border-color: transparent;
  color: inherit;
}

.grid-item-content:hover {
  /* 🔧 移除hover效果，避免与NodeWrapper配置冲突 */
  /* box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15); */
  /* transform: translateY(-1px); */
}

/* 项目头部 */
.grid-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f8f9fa;
  border-bottom: 1px solid #e1e5e9;
  font-size: 14px;
  font-weight: 500;
}

.dark-theme .grid-item-header {
  background: #3a3a3a;
  border-bottom-color: #404040;
}

.grid-item-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.grid-item-actions {
  display: flex;
  gap: 4px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #6c757d;
  cursor: pointer;
  transition: all 0.2s ease;
}

.action-btn:hover {
  background: #e9ecef;
  color: #495057;
}

.dark-theme .action-btn:hover {
  background: #4a4a4a;
  color: white;
}

.delete-btn:hover {
  background: #dc3545;
  color: white;
}

/* 项目内容 */
.grid-item-body {
  flex: 1;
  padding: 0; /* 🔧 移除默认内边距，由内部组件控制 */
  overflow: visible; /* 移除 overflow: auto，让内容自然溢出 */
  /* 🔧 移除默认背景，避免与NodeWrapper配置冲突 */
  background: transparent;
  /* 🔧 确保内部组件样式能够正常显示 */
  border: none;
  border-radius: inherit;
}

.default-item-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #6c757d;
  text-align: center;
}

.item-type {
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 4px;
}

.item-id {
  font-size: 12px;
  opacity: 0.7;
}

/* 拖拽区域 */
.drop-zone {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  border: 2px dashed #ddd;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.9);
  opacity: 0;
  pointer-events: none;
  transition: all 0.3s ease;
  z-index: 1000;
}

.drop-zone.dragging {
  opacity: 1;
  pointer-events: auto;
  border-color: #007bff;
  background: rgba(0, 123, 255, 0.1);
}

.drop-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: #007bff;
  font-size: 16px;
  font-weight: 500;
}

.dark-theme .drop-zone {
  background: rgba(26, 26, 26, 0.9);
  border-color: #404040;
}

.dark-theme .drop-zone.dragging {
  border-color: #4dabf7;
  background: rgba(77, 171, 247, 0.1);
}

.dark-theme .drop-hint {
  color: #4dabf7;
}

/* 只读模式 */
.readonly .grid-item-header {
  display: none;
}

.readonly .grid-item-body {
  padding: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .grid-item-header {
    padding: 6px 8px;
    font-size: 12px;
  }

  .grid-item-body {
    padding: 8px;
  }

  .action-btn {
    width: 20px;
    height: 20px;
  }
}
</style>
