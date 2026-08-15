/**
 * 文件说明：
 * - 单数据源导入时的目标落点处理器。
 * - 负责查找/创建目标 slot、冲突检查、写回 dataSource 配置，并补挂相关交互与 HTTP 绑定。
 * - 这里是导入流程真正改写组件配置的入口，保护逻辑和报错文案需要保持清晰。
 */

import type { SingleDataSourceExport } from './ConfigurationImportExport'

interface SingleDataSourceTargetContext {
  fullConfig: any
  existingConfig: any
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

function getExistingComponentIds(targetComponentId: string, configurationManager: any): Set<string> {
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

function getTargetComponentType(targetComponentId: string, configurationManager: any): string | undefined {
  const fullConfig = configurationManager?.getConfiguration?.(targetComponentId)
  const node = configurationManager?.store?.nodes?.find?.((item: any) => item?.id === targetComponentId)

  return (
    fullConfig?.metadata?.componentType ||
    fullConfig?.component?.type ||
    fullConfig?.component?.properties?.type ||
    node?.type ||
    node?.componentType
  )
}

function ensureDataSourceSlots(dataSourceConfig: any): void {
  if (!dataSourceConfig.dataSources || !Array.isArray(dataSourceConfig.dataSources)) {
    dataSourceConfig.dataSources = []
  }
}

function findOrCreateDataSourceSlot(dataSourceConfig: any, targetSlotId: string): number {
  const existingSlotIndex = dataSourceConfig.dataSources.findIndex((source: any) => source.sourceId === targetSlotId)
  if (existingSlotIndex !== -1) {
    return existingSlotIndex
  }

  dataSourceConfig.dataSources.push(createEmptyDataSourceSlot(targetSlotId))
  return dataSourceConfig.dataSources.length - 1
}

function isOccupiedDataSourceSlot(slot: any): boolean {
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
  targetSlot: any,
  targetSlotId: string,
  options: { overwriteExisting?: boolean }
): void {
  if (isOccupiedDataSourceSlot(targetSlot) && !options.overwriteExisting) {
    throw new Error(`目标数据源槽位已存在配置: ${targetSlotId}`)
  }
}

function updateConfigurationSection(configurationManager: any, componentId: string, section: string, data: any): void {
  if (typeof configurationManager.updateConfiguration === 'function') {
    configurationManager.updateConfiguration(componentId, section, data)
    return
  }

  configurationManager.updateConfigurationSection(componentId, section, data)
}

function prepareTargetDataSourceContext(
  targetComponentId: string,
  targetSlotId: string,
  configurationManager: any,
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
  configurationManager: any,
  fullConfig: any
): void {
  if (!processedConfig.relatedConfig?.interactions?.length) {
    return
  }

  const nextInteractionConfig = {
    ...(fullConfig?.interaction || {}),
    importedInteractions: [
      ...((fullConfig?.interaction as any)?.importedInteractions || []),
      ...processedConfig.relatedConfig.interactions
    ]
  }
  updateConfigurationSection(configurationManager, targetComponentId, 'interaction', nextInteractionConfig)
}

function appendHttpBindings(
  processedConfig: SingleDataSourceExport,
  targetSlotId: string,
  targetComponentId: string,
  configurationManager: any,
  fullConfig: any
): void {
  if (!processedConfig.relatedConfig?.httpBindings?.length) {
    return
  }

  const nextComponentConfig = {
    ...(fullConfig?.component || {}),
    httpBindings: [
      ...((fullConfig?.component as any)?.httpBindings || []),
      ...processedConfig.relatedConfig.httpBindings.map((binding: any) => ({
        ...binding,
        sourceId: targetSlotId
      }))
    ]
  }
  updateConfigurationSection(configurationManager, targetComponentId, 'component', nextComponentConfig)
}

export function getAvailableSingleDataSourceSlots(componentId: string, configurationManager: any) {
  const slots: Array<{
    slotId: string
    slotIndex: number
    isEmpty: boolean
    currentConfig?: any
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
      dataSourceConfig.dataSources.forEach((source: any, index: number) => {
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
  configurationManager: any
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
  configurationManager: any,
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
