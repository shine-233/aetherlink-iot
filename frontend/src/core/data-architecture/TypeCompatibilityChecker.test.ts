/**
 * 文件用途: Type Compatibility Checker 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { beforeEach, describe, expect, it } from 'vitest'

import {
  installTypeCompatibilityDebugGlobal,
  TypeCompatibilityChecker
} from './TypeCompatibilityChecker'
import type { ComponentDataRequirement, HttpConfig } from './types/unified-types'

const checker = TypeCompatibilityChecker.getInstance()

beforeEach(() => {
  checker.clearCheckHistory()
})

const validHttpConfig = (): HttpConfig => ({
  url: '/api/v1/devices/:device_id/telemetry',
  method: 'GET',
  timeout: 5000,
  params: [
    {
      key: 'start_time',
      value: '2026-06-27T00:00:00Z',
      enabled: true,
      isDynamic: false,
      dataType: 'string',
      variableName: '',
      description: 'query start time',
      paramType: 'query',
      valueMode: 'manual'
    }
  ],
  headers: [
    {
      key: 'X-Tenant-ID',
      value: 'tenant-a',
      enabled: true,
      isDynamic: true,
      dataType: 'string',
      variableName: 'tenantId',
      description: 'tenant header',
      paramType: 'header',
      valueMode: 'property'
    }
  ],
  pathParameter: {
    key: 'device_id',
    value: 'device-001',
    isDynamic: true,
    dataType: 'string',
    variableName: 'deviceId',
    description: 'device id'
  }
})

const validRequirement = (): ComponentDataRequirement => ({
  componentId: 'rdi-card-001',
  componentName: 'RDI Card',
  staticParams: [
    {
      key: 'title',
      name: 'Title',
      type: 'string',
      description: 'Card title',
      required: true
    }
  ],
  dataSources: [
    {
      key: 'telemetry',
      name: 'Telemetry',
      description: 'RDI telemetry source',
      supportedTypes: ['api'],
      fieldMappings: {
        value: {
          targetField: 'temperature',
          type: 'value',
          required: true
        }
      },
      required: true
    }
  ]
})

describe('TypeCompatibilityChecker', () => {
  it('accepts a valid dynamic HTTP data-source configuration', () => {
    const result = checker.checkHttpConfigCompatibility(validHttpConfig())

    expect(result.valid).toBe(true)
    expect(result.level).toBe('compatible')
    expect(result.errors).toEqual([])
    expect(result.affectedItems).toEqual([])
  })

  it('flags invalid HTTP config fields and weak dynamic parameter definitions', () => {
    const result = checker.checkHttpConfigCompatibility({
      ...validHttpConfig(),
      url: 'ftp://example.com/devices',
      method: 'TRACE' as HttpConfig['method'],
      timeout: 500,
      params: [
        {
          key: '',
          value: 'x',
          enabled: true,
          isDynamic: true,
          dataType: 'json' as never,
          variableName: '',
          description: 'broken param',
          paramType: 'header' as never,
          valueMode: 'invalid' as never
        }
      ],
      pathParameter: {
        value: true,
        isDynamic: true,
        dataType: 'array' as never,
        variableName: 'deviceId',
        description: 'invalid path parameter'
      },
      pathParams: [
        {
          key: '',
          value: '',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: '',
          description: 'broken path param',
          paramType: 'query' as never,
          valueMode: 'manual'
        }
      ]
    })

    expect(result.valid).toBe(false)
    expect(result.level).toBe('incompatible')
    expect(result.affectedItems).toEqual(
      expect.arrayContaining([
        'url',
        'method',
        'params[0].key',
        'params[0].paramType',
        'params[0].variableName',
        'params[0].valueMode',
        'pathParameter',
        'pathParams[0].key',
        'pathParams[0].paramType',
        'pathParams[0].variableName',
        'timeout'
      ])
    )
    expect(result.errors.join('\n')).toContain('URL格式错误')
    expect(result.warnings.join('\n')).toContain('超时时间设置可能不合理')
  })

  it('validates component data requirements for visual editor cards', () => {
    const compatible = checker.checkComponentDataRequirementCompatibility(validRequirement())

    expect(compatible.valid).toBe(true)
    expect(compatible.level).toBe('compatible')

    const missingSources = checker.checkComponentDataRequirementCompatibility({
      componentId: 'card-with-static-data',
      componentName: 'Static Card',
      dataSources: []
    })

    expect(missingSources.valid).toBe(true)
    expect(missingSources.level).toBe('warning')
    expect(missingSources.warnings).toContain('组件没有定义任何数据源需求')

    const invalid = checker.checkComponentDataRequirementCompatibility({
      ...validRequirement(),
      componentId: '',
      staticParams: [
        {
          key: 'title',
          name: 'Title',
          type: 'string',
          description: 'title'
        },
        {
          key: 'title',
          name: '',
          type: 'unsupported' as never,
          description: 'duplicate invalid title'
        }
      ],
      dataSources: [
        {
          key: '',
          name: 'Broken',
          description: 'Missing key and supported types',
          supportedTypes: [],
          fieldMappings: {}
        }
      ]
    })

    expect(invalid.valid).toBe(false)
    expect(invalid.affectedItems).toEqual(
      expect.arrayContaining(['componentId', 'staticParams[1]', 'dataSources[0]', 'staticParams.title'])
    )
    expect(invalid.errors.join('\n')).toContain('静态参数字段名重复')
  })

  it('reports compatible, warning, and incompatible data type conversions', () => {
    expect(checker.checkDataTypeCompatibility('number', 'number')).toMatchObject({
      valid: true,
      level: 'compatible',
      warnings: []
    })

    expect(checker.checkDataTypeCompatibility('string', 'number')).toMatchObject({
      valid: true,
      level: 'warning',
      suggestions: ['使用转换函数: stringToNumber']
    })

    expect(checker.checkDataTypeCompatibility('string', 'array')).toMatchObject({
      valid: false,
      level: 'incompatible'
    })
  })

  it('aggregates batch compatibility results with item ids and exposes mapping stats', () => {
    const result = checker.batchCompatibilityCheck([
      { id: 'valid-http', type: 'httpConfig', data: validHttpConfig() },
      {
        id: 'needs-cast',
        type: 'dataType',
        data: { sourceType: 'string', targetType: 'boolean' }
      },
      {
        id: 'broken-card',
        type: 'componentRequirement',
        data: { componentId: '', componentName: 'Broken', dataSources: [] }
      }
    ])

    expect(result.valid).toBe(false)
    expect(result.level).toBe('incompatible')
    expect(result.errors.join('\n')).toContain('[broken-card] 组件ID无效或缺失')
    expect(result.warnings.join('\n')).toContain('[needs-cast] 需要进行类型转换')
    expect(result.affectedItems).toEqual(expect.arrayContaining(['broken-card.componentId']))

    expect(checker.getTypeMappingStats()).toMatchObject({
      categories: ['basic', 'httpParameter'],
      categoryStats: {
        basic: 17,
        httpParameter: 3
      },
      totalMappings: 20
    })
  })

  it('records results and returns history snapshots that callers cannot mutate', () => {
    checker.checkDataTypeCompatibility('string', 'number')

    const history = checker.getCheckHistory()
    expect(history).toHaveLength(1)
    expect(history[0]).toMatchObject({
      checkType: 'DataTypeCompatibility',
      level: 'warning'
    })

    history[0].warnings.push('caller mutation')
    history[0].suggestions.length = 0

    expect(checker.getCheckHistory()[0]).toMatchObject({
      warnings: ['需要进行类型转换: string -> number'],
      suggestions: ['使用转换函数: stringToNumber']
    })

    checker.clearCheckHistory()
    expect(checker.getCheckHistory()).toEqual([])
  })

  it('installs and removes the debug API only when explicitly requested', () => {
    const target = {} as unknown as typeof globalThis
    const debugTarget = target as typeof globalThis & Record<string, unknown>
    const cleanup = installTypeCompatibilityDebugGlobal(target)

    expect(debugTarget.__TYPE_COMPATIBILITY_CHECKER__).toMatchObject({ checker })

    cleanup()
    expect(Object.prototype.hasOwnProperty.call(debugTarget, '__TYPE_COMPATIBILITY_CHECKER__')).toBe(false)
  })

  it('restores an existing debug global during cleanup', () => {
    const previousValue = { owner: 'host' }
    const target = {
      __TYPE_COMPATIBILITY_CHECKER__: previousValue
    } as unknown as typeof globalThis
    const debugTarget = target as typeof globalThis & Record<string, unknown>
    const cleanup = installTypeCompatibilityDebugGlobal(target)

    expect(debugTarget.__TYPE_COMPATIBILITY_CHECKER__).not.toBe(previousValue)
    cleanup()
    expect(debugTarget.__TYPE_COMPATIBILITY_CHECKER__).toBe(previousValue)
  })

  it('does not overwrite a host value assigned after debug installation', () => {
    const target = {} as unknown as typeof globalThis
    const debugTarget = target as typeof globalThis & Record<string, unknown>
    const cleanup = installTypeCompatibilityDebugGlobal(target)
    const hostValue = { owner: 'host-after-install' }

    debugTarget.__TYPE_COMPATIBILITY_CHECKER__ = hostValue
    cleanup()

    expect(debugTarget.__TYPE_COMPATIBILITY_CHECKER__).toBe(hostValue)
  })
})
