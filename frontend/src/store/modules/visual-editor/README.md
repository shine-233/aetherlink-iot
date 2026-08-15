# Visual Editor Store Module

## 目录职责

`frontend/src/store/modules/visual-editor` 管理可视化编辑器的统一状态、配置服务、数据流管理和 card2 适配。

## 文件关系

- `index.ts` 暴露模块入口。
- `unified-editor.ts` 汇总编辑器状态和操作。
- `configuration-service.ts` 与 `components/visual-editor/configuration` 的配置桥接保持一致。
- `data-flow-manager.ts` 与 `core/data-architecture` 的运行时数据流相关。

## 重点文件

- `unified-editor.ts`: 编辑器状态主入口。
- `configuration-service.ts`: 配置读写和默认值处理。
- `data-flow-manager.ts`: 数据流状态与运行时行为。

## 审查建议

重点检查持久化配置兼容性、组件 ID 映射和导入导出路径。重构前应先补 import/export、配置回读和 card2 适配测试。
