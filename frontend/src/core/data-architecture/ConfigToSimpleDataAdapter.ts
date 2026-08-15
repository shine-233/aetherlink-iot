/**
 * 文件用途: 复杂配置到 SimpleDataBridge 简化配置的转换适配器。
 * 核心逻辑: 把现有组件数据源配置归一成组件数据需求和简化数据源配置。
 * 关键注意事项: 字段映射需要兼容历史配置，默认值变化会影响已保存组件的数据执行。
 * 重构建议: 将 compatibility 字段归一、默认值填充和错误报告拆成独立转换步骤。
 */

import type { ComponentDataRequirement, SimpleDataSourceConfig } from '@/core/data-architecture/SimpleDataBridge'

type DataSourceBindingMap = Record<string, any>
type BindingMapDescriptor = { bindings: DataSourceBindingMap; parseErrorMessage: string }

function hasConvertibleInput(config: any): boolean {
  return Boolean(config)
}

function hasOwnField(value: any, field: string): boolean {
  return Boolean(value && typeof value === 'object' && Object.prototype.hasOwnProperty.call(value, field))
}

function hasRawDataBindingMap(value: any): value is DataSourceBindingMap {
  return Boolean(
    value &&
      typeof value === 'object' &&
      !Array.isArray(value) &&
      Object.values(value).some((binding) => hasOwnField(binding, 'rawData'))
  )
}

function hasConvertibleRawDataList(value: any): value is any[] {
  return (
    Array.isArray(value) &&
    value.some((item) => item && item.enabled !== false && (hasOwnField(item, 'rawData') || item.type))
  )
}

function createStaticDataSource(id: string, data: any): SimpleDataSourceConfig {
  return {
    id,
    type: 'static',
    config: {
      data
    }
  }
}

function parseRawDataValue(rawData: any, sourceId: string, parseErrorMessage: string): any | null {
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

function mapStaticRawDataSource(rawData: any, sourceId: string, parseErrorMessage: string): SimpleDataSourceConfig | null {
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

function mapRawDataBinding(key: string, binding: any, parseErrorMessage: string): SimpleDataSourceConfig | null {
  if (!hasOwnField(binding, 'rawData')) {
    return null
  }

  return mapStaticRawDataSource(binding.rawData, key, parseErrorMessage)
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

function mapRawDataListItem(item: any, index: number, parseErrorMessage: string): SimpleDataSourceConfig | null {
  if (!item || item.enabled === false) {
    return null
  }

  const sourceId = item.id || item.sourceId || `dataSource${index + 1}`

  if (hasOwnField(item, 'rawData')) {
    return mapStaticRawDataSource(item.rawData, sourceId, parseErrorMessage)
  }

  const normalizedType = normalizeDataSourceType(item.type)
  if (!normalizedType) {
    console.error(`[ConfigAdapter] UNSUPPORTED_DATA_SOURCE_TYPE: ${sourceId} (${String(item.type)})`)
    return null
  }

  return {
    id: sourceId,
    type: normalizedType,
    config: item.config || {},
    filterPath: item.filterPath,
    processScript: item.processScript
  }
}

function mapDataSourceBindings(bindings: DataSourceBindingMap, parseErrorMessage: string): SimpleDataSourceConfig[] {
  return collectMappedDataSources(Object.entries(bindings), ([key, binding]: [string, any]) =>
    mapRawDataBinding(key, binding, parseErrorMessage)
  )
}

function mapRawDataList(rawDataList: any[], parseErrorMessage: string): SimpleDataSourceConfig[] {
  return collectMappedDataSources(rawDataList, (item, index) => mapRawDataListItem(item, index, parseErrorMessage))
}

function collectBindingMaps(config: any): BindingMapDescriptor[] {
  const bindingMaps: BindingMapDescriptor[] = []

  if (hasRawDataBindingMap(config.rawDataSources?.dataSourceBindings)) {
    bindingMaps.push({
      bindings: config.rawDataSources.dataSourceBindings,
      parseErrorMessage: '[ConfigAdapter] Failed to parse rawDataSources rawData'
    })
  }

  if (hasRawDataBindingMap(config.bindings)) {
    bindingMaps.push({
      bindings: config.bindings,
      parseErrorMessage: '[ConfigAdapter] Failed to parse bindings rawData'
    })
  }

  return bindingMaps
}

function mapConfiguredDataSources(config: any): SimpleDataSourceConfig[] {
  const dataSources: SimpleDataSourceConfig[] = []

  if (config.dataSourceBindings) {
    dataSources.push(...mapDataSourceBindings(config.dataSourceBindings, '❌ [ConfigAdapter] 解析rawData失败'))
  }

  if (config.config?.dataSourceBindings) {
    dataSources.push(
      ...mapDataSourceBindings(config.config.dataSourceBindings, '❌ [ConfigAdapter] 解析嵌套rawData失败')
    )
  }

  collectBindingMaps(config).forEach(({ bindings, parseErrorMessage }) => {
    dataSources.push(...mapDataSourceBindings(bindings, parseErrorMessage))
  })

  collectBindingMaps(config.config || {}).forEach(({ bindings, parseErrorMessage }) => {
    dataSources.push(...mapDataSourceBindings(bindings, parseErrorMessage))
  })

  if (hasConvertibleRawDataList(config.rawDataList)) {
    dataSources.push(...mapRawDataList(config.rawDataList, '[ConfigAdapter] Failed to parse rawDataList rawData'))
  }

  if (hasConvertibleRawDataList(config.config?.rawDataList)) {
    dataSources.push(
      ...mapRawDataList(config.config.rawDataList, '[ConfigAdapter] Failed to parse nested rawDataList rawData')
    )
  }

  return dataSources
}

function isSimpleObjectConfig(config: any): boolean {
  return (
    typeof config === 'object' &&
    !Array.isArray(config) &&
    !config.type &&
    !config.dataSourceBindings &&
    !config.rawDataSources &&
    !config.rawDataList &&
    !hasRawDataBindingMap(config.bindings) &&
    !config.config
  )
}

function mapDefaultCompatibleDataSource(config: any): SimpleDataSourceConfig[] {
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
export function convertToSimpleDataRequirement(componentId: string, config: any): ComponentDataRequirement | null {
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
export function shouldConvertConfig(config: any): boolean {
  if (!config || typeof config !== 'object') {
    return false
  }

  if (
    config.dataSourceBindings ||
    config.config?.dataSourceBindings ||
    hasRawDataBindingMap(config.rawDataSources?.dataSourceBindings) ||
    hasRawDataBindingMap(config.bindings) ||
    hasRawDataBindingMap(config.config?.rawDataSources?.dataSourceBindings) ||
    hasRawDataBindingMap(config.config?.bindings) ||
    Array.isArray(config.rawDataList) ||
    Array.isArray(config.config?.rawDataList)
  ) {
    return true
  }

  return !Array.isArray(config) && !config.type && !config.enabled && !config.metadata
}

/**
 * 从配置中提取组件类型
 * @param config 配置对象
 * @returns 组件类型
 */
export function extractComponentType(config: any): string {
  return config?.metadata?.componentType || 'unknown'
}

/**
 * 批量转换多个组件配置
 * @param configs 配置映射 {componentId: config}
 * @returns 转换结果映射
 */
export function batchConvertConfigs(configs: Record<string, any>): Record<string, ComponentDataRequirement> {
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
