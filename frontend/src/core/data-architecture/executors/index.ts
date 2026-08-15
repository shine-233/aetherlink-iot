/**
 * 文件用途: 多层级执行器链导出入口。
 * 核心逻辑: 集中导出获取、处理、合并、整合和链路协调相关类型与实现。
 * 关键注意事项: 导出名称是运行时和测试的稳定依赖，调整前需要全仓检索引用。
 * 重构建议: 分离稳定 public API 与内部层级实现，避免调用方绕过链路入口。
 */

// 第一层：数据项获取器
export {
  DataItemFetcher,
  type IDataItemFetcher,
  type DataItem,
  type JsonDataItemConfig,
  type HttpDataItemConfig,
  type WebSocketDataItemConfig,
  type ScriptDataItemConfig
} from './DataItemFetcher'

// 第二层：数据项处理器
export {
  DataItemProcessor,
  type IDataItemProcessor,
  type ProcessingConfig
} from '@/core/data-architecture/executors/DataItemProcessor'

// 第三层：数据源合并器
export {
  DataSourceMerger,
  type IDataSourceMerger,
  type MergeStrategy
} from '@/core/data-architecture/executors/DataSourceMerger'

// 第四层：多源整合器
export {
  MultiSourceIntegrator,
  type IMultiSourceIntegrator,
  type ComponentData,
  type DataSourceResult
} from './MultiSourceIntegrator'

// 主协调类：多层级执行器链
export {
  MultiLayerExecutorChain,
  type IMultiLayerExecutorChain,
  type DataSourceConfiguration,
  type ExecutionState,
  type ExecutionResult
} from './MultiLayerExecutorChain'

// 导入用于工厂函数
import { MultiLayerExecutorChain } from '@/core/data-architecture/executors/MultiLayerExecutorChain'

// 便捷工厂函数
export function createExecutorChain(): MultiLayerExecutorChain {
  return new MultiLayerExecutorChain()
}

// 类型工具：与执行器实际支持的公开契约保持一致。
export type AllDataItemTypes = 'json' | 'http' | 'websocket' | 'script'
export type AllMergeStrategyTypes = 'object' | 'array' | 'select' | 'script'

// 版本信息
export const EXECUTOR_CHAIN_VERSION = '1.0.0'
