/**
 * 文件用途: Configuration Manager 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ConfigurationManager } from './ConfigurationManager'

const validConfig = (componentId = 'card-a') => ({
  componentId,
  dataSources: [
    {
      sourceId: 'main',
      dataItems: [
        {
          item: {
            type: 'json',
            config: {
              jsonString: '{"temperature":26}'
            }
          },
          processing: {
            filterPath: '$',
            defaultValue: {}
          }
        }
      ],
      mergeStrategy: {
        type: 'object'
      }
    }
  ],
  createdAt: 100,
  updatedAt: 200
})

describe('ConfigurationManager', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(Date, 'now').mockReturnValue(1710000000000)
  })

  it('exposes built-in templates by id and category', () => {
    const manager = new ConfigurationManager()

    expect(manager.getBuiltinTemplates().map(template => template.id)).toEqual([
      'json-basic',
      'http-api',
      'script-generated',
      'multi-source'
    ])
    expect(manager.getTemplate('http-api')?.configuration.dataSources[0].dataItems[0].item.type).toBe('http')
    expect(manager.getTemplatesByCategory('basic').map(template => template.id)).toEqual(['json-basic', 'http-api'])
    expect(manager.getTemplate('missing')).toBeUndefined()
  })

  it('returns isolated deterministic template copies', () => {
    const manager = new ConfigurationManager()
    const templates = manager.getBuiltinTemplates()
    const scriptTemplate = templates.find(template => template.id === 'script-generated')
    const script = scriptTemplate?.configuration.dataSources[0].dataItems[0].item.config.script || ''

    expect(script).not.toContain('Math.random')
    expect(script).toContain('preview: true')

    templates[0].name = 'mutated'
    templates[0].configuration.dataSources[0].sourceId = 'mutated-source'
    const templateById = manager.getTemplate('json-basic')
    expect(templateById?.name).toBe('设备遥测 JSON 模板')
    expect(templateById?.configuration.dataSources[0].sourceId).toBe('device_telemetry_json')

    const categoryTemplates = manager.getTemplatesByCategory('basic')
    categoryTemplates[0].tags.push('mutated')
    expect(manager.getTemplatesByCategory('basic')[0].tags).not.toContain('mutated')
  })

  it('validates complete configs and reports data-source, item, processing, and merge defects', () => {
    const manager = new ConfigurationManager()

    expect(manager.validateConfiguration(validConfig() as any)).toEqual({ valid: true, errors: [], warnings: [] })

    const result = manager.validateConfiguration({
      componentId: '',
      dataSources: [
        {
          sourceId: '',
          dataItems: [],
          mergeStrategy: {}
        },
        {
          sourceId: 'broken-json',
          dataItems: [
            {
              item: { type: 'json', config: { jsonString: '{bad json' } },
              processing: {}
            }
          ],
          mergeStrategy: {}
        },
        {
          sourceId: 'broken-http',
          dataItems: [
            {
              item: { type: 'http', config: { url: '' } },
              processing: {}
            }
          ],
          mergeStrategy: { type: 'object' }
        },
        {
          sourceId: 'broken-script',
          dataItems: [
            {
              item: { type: 'script', config: { script: '' } },
              processing: {}
            }
          ],
          mergeStrategy: { type: 'object' }
        }
      ]
    } as any)

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(6)
    expect(result.errors.join('\n')).toContain('组件ID')
    expect(result.errors.join('\n')).toContain('sourceId')
    expect(result.errors.join('\n')).toContain('至少需要一个数据项')
    expect(result.errors.join('\n')).toContain('JSON')
    expect(result.errors.join('\n')).toContain('HTTP URL')
    expect(result.errors.join('\n')).toContain('脚本内容')
    expect(result.warnings).toHaveLength(6)
    expect(result.warnings.join('\n')).toContain('建议指定HTTP方法')
    expect(result.warnings.join('\n')).toContain('建议设置过滤路径')
    expect(result.warnings.join('\n')).toContain('建议指定合并策略')
  })

  it('imports and exports JSON configuration while refreshing timestamps', () => {
    const manager = new ConfigurationManager()
    const config = validConfig('imported-card')
    const json = manager.exportConfiguration(config as any)

    expect(JSON.parse(json)).toMatchObject({ componentId: 'imported-card' })
    expect(manager.importConfiguration(json)).toMatchObject({
      componentId: 'imported-card',
      createdAt: 100,
      updatedAt: 1710000000000
    })

    const withoutCreatedAt = JSON.stringify({ componentId: 'new-card', dataSources: [] })
    expect(manager.importConfiguration(withoutCreatedAt)).toMatchObject({
      componentId: 'new-card',
      createdAt: 1710000000000,
      updatedAt: 1710000000000
    })

    expect(() => manager.importConfiguration('{"componentId":"bad"}')).toThrow()
    expect(() => manager.importConfiguration('{bad json')).toThrow()
  })

  it('imports configuration from File objects', async () => {
    const manager = new ConfigurationManager()
    const file = new File([manager.exportConfiguration(validConfig('file-card') as any)], 'file-card.json', {
      type: 'application/json'
    })

    await expect(manager.importConfigurationFromFile(file)).resolves.toMatchObject({
      componentId: 'file-card',
      updatedAt: 1710000000000
    })
  })

  it('downloads exported configuration files with generated and explicit names', () => {
    const manager = new ConfigurationManager()
    const click = vi.fn()
    const anchor = {
      href: '',
      download: '',
      click
    }
    const createElementSpy = vi.spyOn(document, 'createElement').mockReturnValue(anchor as unknown as HTMLAnchorElement)
    const createObjectURLSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:config')
    const revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)

    manager.exportConfigurationAsFile(validConfig('download-card') as any)

    expect(createElementSpy).toHaveBeenCalledWith('a')
    expect(anchor.href).toBe('blob:config')
    expect(anchor.download).toBe('download-card-config-1710000000000.json')
    expect(click).toHaveBeenCalledTimes(1)
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:config')

    manager.exportConfigurationAsFile(validConfig('download-card') as any, 'custom.json')
    expect(anchor.download).toBe('custom.json')
  })

  it('revokes the exported configuration URL when the browser download fails', () => {
    const manager = new ConfigurationManager()
    const downloadError = new Error('download blocked')
    const anchor = {
      href: '',
      download: '',
      click: vi.fn(() => {
        throw downloadError
      })
    }
    vi.spyOn(document, 'createElement').mockReturnValue(anchor as unknown as HTMLAnchorElement)
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:failed-config')
    const revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)

    expect(() => manager.exportConfigurationAsFile(validConfig('download-card') as any)).toThrow(downloadError)
    expect(revokeObjectURLSpy).toHaveBeenCalledTimes(1)
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:failed-config')
  })

  it('generates starter configs, clones without sharing nested state, and merges data sources', () => {
    const manager = new ConfigurationManager()

    const starter = manager.generateStarterConfiguration('starter-card')
    expect(starter).toMatchObject({
      componentId: 'starter-card',
      createdAt: 1710000000000,
      updatedAt: 1710000000000
    })

    starter.dataSources[0].sourceId = 'mutated-starter-source'
    const nextStarter = manager.generateStarterConfiguration('next-starter-card')
    expect(nextStarter.dataSources[0].sourceId).toBe('device_telemetry_json')

    const original = validConfig('source-card') as any
    const cloned = manager.cloneConfiguration(original, 'clone-card') as any
    cloned.dataSources[0].dataItems[0].item.config.jsonString = '{"temperature":99}'

    expect(cloned.componentId).toBe('clone-card')
    expect(cloned.createdAt).toBe(1710000000000)
    expect(original.dataSources[0].dataItems[0].item.config.jsonString).toBe('{"temperature":26}')

    const extraConfig = validConfig('extra-card')
    const merged = manager.mergeConfigurations(validConfig('base-card') as any, extraConfig as any)
    expect(merged.componentId).toBe('base-card')
    expect(merged.dataSources).toHaveLength(2)
    expect(merged.updatedAt).toBe(1710000000000)

    merged.dataSources[1].dataItems[0].item.config.jsonString = '{"temperature":88}'
    expect(extraConfig.dataSources[0].dataItems[0].item.config.jsonString).toBe('{"temperature":26}')
  })

  it('falls back to a simple generated starter config when built-in templates are unavailable', () => {
    const manager = new ConfigurationManager()
    ;(manager as any).templates = []

    const starter = manager.generateStarterConfiguration('fallback-card')

    expect(starter.componentId).toBe('fallback-card')
    expect(starter.dataSources[0]).toMatchObject({
      sourceId: 'starter_data',
      mergeStrategy: { type: 'object' }
    })
    expect(starter.createdAt).toBe(1710000000000)
  })
})
