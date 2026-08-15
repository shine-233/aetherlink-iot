<!--
文件用途：提供用户管理列表的列配置浮层，负责列显隐切换与拖拽排序。
核心逻辑：组件接收父页面传入的完整列定义，克隆出本地可编辑列表，通过勾选与拖拽维护展示顺序，
  再以 `update:columns` 回调把最终可见列集合返回给父页面。
关键注意事项：
1. 这里只维护前端展示配置，不应在组件内混入真实用户数据查询、权限判断或持久化请求。
2. 列配置属于页面级 UI 偏好，是否允许当前操作者调整列、是否需要持久化到服务端，应由父页面或更高层统一决定。
3. `checked` 是本组件临时附加的 UI 状态，回传给父组件前必须剥离，避免污染原始列定义契约。
静态审查建议：
1. 当前 `list` 只在初始化时读取 `props.columns`，父组件后续若动态变更列定义，这里不会自动重建本地状态。
2. `watch(list)` 每次拖拽或勾选都会直接 emit，新旧列差异较大时可考虑节流或显式“保存配置”动作。
3. 目前没有针对关键列的不可隐藏约束，后续如存在强依赖列，可在初始化阶段增加锁定策略。
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

// DataTableColumn 是联合类型，selection/expand 分支没有 key/title。
// 模板里直接读 element.key / element.title 会在联合的这些分支上报 TS2339，
// 因此统一从这两个收窄函数取值，而不是在模板内断言。
function columnKey(column: List) {
  return 'key' in column ? column.key : undefined
}

function columnTitle(column: List) {
  return 'title' in column ? column.title : undefined
}

// 本地列表在原始列定义上附加勾选态，专门服务拖拽排序与列显隐编辑。
const list = ref(initList())

function initList(): List[] {
  // 默认全部勾选，保证首次打开列配置时与父页面当前完整列定义保持一致。
  return props.columns.map((item) => ({ ...item, checked: true }))
}

watch(
  list,
  (newValue) => {
    // 先过滤被取消勾选的列，再按当前拖拽顺序回传给父页面作为最新展示列集合。
    const newColumns = newValue.filter((item) => item.checked)

    const columns: Column[] = newColumns.map((item) => {
      const column = { ...item }

      // `checked` 只是子组件内部状态，回调给父组件前需要剥离，避免破坏 DataTableColumn 类型边界。
      delete column.checked

      return column
    }) as Column[]

    // 保存回调通过单一 emit 通道上抛，父页面再决定是否立即生效、缓存或持久化。
    emit('update:columns', columns)
  },
  { deep: true }
)
</script>

<template>
  <NPopover placement="bottom" trigger="click">
    <template #trigger>
      <NButton size="small" type="primary">
        <IconAntDesignSettingOutlined class="mr-4px text-16px" />
        {{ $t('common.changeTableColumns') }}
      </NButton>
    </template>
    <div class="w-180px">
      <VueDraggable v-model="list">
        <template v-for="element in list" :key="String(columnKey(element))">
          <div v-if="columnKey(element)" class="hover:bg-primary_active h-36px flex-y-center px-12px">
            <IconMdiDrag class="mr-8px cursor-move text-20px" />
            <NCheckbox v-model:checked="element.checked">
              {{ columnTitle(element) }}
            </NCheckbox>
          </div>
        </template>
      </VueDraggable>
    </div>
  </NPopover>
</template>

<style scoped></style>
