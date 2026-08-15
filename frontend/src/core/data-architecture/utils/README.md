# Data Architecture 工具层说明

## 目录职责

该目录放置 data-architecture 的通用工具函数，主要覆盖导入导出、绑定路径恢复和单数据源导入落点处理。

## 关键文件

- `ConfigurationImportExport.ts`：配置导入导出总入口，负责整组件/单数据源导出、依赖占位符、冲突预检和写回门面；当前已开始收紧 `configurationManager` 能力边界。
- `binding-path-recovery.ts`：绑定路径修复与恢复。
- `singleDataSourceImportTarget.ts`：单数据源导入时的目标槽位选择与配置写回。

## 维护提示

- 这些工具经常处理旧配置兼容与异常输入，报错文案和保护逻辑要尽量明确。
- 如果调整导入策略，通常要同时检查数据源配置、交互配置和 HTTP 绑定是否都被正确补挂。
- `ConfigurationImportExport.ts` 不应继续扩大 `configurationManager: any` 的使用面；新增能力优先补到显式接口，再下沉到 helper。
