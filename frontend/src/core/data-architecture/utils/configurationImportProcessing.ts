import type { ExportedConfiguration, ImportPreview, ImportResult } from './ConfigurationImportExport'
import { formatImportExportError, restorePlaceholderDeep } from './configurationImportExportShared'

export type ConfigurationSection = 'dataSource' | 'component' | 'interaction'

export interface ConfigurationManagerLike {
  getConfiguration?: (componentId: string, section?: string) => any
  updateConfiguration?: (componentId: string, section: ConfigurationSection, data: any) => void | Promise<void>
  updateConfigurationSection?: (componentId: string, section: ConfigurationSection, data: any) => void | Promise<void>
}

export interface DependencyCheckResult {
  found: string[]
  missing: string[]
  conflicts: string[]
}

export interface ConfigurationConflictResult {
  dataSource: boolean
  component: boolean
  interaction: boolean
  error?: string
}

export function validateConfigurationFormat(config: any): boolean {
  return !!(config && config.version && config.exportTime && config.metadata && config.data)
}

export function parseConfigurationInput(configJson: string | ExportedConfiguration): ExportedConfiguration {
  const config = typeof configJson === 'string' ? JSON.parse(configJson) : configJson

  if (!validateConfigurationFormat(config)) {
    throw new Error('Invalid configuration format')
  }

  return config
}

function getAvailableComponentIds(
  targetComponentId: string,
  configurationManager: any,
  availableComponents?: any[]
): Set<string> {
  const ids = new Set<string>([targetComponentId])

  if (Array.isArray(availableComponents)) {
    availableComponents.forEach((component) => {
      const id =
        typeof component === 'string'
          ? component
          : component?.id || component?.componentId || component?.widgetId || component?.value
      if (typeof id === 'string') {
        ids.add(id)
      }
    })
  }

  const managerCandidates = [
    configurationManager?.store?.nodes,
    configurationManager?.nodes,
    configurationManager?.getNodes?.(),
    configurationManager?.getAllComponents?.()
  ]

  managerCandidates.forEach((candidate) => {
    if (!Array.isArray(candidate)) {
      return
    }

    candidate.forEach((component) => {
      const id = component?.id || component?.componentId || component?.widgetId || component?.value
      if (typeof id === 'string') {
        ids.add(id)
      }
    })
  })

  return ids
}

export function checkConfigurationDependencies(
  config: ExportedConfiguration,
  targetComponentId: string,
  configurationManager: any,
  availableComponents?: any[]
): DependencyCheckResult {
  const dependencies = config.metadata.dependencies || []
  const availableIds = getAvailableComponentIds(targetComponentId, configurationManager, availableComponents)

  const found = dependencies.filter((dep) => availableIds.has(dep))
  const missing = dependencies.filter((dep) => !availableIds.has(dep))
  const duplicateDependencies = dependencies.filter((dep, index) => dependencies.indexOf(dep) !== index)
  const conflicts = Array.from(new Set(duplicateDependencies)).map((dep) => `重复依赖组件: ${dep}`)

  return { found, missing, conflicts }
}

export function checkConfigurationConflicts(
  config: ExportedConfiguration,
  targetComponentId: string,
  configurationManager: ConfigurationManagerLike
): ConfigurationConflictResult {
  try {
    const existingConfig = configurationManager?.getConfiguration?.(targetComponentId)

    if (!existingConfig || !configurationManager) {
      return {
        dataSource: false,
        component: false,
        interaction: false
      }
    }

    return {
      dataSource: !!(
        existingConfig?.dataSource?.dataSources?.length && config.data.dataSourceConfiguration?.dataSources?.length
      ),
      component: !!(
        existingConfig?.component?.properties &&
        Object.keys(existingConfig.component.properties).length &&
        config.data.componentConfiguration?.properties &&
        Object.keys(config.data.componentConfiguration.properties).length
      ),
      interaction: !!(
        existingConfig?.interaction &&
        Object.keys(existingConfig.interaction).length &&
        config.data.interactionConfiguration &&
        Object.keys(config.data.interactionConfiguration).length
      )
    }
  } catch (error) {
    console.error('[ConfigurationImporter] Conflict detection failed:', error)
    return {
      dataSource: false,
      component: false,
      interaction: false,
      error: formatImportExportError(error)
    }
  }
}

export function getOverwriteConflictMessages(conflicts: ConfigurationConflictResult): string[] {
  const messages: string[] = []
  if (conflicts.dataSource) messages.push('数据源配置冲突')
  if (conflicts.component) messages.push('组件配置冲突')
  if (conflicts.interaction) messages.push('交互配置冲突')
  if (conflicts.error) messages.push(`冲突检测失败: ${conflicts.error}`)
  return messages
}

export function getConfigurationImportConflictMessages(
  missingDependencies: string[],
  dependencyConflicts: string[],
  overwriteConflicts: string[]
): string[] {
  return [
    ...missingDependencies.map((dep) => `缺失依赖组件: ${dep}`),
    ...dependencyConflicts,
    ...overwriteConflicts
  ]
}

export function buildConfigurationImportPreview(
  config: ExportedConfiguration,
  targetComponentId: string,
  configurationManager: any,
  availableComponents?: any[]
): ImportPreview {
  const dependencies = checkConfigurationDependencies(
    config,
    targetComponentId,
    configurationManager,
    availableComponents
  )
  const conflicts = checkConfigurationConflicts(config, targetComponentId, configurationManager)
  const conflictList: string[] = [
    ...dependencies.missing.map((dep) => `缺失依赖组件: ${dep}`),
    ...dependencies.conflicts,
    ...getOverwriteConflictMessages(conflicts)
  ]

  return {
    basicInfo: {
      version: config.version,
      exportTime: config.exportTime,
      componentType: config.componentType || '',
      exportSource: config.metadata?.exportSource || 'AetherLink IoT'
    },
    statistics: {
      dataSourceCount: config.metadata?.statistics?.dataSourceCount || 0,
      interactionCount: config.metadata?.statistics?.interactionCount || 0,
      httpConfigCount: config.metadata?.statistics?.httpConfigCount || 0
    },
    dependencies: dependencies.found,
    conflicts: conflictList
  }
}

export function createMissingDependencyWarnings(
  missingDependencies: string[],
  options: { skipMissingDependencies?: boolean }
): string[] {
  if (missingDependencies.length === 0 || !options.skipMissingDependencies) {
    return []
  }

  return [`跳过缺失的依赖组件: ${missingDependencies.join(', ')}`]
}

export function getMissingDependencyFailure(
  missingDependencies: string[],
  options: { skipMissingDependencies?: boolean },
  warnings: string[]
): ImportResult | undefined {
  if (missingDependencies.length === 0 || options.skipMissingDependencies) {
    return undefined
  }

  return {
    success: false,
    errors: [`缺失依赖组件: ${missingDependencies.join(', ')}`],
    warnings
  }
}

export function getOverwriteFailure(
  configurationConflicts: ConfigurationConflictResult,
  overwriteConflicts: string[],
  options: { overwriteExisting?: boolean },
  warnings: string[]
): ImportResult | undefined {
  if (configurationConflicts.error || (overwriteConflicts.length > 0 && !options.overwriteExisting)) {
    return {
      success: false,
      errors: overwriteConflicts,
      warnings
    }
  }

  return undefined
}

export function createSuccessfulImportResult(
  processedConfig: any,
  effectiveMissingDependencies: string[],
  dependencyCheck: DependencyCheckResult,
  warnings: string[]
): ImportResult {
  return {
    success: true,
    errors: [],
    warnings,
    importedData: processedConfig,
    dependencyValidation: {
      missing: effectiveMissingDependencies,
      found: dependencyCheck.found
    }
  }
}

export function processConfigurationForImport(
  config: ExportedConfiguration,
  targetComponentId: string,
  currentComponentPlaceholder: string
): {
  processedConfig: any
  missingDependencies: string[]
} {
  return {
    processedConfig: {
      dataSource: config.data.dataSourceConfiguration
        ? restorePlaceholderDeep(config.data.dataSourceConfiguration, currentComponentPlaceholder, targetComponentId)
        : undefined,
      component: config.data.componentConfiguration
        ? restorePlaceholderDeep(config.data.componentConfiguration, currentComponentPlaceholder, targetComponentId)
        : undefined,
      interaction: config.data.interactionConfiguration
        ? restorePlaceholderDeep(config.data.interactionConfiguration, currentComponentPlaceholder, targetComponentId)
        : undefined
    },
    missingDependencies: []
  }
}

async function updateConfigurationSection(
  configurationManager: ConfigurationManagerLike,
  componentId: string,
  section: ConfigurationSection,
  data: any
): Promise<void> {
  if (typeof configurationManager.updateConfiguration === 'function') {
    await configurationManager.updateConfiguration(componentId, section, data)
    return
  }

  await configurationManager.updateConfigurationSection!(componentId, section, data)
}

export async function applyImportedConfiguration(
  processedConfig: any,
  targetComponentId: string,
  configurationManager: ConfigurationManagerLike
): Promise<void> {
  if (
    !configurationManager ||
    (typeof configurationManager.updateConfiguration !== 'function' &&
      typeof configurationManager.updateConfigurationSection !== 'function')
  ) {
    const error = '配置管理器无效或未提供'
    console.error(`[ConfigurationImporter] ${error}`)
    throw new Error(error)
  }

  if (processedConfig.dataSource) {
    await updateConfigurationSection(configurationManager, targetComponentId, 'dataSource', processedConfig.dataSource)
  }

  if (processedConfig.component) {
    await updateConfigurationSection(configurationManager, targetComponentId, 'component', processedConfig.component)
  }

  if (processedConfig.interaction) {
    await updateConfigurationSection(
      configurationManager,
      targetComponentId,
      'interaction',
      processedConfig.interaction
    )
  }
}
