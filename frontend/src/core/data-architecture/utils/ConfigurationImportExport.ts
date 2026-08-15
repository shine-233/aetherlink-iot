/**
 * 文件用途：承接可视化编辑器配置的整组件/单数据源导入导出入口。
 * 核心逻辑：统一组织导出 payload、依赖占位符、冲突预检、单数据源落点选择与导入写回。
 * 关键注意事项：这里是导入导出规则门面，允许下沉 helper，但不要把规则复制回 UI 组件层。
 * 重构建议：持续收紧 `configurationManager` 边界，优先通过显式接口约束能力，而不是继续放大 `any`。
 */
import type { DataSourceConfiguration } from '@/core/data-architecture/index'
import type { configurationIntegrationBridge } from '@/components/visual-editor/configuration/ConfigurationIntegrationBridge'
import type { ConfigurationManagerLike } from './configurationImportProcessing'
import { smartDeepClone } from '@/utils/deep-clone'
import {
  applySingleDataSourceImportTarget,
  checkSingleDataSourceImportConflicts,
  getAvailableSingleDataSourceSlots
} from './singleDataSourceImportTarget'
import {
  collectRelatedSingleDataSourceConfig,
  processSingleDataSourceForExport
} from './singleDataSourceExportProcessing'
import { formatImportExportError } from './configurationImportExportShared'
import { processConfigurationForExport } from './configurationExportProcessing'
import { type ConfigurationImportOptions } from './configurationImportPlan'
import { executeConfigurationImport, generateConfigurationImportPreview } from './configurationImporterFlow'
import { processSingleDataSourceImportPayload } from './singleDataSourceImportProcessing'

export interface ConfigurationImportRuntimeManager extends ConfigurationManagerLike {
  store?: {
    nodes?: unknown[]
  }
  nodes?: unknown[]
  getNodes?: () => unknown[]
  getAllComponents?: () => unknown[]
}

/**
 * Configuration import/export note. */
export interface ExportedConfiguration {
  /** Configuration import/export field. */
  version: string
  /** Configuration import/export field. */
  exportTime: number
  /** Configuration import/export field. */
  componentType?: string
  /** Configuration import/export field. */
  metadata: {
    /** Configuration import/export field. */
    originalComponentId: string
    /** Configuration import/export field. */
    exportSource: string
    /** Configuration import/export field. */
    dependencies: string[]
    /** Configuration import/export field. */
    statistics: {
      dataSourceCount: number
      interactionCount: number
      httpConfigCount: number
    }
  }
  /** Configuration import/export field. */
  data: {
    /** Configuration import/export field. */
    dataSourceConfiguration?: any
    /** 缂佸嫪娆㈤柊宥囩枂 */
    componentConfiguration?: any
    /** 娴溿倓绨伴柊宥囩枂 */
    interactionConfiguration?: any
  }
  /** Configuration import/export field. */
  mapping: {
    /** Configuration import/export field. */
    placeholders: {
      [placeholder: string]: 'current_component' | 'external_component'
    }
    /** Configuration import/export field. */
    dependencies: {
      [externalComponentId: string]: {
        usage: string[]
        required: boolean
      }
    }
  }
}

/**
 * Configuration import/export note. */
export interface ImportResult {
  /** Configuration import/export field. */
  success: boolean
  /** Configuration import/export field. */
  errors: string[]
  /** Configuration import/export field. */
  warnings: string[]
  /** Configuration import/export field. */
  importedData?: any
  /** Configuration import/export field. */
  dependencyValidation?: {
    missing: string[]
    found: string[]
  }
}

/**
 * Configuration import/export note. */
export interface ImportPreview {
  /** Configuration import/export field. */
  basicInfo: {
    version: string
    exportTime: number
    componentType: string
    exportSource: string
  }
  /** Configuration import/export field. */
  statistics: {
    dataSourceCount: number
    interactionCount: number
    httpConfigCount: number
  }
  /** Configuration import/export field. */
  dependencies: string[]
  /** Configuration import/export field. */
  conflicts: string[]
}

/**
 * Configuration import/export note. */
export interface SingleDataSourceExport {
  /** Configuration import/export field. */
  version: string
  /** Configuration import/export field. */
  exportType: 'single-datasource'
  /** Configuration import/export field. */
  exportTime: number
  /** Configuration import/export field. */
  sourceMetadata: {
    /** Configuration import/export field. */
    originalSourceId: string
    /** Configuration import/export field. */
    sourceIndex: number
    /** Configuration import/export field. */
    originalComponentId: string
    /** Configuration import/export field. */
    exportSource: string
    /** Configuration import/export field. */
    componentType?: string
  }
  /** Configuration import/export field. */
  dataSourceConfig: {
    /** Configuration import/export field. */
    dataItems: any[]
    /** Configuration import/export field. */
    mergeStrategy: any
    /** Configuration import/export field. */
    processing?: any
  }
  /** Configuration import/export field. */
  relatedConfig: {
    /** Configuration import/export field. */
    interactions: any[]
    /** Configuration import/export field. */
    httpBindings: any[]
  }
  /** Configuration import/export field. */
  mapping: {
    /** Configuration import/export field. */
    placeholders: Record<string, string>
    /** Configuration import/export field. */
    dependencies: string[]
  }
}

/**
 * Configuration import/export note. */
export interface SingleDataSourceImportPreview {
  /** Configuration import/export field. */
  basicInfo: {
    version: string
    exportType: string
    exportTime: number
    originalSourceId: string
    sourceIndex: number
    exportSource: string
  }
  /** Configuration import/export field. */
  configSummary: {
    dataItemCount: number
    mergeStrategy: string
    hasProcessing: boolean
  }
  /** Configuration import/export field. */
  relatedConfig: {
    interactionCount: number
    httpBindingCount: number
  }
  /** Configuration import/export field. */
  dependencies: string[]
  /** Configuration import/export field. */
  conflicts: string[]
  /** Configuration import/export field. */
  availableSlots: Array<{
    slotId: string
    slotIndex: number
    isEmpty: boolean
    currentConfig?: any
  }>
}

/**
 * Configuration import/export note.
 */
export class ConfigurationExporter {
  private readonly CURRENT_COMPONENT_PLACEHOLDER = '__CURRENT_COMPONENT__'
  private readonly EXPORT_VERSION = '1.0.0'
  // Persisted import/export source identifier for existing files, not a current UI component name.
  private readonly EXPORT_SOURCE_IDENTIFIER = 'SimpleConfigurationEditor'

  /**
   * Configuration import/export note.
   * Configuration import/export note.
   * Configuration import/export note. */
  async exportConfiguration(
    componentId: string,
    configurationManager: ConfigurationManagerLike,
    componentType?: string
  ): Promise<ExportedConfiguration> {
    // Configuration import/export step.
    const fullConfig = configurationManager.getConfiguration(componentId)
    if (!fullConfig) {
      throw new Error(`组件 ${componentId} 的配置不存在`)
    }

    // Configuration import/export step.
    const { processedConfig, dependencies, statistics, dependencyMapping } = processConfigurationForExport(
      fullConfig,
      componentId,
      this.CURRENT_COMPONENT_PLACEHOLDER
    )

    // Configuration import/export step.
    const exportedConfig: ExportedConfiguration = {
      version: this.EXPORT_VERSION,
      exportTime: Date.now(),
      componentType,
      metadata: {
        originalComponentId: componentId,
        exportSource: this.EXPORT_SOURCE_IDENTIFIER,
        dependencies,
        statistics
      },
      data: {
        dataSourceConfiguration: processedConfig.dataSource,
        componentConfiguration: processedConfig.component,
        interactionConfiguration: processedConfig.interaction
      },
      mapping: {
        placeholders: {
          [this.CURRENT_COMPONENT_PLACEHOLDER]: 'current_component'
        },
        dependencies: dependencyMapping
      }
    }

    return exportedConfig
  }
}

/**
 * Configuration import service.
 */
export class ConfigurationImporter {
  private readonly CURRENT_COMPONENT_PLACEHOLDER = '__CURRENT_COMPONENT__'

  generateImportPreview(
    configJson: string | ExportedConfiguration,
    targetComponentId: string,
    configurationManager: ConfigurationImportRuntimeManager,
    availableComponents?: any[]
  ): ImportPreview {
    return generateConfigurationImportPreview(
      configJson,
      targetComponentId,
      configurationManager,
      { currentComponentPlaceholder: this.CURRENT_COMPONENT_PLACEHOLDER },
      { availableComponents }
    )
  }

  /**
   * Configuration import/export note.
   * Configuration import/export note.
   */
  async importConfiguration(
    configJson: string | ExportedConfiguration,
    targetComponentId: string,
    configurationManager: ConfigurationImportRuntimeManager,
    options: ConfigurationImportOptions = {}
  ): Promise<ImportResult> {
    return executeConfigurationImport(
      configJson,
      targetComponentId,
      configurationManager,
      { currentComponentPlaceholder: this.CURRENT_COMPONENT_PLACEHOLDER },
      options
    )
  }
}

/**
 * Configuration import/export note.
 * Configuration import/export note. */
export class SingleDataSourceExporter {
  private readonly CURRENT_COMPONENT_PLACEHOLDER = '__CURRENT_COMPONENT__'
  private readonly EXPORT_VERSION = '1.0.0'

  /**
   * Configuration import/export note.
   * Configuration import/export note.
   * Configuration import/export note.
   */
  async exportSingleDataSource(
    componentId: string,
    sourceId: string,
    configurationManager: ConfigurationImportRuntimeManager,
    componentType?: string
  ): Promise<SingleDataSourceExport> {
    if (!configurationManager) {
      throw new Error('Configuration manager is required')
    }

    try {
      const fullConfig = configurationManager.getConfiguration(componentId)

      const dataSourceConfig = fullConfig?.dataSource
      if (!dataSourceConfig || !dataSourceConfig.dataSources) {
        throw new Error('Data source configuration is missing')
      }

      // Configuration import/export step.
      const targetSourceIndex = dataSourceConfig.dataSources.findIndex((source: any) => source.sourceId === sourceId)
      if (targetSourceIndex === -1) {
        throw new Error(`Data source not found: ${sourceId}`)
      }

      const targetSource = dataSourceConfig.dataSources[targetSourceIndex]
      const dependencies = new Set<string>()

      const processedDataSourceConfig = processSingleDataSourceForExport(
        smartDeepClone(targetSource),
        componentId,
        this.CURRENT_COMPONENT_PLACEHOLDER,
        dependencies
      )

      // Configuration import/export step.
      const relatedConfig = collectRelatedSingleDataSourceConfig(
        componentId,
        sourceId,
        configurationManager,
        this.CURRENT_COMPONENT_PLACEHOLDER,
        dependencies
      )

      const exportData: SingleDataSourceExport = {
        version: this.EXPORT_VERSION,
        exportType: 'single-datasource',
        exportTime: Date.now(),
        sourceMetadata: {
          originalSourceId: sourceId,
          sourceIndex: targetSourceIndex,
          originalComponentId: componentId,
          exportSource: 'SingleDataSourceExporter',
          componentType
        },
        dataSourceConfig: {
          dataItems: processedDataSourceConfig.dataItems || [],
          mergeStrategy: processedDataSourceConfig.mergeStrategy || { type: 'object' },
          processing: processedDataSourceConfig.processing
        },
        relatedConfig,
        mapping: {
          placeholders: {
            [this.CURRENT_COMPONENT_PLACEHOLDER]: 'current_component'
          },
          dependencies: Array.from(dependencies)
        }
      }

      return exportData
    } catch (error) {
      console.error('[SingleDataSourceExporter] export failed:', error)
      throw new Error(`Single data source export failed: ${formatImportExportError(error)}`)
    }
  }

  /**
   * Configuration import/export note. */
  getAvailableDataSources(
    componentId: string,
    configurationManager: ConfigurationImportRuntimeManager
  ): Array<{
    sourceId: string
    sourceIndex: number
    hasData: boolean
    dataItemCount: number
  }> {
    try {
      const fullConfig = configurationManager.getConfiguration(componentId)
      const dataSourceConfig = fullConfig?.dataSource
      if (!dataSourceConfig || !dataSourceConfig.dataSources) {
        return []
      }

      return dataSourceConfig.dataSources.map((source: any, index: number) => ({
        sourceId: source.sourceId,
        sourceIndex: index,
        hasData: !!(source.dataItems && source.dataItems.length > 0),
        dataItemCount: source.dataItems?.length || 0
      }))
    } catch (error) {
      console.error('[SingleDataSourceExporter] export failed:', error)
      return []
    }
  }
}

/**
 * Configuration import/export note.
 */
export class SingleDataSourceImporter {
  private readonly CURRENT_COMPONENT_PLACEHOLDER = '__CURRENT_COMPONENT__'

  /**
   * Configuration import/export note.
   */
  generateImportPreview(
    importData: SingleDataSourceExport,
    targetComponentId: string,
    configurationManager: ConfigurationImportRuntimeManager
  ): SingleDataSourceImportPreview {
    try {
      const availableSlots = this.getAvailableDataSourceSlots(targetComponentId, configurationManager)

      const dependencies = importData.mapping.dependencies || []
      const conflicts = this.checkImportConflicts(importData, targetComponentId, configurationManager)

      return {
        basicInfo: {
          version: importData.version,
          exportType: importData.exportType,
          exportTime: importData.exportTime,
          originalSourceId: importData.sourceMetadata.originalSourceId,
          sourceIndex: importData.sourceMetadata.sourceIndex,
          exportSource: importData.sourceMetadata.exportSource
        },
        configSummary: {
          dataItemCount: importData.dataSourceConfig.dataItems.length,
          mergeStrategy: importData.dataSourceConfig.mergeStrategy.type || 'object',
          hasProcessing: !!importData.dataSourceConfig.processing
        },
        relatedConfig: {
          interactionCount: importData.relatedConfig.interactions.length,
          httpBindingCount: importData.relatedConfig.httpBindings.length
        },
        dependencies,
        conflicts,
        availableSlots
      }
    } catch (error) {
      console.error('[SingleDataSourceImporter] import failed:', error)
      throw new Error(`Single data source import preview failed: ${formatImportExportError(error)}`)
    }
  }

  /**
   * Configuration import/export note. */
  private getAvailableDataSourceSlots(componentId: string, configurationManager: ConfigurationImportRuntimeManager) {
    return getAvailableSingleDataSourceSlots(componentId, configurationManager)
  }

  /**
   * Configuration import/export note. */
  private checkImportConflicts(
    importData: SingleDataSourceExport,
    targetComponentId: string,
    configurationManager: ConfigurationImportRuntimeManager
  ): string[] {
    return checkSingleDataSourceImportConflicts(importData, targetComponentId, configurationManager)
  }

  /**
   * Configuration import/export note. */
  async importSingleDataSource(
    importData: SingleDataSourceExport,
    targetComponentId: string,
    targetSlotId: string,
    configurationManager: ConfigurationImportRuntimeManager,
    options: {
      overwriteExisting?: boolean
    } = {}
  ): Promise<void> {
    if (
      !configurationManager ||
      (typeof configurationManager.updateConfiguration !== 'function' &&
        typeof configurationManager.updateConfigurationSection !== 'function')
    ) {
      throw new Error('Configuration manager must support configuration updates')
    }

    try {
      this.assertNoSingleDataSourceConflicts(importData, targetComponentId, configurationManager)
      const processedConfig = this.processConfigurationForImport(importData, targetComponentId)
      applySingleDataSourceImportTarget(processedConfig, targetComponentId, targetSlotId, configurationManager, options)
    } catch (error) {
      console.error('[SingleDataSourceImporter] import failed:', error)
      throw new Error(`Single data source import failed: ${formatImportExportError(error)}`)
    }
  }

  private assertNoSingleDataSourceConflicts(
    importData: SingleDataSourceExport,
    targetComponentId: string,
    configurationManager: ConfigurationImportRuntimeManager
  ): void {
    const conflicts = this.checkImportConflicts(importData, targetComponentId, configurationManager)
    if (conflicts.length > 0) {
      throw new Error(`Single data source import conflicts: ${conflicts.join('; ')}`)
    }
  }

  /**
   * Configuration import/export note.
   */
  private processConfigurationForImport(
    importData: SingleDataSourceExport,
    targetComponentId: string
  ): SingleDataSourceExport {
    return processSingleDataSourceImportPayload(importData, targetComponentId, this.CURRENT_COMPONENT_PLACEHOLDER)
  }
}

/**
 * Configuration import/export note. */
export const configurationExporter = new ConfigurationExporter()
export const configurationImporter = new ConfigurationImporter()
export const singleDataSourceExporter = new SingleDataSourceExporter()
export const singleDataSourceImporter = new SingleDataSourceImporter()
