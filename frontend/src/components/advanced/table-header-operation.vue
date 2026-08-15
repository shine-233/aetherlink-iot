<!--
  文件用途：表格头部通用操作组件，负责集中提供新增、批量删除、刷新和列设置入口。
  核心逻辑：通过 `props` 控制按钮禁用态和加载态，通过 `emit` 把动作事件交还给业务页面处理，同时把列设置模型透传给配套组件。
  关键数据流：
  1. 外层页面传入按钮状态和列模型。
  2. 当前组件只负责触发 `add / delete / refresh` 事件，不在本层做业务判断。
  3. 列配置能力通过 `columns` v-model 与列设置组件保持同步。
  使用注意：
  - 当前组件定位是“轻量交互壳层”，不要继续往里塞接口请求或权限判断。
  - `columns` 默认为空数组，后续如果希望列设置成为可选能力，建议补更清晰的条件渲染说明。
  静态审查建议：
  - 如果后续列表页动作继续增多，优先考虑插槽或配置驱动扩展，而不是把业务分支写死在这里。
  - 目录内两个组件都依赖外层页面提供稳定列模型，建议后续继续强化 `FilteredColumn` 的类型约束。
-->
<script setup lang="ts">
import type { FilteredColumn } from '@/hooks/common/table'
import { $t } from '~/src/locales'

defineOptions({
  name: 'TableHeaderOperation'
})

interface Props {
  disabledDelete?: boolean
  loading?: boolean
}

defineProps<Props>()

interface Emits {
  (e: 'add'): void
  (e: 'delete'): void
  (e: 'refresh'): void
}

const emit = defineEmits<Emits>()

// 列配置模型由外层页面持有，这里只做表头层的透传和挂载。
const columns = defineModel<FilteredColumn[]>('columns', {
  default: () => []
})

// 这些动作函数都只是事件转发层，真实业务处理仍由上层页面负责。
function add() {
  emit('add')
}

function batchDelete() {
  emit('delete')
}

function refresh() {
  emit('refresh')
}
</script>

<template>
  <NSpace wrap justify="end" class="<sm:w-200px">
    <NButton size="small" ghost type="primary" @click="add">
      <template #icon>
        <IconIcRoundPlus class="text-icon" />
      </template>
      {{ $t('common.add') }}
    </NButton>
    <NPopconfirm @positive-click="batchDelete">
      <template #trigger>
        <NButton size="small" ghost type="error" :disabled="disabledDelete">
          <template #icon>
            <IconIcRoundDelete class="text-icon" />
          </template>
          {{ $t('common.batchDelete') }}
        </NButton>
      </template>
      {{ $t('common.confirmDelete') }}
    </NPopconfirm>
    <NButton size="small" @click="refresh">
      <template #icon>
        <IconMdiRefresh class="text-icon" :class="{ 'animate-spin': loading }" />
      </template>
      {{ $t('common.refresh') }}
    </NButton>
    <TableColumnSetting v-model:columns="columns" />
  </NSpace>
</template>

<style scoped></style>
