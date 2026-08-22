/**
 * 文件说明：
 * - 单数据源导入时的目标落点处理器。
 * - 负责查找/创建目标 slot、冲突检查、写回 dataSource 配置，并补挂相关交互与 HTTP 绑定。
 * - 这里是导入流程真正改写组件配置的入口，保护逻辑和报错文案需要保持清晰。
 */

import type { SingleDataSourceExport } from './ConfigurationImportExport'
import type { ConfigurationManagerLike } from './configurationImportProcessing'

/** 编辑器节点视图（导入工具只读取标识字段） */
interface EditorNodeLike {
  id?: unknown
  componentId?: unknown
  widgetId?: unknown
  type?: unknown
  componentType?: unknown
  [key: string]: unknown
}

/** 数据源槽位（导入落点，字段宽松） */
interface DataSourceSlotLike {
  sourceId?: unknown
  dataItems?: unknown[]
  mergeStrategy?: unknown
  processing?: Record<string, unknown> | null
  [key: string]: unknown
}

/** 组件完整配置的局部视图（导入流程只读写这些段） */
interface ComponentFullConfigLike {
  dataSource?: DataSourceConfigLike | null
  interaction?: { importedInteractions?: unknown[]; [key: string]: unknown } | null
  component?: { httpBindings?: Array<Record<string, unknown>>; [key: string]: unknown } | null
  [key: string]: unknown
}

/** 组件 dataSource 配置段 */
interface DataSourceConfigLike {
  dataSources?: DataSourceSlotLike[] | null
  createdAt?: number
  updatedAt?: number
  [key: string]: unknown
}

/** 单数据源导入目标处理所需的配置管理器视图（共享契约 + store/nodes 访问） */
type SingleDataSourceManagerLike = ConfigurationManagerLike & {
  store?: { nodes?: EditorNodeLike[] | null } | null
  nodes?: EditorNodeLike[] | null
  getNodes?: () => EditorNodeLike[] | null
  getAllComponents?: () => EditorNodeLike[] | null
}

interface SingleDataSourceTargetContext {
  fullConfig: ComponentFullConfigLike
  existingConfig: DataSourceConfigLike
  targetSlotIndex: number
}

function createDefaultMergeStrategy(): { type: 'object' } {
  return { type: 'object' }
}

function createEmptyDataSourceSlot(sourceId: string) {
  return {
    sourceId,
    dataItems: [],
    mergeStrategy: createDefaultMergeStrategy()
  }
}

function createEmptySlotPreview(slotIndex: number) {
  return {
    slotId: `dataSource${slotIndex + 1}`,
    slotIndex,
    isEmpty: true
  }
}

function getExistingComponentIds(targetComponentId: string, configurationManager: SingleDataSourceManagerLike): Set<string> {
  const ids = new Set<string>([targetComponentId])
  const candidates = [
    configurationManager?.store?.nodes,
    configurationManager?.nodes,
    configurationManager?.getNodes?.(),
    configurationManager?.getAllComponents?.()
  ]

  candidates.forEach((candidate) => {
    if (Array.isArray(candidate)) {
      candidate.forEach((item) => {
        const id = item?.id || item?.componentId || item?.widgetId
        if (typeof id === 'string') ids.add(id)
      })
    }
  })

  return ids
}

function getTargetComponentType(targetComponentId: string, configurationManager: SingleDataSourceManagerLike): string | undefined {
  const fullConfig = configurationManager?.getConfiguration?.(targetComponentId)
  const node = configurationManager?.store?.nodes?.find?.(item => item?.id === targetComponentId)

  return (
    fullConfig?.metadata?.componentType ||
    fullConfig?.component?.type ||
    fullConfig?.component?.properties?.type ||
    node?.type ||
    node?.componentType
  )
}

function ensureDataSourceSlots(dataSourceConfig: DataSourceConfigLike): void {
  if (!dataSourceConfig.dataSources || !Array.isArray(dataSourceConfig.dataSources)) {
    dataSourceConfig.dataSources = []
  }
}

function findOrCreateDataSourceSlot(dataSourceConfig: DataSourceConfigLike, targetSlotId: string): number {
  const existingSlotIndex = dataSourceConfig.dataSources.findIndex(source => source.sourceId === targetSlotId)
  if (existingSlotIndex !== -1) {
    return existingSlotIndex
  }

  dataSourceConfig.dataSources.push(createEmptyDataSourceSlot(targetSlotId))
  return dataSourceConfig.dataSources.length - 1
}

function isOccupiedDataSourceSlot(slot: DataSourceSlotLike | null | undefined): boolean {
  if (!slot || typeof slot !== 'object') {
    return false
  }

  if (Array.isArray(slot.dataItems) && slot.dataItems.length > 0) {
    return true
  }

  if (slot.processing && Object.keys(slot.processing).length > 0) {
    return true
  }

  return !!(slot.mergeStrategy && JSON.stringify(slot.mergeStrategy) !== JSON.stringify(createDefaultMergeStrategy()))
}

function assertDataSourceSlotWritable(
  targetSlot: DataSourceSlotLike | null | undefined,
  targetSlotId: string,
  options: { overwriteExisting?: boolean }
): void {
  if (isOccupiedDataSourceSlot(targetSlot) && !options.overwriteExisting) {
    throw new Error(`目标数据源槽位已存在配置: ${targetSlotId}`)
  }
}

function updateConfigurationSection(
  configurationManager: SingleDataSourceManagerLike,
  componentId: string,
  section: string,
  data: unknown
): void {
  if (typeof configurationManager.updateConfiguration === 'function') {
    configurationManager.updateConfiguration(componentId, section, data)
    return
  }

  configurationManager.updateConfigurationSection(componentId, section, data)
}

function prepareTargetDataSourceContext(
  targetComponentId: string,
  targetSlotId: string,
  configurationManager: SingleDataSourceManagerLike,
  options: { overwriteExisting?: boolean }
): SingleDataSourceTargetContext {
  const fullConfig = configurationManager.getConfiguration(targetComponentId)
  const existingConfig = fullConfig?.dataSource || {
    componentId: targetComponentId,
    dataSources: [],
    createdAt: Date.now(),
    updatedAt: Date.now()
  }

  ensureDataSourceSlots(existingConfig)
  // 导入只覆盖命中的 slot，其他数据源配置保持原样。
  const targetSlotIndex = findOrCreateDataSourceSlot(existingConfig, targetSlotId)
  assertDataSourceSlotWritable(existingConfig.dataSources[targetSlotIndex], targetSlotId, options)

  return {
    fullConfig,
    existingConfig,
    targetSlotIndex
  }
}

function applyDataSourceSlotImport(
  processedConfig: SingleDataSourceExport,
  targetSlotId: string,
  targetContext: SingleDataSourceTargetContext
): void {
  targetContext.existingConfig.dataSources[targetContext.targetSlotIndex] = {
    sourceId: targetSlotId,
    dataItems: processedConfig.dataSourceConfig?.dataItems || [],
    mergeStrategy: processedConfig.dataSourceConfig?.mergeStrategy || createDefaultMergeStrategy(),
    ...(processedConfig.dataSourceConfig?.processing && {
      processing: processedConfig.dataSourceConfig.processing
    })
  }

  targetContext.existingConfig.updatedAt = Date.now()
}

function appendImportedInteractions(
  processedConfig: SingleDataSourceExport,
  targetComponentId: string,
  configurationManager: SingleDataSourceManagerLike,
  fullConfig: ComponentFullConfigLike
): void {
  if (!processedConfig.relatedConfig?.interactions?.length) {
    return
  }

  const nextInteractionConfig = {
    ...(fullConfig?.interaction || {}),
    importedInteractions: [
      ...(fullConfig?.interaction?.importedInteractions || []),
      ...processedConfig.relatedConfig.interactions
    ]
  }
  updateConfigurationSection(configurationManager, targetComponentId, 'interaction', nextInteractionConfig)
}

function appendHttpBindings(
  processedConfig: SingleDataSourceExport,
  targetSlotId: string,
  targetComponentId: string,
  configurationManager: SingleDataSourceManagerLike,
  fullConfig: ComponentFullConfigLike
): void {
  if (!processedConfig.relatedConfig?.httpBindings?.length) {
    return
  }

  const nextComponentConfig = {
    ...(fullConfig?.component || {}),
    httpBindings: [
      ...(fullConfig?.component?.httpBindings || []),
      ...processedConfig.relatedConfig.httpBindings.map(binding => ({
        ...binding,
        sourceId: targetSlotId
      }))
    ]
  }
  updateConfigurationSection(configurationManager, targetComponentId, 'component', nextComponentConfig)
}

export function getAvailableSingleDataSourceSlots(componentId: string, configurationManager: SingleDataSourceManagerLike) {
  const slots: Array<{
    slotId: unknown
    slotIndex: number
    isEmpty: boolean
    currentConfig?: {
      dataItemCount: number
      mergeStrategy: unknown
    }
  }> = []

  try {
    const fullConfig = configurationManager?.getConfiguration?.(componentId)
    const dataSourceConfig = fullConfig?.dataSource

    if (
      !dataSourceConfig ||
      !Array.isArray(dataSourceConfig.dataSources) ||
      dataSourceConfig.dataSources.length === 0
    ) {
      for (let i = 0; i < 3; i++) {
        slots.push(createEmptySlotPreview(i))
      }
    } else {
      dataSourceConfig.dataSources.forEach((source, index) => {
        slots.push({
          slotId: source.sourceId,
          slotIndex: index,
          isEmpty: !source.dataItems || source.dataItems.length === 0,
          currentConfig:
            source.dataItems?.length > 0
              ? {
                  dataItemCount: source.dataItems.length,
                  mergeStrategy: source.mergeStrategy?.type || 'object'
                }
              : undefined
        })
      })
    }
  } catch (error) {
    console.error('[SingleDataSourceImporter] 获取数据源槽位失败', error)
  }

  return slots
}

export function checkSingleDataSourceImportConflicts(
  importData: SingleDataSourceExport,
  targetComponentId: string,
  configurationManager: SingleDataSourceManagerLike
): string[] {
  const conflicts: string[] = []

  try {
    if (importData.exportType !== 'single-datasource') {
      conflicts.push('导入文件不是单数据源配置')
    }

    if (!importData.dataSourceConfig?.dataItems?.length) {
      conflicts.push('导入文件不包含有效的数据项配置')
    }

    const dependencies = importData.mapping.dependencies || []
    const existingComponentIds = getExistingComponentIds(targetComponentId, configurationManager)
    const missingDependencies = dependencies.filter((dep) => !existingComponentIds.has(dep))
    if (missingDependencies.length > 0) {
      conflicts.push(`缺失外部依赖组件: ${missingDependencies.join(', ')}`)
    }

    const sourceComponentType = importData.sourceMetadata.componentType
    const targetComponentType = getTargetComponentType(targetComponentId, configurationManager)
    if (sourceComponentType && targetComponentType && sourceComponentType !== targetComponentType) {
      conflicts.push(`组件类型不匹配: ${sourceComponentType} -> ${targetComponentType}`)
    }
  } catch (error) {
    console.error('[SingleDataSourceImporter] 导入冲突检查失败', error)
  }

  return conflicts
}

export function applySingleDataSourceImportTarget(
  processedConfig: SingleDataSourceExport,
  targetComponentId: string,
  targetSlotId: string,
  configurationManager: SingleDataSourceManagerLike,
  options: { overwriteExisting?: boolean } = {}
): void {
  // 先写入 dataSource 主配置，再补挂导入附带的交互与 HTTP 绑定。
  const targetContext = prepareTargetDataSourceContext(
    targetComponentId,
    targetSlotId,
    configurationManager,
    options
  )

  applyDataSourceSlotImport(processedConfig, targetSlotId, targetContext)
  updateConfigurationSection(
    configurationManager,
    targetComponentId,
    'dataSource',
    targetContext.existingConfig
  )
  appendImportedInteractions(processedConfig, targetComponentId, configurationManager, targetContext.fullConfig)
  appendHttpBindings(
    processedConfig,
    targetSlotId,
    targetComponentId,
    configurationManager,
    targetContext.fullConfig
  )
}
