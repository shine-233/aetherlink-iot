/**
 * 文件用途: 覆盖 DynamicParameterEditor 参数 helper 的纯函数契约。
 * 核心逻辑: 验证 API 模板 seed、过滤、构建和按 key 合并行为。
 * 关键注意事项: 这些测试不挂载 Vue 组件，专门保护数据架构参数合并语义。
 * 重构建议: 若后续拆出设备参数合并，应新增独立测试文件，避免混入 API 模板契约。
 */
import { describe, expect, it } from 'vitest'

import type { EnhancedParameter } from '@/core/data-architecture/types/parameter-editor'
import {
  buildApiTemplateParameters,
  filterApiTemplateParams,
  mergeTemplateParameters,
  resolveApiTemplateSeedValue,
  type ApiTemplateInfo
} from './parameter-editor-helpers'

const createDefaultParameter = (overrides: Partial<EnhancedParameter> = {}): EnhancedParameter => ({
  key: 'placeholder',
  value: '',
  enabled: true,
  isDynamic: false,
  valueMode: 'manual',
  selectedTemplate: 'manual',
  dataType: 'string',
  variableName: '',
  description: '',
  _id: `id-${overrides.key || 'placeholder'}`,
  ...overrides
})

describe('parameter-editor-helpers', () => {
  it('resolves API template seed values without dropping falsy examples', () => {
    expect(resolveApiTemplateSeedValue({ example: 0, defaultValue: 10 })).toBe(0)
    expect(resolveApiTemplateSeedValue({ example: false, defaultValue: true })).toBe(false)
    expect(resolveApiTemplateSeedValue({ example: '', defaultValue: 'fallback' })).toBe('')
    expect(resolveApiTemplateSeedValue({ defaultValue: 'defaulted' })).toBe('defaulted')
    expect(resolveApiTemplateSeedValue({})).toBe('')
  })

  it('filters API template params by query, path, and header context', () => {
    const apiInfo: ApiTemplateInfo = {
      url: '/api/devices/:deviceId/telemetry',
      pathParamNames: ['deviceId'],
      commonParams: [
        { name: 'deviceId', type: 'string' },
        { name: 'page', type: 'number' },
        { name: 'X-Trace-Id', type: 'string', paramType: 'header' },
        { name: 'Authorization', type: 'string', in: 'header' }
      ]
    }

    expect(filterApiTemplateParams(apiInfo, 'query').map(param => param.name)).toEqual(['page'])
    expect(filterApiTemplateParams(apiInfo, 'path').map(param => param.name)).toEqual(['deviceId'])
    expect(filterApiTemplateParams(apiInfo, 'header').map(param => param.name)).toEqual(['X-Trace-Id', 'Authorization'])
  })

  it('builds API template parameters with manual templates, data types, and fallback keys', () => {
    const params = buildApiTemplateParameters(
      {
        url: '/api/devices/:deviceId/telemetry',
        pathParamNames: ['deviceId'],
        commonParams: [
          { name: 'page', type: 'number', description: 'page index', example: 0 },
          { name: 'active', type: 'boolean', description: 'active flag', example: false },
          { name: 'payload', type: 'object', defaultValue: '' }
        ]
      },
      'query',
      () => createDefaultParameter()
    )

    expect(params).toEqual([
      expect.objectContaining({
        key: 'page',
        value: 0,
        defaultValue: 0,
        dataType: 'number',
        valueMode: 'manual',
        selectedTemplate: 'manual'
      }),
      expect.objectContaining({
        key: 'active',
        value: false,
        defaultValue: false,
        dataType: 'boolean'
      }),
      expect.objectContaining({
        key: 'payload',
        value: '',
        defaultValue: '',
        dataType: 'string'
      })
    ])

    expect(
      buildApiTemplateParameters({ url: '/api/group/current', commonParams: [] }, 'query', () =>
        createDefaultParameter()
      )
    ).toEqual([expect.objectContaining({ key: 'groupId', description: '分组ID', valueMode: 'manual' })])
  })

  it('merges template parameters by key while preserving existing ids and unrelated metadata', () => {
    const deviceContext = { sourceType: 'device-selection' as const, timestamp: 1000 }
    const parameterGroup = { groupId: 'group-1', role: 'primary' as const, isDerived: false }
    const existingDeviceParam = createDefaultParameter({
      key: 'deviceId',
      value: 'original-device',
      _id: 'keep-device-id',
      deviceContext,
      parameterGroup
    })
    const existingPageParam = createDefaultParameter({
      key: 'page',
      value: 'old-page',
      _id: 'keep-page-id'
    })
    const templatePageParam = createDefaultParameter({
      key: 'page',
      value: 1,
      defaultValue: 1,
      dataType: 'number',
      _id: 'template-page-id'
    })
    const templateActiveParam = createDefaultParameter({
      key: 'active',
      value: false,
      defaultValue: false,
      dataType: 'boolean',
      _id: 'template-active-id'
    })

    const merged = mergeTemplateParameters(
      [existingDeviceParam, existingPageParam],
      [templatePageParam, templateActiveParam]
    )

    expect(merged).toEqual([
      existingDeviceParam,
      expect.objectContaining({
        key: 'page',
        value: 1,
        defaultValue: 1,
        _id: 'keep-page-id'
      }),
      templateActiveParam
    ])
    expect(merged[0]).toBe(existingDeviceParam)
    expect(merged[0].deviceContext).toBe(deviceContext)
    expect(merged[0].parameterGroup).toBe(parameterGroup)
  })
})
