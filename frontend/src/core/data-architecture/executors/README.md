# Data Architecture Executors

## 目录职责

维护运行时数据执行链，负责从数据项请求、脚本处理、数据源合并到组件最终数据整合的完整流程。

## 文件关系

- `DataItemFetcher.ts` 是第一层，负责按配置获取或构造原始数据项。
- `DataItemProcessor.ts` 是第二层，负责过滤和脚本处理。
- `DataSourceMerger.ts` 是第三层，负责多数据项合并。
- `MultiSourceIntegrator.ts` 是第四层，负责多数据源整合为组件数据。
- `MultiLayerExecutorChain.ts` 串联四层并对外提供执行入口。

## 重点文件

- `DataItemFetcher.ts`：请求构造、绑定路径恢复、缓存 key 和 fallback 的主要风险点。
- `MultiLayerExecutorChain.ts`：执行链编排入口。
- `DataSourceMerger.ts`：合并策略和脚本合并入口。
- `MultiSourceIntegrator.ts`：组件最终数据结构整合入口。

## 审查建议

- 审查运行时改动时要看成功、失败、部分成功、空数据和脚本异常。
- 请求参数、缓存 key、fallback 和绑定路径属于兼容热点，不能只靠类型检查判断安全。
- 新增数据源类型时，确认四层执行链和 public 导出都覆盖到。
