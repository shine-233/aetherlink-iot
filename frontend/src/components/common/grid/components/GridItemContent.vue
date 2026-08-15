<!--
  文件用途：包装单个网格项内容，统一标题、只读状态、动态组件和默认插槽渲染。
  核心逻辑：根据 item 配置选择渲染动态组件或插槽内容，并透传组件 props。
  关键注意事项：插槽参数和动态组件 props 是业务组件接入网格的关键契约。
  重构建议：可把标题栏、操作区和动态组件渲染拆成更小组件，便于复用和测试。
-->
<template>
  <div class="grid-item-content" :class="item.className" :style="item.style">
    <!-- 标题栏 -->
    <div v-if="!readonly && showTitle" class="grid-item-header">
      <span class="grid-item-title">{{ getItemTitle(item) }}</span>
    </div>

    <!-- 内容区域 -->
    <div class="grid-item-body" :class="{ 'with-padding': contentPadding }">
      <!-- 保持 item 透传给插槽，业务组件通常依赖它读取尺寸、类型或扩展字段。 -->
      <slot :item="item">
        <!-- 默认内容 -->
        <div class="default-item-content">
          <div class="item-type">{{ item.type || $t('custom.grid.componentFallback') }}</div>
          <div class="item-id">{{ item.i }}</div>
        </div>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * Grid Item 内容组件。
 * 这里不关心布局算法，只负责为单个网格项提供统一外壳和回退内容。
 */

import { $t } from '@/locales'
import type { GridLayoutPlusItem } from '../gridLayoutPlusTypes'

interface Props {
  /** 网格项数据 */
  item: GridLayoutPlusItem
  /** 是否只读模式 */
  readonly?: boolean
  /** 是否显示标题 */
  showTitle?: boolean
  /** 是否保留默认内容内边距 */
  contentPadding?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  showTitle: false,
  contentPadding: true
})

/**
 * 获取网格项标题
 * 优先级：title > type > 默认格式。
 * 统一标题生成逻辑，避免不同调用方各自拼接文案。
 */
const getItemTitle = (item: GridLayoutPlusItem): string => {
  return item.title || item.type || $t('custom.grid.itemTitleFallback', { index: item.i })
}
</script>

<style scoped>
.grid-item-content {
  width: 100%;
  /* 保持占满 GridItem 容器，避免内部内容高度塌陷后影响业务组件布局。 */
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.grid-item-header {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color);
  background: var(--card-color);
  flex-shrink: 0;
}

.grid-item-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
  line-height: 1.4;
}

.grid-item-body {
  flex: 1;
  /* 由业务组件自己决定内部滚动策略，这里先截断外溢内容。 */
  overflow: hidden;
}

.grid-item-body.with-padding {
  padding: 12px;
}

.default-item-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  color: var(--text-color-3);
}

.item-type {
  font-size: 16px;
  font-weight: 500;
  margin-bottom: 4px;
}

.item-id {
  font-size: 12px;
  opacity: 0.7;
}

/* 响应主题变化 */
[data-theme='dark'] .grid-item-header {
  border-bottom-color: var(--border-color);
}
</style>
