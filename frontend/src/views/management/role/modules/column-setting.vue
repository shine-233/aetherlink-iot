<!--
文件用途：提供角色管理页面内的 column-setting 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import type { DataTableColumn } from 'naive-ui'
import { VueDraggable } from 'vue-draggable-plus'
import { $t } from '@/locales'

type Column = DataTableColumn<UserManagement.User>

interface Props {
  columns: Column[]
}

const props = defineProps<Props>()

interface Emits {
  (e: 'update:columns', columns: Column[]): void
}

const emit = defineEmits<Emits>()

type List = Column & { checked?: boolean }

const list = ref(initList())

function initList(): List[] {
  return props.columns.map((item) => ({ ...item, checked: true }))
}

// DataTableColumn 是联合类型，selection 列没有 key/title 字段，
// 模板里直接访问会触发 TS2339。这两个 helper 做一次窄化，
// 让模板只读取实际存在的字段，同时保持 selection 列被 v-if 过滤掉的行为。
function columnKey(column: List) {
  return 'key' in column ? column.key : undefined
}

function columnTitle(column: List) {
  return 'title' in column ? column.title : undefined
}

watch(
  list,
  (newValue) => {
    const newColumns = newValue.filter((item) => item.checked)

    const columns: Column[] = newColumns.map((item) => {
      const column = { ...item }
      delete column.checked

      return column
    }) as Column[]

    emit('update:columns', columns)
  },
  { deep: true }
)
</script>

<template>
  <n-popover placement="bottom" trigger="click">
    <template #trigger>
      <n-button size="small" type="primary">
        <icon-ant-design-setting-outlined class="mr-4px text-16px" />
        {{ $t('common.changeTableColumns') }}
      </n-button>
    </template>
    <div class="w-180px">
      <VueDraggable v-model="list">
        <template v-for="element in list" :key="String(columnKey(element))">
          <div v-if="columnKey(element)" class="hover:bg-primary_active h-36px flex-y-center px-12px">
            <icon-mdi-drag class="mr-8px cursor-move text-20px" />
            <n-checkbox v-model:checked="element.checked">
              {{ columnTitle(element) }}
            </n-checkbox>
          </div>
        </template>
      </VueDraggable>
    </div>
  </n-popover>
</template>

<style scoped></style>
