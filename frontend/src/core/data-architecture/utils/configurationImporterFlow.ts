/**
 * 文件用途：承接整组件配置导入的 preview / import 共享流程。
 * 核心逻辑：统一处理 parse -> plan -> preview/build 或 apply/result 的总装路径。
 * 关键注意事项：这里是 `ConfigurationImporter` 的流程层，不要把规则复制回 UI 或导入导出门面。
 */
import type {
  ConfigurationImportRuntimeManager,
  ExportedConfiguration,
  ImportPreview,
  ImportResult
} from './ConfigurationImportExport'
import {
  applyImportedConfiguration,
  createSuccessfulImportResult,
  parseConfigurationInput
} from './configurationImportProcessing'
import { formatImportExportError } from './configurationImportExportShared'
import {
  buildConfigurationImportPreviewFromPlan,
  planConfigurationImport,
  type ConfigurationImportOptions
} from './configurationImportPlan'

type GenerateConfigurationImportPreviewOptions = {
  availableComponents?: Array<Record<string, unknown>>
}

type ConfigurationImporterFlowOptions = {
  currentComponentPlaceholder: string
}

export const generateConfigurationImportPreview = (
  configJson: string | ExportedConfiguration,
  targetComponentId: string,
  configurationManager: ConfigurationImportRuntimeManager,
  { currentComponentPlaceholder }: ConfigurationImporterFlowOptions,
  { availableComponents }: GenerateConfigurationImportPreviewOptions = {}
): ImportPreview => {
  try {
    const config = parseConfigurationInput(configJson)
    const plan = planConfigurationImport(config, targetComponentId, configurationManager, currentComponentPlaceholder, {
      availableComponents,
      overwriteExisting: true,
      skipMissingDependencies: true
    })

    return buildConfigurationImportPreviewFromPlan(config, plan)
  } catch (error) {
    console.error('[ConfigurationImporter] 妫板嫯顫嶆径杈Е:', error)
    throw new Error(`Failed to generate import preview: ${formatImportExportError(error)}`)
  }
}

export const executeConfigurationImport = async (
  configJson: string | ExportedConfiguration,
  targetComponentId: string,
  configurationManager: ConfigurationImportRuntimeManager,
  { currentComponentPlaceholder }: ConfigurationImporterFlowOptions,
  options: ConfigurationImportOptions = {}
): Promise<ImportResult> => {
  try {
    const config = parseConfigurationInput(configJson)
    const plan = planConfigurationImport(
      config,
      targetComponentId,
      configurationManager,
      currentComponentPlaceholder,
      options
    )

    if (plan.failure) {
      return plan.failure
    }

    await applyImportedConfiguration(plan.processedConfig, targetComponentId, configurationManager)

    return createSuccessfulImportResult(
      plan.processedConfig,
      plan.effectiveMissingDependencies,
      plan.dependencyCheck,
      plan.warnings
    )
  } catch (error) {
    console.error('[ConfigurationImporter] import failed:', error)
    return {
      success: false,
      errors: [formatImportExportError(error)],
      warnings: []
    }
  }
}
