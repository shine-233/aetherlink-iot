/**
 * 文件用途: data-architecture 模块总导出入口。
 * 核心逻辑: 重新导出执行器、类型、适配器和配置生成能力，供上层模块统一引用。
 * 关键注意事项: 该入口会放大破坏性导出变更，调整前需要确认外部依赖范围。
 * 重构建议: 拆分稳定公共入口和内部维护入口，降低模块间耦合。
 */

export * from '@/core/data-architecture/executors'
export * from '@/core/data-architecture/types'
export * from '@/core/data-architecture/adapters/ConfigurationAdapter'
export * from '@/core/data-architecture/config-generation'

export {
  ConfigurationManager,
  configurationManager,
  type ConfigurationTemplate
} from '@/core/data-architecture/services/ConfigurationManager'

export { EXECUTOR_CHAIN_VERSION } from '@/core/data-architecture/executors'
export { TYPE_SYSTEM_VERSION, SUPPORTED_CONFIG_VERSIONS } from '@/core/data-architecture/types'
export { ADAPTER_VERSION } from '@/core/data-architecture/adapters'

export const DATA_ARCHITECTURE_VERSION = {
  EXECUTORS: '1.0.0',
  TYPES: '2.0.0',
  ADAPTERS: '1.0.0',
  OVERALL: '2.0.0'
} as const

export type {
  DataSourceConfiguration,
  ExecutionState,
  ExecutionResult,
  ComponentData,
  DataItem,
  MergeStrategy,
  ProcessingConfig,
  AllDataItemTypes,
  AllMergeStrategyTypes
} from './executors'
