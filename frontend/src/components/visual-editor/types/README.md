# Visual Editor Types

## 目录职责

`frontend/src/components/visual-editor/types` 定义视觉编辑器组件、配置和基础类型。

## 文件关系

- `base-types.ts` 保存基础结构。
- `index.ts` 汇总对外类型导出。
- `configuration/types.ts` 与这里的类型共同描述持久化配置形态。

## 重点文件

- `base-types.ts`: 编辑器基础实体类型。
- `index.ts`: 类型对外出口。

## 审查建议

类型变更会影响 persisted dashboard、配置导入导出和 store。重命名字段前应先确认迁移策略和兼容别名。
