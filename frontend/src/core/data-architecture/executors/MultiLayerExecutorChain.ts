/**
 * 文件用途: 多层级执行器链协调器。
 * 核心逻辑: 串联数据项获取、处理、合并和多源整合，完成数据源到组件数据的转换。
 * 关键注意事项: 每层返回结构和错误传播方式会影响组件最终渲染与诊断信息。
 * 重构建议: 将链路编排抽成可观测 pipeline，并为每层建立更明确的接口契约。
 */

import type { DataItem } from '@/core/data-architecture/executors/DataItemFetcher'
import {
  DataItemProcessor,
  ProcessingConfig,
  IDataItemProcessor
} from '@/core/data-architecture/executors/DataItemProcessor'
import { DataSourceMerger, MergeStrategy, IDataSourceMerger } from '@/core/data-architecture/executors/DataSourceMerger'
import {
  MultiSourceIntegrator,
  ComponentData,
  DataSourceResult,
  IMultiSourceIntegrator
} from '@/core/data-architecture/executors/MultiSourceIntegrator'
import { unifiedDataExecutor, type UnifiedDataConfig } from '@/core/data-architecture/UnifiedDataExecutor'
import { shouldSuppressUnmockedTestConsole } from '@/utils/test-console'

function chainWarn(...args: any[]): void {
  if (!shouldSuppressUnmockedTestConsole(console.warn)) console.warn(...args)
}

function chainError(...args: any[]): void {
  if (!shouldSuppressUnmockedTestConsole(console.error)) console.error(...args)
}

/** 在内部抛出时保留统一执行器的稳定错误代码。 */
class DataItemExecutionError extends Error {
  constructor(message: string, readonly code?: string) {
    super(message)
    this.name = 'DataItemExecutionError'
  }
}

/**
 * 数据源配置结构
 */
export interface DataSourceConfiguration {
  componentId: string
  dataSources: Array<{
    sourceId: string
    dataItems: Array<{
      item: DataItem
      processing: ProcessingConfig
    }>
    mergeStrategy: MergeStrategy
  }>
  createdAt: number
  updatedAt: number
}

/**
 * 执行状态跟踪 (用于调试监控)
 */
export interface ExecutionState {
  componentId: string
  dataSourceId: string
  stages: {
    /** 第一层: 原始数据获取结果 */
    rawData: Map<string, { data: any; timestamp: number; success: boolean }>
    /** 第二层: 数据处理结果 */
    processedData: Map<string, { data: any; timestamp: number; success: boolean }>
    /** 第三层: 数据源合并结果 */
    mergedData: { data: any; timestamp: number; success: boolean } | null
    /** 第四层: 最终组件数据 */
    finalData: { data: any; timestamp: number; success: boolean } | null
  }
  debugMode: boolean
  lastExecuted: number
}

/**
 * 执行结果
 */
export interface ExecutionResult {
  /** 是否成功 */
  success: boolean
  /** 组件数据 */
  componentData?: ComponentData
  /** 错误信息 */
  error?: string
  /** 稳定错误代码 */
  errorCode?: string
  /** 是否没有可用组件数据 */
  isEmpty?: boolean
  /** 执行时间（毫秒） */
  executionTime: number
  /** 时间戳 */
  timestamp: number
  /** 调试状态 */
  executionState?: ExecutionState
}

/**
 * 多层级执行器链接口
 */
export interface IMultiLayerExecutorChain {
  /**
   * 执行完整的数据处理管道
   * @param config 数据源配置
   * @param debugMode 是否开启调试模式
   * @returns 执行结果
   */
  executeDataProcessingChain(config: DataSourceConfiguration, debugMode?: boolean): Promise<ExecutionResult>
}

/**
 * 多层级执行器链实现类
 */
export class MultiLayerExecutorChain implements IMultiLayerExecutorChain {
  private dataItemProcessor: IDataItemProcessor
  private dataSourceMerger: IDataSourceMerger
  private multiSourceIntegrator: IMultiSourceIntegrator

  constructor() {
    this.dataItemProcessor = new DataItemProcessor()
    this.dataSourceMerger = new DataSourceMerger()
    this.multiSourceIntegrator = new MultiSourceIntegrator()
  }

  /**
   * 执行完整的数据处理管道
   */
  async executeDataProcessingChain(
    config: DataSourceConfiguration,
    debugMode: boolean = false
  ): Promise<ExecutionResult> {
    const startTime = Date.now()

    try {
      const dataSourceResults: DataSourceResult[] = []
      let executionState: ExecutionState | undefined

      // 初始化调试状态
      if (debugMode) {
        executionState = this.initializeExecutionState(config.componentId)
      }

      // 处理每个数据源
      for (const dataSourceConfig of config.dataSources) {
        if (!dataSourceConfig.dataItems || dataSourceConfig.dataItems.length === 0) {
          chainError('[MultiLayerExecutorChain] Data source has no dataItems.', {
            sourceId: dataSourceConfig.sourceId,
            dataItemsType: typeof dataSourceConfig.dataItems,
            dataItems: dataSourceConfig.dataItems
          })
        }

        try {
          const sourceResult = await this.processDataSource(dataSourceConfig, executionState, config.componentId)

          dataSourceResults.push(sourceResult)
        } catch (error) {
          chainError('[MultiLayerExecutorChain] processDataSource failed:', {
            sourceId: dataSourceConfig.sourceId,
            errorType: typeof error,
            errorMessage: error instanceof Error ? error.message : error,
            errorStack: error instanceof Error ? error.stack : undefined
          })

          dataSourceResults.push({
            sourceId: dataSourceConfig.sourceId,
            type: 'unknown',
            data: {},
            success: false,
            error: error instanceof Error ? error.message : 'Unknown error'
          })
        }
      }

      // 第四层：多源整合
      const componentData = await this.multiSourceIntegrator.integrateDataSources(dataSourceResults, config.componentId)
      const successfulSources = dataSourceResults.filter(source => source.success).length
      const allSourcesFailed = dataSourceResults.length === 0 || successfulSources === 0

      // 更新调试状态
      if (executionState) {
        executionState.stages.finalData = {
          data: componentData,
          timestamp: Date.now(),
          success: Object.keys(componentData).length > 0
        }
        executionState.lastExecuted = Date.now()
      }

      const executionTime = Date.now() - startTime

      const firstFailure = dataSourceResults.find(source => !source.success)

      return {
        success: !allSourcesFailed,
        componentData,
        error: allSourcesFailed ? firstFailure?.error || '所有数据源执行失败' : undefined,
        errorCode: allSourcesFailed ? firstFailure?.errorCode : undefined,
        executionTime,
        timestamp: Date.now(),
        executionState,
        isEmpty: allSourcesFailed
      }
    } catch (error) {
      const executionTime = Date.now() - startTime

      return {
        success: false,
        error: error instanceof Error ? error.message : 'Executor chain failed',
        executionTime,
        timestamp: Date.now()
      }
    }
  }

  /**
   * 处理单个数据源
   */
  private async processDataSource(
    dataSourceConfig: {
      sourceId: string
      dataItems: Array<{ item: DataItem; processing: ProcessingConfig }>
      mergeStrategy: MergeStrategy
    },
    executionState?: ExecutionState,
    componentId?: string
  ): Promise<DataSourceResult> {
    try {
      const processedItems: any[] = []
      let successfulItems = 0
      let firstItemError: DataItemExecutionError | undefined

      if (dataSourceConfig.dataItems.length === 0) {
        chainWarn('[MultiLayerExecutorChain] dataItems is empty; no data fetch will run.', {
          sourceId: dataSourceConfig.sourceId
        })
      }

      // 处理每个数据项
      for (let i = 0; i < dataSourceConfig.dataItems.length; i++) {
        const { item, processing } = dataSourceConfig.dataItems[i]
        const itemId = `${dataSourceConfig.sourceId}_item_${i}`

        try {
          if (item.type === 'http' && item.config) {
            const allParams = [
              ...(item.config.params || []),
              ...(item.config.parameters || []),
              ...(item.config.pathParams || [])
            ]

            allParams.forEach((param, paramIndex) => {
              if (param.value && typeof param.value === 'string') {
                const isSuspiciousPath = !param.value.includes('.') && param.value.length < 10 && param.variableName

                if (isSuspiciousPath) {
                  chainError('[MultiLayerExecutorChain] Suspicious binding path before fetchData.', {
                    componentId: componentId || 'unknown',
                    sourceId: dataSourceConfig.sourceId,
                    paramIndex,
                    key: param.key,
                    value: param.value,
                    variableName: param.variableName,
                    param: JSON.stringify(param, null, 2)
                  })
                }
              }
            })
          }

          const rawData = await this.fetchRawData(item, dataSourceConfig.sourceId, i)
          successfulItems += 1

          // 更新调试状态
          if (executionState) {
            executionState.stages.rawData.set(itemId, {
              data: rawData,
              timestamp: Date.now(),
              success: Object.keys(rawData || {}).length > 0
            })
          }

          // 第二层：数据项处理
          const processedData = await this.dataItemProcessor.processData(rawData, processing)

          // 更新调试状态
          if (executionState) {
            executionState.stages.processedData.set(itemId, {
              data: processedData,
              timestamp: Date.now(),
              success: Object.keys(processedData || {}).length > 0
            })
          }

          processedItems.push(processedData)
        } catch (error) {
          chainError('[MultiLayerExecutorChain] Data item processing failed:', {
            itemId,
            itemType: item.type,
            itemIndex: i,
            errorType: typeof error,
            errorMessage: error instanceof Error ? error.message : error,
            errorStack: error instanceof Error ? error.stack : undefined,
            itemConfig: item.config
          })
          if (!firstItemError) {
            firstItemError =
              error instanceof DataItemExecutionError
                ? error
                : new DataItemExecutionError(error instanceof Error ? error.message : String(error))
          }
          processedItems.push(null)
        }
      }

      // 第三层：数据源合并
      const mergedData = await this.dataSourceMerger.mergeDataItems(processedItems, dataSourceConfig.mergeStrategy)

      // 更新调试状态
      if (executionState) {
        executionState.stages.mergedData = {
          data: mergedData,
          timestamp: Date.now(),
          success: Object.keys(mergedData || {}).length > 0
        }
      }

      return {
        sourceId: dataSourceConfig.sourceId,
        type: dataSourceConfig.dataItems[0]?.item.type || 'unknown',
        data: successfulItems > 0 ? mergedData : null,
        success: successfulItems > 0,
        error: successfulItems === 0 ? firstItemError?.message || '数据源没有可执行的数据项' : undefined,
        errorCode: successfulItems === 0 ? firstItemError?.code || 'DATA_SOURCE_NO_SUCCESSFUL_ITEMS' : undefined
      }
    } catch (error) {
      return {
        sourceId: dataSourceConfig.sourceId,
        type: 'unknown',
        data: {},
        success: false,
        error: error instanceof Error ? error.message : '数据源处理失败'
      }
    }
  }

  private async fetchRawData(item: DataItem, sourceId: string, index: number): Promise<any> {
    const config: UnifiedDataConfig = {
      id: `${sourceId}_${index}`,
      type: item.type as UnifiedDataConfig['type'],
      config: this.normalizeUnifiedConfig(item.config)
    }
    const result = await unifiedDataExecutor.execute(config)

    if (!result.success) {
      throw new DataItemExecutionError(result.error || `Data item ${config.id} execution failed`, result.errorCode)
    }

    return result.data
  }

  private normalizeUnifiedConfig(config: Record<string, any> = {}): Record<string, any> {
    return {
      ...config,
      jsonContent: config.jsonContent ?? config.jsonString
    }
  }

  /**
   * 初始化执行状态
   */
  private initializeExecutionState(componentId: string): ExecutionState {
    return {
      componentId,
      dataSourceId: '',
      stages: {
        rawData: new Map(),
        processedData: new Map(),
        mergedData: null,
        finalData: null
      },
      debugMode: true,
      lastExecuted: 0
    }
  }

  /**
   * 验证数据源配置
   */
  validateConfiguration(config: DataSourceConfiguration): boolean {
    if (!config.componentId || !config.dataSources) {
      return false
    }

    // 允许数据项数组为空，这样可以返回 null 数据
    return config.dataSources.every((ds) => ds.sourceId && Array.isArray(ds.dataItems) && ds.mergeStrategy)
  }

  /**
   * 获取执行器链统计信息
   */
  getChainStatistics(): {
    version: string
    supportedDataTypes: string[]
    supportedMergeStrategies: string[]
    features: string[]
  } {
    return {
      version: '1.0.0',
      supportedDataTypes: ['json', 'http', 'websocket', 'script'],
      supportedMergeStrategies: ['object', 'array', 'select', 'script'],
      features: [
        'JSONPath数据过滤',
        '自定义脚本处理',
        '多种合并策略',
        '调试监控机制',
        'Visual Editor兼容',
        'Card2.1兼容'
      ]
    }
  }
}
