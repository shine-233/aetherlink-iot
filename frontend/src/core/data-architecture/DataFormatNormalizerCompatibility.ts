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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export function isStandardDataItem(item: unknown): item is StandardDataItem {
  const record = item as Record<string, unknown> | null | undefined
  return !!(
    record &&
    typeof record === 'object' &&
    record.item &&
    typeof record.item === 'object' &&
    ALLOWED_DATA_SOURCE_TYPES.includes((record.item as Record<string, unknown>).type as StandardDataSourceType) &&
    (record.item as Record<string, unknown>).config &&
    typeof (record.item as Record<string, unknown>).config === 'object' &&
    record.processing &&
    typeof record.processing === 'object' &&
    typeof (record.processing as Record<string, unknown>).filterPath === 'string'
  )
}

export function isSimpleConfigEditorFormat(data: unknown): boolean {
  const record = data as Record<string, unknown> | null | undefined
  return !!(
    record &&
    typeof record === 'object' &&
    'dataSources' in record &&
    Array.isArray(record.dataSources) &&
    (record.dataSources as unknown[]).some((ds) => {
      const dsRecord = ds as Record<string, unknown> | null | undefined
      return !!(dsRecord && ('sourceId' in dsRecord || 'id' in dsRecord) && Array.isArray(dsRecord.dataItems))
    })
  )
}

export function isImportExportFormat(data: unknown): boolean {
  const record = data as Record<string, unknown> | null | undefined
  return !!(
    record &&
    typeof record === 'object' &&
    'dataSourceConfig' in record &&
    (record.dataSourceConfig as Record<string, unknown> | null | undefined)?.dataItems &&
    Array.isArray((record.dataSourceConfig as Record<string, unknown>).dataItems)
  )
}

export function normalizePersistedSourceId(source: unknown, fallback: string): string {
  const sourceRecord = source as { sourceId?: unknown; id?: unknown } | null | undefined
  return (sourceRecord?.sourceId || sourceRecord?.id || fallback) as string
}

export function normalizePersistedMergeStrategy(strategy: unknown): StandardMergeStrategy {
  if (
    typeof strategy === 'string' &&
    ALLOWED_MERGE_STRATEGY_TYPES.includes(strategy as StandardMergeStrategy['type'])
  ) {
    return { type: strategy as StandardMergeStrategy['type'] }
  }

  if (
    strategy &&
    typeof strategy === 'object' &&
    ALLOWED_MERGE_STRATEGY_TYPES.includes(
      ((strategy as Record<string, unknown>).type ?? null) as StandardMergeStrategy['type']
    )
  ) {
    return strategy as StandardMergeStrategy
  }

  return { type: 'object' }
}

export function normalizePersistedDataItem(rawItem: unknown): StandardDataItem {
  if (isStandardDataItem(rawItem)) {
    return rawItem
  }

  const raw = rawItem as Record<string, unknown> | null | undefined
  const wrappedItem = (raw?.item && typeof raw.item === 'object'
    ? raw.item
    : rawItem ?? {}) as Record<string, unknown>
  const processing = (raw?.processing ?? {}) as Record<string, unknown>

  return {
    item: {
      type: normalizePersistedDataSourceType(wrappedItem.type ?? 'static'),
      config: (wrappedItem.config ?? wrappedItem.data ?? wrappedItem) as Record<string, unknown>
    },
    processing: {
      filterPath: (processing.filterPath ?? raw?.filterPath ?? '$') as string,
      customScript: (processing.customScript ?? raw?.customScript ?? raw?.processScript) as string | undefined,
      defaultValue: processing.defaultValue ?? raw?.defaultValue
    }
  }
}
