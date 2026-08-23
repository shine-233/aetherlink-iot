/**
 * 文件说明：
 * - 负责把多种历史/运行时数据源配置格式归一化为 data-architecture 标准结构。
 * - 服务于导入导出、执行链、编辑器桥接等多个入口，是配置兼容层的核心门面。
 * - 任何格式分支调整都可能影响旧数据回放与新配置保存，改动前要先确认上下游契约。
 */

import {
  isImportExportFormat,
  isSimpleConfigEditorFormat,
  isStandardDataItem,
  normalizePersistedDataItem,
  normalizePersistedDataSourceType,
  normalizePersistedMergeStrategy,
  normalizePersistedSourceId
} from './DataFormatNormalizerCompatibility'

export interface StandardDataItem {
  item: {
    type: 'static' | 'http' | 'json' | 'websocket' | 'file' | 'data-source-bindings'
    config: Record<string, unknown>
  }
  processing: {
    filterPath: string
    customScript?: string
    defaultValue?: unknown
  }
}

export interface StandardDataSourceConfig {
  componentId: string
  dataSources: Array<{
    sourceId: string
    dataItems: StandardDataItem[]
    mergeStrategy: { type: 'object' | 'array' | 'replace' }
  }>
  createdAt: number
  updatedAt: number
}

type StandardDataSource = StandardDataSourceConfig['dataSources'][number]
type StandardProcessing = StandardDataItem['processing']

/** 历史配置输入（自由 JSON，字段运行时逐个校验） */
type UnknownRecord = Record<string, unknown>

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === 'object' && value !== null
}

const DEFAULT_FILTER_PATH = '$'
const DEFAULT_SOURCE_ID = 'main'
const DEFAULT_MERGE_STRATEGY: StandardDataSource['mergeStrategy'] = { type: 'object' }

export class DataFormatNormalizer {
  static normalizeToStandard(data: unknown, componentId: string): StandardDataSourceConfig {
    if (this.isStandardFormat(data)) {
      return data as StandardDataSourceConfig
    }

    // SimpleConfigurationEditor 是持久化格式标识，不代表运行时组件依赖。
    if (isSimpleConfigEditorFormat(data)) {
      return this.convertFromSimpleConfigEditor(data, componentId)
    }

    if (isImportExportFormat(data)) {
      return this.convertFromImportExport(data, componentId)
    }

    if (this.isCard2ExecutorFormat(data)) {
      return this.convertFromCard2Executor(data, componentId)
    }

    if (this.isEditorManagerFormat(data)) {
      return this.convertFromEditorManager(data, componentId)
    }

    return this.convertFromGenericObject(data, componentId)
  }

  static convertFromStandard(
    standardData: StandardDataSourceConfig,
    targetFormat: 'simpleConfigEditor' | 'importExport' | 'card2Executor'
  ): unknown {
    switch (targetFormat) {
      case 'simpleConfigEditor':
        return this.convertToSimpleConfigEditor(standardData)
      case 'importExport':
        return this.convertToImportExport(standardData)
      case 'card2Executor':
        return this.convertToCard2Executor(standardData)
      default:
        return standardData
    }
  }

  private static isStandardFormat(data: unknown): boolean {
    return !!(
      isRecord(data) &&
      'componentId' in data &&
      'dataSources' in data &&
      Array.isArray(data.dataSources) &&
      data.dataSources.every((ds) => {
        const dsRecord = ds as UnknownRecord | null | undefined
        return !!(
          dsRecord &&
          typeof dsRecord === 'object' &&
          'sourceId' in dsRecord &&
          'dataItems' in dsRecord &&
          Array.isArray(dsRecord.dataItems) &&
          (dsRecord.dataItems as unknown[]).every((item) => isStandardDataItem(item))
        )
      })
    )
  }

  private static isCard2ExecutorFormat(data: unknown): boolean {
    if (!isRecord(data)) {
      return false
    }
    return Object.keys(data).some((key) => {
      const value = data[key] as UnknownRecord | null | undefined
      return !!(
        value &&
        typeof value === 'object' &&
        'type' in value &&
        'data' in value &&
        'metadata' in value
      )
    })
  }

  private static isEditorManagerFormat(data: unknown): boolean {
    return !!(
      isRecord(data) &&
      'type' in data &&
      'config' in data &&
      !('item' in data && 'processing' in data)
    )
  }

  private static convertFromSimpleConfigEditor(data: unknown, componentId: string): StandardDataSourceConfig {
    const record = (data ?? {}) as UnknownRecord
    const rawSources = (record.dataSources || []) as Array<UnknownRecord | null | undefined>
    const dataSources = rawSources.map((ds) => {
      const dsRecord = (ds ?? {}) as UnknownRecord
      const rawItems = (dsRecord.dataItems || []) as unknown[]
      return this.createStandardDataSource(
        normalizePersistedSourceId(dsRecord, 'default'),
        rawItems.map((item): StandardDataItem => normalizePersistedDataItem(item)),
        normalizePersistedMergeStrategy(dsRecord.mergeStrategy)
      )
    })

    return this.createStandardConfig(componentId, dataSources, record.createdAt as number | undefined)
  }

  private static convertFromImportExport(data: unknown, componentId: string): StandardDataSourceConfig {
    const record = (data ?? {}) as UnknownRecord
    // 导入导出格式是单数据源壳结构，这里补成标准 dataSources 数组。
    const dataSourceConfig = (record.dataSourceConfig || {}) as UnknownRecord
    // 保持与原实现一致：非数组 truthy 值会在 .map 处抛出 TypeError。
    const rawItems = (dataSourceConfig.dataItems || []) as unknown[]
    const dataItems = rawItems.map((rawItem): StandardDataItem => normalizePersistedDataItem(rawItem))

    return this.createStandardConfig(componentId, [
      this.createStandardDataSource(
        normalizePersistedSourceId(dataSourceConfig, DEFAULT_SOURCE_ID),
        dataItems,
        normalizePersistedMergeStrategy(dataSourceConfig.mergeStrategy)
      )
    ])
  }

  private static convertFromCard2Executor(data: unknown, componentId: string): StandardDataSourceConfig {
    const entries = Object.entries((data ?? {}) as UnknownRecord)
    const dataSources = entries.map(([sourceId, sourceData]) => {
      const sourceRecord = (sourceData ?? {}) as UnknownRecord
      return this.createStandardDataSource(sourceId, [
        this.createStandardItem(sourceRecord.type || 'static', (sourceRecord.data || sourceRecord) as UnknownRecord)
      ])
    })

    return this.createStandardConfig(componentId, dataSources)
  }

  private static convertFromEditorManager(data: unknown, componentId: string): StandardDataSourceConfig {
    const record = (data ?? {}) as UnknownRecord
    return this.createStandardConfig(componentId, [
      this.createStandardDataSource(DEFAULT_SOURCE_ID, [
        this.createStandardItem(record.type || 'static', (record.config || record) as UnknownRecord, {
          filterPath: (record.filterPath || DEFAULT_FILTER_PATH) as string,
          customScript: record.processScript as string | undefined
        })
      ])
    ])
  }

  private static convertFromGenericObject(data: unknown, componentId: string): StandardDataSourceConfig {
    return this.createStandardConfig(componentId, [
      this.createStandardDataSource(DEFAULT_SOURCE_ID, [this.createStandardItem('static', data as UnknownRecord)])
    ])
  }

  private static convertToSimpleConfigEditor(standardData: StandardDataSourceConfig): unknown {
    return {
      dataSources: standardData.dataSources.map(ds => ({
        sourceId: ds.sourceId,
        dataItems: ds.dataItems,
        mergeStrategy: ds.mergeStrategy
      })),
      createdAt: standardData.createdAt,
      updatedAt: standardData.updatedAt
    }
  }

  private static convertToImportExport(standardData: StandardDataSourceConfig): unknown {
    const dataItems = standardData.dataSources.flatMap(ds => ds.dataItems.map(item => item.item))

    return {
      dataSourceConfig: {
        dataItems,
        mergeStrategy: standardData.dataSources[0]?.mergeStrategy || DEFAULT_MERGE_STRATEGY
      }
    }
  }

  private static convertToCard2Executor(standardData: StandardDataSourceConfig): unknown {
    const result: Record<string, unknown> = {}

    standardData.dataSources.forEach(ds => {
      ds.dataItems.forEach((item, index) => {
        // card2 executor 不支持一个 sourceId 下挂数组，因此多项时追加索引展开。
        const key = ds.dataItems.length === 1 ? ds.sourceId : `${ds.sourceId}_${index}`
        result[key] = {
          type: item.item.type,
          data: item.item.config,
          metadata: {
            sourceId: ds.sourceId,
            processing: item.processing
          }
        }
      })
    })

    return result
  }

  static normalizeMultiple(dataList: Array<{ data: unknown; componentId: string }>): StandardDataSourceConfig[] {
    return dataList.map(({ data, componentId }) => this.normalizeToStandard(data, componentId))
  }

  static validateStandardFormat(data: StandardDataSourceConfig): { valid: boolean; errors: string[] } {
    const errors: string[] = []

    if (!data.componentId) {
      errors.push('componentId is required')
    }

    if (!Array.isArray(data.dataSources)) {
      errors.push('dataSources must be an array')
    } else {
      data.dataSources.forEach((ds, dsIndex) => {
        if (!ds.sourceId) {
          errors.push(`dataSources[${dsIndex}].sourceId is required`)
        }

        if (!Array.isArray(ds.dataItems)) {
          errors.push(`dataSources[${dsIndex}].dataItems must be an array`)
        } else {
          ds.dataItems.forEach((item, itemIndex) => {
            if (!item.item || !item.processing) {
              errors.push(`dataSources[${dsIndex}].dataItems[${itemIndex}] must include item and processing`)
            }
          })
        }
      })
    }

    return {
      valid: errors.length === 0,
      errors
    }
  }

  private static createStandardConfig(
    componentId: string,
    dataSources: StandardDataSource[],
    createdAt?: number
  ): StandardDataSourceConfig {
    const now = Date.now()
    return {
      componentId,
      dataSources,
      createdAt: createdAt ?? now,
      updatedAt: now
    }
  }

  private static createStandardDataSource(
    sourceId: string,
    dataItems: StandardDataItem[],
    mergeStrategy: StandardDataSource['mergeStrategy'] = DEFAULT_MERGE_STRATEGY
  ): StandardDataSource {
    return {
      sourceId,
      dataItems,
      mergeStrategy
    }
  }

  private static createStandardItem(
    type: unknown,
    config: Record<string, unknown>,
    processing: Partial<StandardProcessing> = {}
  ): StandardDataItem {
    return {
      item: {
        type: normalizePersistedDataSourceType(type),
        config
      },
      processing: {
        filterPath: processing.filterPath ?? DEFAULT_FILTER_PATH,
        customScript: processing.customScript,
        defaultValue: processing.defaultValue
      }
    }
  }
}
