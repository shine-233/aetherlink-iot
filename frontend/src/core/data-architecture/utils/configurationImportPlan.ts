import type { ExportedConfiguration, ImportPreview, ImportResult } from './ConfigurationImportExport'
import {
  checkConfigurationConflicts,
  checkConfigurationDependencies,
  createMissingDependencyWarnings,
  getConfigurationImportConflictMessages,
  getMissingDependencyFailure,
  getOverwriteConflictMessages,
  getOverwriteFailure,
  processConfigurationForImport,
  type DependencyCheckResult
} from './configurationImportProcessing'

export type ConfigurationImportOptions = {
  overwriteExisting?: boolean
  skipMissingDependencies?: boolean
  availableComponents?: any[]
}

export type ConfigurationImportPlan = {
  configurationConflicts: ReturnType<typeof checkConfigurationConflicts>
  conflictMessages: string[]
  dependencyCheck: DependencyCheckResult
  effectiveMissingDependencies: string[]
  failure?: ImportResult
  processedConfig: any
  warnings: string[]
}

export const planConfigurationImport = (
  config: ExportedConfiguration,
  targetComponentId: string,
  configurationManager: any,
  currentComponentPlaceholder: string,
  options: ConfigurationImportOptions = {}
): ConfigurationImportPlan => {
  const dependencyCheck = checkConfigurationDependencies(
    config,
    targetComponentId,
    configurationManager,
    options.availableComponents
  )

  const { processedConfig, missingDependencies } = processConfigurationForImport(
    config,
    targetComponentId,
    currentComponentPlaceholder
  )

  const effectiveMissingDependencies = Array.from(new Set([...dependencyCheck.missing, ...missingDependencies]))
  const warnings = createMissingDependencyWarnings(effectiveMissingDependencies, options)

  const dependencyFailure = getMissingDependencyFailure(effectiveMissingDependencies, options, warnings)
  if (dependencyFailure) {
    const configurationConflicts = checkConfigurationConflicts(config, targetComponentId, configurationManager)
    const conflictMessages = getConfigurationImportConflictMessages(
      effectiveMissingDependencies,
      dependencyCheck.conflicts,
      getOverwriteConflictMessages(configurationConflicts)
    )
    return {
      configurationConflicts,
      conflictMessages,
      dependencyCheck,
      effectiveMissingDependencies,
      failure: dependencyFailure,
      processedConfig,
      warnings
    }
  }

  const configurationConflicts = checkConfigurationConflicts(config, targetComponentId, configurationManager)
  const overwriteConflicts = getOverwriteConflictMessages(configurationConflicts)
  const overwriteFailure = getOverwriteFailure(configurationConflicts, overwriteConflicts, options, warnings)
  const conflictMessages = getConfigurationImportConflictMessages(
    effectiveMissingDependencies,
    dependencyCheck.conflicts,
    overwriteConflicts
  )

  return {
    configurationConflicts,
    conflictMessages,
    dependencyCheck,
    effectiveMissingDependencies,
    failure: overwriteFailure,
    processedConfig,
    warnings
  }
}

export const buildConfigurationImportPreviewFromPlan = (
  config: ExportedConfiguration,
  plan: ConfigurationImportPlan
): ImportPreview => ({
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
  dependencies: plan.dependencyCheck.found,
  conflicts: plan.conflictMessages
})
