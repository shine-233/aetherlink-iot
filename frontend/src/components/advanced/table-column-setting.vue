<!--
  文件用途：表格列设置弹层组件，负责列显隐勾选与拖拽排序结果的本地编辑。
  核心逻辑：通过 `defineModel('columns')` 直接消费外层传入的列配置数组，再用 `VueDraggable` 维护列顺序、用复选框维护列显隐状态。
  关键数据流：
  1. 外层页面把 `FilteredColumn[]` 作为 v-model 传入。
  2. 组件内部直接对数组项的 `checked` 和顺序进行修改。
  3. 外层表格消费更新后的列配置，完成列展示刷新。
  使用注意：
  - 该组件假设 `item.key` 唯一且稳定，否则拖拽和勾选结果可能错位。
  - 当前属于“直接改传入模型”的轻交互组件，后续如引入撤销/重置能力，建议补中间态副本。
  静态审查建议：
  - `FilteredColumn` 若继续扩张职责，建议把排序、显隐、锁定列等规则拆成更显式的 helper。
  - 当前交互完全依赖数组原地变更，后续若遇到不可变数据流页面，需要注意兼容性。
-->
<script setup lang="ts" generic="T extends Record<string, unknown>, K = never">
import { defineAsyncComponent, ref } from 'vue'
import type { FilteredColumn } from '@/hooks/common/table'
import { $t } from '@/locales'

defineOptions({
  name: 'TableColumnSetting'
})

const columns = defineModel<FilteredColumn[]>('columns', {
  required: true
})

const popoverVisible = ref(false)
const shouldLoadDraggable = ref(false)
const AsyncVueDraggable = defineAsyncComponent(() => import('vue-draggable-plus').then(module => module.VueDraggable))

const handlePopoverVisibleUpdate = (show: boolean) => {
  popoverVisible.value = show
  if (show) {
    shouldLoadDraggable.value = true
  }
}
</script>

<template>
  <NPopover :show="popoverVisible" placement="bottom-end" trigger="click" @update:show="handlePopoverVisibleUpdate">
    <template #trigger>
      <NButton size="small">
        <template #icon>
          <IconAntDesignSettingOutlined class="text-icon" />
        </template>
        {{ $t('common.columnSetting') }}
      </NButton>
    </template>
    <AsyncVueDraggable v-if="shouldLoadDraggable" v-model="columns">
      <div v-for="item in columns" :key="item.key" class="h-36px flex-y-center rd-4px hover:(bg-primary bg-opacity-20)">
        <IconMdiDrag class="mr-8px cursor-move text-icon" />
        <NCheckbox v-model:checked="item.checked">
          {{ item.title }}
        </NCheckbox>
      </div>
    </AsyncVueDraggable>
    <div v-else class="table-column-setting__loading">
      {{ $t('common.loading') }}
    </div>
  </NPopover>
</template>

<style scoped>
.table-column-setting__loading {
  min-width: 180px;
  padding: 12px;
  color: var(--n-text-color-3);
  text-align: center;
}
</style>
