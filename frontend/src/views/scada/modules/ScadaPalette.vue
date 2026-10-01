<!--
  文件用途：SCADA 编辑器左侧组件/符号面板。
  核心逻辑：按符号库分类渲染按钮，点击后向父组件抛出要添加的节点描述；本组件不持有画布状态。
-->
<script setup lang="ts">
import { NButton, NCollapse, NCollapseItem, NSpace } from 'naive-ui'

import { listScadaSymbolCategories, listScadaSymbolsByCategory, type ScadaSymbol } from '../core/symbolLibrary'

const emit = defineEmits<{
  (event: 'add-symbol', symbol: ScadaSymbol): void
  (event: 'add-widget'): void
}>()

// 符号库是静态模块常量，无需 computed。
const palette = listScadaSymbolCategories().map((category) => ({
  category,
  symbols: listScadaSymbolsByCategory(category)
}))
</script>

<template>
  <aside class="scada-palette">
    <NCollapse>
      <NCollapseItem title="widgets" name="widgets">
        <NButton size="tiny" @click="emit('add-widget')">+ timeseries</NButton>
      </NCollapseItem>
      <NCollapseItem v-for="group in palette" :key="group.category" :title="group.category" :name="group.category">
        <NSpace vertical :size="4">
          <NButton
            v-for="symbol in group.symbols"
            :key="symbol.key"
            size="tiny"
            quaternary
            @click="emit('add-symbol', symbol)"
          >
            {{ symbol.label }}
          </NButton>
        </NSpace>
      </NCollapseItem>
    </NCollapse>
  </aside>
</template>

<style scoped>
.scada-palette {
  width: 200px;
  flex: 0 0 200px;
}
</style>
