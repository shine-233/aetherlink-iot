/**
 * 文件用途: 复杂配置到 SimpleDataBridge 简化配置的转换适配器。
 * 核心逻辑: 把现有组件数据源配置归一成组件数据需求和简化数据源配置。
 * 关键注意事项: 字段映射需要兼容历史配置，默认值变化会影响已保存组件的数据执行。
 * 重构建议: 将 compatibility 字段归一、默认值填充和错误报告拆成独立转换步骤。
 */

import type { ComponentDataRequirement, SimpleDataSourceConfig } from '@/core/data-architecture/SimpleDataBridge'

type LooseRecord = Record<string, unknown>
type DataSourceBindingMap = LooseRecord
type BindingMapDescriptor = { bindings: DataSourceBindingMap; parseErrorMessage: string }

function hasConvertibleInput(config: unknown): boolean {
  return Boolean(config)
}

function hasOwnField(value: unknown, field: string): boolean {
  return Boolean(value && typeof value === 'object' && Object.prototype.hasOwnProperty.call(value, field))
}

function hasRawDataBindingMap(value: unknown): value is DataSourceBindingMap {
  return Boolean(
    value &&
    typeof value === 'object' &&
    !Array.isArray(value) &&
    Object.values(value as LooseRecord).some((binding) => hasOwnField(binding, 'rawData'))
  )
}

function hasConvertibleRawDataList(value: unknown): value is unknown[] {
  return (
    Array.isArray(value) &&
    value.some((item) => {
      const record = item as LooseRecord | null | undefined
      return Boolean(record && record.enabled !== false && (hasOwnField(record, 'rawData') || record.type))
    })
  )
}

function createStaticDataSource(id: string, data: unknown): SimpleDataSourceConfig {
  return {
    id,
    type: 'static',
    config: {
      data
    }
  }
}

function parseRawDataValue(rawData: unknown, sourceId: string, parseErrorMessage: string): unknown | null {
  if (typeof rawData !== 'string') {
    return rawData
  }

  try {
    return JSON.parse(rawData)
  } catch (error) {
    console.error(`${parseErrorMessage}: ${sourceId}`, error)
    return null
  }
}

function mapStaticRawDataSource(
  rawData: unknown,
  sourceId: string,
  parseErrorMessage: string
): SimpleDataSourceConfig | null {
  const parsedData = parseRawDataValue(rawData, sourceId, parseErrorMessage)

  if (parsedData === null) {
    return null
  }

  return createStaticDataSource(sourceId, parsedData)
}

function collectMappedDataSources<T>(
  items: T[],
  mapItem: (item: T, index: number) => SimpleDataSourceConfig | null
): SimpleDataSourceConfig[] {
  const dataSources: SimpleDataSourceConfig[] = []

  items.forEach((item, index) => {
    const dataSource = mapItem(item, index)

    if (dataSource) {
      dataSources.push(dataSource)
    }
  })

  return dataSources
}

function mapRawDataBinding(key: string, binding: unknown, parseErrorMessage: string): SimpleDataSourceConfig | null {
  if (!hasOwnField(binding, 'rawData')) {
    return null
  }

  return mapStaticRawDataSource((binding as LooseRecord).rawData, key, parseErrorMessage)
}

function normalizeDataSourceType(type: unknown): SimpleDataSourceConfig['type'] | null {
  if (type === 'api') {
    return 'http'
  }

  if (['static', 'http', 'json', 'websocket', 'file', 'data-source-bindings'].includes(String(type))) {
    return type as SimpleDataSourceConfig['type']
  }

  return null
}

function mapRawDataListItem(item: unknown, index: number, parseErrorMessage: string): SimpleDataSourceConfig | null {
  const record = item as LooseRecord | null | undefined
  if (!record || record.enabled === false) {
    return null
  }

  const sourceId = (record.id || record.sourceId || `dataSource${index + 1}`) as string

  if (hasOwnField(record, 'rawData')) {
    return mapStaticRawDataSource(record.rawData, sourceId, parseErrorMessage)
  }

  const normalizedType = normalizeDataSourceType(record.type)
  if (!normalizedType) {
    console.error(`[ConfigAdapter] UNSUPPORTED_DATA_SOURCE_TYPE: ${sourceId} (${String(record.type)})`)
    return null
  }

  return {
    id: sourceId,
    type: normalizedType,
    config: (record.config || {}) as LooseRecord,
    filterPath: record.filterPath as string | undefined,
    processScript: record.processScript as string | undefined
  }
}

function mapDataSourceBindings(bindings: DataSourceBindingMap, parseErrorMessage: string): SimpleDataSourceConfig[] {
  return collectMappedDataSources(Object.entries(bindings), ([key, binding]) =>
    mapRawDataBinding(key, binding, parseErrorMessage)
  )
}

function mapRawDataList(rawDataList: unknown[], parseErrorMessage: string): SimpleDataSourceConfig[] {
  return collectMappedDataSources(rawDataList, (item, index) => mapRawDataListItem(item, index, parseErrorMessage))
}

function collectBindingMaps(config: unknown): BindingMapDescriptor[] {
  const bindingMaps: BindingMapDescriptor[] = []
  const record = (config ?? {}) as LooseRecord
  const rawDataSources = record.rawDataSources as LooseRecord | null | undefined

  if (hasRawDataBindingMap(rawDataSources?.dataSourceBindings)) {
    bindingMaps.push({
      bindings: rawDataSources.dataSourceBindings as DataSourceBindingMap,
      parseErrorMessage: '[ConfigAdapter] Failed to parse rawDataSources rawData'
    })
  }

  if (hasRawDataBindingMap(record.bindings)) {
    bindingMaps.push({
      bindings: record.bindings as DataSourceBindingMap,
      parseErrorMessage: '[ConfigAdapter] Failed to parse bindings rawData'
    })
  }

  return bindingMaps
}

function mapConfiguredDataSources(config: unknown): SimpleDataSourceConfig[] {
  const dataSources: SimpleDataSourceConfig[] = []
  const record = (config ?? {}) as LooseRecord
  const nested = record.config as LooseRecord | null | undefined

  if (record.dataSourceBindings) {
    dataSources.push(
      ...mapDataSourceBindings(record.dataSourceBindings as DataSourceBindingMap, '❌ [ConfigAdapter] 解析rawData失败')
    )
  }

  if (nested?.dataSourceBindings) {
    dataSources.push(
      ...mapDataSourceBindings(
        nested.dataSourceBindings as DataSourceBindingMap,
        '❌ [ConfigAdapter] 解析嵌套rawData失败'
      )
    )
  }

  collectBindingMaps(config).forEach(({ bindings, parseErrorMessage }) => {
    dataSources.push(...mapDataSourceBindings(bindings, parseErrorMessage))
  })

  collectBindingMaps(nested || {}).forEach(({ bindings, parseErrorMessage }) => {
    dataSources.push(...mapDataSourceBindings(bindings, parseErrorMessage))
  })

  if (hasConvertibleRawDataList(record.rawDataList)) {
    dataSources.push(...mapRawDataList(record.rawDataList, '[ConfigAdapter] Failed to parse rawDataList rawData'))
  }

  if (hasConvertibleRawDataList(nested?.rawDataList)) {
    dataSources.push(
      ...mapRawDataList(nested.rawDataList, '[ConfigAdapter] Failed to parse nested rawDataList rawData')
    )
  }

  return dataSources
}

function isSimpleObjectConfig(config: unknown): boolean {
  const record = config as LooseRecord
  return (
    typeof record === 'object' &&
    !Array.isArray(record) &&
    !record.type &&
    !record.dataSourceBindings &&
    !record.rawDataSources &&
    !record.rawDataList &&
    !hasRawDataBindingMap(record.bindings) &&
    !record.config
  )
}

function mapDefaultCompatibleDataSource(config: unknown): SimpleDataSourceConfig[] {
  if (!isSimpleObjectConfig(config)) {
    return []
  }

  return [createStaticDataSource('main', config)]
}

function assembleRequirement(
  componentId: string,
  dataSources: SimpleDataSourceConfig[]
): ComponentDataRequirement | null {
  if (dataSources.length === 0) {
    console.error(`⚠️ [ConfigAdapter] 没有找到有效的数据源配置: ${componentId}`)
    return null
  }

  return {
    componentId,
    dataSources
  }
}

/**
 * 将复杂的数据源配置转换为简化格式
 * @param componentId 组件ID
 * @param config 原始配置对象
 * @returns 简化的组件数据需求
 */
export function convertToSimpleDataRequirement(componentId: string, config: unknown): ComponentDataRequirement | null {
  if (!hasConvertibleInput(config)) {
    return null
  }

  const dataSources = [...mapConfiguredDataSources(config), ...mapDefaultCompatibleDataSource(config)]

  return assembleRequirement(componentId, dataSources)
}

/**
 * 检查配置是否需要转换
 * @param config 配置对象
 * @returns 是否需要转换
 */
export function shouldConvertConfig(config: unknown): boolean {
  if (!config || typeof config !== 'object') {
    return false
  }

  const record = config as LooseRecord
  const nested = record.config as LooseRecord | null | undefined
  const rawDataSources = record.rawDataSources as LooseRecord | null | undefined
  const nestedRawDataSources = nested?.rawDataSources as LooseRecord | null | undefined

  if (
    record.dataSourceBindings ||
    nested?.dataSourceBindings ||
    hasRawDataBindingMap(rawDataSources?.dataSourceBindings) ||
    hasRawDataBindingMap(record.bindings) ||
    hasRawDataBindingMap(nestedRawDataSources?.dataSourceBindings) ||
    hasRawDataBindingMap(nested?.bindings) ||
    Array.isArray(record.rawDataList) ||
    Array.isArray(nested?.rawDataList)
  ) {
    return true
  }

  return !Array.isArray(config) && !record.type && !record.enabled && !record.metadata
}

/**
 * 从配置中提取组件类型
 * @param config 配置对象
 * @returns 组件类型
 */
export function extractComponentType(config: unknown): string {
  const record = config as LooseRecord | null | undefined
  const metadata = record?.metadata as LooseRecord | null | undefined
  return (metadata?.componentType || 'unknown') as string
}

/**
 * 批量转换多个组件配置
 * @param configs 配置映射 {componentId: config}
 * @returns 转换结果映射
 */
export function batchConvertConfigs(configs: Record<string, unknown>): Record<string, ComponentDataRequirement> {
  const results: Record<string, ComponentDataRequirement> = {}

  Object.entries(configs).forEach(([componentId, config]) => {
    if (shouldConvertConfig(config)) {
      const requirement = convertToSimpleDataRequirement(componentId, config)
      if (requirement) {
        results[componentId] = requirement
      }
    }
  })

  return results
}
