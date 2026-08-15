# Visual Editor Store Facade

## 目录职责

`frontend/src/components/visual-editor/store` 为组件侧提供编辑器 store facade，避免组件直接依赖更深层的 store 路径。

## 文件关系

- `editor.ts` 当前转发 `store/modules/editor` 的 store。
- 组件层应通过该 facade 接入编辑器状态，便于后续迁移。

## 重点文件

- `editor.ts`: 组件侧编辑器 store 导出入口。

## 审查建议

若迁移 store 结构，应保留该 facade 的兼容导出并补调用点搜索。避免让组件同时依赖多个编辑器 store 入口。
