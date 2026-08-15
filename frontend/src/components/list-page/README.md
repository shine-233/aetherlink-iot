# ListPage 组件说明

## 目录定位

`frontend/src/components/list-page/` 是列表页布局容器目录，核心文件为 `index.vue`。这一层负责提供统一的列表页骨架，包括搜索区、头部操作区、视图切换、刷新按钮和底部区域，业务页面只需要通过插槽填充内容即可。

## 与文件的关系

- `index.vue` 是真正的组件实现。
- `README.md` 负责说明组件能力、插槽约定和使用边界。

## 组件职责

`ListPage` 适合承载“先筛选、后展示”的页面结构，例如设备列表、历史记录列表、配置列表等。它的目标不是封装业务数据请求，而是统一列表页的布局和交互骨架。

## 主要能力

- 根据插槽自动决定是否展示搜索区和内容视图。
- 支持默认新增按钮，也支持完全自定义左侧头部区域。
- 支持卡片、列表、地图等多视图切换。
- 支持刷新事件，便于父组件统一重新拉取数据。
- 支持移动端断点适配。
- 支持可选的视图记忆能力，避免页面切换后重复选择视图。

## Props

| 属性 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `addButtonText` | `string \| (() => string)` | `''` | 新增按钮文案，可传字符串或函数。 |
| `addButtonI18nKey` | `string` | `'card.addButton'` | 新增按钮的国际化 key。 |
| `initialView` | `string` | `''` | 初始视图类型，例如 `card`、`list`、`map`。 |
| `availableViews` | `ViewItem[]` | `[{ card, list, map }]` | 可用视图配置，按插槽存在情况再过滤。 |
| `showQueryButton` | `boolean` | `true` | 控制搜索区是否可以被显示。 |
| `showResetButton` | `boolean` | `true` | 控制搜索区是否可以被显示。 |
| `showAddButton` | `boolean` | `true` | 控制默认新增按钮是否显示。 |
| `mobileBreakpoint` | `number` | `768` | 移动端断点。 |
| `useViewMemory` | `boolean` | `false` | 是否记住上次视图。 |
| `memoryKey` | `string` | `'advanced-list-view'` | 视图记忆的存储 key。 |

### `ViewItem`

```ts
interface ViewItem {
  key: string
  icon: any
  label?: string
}
```

## Events

| 事件 | 参数 | 说明 |
| --- | --- | --- |
| `query` | `filterData: Record<string, any>` | 查询事件契约，当前实现保留了事件定义，但组件本身不会主动触发。 |
| `reset` | 无 | 重置事件。 |
| `add-new` | 无 | 新增事件。 |
| `view-change` | `{ viewType: string }` | 视图切换事件。 |
| `refresh` | 无 | 刷新事件。 |

## Slots

| 插槽 | 说明 |
| --- | --- |
| `search-form-content` | 搜索表单内容。 |
| `header-left` | 左侧头部内容，优先级高于默认新增按钮。 |
| `add-button` | 只自定义新增按钮本身。 |
| `header-right` | 右侧头部内容。 |
| `card-view` | 卡片视图内容。 |
| `list-view` | 列表视图内容。 |
| `map-view` | 地图视图内容。 |
| `footer` | 底部内容，常用于分页。 |

## 推荐用法

```vue
<template>
  <ListPage
    initial-view="list"
    :show-add-button="true"
    :use-view-memory="true"
    @add-new="handleAddNew"
    @view-change="handleViewChange"
    @refresh="handleRefresh"
  >
    <template #search-form-content>
      <n-form inline :model="filterForm">
        <n-form-item label="名称">
          <n-input v-model:value="filterForm.name" placeholder="请输入名称" />
        </n-form-item>
      </n-form>
    </template>

    <template #list-view>
      <n-data-table :columns="columns" :data="rows" :loading="loading" />
    </template>

    <template #footer>
      <n-pagination
        v-model:page="pagination.page"
        :page-count="pagination.pageCount"
        @update:page="handlePageChange"
      />
    </template>
  </ListPage>
</template>
```

## 适用场景

- 标准列表页。
- 同一页面需要在卡片、列表、地图之间切换。
- 页面顶部有统一筛选条件和新增入口。
- 需要父组件统一管理查询、重置、刷新和分页。

## 使用注意事项

1. `ListPage` 只负责布局，不负责发请求。
2. `search-form-content` 为空时，搜索区是否出现由 `showQueryButton` 和 `showResetButton` 共同决定。
3. `query` 事件目前是契约保留项，若业务需要查询按钮，建议在父组件或插槽中自行接入。
4. 视图切换只会在对应插槽存在时生效。
5. `header-left` 会完全覆盖默认新增按钮，`add-button` 只适合替换按钮本身。
6. `useViewMemory` 会借助本地存储记忆视图，适合页面结构稳定的场景。

## 静态审查建议

- 检查所有使用 `query` 事件的页面是否真的会触发它，避免出现“定义了但从未调用”的死契约。
- 如果某些页面只提供一个视图插槽，不要强依赖视图切换按钮。
- 如果页面内容可能超高，建议父组件自行确认分页与滚动容器的组合方式。
- 如果项目开启了国际化，`addButtonI18nKey` 的 key 应与实际词条保持一致。

## 后续改进方向

- 将视图切换、按钮显示和插槽检测拆分为更小的组合式模块。
- 为 `query` 事件补齐明确的触发来源，或者删除无实际触发的保留契约。
- 为视图记忆增加命名空间说明，避免不同页面复用同一个存储 key。
