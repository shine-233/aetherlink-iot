/**
 * 文件用途: Configuration Import Export 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { describe, expect, it, vi } from 'vitest'

import {
  ConfigurationExporter,
  ConfigurationImporter,
  SingleDataSourceExporter,
  SingleDataSourceImporter
} from './ConfigurationImportExport'
import type { ExportedConfiguration, SingleDataSourceExport } from './ConfigurationImportExport'

const sourceComponentId = 'comp_current'
const targetComponentId = 'comp_target'
const externalComponentId = 'comp_external'

const createSourceConfig = () => ({
  dataSource: {
    dataSources: [
      {
        id: 'source-1',
        type: 'http',
        componentId: sourceComponentId,
        httpConfigData: {
          url: `/api/widgets/${sourceComponentId}/metrics`,
          targetComponentId: externalComponentId
        }
      },
      {
        id: 'source-2',
        type: 'static',
        componentId: externalComponentId
      }
    ]
  },
  component: {
    properties: {
      componentId: sourceComponentId,
      sourceComponentId,
      labelTemplate: `current=${sourceComponentId};external=${externalComponentId}`
    }
  },
  interaction: {
    responses: [
      {
        action: 'open',
        targetComponentId: externalComponentId
      },
      {
        action: 'refresh',
        targetComponentId: sourceComponentId
      }
    ]
  }
})

const createManager = (initialConfig = createSourceConfig()) => {
  const updates: Array<{ componentId: string; section: string; data: any }> = []

  return {
    updates,
    getConfiguration: vi.fn((componentId: string) =>
      componentId === sourceComponentId || componentId === targetComponentId ? initialConfig : null
    ),
    updateConfiguration: vi.fn((componentId: string, section: string, data: any) => {
      updates.push({ componentId, section, data })
    })
  }
}

describe('ConfigurationImportExport', () => {
  it('exports component configuration with placeholders, dependencies, statistics, and persisted source identifier', async () => {
    const exporter = new ConfigurationExporter()
    const manager = createManager()

    const exported = await exporter.exportConfiguration(sourceComponentId, manager, 'rdi-card')

    expect(exported.version).toBe('1.0.0')
    expect(exported.componentType).toBe('rdi-card')
    expect(exported.metadata.originalComponentId).toBe(sourceComponentId)
    expect(exported.metadata.exportSource).toBe('SimpleConfigurationEditor')
    expect(exported.metadata.statistics).toEqual({
      dataSourceCount: 2,
      interactionCount: 2,
      httpConfigCount: 2
    })
    expect(exported.metadata.dependencies).toContain(externalComponentId)
    expect(exported.data.dataSourceConfiguration.dataSources[0].componentId).toBe('__CURRENT_COMPONENT__')
    expect(exported.data.dataSourceConfiguration.dataSources[0].httpConfigData.url).toBe(
      '/api/widgets/__CURRENT_COMPONENT__/metrics'
    )
    expect(exported.data.dataSourceConfiguration.dataSources[0].httpConfigData.targetComponentId).toBe(
      externalComponentId
    )
    expect(exported.data.componentConfiguration.properties.labelTemplate).toBe(
      'current=__CURRENT_COMPONENT__;external=comp_external'
    )
    expect(exported.data.interactionConfiguration.responses[1].targetComponentId).toBe('__CURRENT_COMPONENT__')
    expect(exported.mapping.dependencies[externalComponentId]).toMatchObject({
      required: true,
      usage: expect.arrayContaining([
        'dataSource.dataSources[0].httpConfigData.targetComponentId',
        'interaction.responses[0].targetComponentId',
        expect.stringContaining('targetComponentId'),
        expect.stringContaining('labelTemplate')
      ])
    })
  })

  it('rejects exporting a missing component', async () => {
    const exporter = new ConfigurationExporter()
    const manager = createManager()

    await expect(exporter.exportConfiguration('missing-component', manager)).rejects.toThrow(
      '组件 missing-component 的配置不存在'
    )
  })

  it('previews persisted-source metadata, dependencies, and overwrite conflicts before import', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const manager = createManager()
    const exported = await exporter.exportConfiguration(sourceComponentId, manager, 'rdi-card')

    const preview = importer.generateImportPreview(JSON.stringify(exported), targetComponentId, manager, [
      { id: externalComponentId }
    ])

    expect(preview.basicInfo).toMatchObject({
      version: '1.0.0',
      componentType: 'rdi-card',
      exportSource: 'SimpleConfigurationEditor'
    })
    expect(preview.dependencies).toEqual([externalComponentId])
    expect(preview.conflicts).toEqual(expect.arrayContaining(['数据源配置冲突', '组件配置冲突', '交互配置冲突']))

    const missingPreview = importer.generateImportPreview(
      exported,
      targetComponentId,
      createManager({
        dataSource: { dataSources: [] },
        component: { properties: {} },
        interaction: {}
      }),
      []
    )
    expect(missingPreview.conflicts).toContain(`缺失依赖组件: ${externalComponentId}`)
  })

  it('includes conflict detection failures in import preview instead of silently clearing conflicts', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const manager = createManager()
    const exported = await exporter.exportConfiguration(sourceComponentId, manager, 'rdi-card')
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)

    const preview = importer.generateImportPreview(exported, targetComponentId, {
      getConfiguration: vi.fn(() => {
        throw new Error('manager read failed')
      })
    })

    expect(preview.conflicts).toContain('冲突检测失败: manager read failed')
    errorSpy.mockRestore()
  })

  it('imports exported sections and restores current component placeholders to the target component', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const manager = createManager()
    const exported = await exporter.exportConfiguration(sourceComponentId, manager, 'rdi-card')

    const result = await importer.importConfiguration(exported, targetComponentId, manager, {
      overwriteExisting: true,
      skipMissingDependencies: true
    })

    expect(result.success).toBe(true)
    expect(manager.updateConfiguration).toHaveBeenCalledTimes(3)
    expect(manager.updates.map(update => update.section)).toEqual(['dataSource', 'component', 'interaction'])
    expect(manager.updates[0].data.dataSources[0].componentId).toBe(targetComponentId)
    expect(manager.updates[0].data.dataSources[0].httpConfigData.url).toBe('/api/widgets/comp_target/metrics')
    expect(manager.updates[1].data.properties.labelTemplate).toBe('current=comp_target;external=comp_external')
    expect(manager.updates[2].data.responses[1].targetComponentId).toBe(targetComponentId)
    expect(result.importedData?.dataSource.dataSources[0].componentId).toBe(targetComponentId)
  })

  it('fails full import when exported dependencies are missing from the target environment', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const manager = {
      ...createManager(),
      nodes: [{ id: targetComponentId }]
    }
    const exported = await exporter.exportConfiguration(sourceComponentId, manager, 'rdi-card')

    const result = await importer.importConfiguration(exported, targetComponentId, manager)

    expect(result.success).toBe(false)
    expect(result.errors).toEqual([`缺失依赖组件: ${externalComponentId}`])
    expect(manager.updateConfiguration).not.toHaveBeenCalled()
  })

  it('refuses to overwrite existing full configuration unless overwriteExisting is enabled', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const manager = createManager()
    const exported = await exporter.exportConfiguration(sourceComponentId, manager, 'rdi-card')

    const result = await importer.importConfiguration(exported, targetComponentId, manager, {
      skipMissingDependencies: true
    })

    expect(result.success).toBe(false)
    expect(result.errors).toEqual(['数据源配置冲突', '组件配置冲突', '交互配置冲突'])
    expect(manager.updateConfiguration).not.toHaveBeenCalled()
  })

  it('does not force overwrite when existing configuration cannot be read', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const sourceManager = createManager()
    const exported = await exporter.exportConfiguration(sourceComponentId, sourceManager, 'rdi-card')
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const manager = {
      nodes: [{ id: targetComponentId }, { id: externalComponentId }],
      getConfiguration: vi.fn(() => {
        throw new Error('target read failed')
      }),
      updateConfiguration: vi.fn()
    }

    try {
      const result = await importer.importConfiguration(exported, targetComponentId, manager, {
        overwriteExisting: true
      })

      expect(result.success).toBe(false)
      expect(result.errors).toEqual(['冲突检测失败: target read failed'])
      expect(manager.updateConfiguration).not.toHaveBeenCalled()
    } finally {
      errorSpy.mockRestore()
    }
  })

  it('imports through managers that expose updateConfigurationSection only', async () => {
    const exporter = new ConfigurationExporter()
    const importer = new ConfigurationImporter()
    const sourceManager = createManager()
    const exported = await exporter.exportConfiguration(sourceComponentId, sourceManager, 'rdi-card')
    const updates: Array<{ componentId: string; section: string; data: any }> = []
    const sectionOnlyManager = {
      updateConfigurationSection: vi.fn((componentId: string, section: string, data: any) => {
        updates.push({ componentId, section, data })
      })
    }

    const result = await importer.importConfiguration(exported, targetComponentId, sectionOnlyManager, {
      skipMissingDependencies: true
    })

    expect(result.success).toBe(true)
    expect(sectionOnlyManager.updateConfigurationSection).toHaveBeenCalledTimes(3)
    expect(updates.map(update => update.section)).toEqual(['dataSource', 'component', 'interaction'])
    expect(updates[0].data.dataSources[0].componentId).toBe(targetComponentId)
  })

  it('returns non-Error import failures as readable error strings', async () => {
    const importer = new ConfigurationImporter()
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const config: ExportedConfiguration = {
      version: '1.0.0',
      exportTime: Date.now(),
      metadata: {
        originalComponentId: sourceComponentId,
        exportSource: 'test',
        dependencies: [],
        statistics: { dataSourceCount: 0, interactionCount: 0, httpConfigCount: 0 }
      },
      data: {
        componentConfiguration: { properties: { componentId: '__CURRENT_COMPONENT__' } }
      },
      mapping: {
        placeholders: { __CURRENT_COMPONENT__: 'current_component' },
        dependencies: {}
      }
    }

    const result = await importer.importConfiguration(config, targetComponentId, {
      updateConfiguration: vi.fn(() => {
        throw 'write failed'
      })
    })

    expect(result.success).toBe(false)
    expect(result.errors).toEqual(['write failed'])
    errorSpy.mockRestore()
  })

  it('returns a failed import result when the configuration manager cannot apply changes', async () => {
    const importer = new ConfigurationImporter()
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const config: ExportedConfiguration = {
      version: '1.0.0',
      exportTime: Date.now(),
      metadata: {
        originalComponentId: sourceComponentId,
        exportSource: 'test',
        dependencies: [],
        statistics: { dataSourceCount: 0, interactionCount: 0, httpConfigCount: 0 }
      },
      data: {
        componentConfiguration: { properties: { componentId: '__CURRENT_COMPONENT__' } }
      },
      mapping: {
        placeholders: { __CURRENT_COMPONENT__: 'current_component' },
        dependencies: {}
      }
    }

    const result = await importer.importConfiguration(config, targetComponentId, {})

    expect(result.success).toBe(false)
    expect(result.errors.join('\n')).toContain('配置管理器无效或未提供')
    errorSpy.mockRestore()
  })

  it('exports a real single data source with placeholders, related config, and dependency metadata', async () => {
    const exporter = new SingleDataSourceExporter()
    const manager = {
      getConfiguration: vi.fn((componentId: string, section?: string) => {
        if (componentId !== sourceComponentId) return null
        if (section === 'interaction') {
          return {
            responses: [
              { action: 'refresh', sourceId: 'main', targetComponentId: sourceComponentId },
              { action: 'ignore', sourceId: 'other' }
            ]
          }
        }
        if (section === 'component') {
          return {
            httpBindings: [
              { sourceId: 'main', targetComponentId: 'external_123' },
              { sourceId: 'other', targetComponentId: 'external_999' }
            ]
          }
        }
        return {
          dataSource: {
            dataSources: [
              {
                sourceId: 'main',
                dataItems: [
                  {
                    item: {
                      type: 'http',
                      config: {
                        url: `/api/widgets/${sourceComponentId}/metrics`,
                        params: [{ key: 'deviceId', value: `${sourceComponentId}.base.deviceId` }]
                      }
                    },
                    processing: { filterPath: '$.data' }
                  }
                ],
                mergeStrategy: { type: 'object' }
              }
            ]
          }
        }
      })
    }

    const exported = await exporter.exportSingleDataSource(sourceComponentId, 'main', manager, 'rdi-card')

    expect(exported.exportType).toBe('single-datasource')
    expect(exported.sourceMetadata).toMatchObject({
      originalSourceId: 'main',
      sourceIndex: 0,
      originalComponentId: sourceComponentId,
      exportSource: 'SingleDataSourceExporter',
      componentType: 'rdi-card'
    })
    expect(exported.dataSourceConfig.dataItems[0].item.config.url).toBe('/api/widgets/__CURRENT_COMPONENT__/metrics')
    expect(exported.dataSourceConfig.dataItems[0].item.config.params[0].value).toBe(
      '__CURRENT_COMPONENT__.base.deviceId'
    )
    expect(exported.relatedConfig.interactions).toHaveLength(1)
    expect(exported.relatedConfig.interactions[0].targetComponentId).toBe('__CURRENT_COMPONENT__')
    expect(exported.relatedConfig.httpBindings).toEqual([{ sourceId: 'main', targetComponentId: 'external_123' }])
    expect(exported.mapping.dependencies).toContain('external_123')
  })

  it('previews real single data source conflicts for missing dependencies and component type mismatch', () => {
    const importer = new SingleDataSourceImporter()
    const importData = createSingleDataSourceExport({
      mapping: { placeholders: { __CURRENT_COMPONENT__: 'current_component' }, dependencies: ['external_123'] }
    })
    const manager = {
      store: { nodes: [{ id: targetComponentId, type: 'gauge-card' }] },
      getConfiguration: vi.fn(() => ({ metadata: { componentType: 'gauge-card' }, dataSource: { dataSources: [] } }))
    }

    const preview = importer.generateImportPreview(importData, targetComponentId, manager)

    expect(preview.conflicts).toEqual(
      expect.arrayContaining(['缺失外部依赖组件: external_123', '组件类型不匹配: rdi-card -> gauge-card'])
    )
    expect(preview.availableSlots).toEqual([
      { slotId: 'dataSource1', slotIndex: 0, isEmpty: true },
      { slotId: 'dataSource2', slotIndex: 1, isEmpty: true },
      { slotId: 'dataSource3', slotIndex: 2, isEmpty: true }
    ])
  })

  it('imports a real single data source into a target slot and merges related interaction/http binding config', async () => {
    const importer = new SingleDataSourceImporter()
    const updates: Array<{ section: string; data: any }> = []
    const manager = {
      nodes: [{ id: targetComponentId }, { id: 'external_123' }],
      getConfiguration: vi.fn((componentId: string) =>
        componentId === targetComponentId
          ? {
              metadata: { componentType: 'rdi-card' },
              dataSource: {
                componentId: targetComponentId,
                dataSources: [
                  { sourceId: 'occupied', dataItems: [{ item: { type: 'json' } }], mergeStrategy: { type: 'object' } }
                ],
                createdAt: 1,
                updatedAt: 1
              },
              interaction: { importedInteractions: [{ action: 'existing' }] },
              component: { httpBindings: [{ sourceId: 'existing' }] }
            }
          : null
      ),
      updateConfiguration: vi.fn((_componentId: string, section: string, data: any) => {
        updates.push({ section, data })
      })
    }
    const importData = createSingleDataSourceExport({
      mapping: { placeholders: { __CURRENT_COMPONENT__: 'current_component' }, dependencies: ['external_123'] },
      relatedConfig: {
        interactions: [{ action: 'refresh', targetComponentId: '__CURRENT_COMPONENT__' }],
        httpBindings: [{ sourceId: 'main', targetComponentId: 'external_123' }]
      }
    })

    await importer.importSingleDataSource(importData, targetComponentId, 'newSlot', manager)

    expect(manager.updateConfiguration).toHaveBeenCalledTimes(3)
    expect(updates.map(update => update.section)).toEqual(['dataSource', 'interaction', 'component'])
    expect(updates[0].data.dataSources).toHaveLength(2)
    expect(updates[0].data.dataSources[1]).toMatchObject({
      sourceId: 'newSlot',
      mergeStrategy: { type: 'object' }
    })
    expect(updates[0].data.dataSources[1].dataItems[0].item.config.url).toBe('/api/widgets/comp_target/metrics')
    expect(updates[0].data.dataSources[1].dataItems[0].item.config.params[0]).toMatchObject({
      isDynamic: true,
      value: 'comp_target.base.deviceId'
    })
    expect(updates[1].data.importedInteractions).toEqual([
      { action: 'existing' },
      { action: 'refresh', targetComponentId: targetComponentId }
    ])
    expect(updates[2].data.httpBindings).toEqual([
      { sourceId: 'existing' },
      { sourceId: 'newSlot', targetComponentId: 'external_123' }
    ])
  })

  it('refuses to overwrite an occupied single data source slot unless overwriteExisting is enabled', async () => {
    const importer = new SingleDataSourceImporter()
    const manager = createSingleDataSourceTargetManager()
    const importData = createSingleDataSourceExport()
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)

    try {
      await expect(importer.importSingleDataSource(importData, targetComponentId, 'occupied', manager)).rejects.toThrow(
        '目标数据源槽位已存在配置: occupied'
      )
      expect(manager.updateConfiguration).not.toHaveBeenCalled()
    } finally {
      errorSpy.mockRestore()
    }
  })

  it('overwrites only the selected single data source slot when overwriteExisting is enabled', async () => {
    const importer = new SingleDataSourceImporter()
    const manager = createSingleDataSourceTargetManager()
    const importData = createSingleDataSourceExport()

    await importer.importSingleDataSource(importData, targetComponentId, 'occupied', manager, {
      overwriteExisting: true
    })

    expect(manager.updateConfiguration).toHaveBeenCalledTimes(1)
    const updatedDataSource = manager.updates[0].data
    expect(updatedDataSource.dataSources).toHaveLength(2)
    expect(updatedDataSource.dataSources[0].sourceId).toBe('occupied')
    expect(updatedDataSource.dataSources[0].dataItems[0].item.config.url).toBe('/api/widgets/comp_target/metrics')
    expect(updatedDataSource.dataSources[1]).toMatchObject({
      sourceId: 'other',
      dataItems: [{ item: { type: 'json', config: { value: 2 } } }]
    })
  })
})

const createSingleDataSourceExport = (overrides: Partial<SingleDataSourceExport> = {}): SingleDataSourceExport => ({
  version: '1.0.0',
  exportType: 'single-datasource',
  exportTime: 1782536400000,
  sourceMetadata: {
    originalSourceId: 'main',
    sourceIndex: 0,
    originalComponentId: sourceComponentId,
    exportSource: 'SingleDataSourceExporter',
    componentType: 'rdi-card'
  },
  dataSourceConfig: {
    dataItems: [
      {
        item: {
          type: 'http',
          config: {
            url: '/api/widgets/__CURRENT_COMPONENT__/metrics',
            params: [
              {
                key: 'deviceId',
                value: 'deviceId',
                variableName: '__CURRENT_COMPONENT___deviceId',
                valueMode: 'component',
                selectedTemplate: 'component-property-binding',
                isDynamic: false
              }
            ]
          }
        },
        processing: { filterPath: '$.data' }
      }
    ],
    mergeStrategy: { type: 'object' }
  },
  relatedConfig: {
    interactions: [],
    httpBindings: []
  },
  mapping: {
    placeholders: { __CURRENT_COMPONENT__: 'current_component' },
    dependencies: []
  },
  ...overrides
})

const createSingleDataSourceTargetManager = () => {
  const updates: Array<{ componentId: string; section: string; data: any }> = []
  const manager = {
    updates,
    nodes: [{ id: targetComponentId }],
    getConfiguration: vi.fn((componentId: string) =>
      componentId === targetComponentId
        ? {
            metadata: { componentType: 'rdi-card' },
            dataSource: {
              componentId: targetComponentId,
              dataSources: [
                {
                  sourceId: 'occupied',
                  dataItems: [{ item: { type: 'json', config: { value: 1 } } }],
                  mergeStrategy: { type: 'object' }
                },
                {
                  sourceId: 'other',
                  dataItems: [{ item: { type: 'json', config: { value: 2 } } }],
                  mergeStrategy: { type: 'object' }
                }
              ],
              createdAt: 1,
              updatedAt: 1
            }
          }
        : null
    ),
    updateConfiguration: vi.fn((componentId: string, section: string, data: any) => {
      updates.push({ componentId, section, data })
    })
  }

  return manager
}
