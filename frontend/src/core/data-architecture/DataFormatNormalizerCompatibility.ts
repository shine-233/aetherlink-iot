/**
 * 文件用途: DataFormatNormalizer 的历史配置兼容 helper。
 * 核心逻辑: 识别旧配置形状，并把旧数据源字段、原始 data item 和 mergeStrategy 归一化。
 * 关键注意事项: 只能收敛兼容判断，不能改写已持久化的 SimpleConfigurationEditor、rawData、_id 语义。
 * 重构建议: 后续若恢复配置编辑器组件，应继续复用这里的纯函数，避免在 UI 层重复兼容链。
 */

import type { StandardDataItem, StandardDataSourceConfig } from './DataFormatNormalizer'

type StandardMergeStrategy = StandardDataSourceConfig['dataSources'][number]['mergeStrategy']
type StandardDataSourceType = StandardDataItem['item']['type']

const ALLOWED_MERGE_STRATEGY_TYPES: StandardMergeStrategy['type'][] = ['object', 'array', 'replace']
const ALLOWED_DATA_SOURCE_TYPES: StandardDataSourceType[] = [
  'static',
  'http',
  'json',
  'websocket',
  'file',
  'data-source-bindings'
]

export function normalizePersistedDataSourceType(type: unknown): StandardDataSourceType {
  if (type === 'api') {
    return 'http'
  }

  if (ALLOWED_DATA_SOURCE_TYPES.includes(type as StandardDataSourceType)) {
    return type as StandardDataSourceType
  }

  throw new Error(`UNSUPPORTED_DATA_SOURCE_TYPE:${String(type)}`)
}

export function isStandardDataItem(item: any): item is StandardDataItem {
  return !!(
    item &&
    typeof item === 'object' &&
    item.item &&
    typeof item.item === 'object' &&
    ALLOWED_DATA_SOURCE_TYPES.includes(item.item.type) &&
    item.item.config &&
    typeof item.item.config === 'object' &&
    item.processing &&
    typeof item.processing === 'object' &&
    typeof item.processing.filterPath === 'string'
  )
}

export function isSimpleConfigEditorFormat(data: any): boolean {
  return !!(
    data &&
    typeof data === 'object' &&
    'dataSources' in data &&
    Array.isArray(data.dataSources) &&
    data.dataSources.some((ds: any) => ds && ('sourceId' in ds || 'id' in ds) && Array.isArray(ds.dataItems))
  )
}

export function isImportExportFormat(data: any): boolean {
  return !!(
    data &&
    typeof data === 'object' &&
    'dataSourceConfig' in data &&
    data.dataSourceConfig?.dataItems &&
    Array.isArray(data.dataSourceConfig.dataItems)
  )
}

export function normalizePersistedSourceId(source: any, fallback: string): string {
  return source?.sourceId || source?.id || fallback
}

export function normalizePersistedMergeStrategy(strategy: any): StandardMergeStrategy {
  if (
    typeof strategy === 'string' &&
    ALLOWED_MERGE_STRATEGY_TYPES.includes(strategy as StandardMergeStrategy['type'])
  ) {
    return { type: strategy as StandardMergeStrategy['type'] }
  }

  if (strategy && typeof strategy === 'object' && ALLOWED_MERGE_STRATEGY_TYPES.includes(strategy.type)) {
    return strategy as StandardMergeStrategy
  }

  return { type: 'object' }
}

export function normalizePersistedDataItem(rawItem: any): StandardDataItem {
  if (isStandardDataItem(rawItem)) {
    return rawItem
  }

  const wrappedItem = rawItem?.item && typeof rawItem.item === 'object' ? rawItem.item : rawItem ?? {}
  const processing = rawItem?.processing ?? {}

  return {
    item: {
      type: normalizePersistedDataSourceType(wrappedItem.type ?? 'static'),
      config: wrappedItem.config ?? wrappedItem.data ?? wrappedItem
    },
    processing: {
      filterPath: processing.filterPath ?? rawItem?.filterPath ?? '$',
      customScript: processing.customScript ?? rawItem?.customScript ?? rawItem?.processScript,
      defaultValue: processing.defaultValue ?? rawItem?.defaultValue
    }
  }
}
