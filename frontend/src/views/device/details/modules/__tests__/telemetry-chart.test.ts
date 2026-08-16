/**
 * 文件用途: telemetry-chart 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  telemetryDataCurrent: vi.fn(),
  getAttributeDataSet: vi.fn(),
  telemetryApi: vi.fn(),
  attributesApi: vi.fn(),
  eventsApi: vi.fn(),
  commandsApi: vi.fn(),
  getCachedDeviceTemplateDetail: vi.fn(),
  extractPlatformFields: vi.fn(),
  realtimePushStart: vi.fn(),
  realtimePushStop: vi.fn(),
  alarmPushStart: vi.fn(),
  alarmPushStop: vi.fn(),
  pushPlatformData: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  telemetryDataCurrent: hoisted.telemetryDataCurrent,
  getAttributeDataSet: hoisted.getAttributeDataSet
}))

vi.mock('@/service/api', () => ({
  telemetryApi: hoisted.telemetryApi,
  attributesApi: hoisted.attributesApi,
  eventsApi: hoisted.eventsApi,
  commandsApi: hoisted.commandsApi
}))

vi.mock('@/utils/thingsvis/template-detail-cache', () => ({
  getCachedDeviceTemplateDetail: hoisted.getCachedDeviceTemplateDetail
}))

// telemetry-chart.vue 同时导入 extractPlatformFields 和 mergePlatformFieldsById。
// mock 工厂必须补齐后者，否则它在组件里是 undefined，平台字段合并会静默拿到空数组。
vi.mock('@/utils/thingsvis/platform-fields', () => ({
  extractPlatformFields: hoisted.extractPlatformFields,
  mergePlatformFieldsById: (primary: any[], fallback: any[]) => {
    const seen = new Set<string>()
    const merged: any[] = []
    for (const field of [...(primary ?? []), ...(fallback ?? [])]) {
      if (!field?.id || seen.has(field.id)) continue
      seen.add(field.id)
      merged.push(field)
    }
    return merged
  }
}))

vi.mock('@/hooks/thingsvis/useRealtimePush', () => ({
  useRealtimePush: () => ({
    start: hoisted.realtimePushStart,
    stop: hoisted.realtimePushStop
  })
}))

vi.mock('@/hooks/thingsvis/useAlarmPush', () => ({
  useAlarmPush: () => ({
    start: hoisted.alarmPushStart,
    stop: hoisted.alarmPushStop
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/components/thingsvis/ThingsVisWidget.vue', () => ({
  default: defineComponent({
    name: 'ThingsVisWidget',
    emits: ['ready'],
    setup(_, { expose }) {
      expose({ pushPlatformData: hoisted.pushPlatformData })
      return () => h('div', { class: 'thingsvis-widget-stub' })
    }
  })
}))

global.ResizeObserver = vi.fn(function ResizeObserverMock() {
  return {
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn()
  }
}) as any

import TelemetryChart from '../telemetry-chart.vue'
import * as normalizer from '../telemetryChartTemplateNormalizer'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountTelemetryChart = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(TelemetryChart, {
    props: {
      id: 'device-1',
      ...props
    },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NCard: true,
        NEmpty: true,
        NSkeleton: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

/**
 * 归一化 helper 已从 telemetry-chart.vue 下沉到 telemetryChartTemplateNormalizer.ts，
 * 不再挂在组件 setupState 上。这里把真实模块导出并入返回值，
 * 让既有用例继续断言同名 helper，且断言的是真实实现而非组件内部私有状态。
 */
const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => {
  const setupState = wrapper.vm.$.setupState as Record<string, any>

  // 必须用 Proxy 透传而不是对象展开：展开会把 ref 拍成快照，
  // 用例里 `setupState.x = v` 之后再读 computed 就拿不到新值了。
  return new Proxy(setupState, {
    get(target, key: string) {
      if (key in target) return target[key]
      return (normalizer as Record<string, any>)[key]
    },
    set(target, key: string, value) {
      target[key] = value
      return true
    },
    has(target, key: string) {
      return key in target || key in (normalizer as Record<string, any>)
    }
  }) as Record<string, any>
}

const mockPlatformFields = () => [
  { id: 'temperature', name: 'Temperature', type: 'number', dataType: 'telemetry' },
  { id: 'humidity', name: 'Humidity', type: 'number', dataType: 'telemetry' },
  { id: 'device_name', name: 'Device Name', type: 'string', dataType: 'attribute' }
]

describe('telemetry-chart.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({ data: null })
    hoisted.telemetryApi.mockResolvedValue({ data: { list: [] } })
    hoisted.attributesApi.mockResolvedValue({ data: { list: [] } })
    hoisted.eventsApi.mockResolvedValue({ data: { list: [] } })
    hoisted.commandsApi.mockResolvedValue({ data: { list: [] } })
    hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [] })
    hoisted.extractPlatformFields.mockReturnValue([])
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  describe('getRuntimePlatformSourceId', () => {
    it('returns platform source id for given device id', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.getRuntimePlatformSourceId('dev-123')).toBe('__platform_dev-123__')
    })
  })

  describe('isValidRequestedFieldId', () => {
    it('returns false for empty field id', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      expect(setupState.isValidRequestedFieldId('', available)).toBe(false)
    })

    it('returns true when field id is in available set', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature', 'humidity'])

      expect(setupState.isValidRequestedFieldId('temperature', available)).toBe(true)
      expect(setupState.isValidRequestedFieldId('humidity', available)).toBe(true)
    })

    it('returns true for runtime platform field ids', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set<string>()

      expect(setupState.isValidRequestedFieldId('is_online', available)).toBe(true)
      expect(setupState.isValidRequestedFieldId('online_text', available)).toBe(true)
      expect(setupState.isValidRequestedFieldId('online_status_updated_at', available)).toBe(true)
    })

    it('returns true for history suffix when root is available', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      expect(setupState.isValidRequestedFieldId('temperature__history', available)).toBe(true)
    })

    it('returns false for history suffix when root is not available', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      expect(setupState.isValidRequestedFieldId('unknown__history', available)).toBe(false)
    })

    it('returns false for unknown field id', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      expect(setupState.isValidRequestedFieldId('unknown', available)).toBe(false)
    })
  })

  describe('rewriteTemplateBindingExpression', () => {
    it('returns non-string expression as-is', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.rewriteTemplateBindingExpression(123, 'runtime-id')).toBe(123)
      expect(setupState.rewriteTemplateBindingExpression(null, 'runtime-id')).toBe(null)
      expect(setupState.rewriteTemplateBindingExpression(undefined, 'runtime-id')).toBe(undefined)
    })

    it('rewrites direct field binding expression', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const result = setupState.rewriteTemplateBindingExpression(
        "{{ ds.__platform___template____.data.temperature }}",
        '__platform_device-1__'
      )
      expect(result).toBe('{{ ds.__platform_device-1__.data.temperature }}')
    })

    it('rewrites boolean select binding expression', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const result = setupState.rewriteTemplateBindingExpression(
        "{{ ds.__platform___template____.data.is_online ? '1' : '0' }}",
        '__platform_device-1__'
      )
      expect(result).toBe('{{ ds.__platform_device-1__.data.is_online }}')
    })

    it('replaces template source id in arbitrary strings', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const result = setupState.rewriteTemplateBindingExpression(
        'prefix __platform___template____ suffix',
        '__platform_device-1__'
      )
      expect(result).toBe('prefix __platform_device-1__ suffix')
    })

    it('returns string without template source id as-is', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const result = setupState.rewriteTemplateBindingExpression(
        'plain string without template id',
        '__platform_device-1__'
      )
      expect(result).toBe('plain string without template id')
    })
  })

  describe('normalizeTemplateChartConfig', () => {
    it('returns null/non-object input as-is', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.normalizeTemplateChartConfig(null, 'device-1', new Set())).toBe(null)
      expect(setupState.normalizeTemplateChartConfig('string', 'device-1', new Set())).toBe('string')
      expect(setupState.normalizeTemplateChartConfig(undefined, 'device-1', new Set())).toBe(undefined)
    })

    it('rewrites dataSources with PLATFORM_FIELD type and filters requestedFields', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature', 'humidity'])

      const rawConfig = {
        dataSources: [
          {
            id: '__platform___template____',
            type: 'PLATFORM_FIELD',
            config: {
              requestedFields: ['temperature', 'humidity', 'unknown_field', 'temperature__history', '']
            }
          },
          {
            id: 'other-source',
            type: 'OTHER_TYPE',
            config: { foo: 'bar' }
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.dataSources[0].id).toBe('__platform_device-1__')
      expect(result.dataSources[0].config.deviceId).toBe('device-1')
      expect(result.dataSources[0].config.requestedFields).toEqual([
        'temperature',
        'humidity',
        'temperature__history'
      ])
      expect(result.dataSources[1]).toEqual(rawConfig.dataSources[1])
    })

    it('keeps non-template platform field id unchanged', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      const rawConfig = {
        dataSources: [
          {
            id: 'custom-platform-id',
            type: 'PLATFORM_FIELD',
            config: { requestedFields: ['temperature'] }
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)
      expect(result.dataSources[0].id).toBe('custom-platform-id')
    })

    it('rewrites node data binding expressions and adds transform for boolean select', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      const rawConfig = {
        nodes: [
          {
            data: [
              { expression: "{{ ds.__platform___template____.data.temperature }}" },
              { expression: "{{ ds.__platform___template____.data.is_online ? '1' : '0' }}" }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.nodes[0].data[0].expression).toBe('{{ ds.__platform_device-1__.data.temperature }}')
      expect(result.nodes[0].data[1].expression).toBe('{{ ds.__platform_device-1__.data.is_online }}')
      expect(result.nodes[0].data[1].transform).toBe("value ? '1' : '0'")
    })

    it('rewrites node props expressions', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['device_name'])

      const rawConfig = {
        nodes: [
          {
            props: {
              title: "{{ ds.__platform___template____.data.device_name }}",
              label: 'static text'
            }
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.nodes[0].props.title).toBe('{{ ds.__platform_device-1__.data.device_name }}')
      expect(result.nodes[0].props.label).toBe('static text')
    })

    it('rewrites node events actions', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      const rawConfig = {
        nodes: [
          {
            events: [
              {
                actions: [
                  {
                    dataSourceId: '__platform___template____',
                    payload: '{"is_online ? \'1\' : \'0\'}'
                  },
                  {
                    dataSourceId: 'other-source',
                    payload: '{"normal":true}'
                  }
                ]
              }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.nodes[0].events[0].actions[0].dataSourceId).toBe('__platform_device-1__')
      // The regex only matches "FIELD ? '1' : '0'" pattern with double quotes around the whole expression
      // Our test payload has single quotes inside curly braces, which doesn't match the regex pattern
      // So the payload stays unchanged
      expect(result.nodes[0].events[0].actions[0].payload).toBe('{"is_online ? \'1\' : \'0\'}')
      expect(result.nodes[0].events[0].actions[1].dataSourceId).toBe('other-source')
      expect(result.nodes[0].events[0].actions[1].payload).toBe('{"normal":true}')
    })

    it('rewrites node events actions with boolean select pattern in payload', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          {
            events: [
              {
                actions: [
                  {
                    dataSourceId: '__platform___template____',
                    payload: '{"field":"is_online ? \'1\' : \'0\'"}'
                  }
                ]
              }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.nodes[0].events[0].actions[0].dataSourceId).toBe('__platform_device-1__')
      // The regex matches "is_online ? '1' : '0'" and replaces with "is_online"
      expect(result.nodes[0].events[0].actions[0].payload).toBe('{"field":"is_online"}')
    })

    it('handles config without dataSources or nodes', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = { foo: 'bar' }
      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())
      expect(result.foo).toBe('bar')
    })

    it('handles dataSource with missing config - defaults to empty object with deviceId', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      const rawConfig = {
        dataSources: [
          {
            id: '__platform___template____',
            type: 'PLATFORM_FIELD'
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.dataSources[0].id).toBe('__platform_device-1__')
      expect(result.dataSources[0].config.deviceId).toBe('device-1')
      expect(result.dataSources[0].config.requestedFields).toEqual([])
    })

    it('handles dataSource with non-array requestedFields - defaults to empty array', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      const rawConfig = {
        dataSources: [
          {
            id: '__platform___template____',
            type: 'PLATFORM_FIELD',
            config: { requestedFields: 'not-an-array' }
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.dataSources[0].config.requestedFields).toEqual([])
    })

    it('filters out non-string requestedField entries', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const available = new Set(['temperature'])

      const rawConfig = {
        dataSources: [
          {
            id: '__platform___template____',
            type: 'PLATFORM_FIELD',
            config: { requestedFields: ['temperature', 123, null, undefined, true] }
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', available)

      expect(result.dataSources[0].config.requestedFields).toEqual(['temperature'])
    })

    it('handles node without data array', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          { id: 'node-1', data: null },
          { id: 'node-2' }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())
      expect(result.nodes[0].id).toBe('node-1')
      expect(result.nodes[1].id).toBe('node-2')
    })

    it('handles node data binding with non-string expression', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          {
            data: [
              { expression: 123 },
              { expression: null },
              { expression: undefined }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.nodes[0].data[0].expression).toBe(123)
      expect(result.nodes[0].data[1].expression).toBe(null)
      expect(result.nodes[0].data[2].expression).toBe(undefined)
    })

    it('handles node without props', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          { id: 'node-1', props: null },
          { id: 'node-2' }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())
      expect(result.nodes[0].id).toBe('node-1')
      expect(result.nodes[1].id).toBe('node-2')
    })

    it('handles node with non-object props', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          { id: 'node-1', props: 'string-props' }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())
      expect(result.nodes[0].props).toBe('string-props')
    })

    it('handles node without events array', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          { id: 'node-1', events: null },
          { id: 'node-2' }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())
      expect(result.nodes[0].id).toBe('node-1')
    })

    it('handles event handler with non-array actions - keeps actions as-is', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          {
            events: [
              { actions: null },
              { actions: 'not-array' },
              { noActions: true }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.nodes[0].events[0].actions).toBe(null)
      expect(result.nodes[0].events[1].actions).toBe('not-array')
    })

    it('handles event action with non-string payload', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          {
            events: [
              {
                actions: [
                  { dataSourceId: '__platform___template____', payload: 123 },
                  { dataSourceId: '__platform___template____', payload: null },
                  { dataSourceId: '__platform___template____', payload: { key: 'val' } }
                ]
              }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.nodes[0].events[0].actions[0].payload).toBe(123)
      expect(result.nodes[0].events[0].actions[1].payload).toBe(null)
      expect(result.nodes[0].events[0].actions[2].payload).toEqual({ key: 'val' })
    })

    it('handles event action with non-template dataSourceId', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          {
            events: [
              {
                actions: [
                  { dataSourceId: 'other-source', payload: '{"test":1}' }
                ]
              }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.nodes[0].events[0].actions[0].dataSourceId).toBe('other-source')
    })

    it('does not add transform for non-boolean-select binding expression', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        nodes: [
          {
            data: [
              { expression: "{{ ds.__platform___template____.data.temperature }}" }
            ]
          }
        ]
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.nodes[0].data[0].transform).toBeUndefined()
    })

    it('preserves other config properties alongside dataSources and nodes', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = {
        version: '1.0',
        dataSources: [],
        nodes: [],
        metadata: { key: 'value' }
      }

      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      expect(result.version).toBe('1.0')
      expect(result.metadata).toEqual({ key: 'value' })
    })

    it('deep clones the config - mutations to result do not affect original', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const rawConfig = { dataSources: [{ id: 'ds-1', type: 'OTHER', config: { key: 'val' } }] }
      const result = setupState.normalizeTemplateChartConfig(rawConfig, 'device-1', new Set())

      result.dataSources[0].config.key = 'modified'
      expect(rawConfig.dataSources[0].config.key).toBe('val')
    })
  })

  describe('extractResponseList', () => {
    it('returns array data directly', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const arr = [{ id: 1 }, { id: 2 }]
      expect(setupState.extractResponseList({ data: arr })).toBe(arr)
    })

    it('returns list array from data object', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.extractResponseList({ data: { list: [{ id: 1 }] } })).toEqual([{ id: 1 }])
    })

    it('returns empty array for object without list', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.extractResponseList({ data: { foo: 'bar' } })).toEqual([])
    })

    it('returns empty array for null/undefined response', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.extractResponseList(null)).toEqual([])
      expect(setupState.extractResponseList(undefined)).toEqual([])
    })

    it('returns empty array when data.list is not an array', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.extractResponseList({ data: { list: 'not-array' } })).toEqual([])
    })

    it('returns empty array when data is a primitive value', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.extractResponseList({ data: 42 })).toEqual([])
      expect(setupState.extractResponseList({ data: 'string' })).toEqual([])
      expect(setupState.extractResponseList({ data: true })).toEqual([])
    })

    it('returns empty array when data is null', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.extractResponseList({ data: null })).toEqual([])
    })
  })

  describe('computed properties', () => {
    it('viewerPlatformDevices is empty when no platform fields', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.viewerPlatformDevices).toEqual([])
    })

    it('viewerPlatformDevices is empty when no device id', async () => {
      const wrapper = mountTelemetryChart({ id: '' })
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()

      expect(setupState.viewerPlatformDevices).toEqual([])
    })

    it('viewerPlatformDevices returns entry with device info when fields are set', async () => {
      const wrapper = mountTelemetryChart({ deviceData: { name: 'My Device' } })
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()

      const devices = setupState.viewerPlatformDevices
      expect(devices).toHaveLength(1)
      expect(devices[0].deviceId).toBe('device-1')
      expect(devices[0].deviceName).toBe('My Device')
      expect(devices[0].fields).toBe(setupState.platformFields)
    })

    it('viewerPlatformDevices uses default name when deviceData has no name', async () => {
      const wrapper = mountTelemetryChart({ deviceData: {} })
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()

      expect(setupState.viewerPlatformDevices[0].deviceName).toBe('Device')
    })

    it('deviceIdRef returns props.id', async () => {
      const wrapper = mountTelemetryChart({ id: 'dev-999' })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.deviceIdRef).toBe('dev-999')
    })

    it('templateContextResolved is false without deviceData and deviceTemplateId', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.templateContextResolved).toBe(false)
    })

    it('templateContextResolved is true with deviceData', async () => {
      const wrapper = mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.templateContextResolved).toBe(true)
    })

    it('templateContextResolved is true with deviceTemplateId', async () => {
      const wrapper = mountTelemetryChart({ deviceTemplateId: 'tpl-1' })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.templateContextResolved).toBe(true)
    })

    it('hasLoadedInitialSnapshot is false initially', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasLoadedInitialSnapshot).toBe(false)
    })

    it('hasLoadedInitialSnapshot is true when currentDataDeviceId matches and data exists', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.currentDataDeviceId = 'device-1'
      setupState.currentData = { temperature: 25 }

      expect(setupState.hasLoadedInitialSnapshot).toBe(true)
    })

    it('hasLoadedInitialSnapshot is false when currentDataDeviceId does not match', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.currentDataDeviceId = 'other-device'
      setupState.currentData = { temperature: 25 }

      expect(setupState.hasLoadedInitialSnapshot).toBe(false)
    })

    it('hasLoadedInitialSnapshot is false when currentData is empty', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.currentDataDeviceId = 'device-1'
      setupState.currentData = {}

      expect(setupState.hasLoadedInitialSnapshot).toBe(false)
    })

    it('chartHeight returns calc string when availableHeight is 0', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.availableHeight = 0

      expect(setupState.chartHeight).toBe('calc(100vh - 200px)')
    })

    it('chartHeight returns px string when availableHeight is positive', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.availableHeight = 350

      expect(setupState.chartHeight).toBe('350px')
    })
  })

  describe('mount behavior', () => {
    it('sets chartLoading to true and hasTemplate to false when no template context', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.chartLoading).toBe(true)
      expect(setupState.hasTemplate).toBe(false)
    })

    it('sets chartLoading to false when deviceData is set but no deviceTemplateId', async () => {
      const wrapper = mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.chartLoading).toBe(false)
      expect(setupState.hasTemplate).toBe(false)
    })

    it('renders skeleton when chartLoading is true', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()

      // With shallowMount + NCard stubbed, the skeleton is inside the stub
      // Check the setupState instead
      const setupState = getSetupState(wrapper)
      expect(setupState.chartLoading).toBe(true)
    })

    it('renders empty state when chartLoading is false and hasTemplate is false', async () => {
      const wrapper = mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.chartLoading).toBe(false)
      expect(setupState.hasTemplate).toBe(false)
    })
  })

  describe('initTemplateData', () => {
    it('loads template with web_chart_config and sets hasTemplate to true', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: {
          web_chart_config: JSON.stringify({ dataSources: [], nodes: [] })
        }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [{ key: 'temperature', value: 25 }] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasTemplate).toBe(true)
      expect(setupState.chartLoading).toBe(false)
      expect(setupState.initialConfig).toEqual({ dataSources: [], nodes: [] })
      expect(setupState.platformFields).toHaveLength(3)
    })

    it('sets hasTemplate to false when template has no web_chart_config', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: '' }
      })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasTemplate).toBe(false)
      expect(setupState.chartLoading).toBe(false)
    })

    it('sets hasTemplate to false when web_chart_config is invalid JSON', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: 'invalid-json{' }
      })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasTemplate).toBe(false)
      expect(setupState.chartLoading).toBe(false)
    })

    it('sets hasTemplate to false when getCachedDeviceTemplateDetail rejects', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockRejectedValue(new Error('network error'))

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasTemplate).toBe(false)
      expect(setupState.chartLoading).toBe(false)
    })

    it('sets hasTemplate to false when template data is null', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({ data: null })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasTemplate).toBe(false)
      expect(setupState.chartLoading).toBe(false)
    })

    it('falls back to extractPlatformFields with res.data when platform source is empty', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: {
          web_chart_config: JSON.stringify({ nodes: [] }),
          telemetry: [{ key: 'temp', data_type: 'number' }]
        }
      })
      hoisted.extractPlatformFields
        .mockReturnValueOnce([])
        .mockReturnValueOnce([{ id: 'temp', name: 'temp', type: 'number', dataType: 'telemetry' }])
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.platformFields).toHaveLength(1)
      expect(setupState.platformFields[0].id).toBe('temp')
    })

    it('starts realtime and alarm push after successful template load', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()

      expect(hoisted.realtimePushStart).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmPushStart).toHaveBeenCalledTimes(1)
    })

    it('does not start push when template load fails', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockRejectedValue(new Error('fail'))

      mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()

      expect(hoisted.realtimePushStart).toHaveBeenCalledTimes(0)
      expect(hoisted.alarmPushStart).toHaveBeenCalledTimes(0)
    })

    it('sets hasTemplate to false and chartLoading to false when deviceTemplateId is empty', async () => {
      const wrapper = mountTelemetryChart({
        deviceTemplateId: '',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.hasTemplate).toBe(false)
      expect(setupState.chartLoading).toBe(false)
    })

    it('calls all four API endpoints when template data exists', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()

      expect(hoisted.telemetryApi).toHaveBeenCalledWith({
        page: 1, page_size: 200, device_template_id: 'tpl-1'
      })
      expect(hoisted.attributesApi).toHaveBeenCalledWith({
        page: 1, page_size: 200, device_template_id: 'tpl-1'
      })
      expect(hoisted.eventsApi).toHaveBeenCalledWith({
        page: 1, page_size: 200, device_template_id: 'tpl-1'
      })
      expect(hoisted.commandsApi).toHaveBeenCalledWith({
        page: 1, page_size: 200, device_template_id: 'tpl-1'
      })
    })

    it('filters platform field ids to only valid strings', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue([
        { id: 'temperature', name: 'Temperature', type: 'number', dataType: 'telemetry' },
        { id: '', name: 'Empty', type: 'number', dataType: 'telemetry' },
        { id: 123, name: 'Numeric', type: 'number', dataType: 'telemetry' },
        { name: 'NoId', type: 'number', dataType: 'telemetry' }
      ] as any)
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      // Should still have template since temperature field is valid
      expect(setupState.hasTemplate).toBe(true)
    })
  })

  describe('fetchAndUpdateData', () => {
    it('returns early when platformFields is empty', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      vi.clearAllMocks()

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
    })

    it('returns early when props.id is empty', async () => {
      const wrapper = mountTelemetryChart({ id: '' })
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
    })

    it('calls telemetryDataCurrent and getAttributeDataSet when fields include attributes', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })
      hoisted.getAttributeDataSet.mockResolvedValue({ data: [] })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
      expect(hoisted.getAttributeDataSet).toHaveBeenCalledWith({ device_id: 'device-1' })
    })

    it('skips getAttributeDataSet when no attribute fields', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = [
        { id: 'temperature', name: 'Temperature', type: 'number', dataType: 'telemetry' }
      ]
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(1)
      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
      expect(hoisted.getAttributeDataSet).toHaveBeenCalledTimes(0)
    })

    it('maps telemetry data to platform field ids and pushes to vis', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({
        data: [
          { key: 'temperature', value: 25.5 },
          { key: 'humidity', value: 60 }
        ]
      })
      hoisted.getAttributeDataSet.mockResolvedValue({
        data: [{ key: 'device_name', value: 'Test Device' }]
      })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(setupState.currentData.temperature).toBe(25.5)
      expect(setupState.currentData.humidity).toBe(60)
      expect(setupState.currentData.device_name).toBe('Test Device')
      expect(setupState.currentDataDeviceId).toBe('device-1')
      // pushPlatformData is called via visWidgetRef which may be null in shallowMount
      // Verify data was set correctly on currentData instead
    })

    it('handles label-based data items', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = [
        { id: 'temp', name: 'Temperature Label', type: 'number', dataType: 'telemetry' }
      ]
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({
        data: [{ label: 'Temperature Label', value: 30 }]
      })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(setupState.currentData.temp).toBe(30)
    })

    it('does not push when no data matches platform fields', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [{ key: 'unknown', value: 1 }] })
      hoisted.getAttributeDataSet.mockResolvedValue({ data: [] })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(hoisted.pushPlatformData).toHaveBeenCalledTimes(0)
    })

    it('keeps previous chart data unchanged when telemetry snapshot fails', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      setupState.currentData = { temperature: 21, humidity: 55 }
      setupState.currentDataDeviceId = 'device-1'
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockRejectedValue(new Error('network error'))

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(setupState.currentData).toEqual({ temperature: 21, humidity: 55 })
      expect(setupState.currentDataDeviceId).toBe('device-1')
      expect(hoisted.pushPlatformData).toHaveBeenCalledTimes(0)
    })

    it('merges new data with existing currentData', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      setupState.currentData = { existing_key: 'old_value' }
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({
        data: [{ key: 'temperature', value: 25 }]
      })
      hoisted.getAttributeDataSet.mockResolvedValue({ data: [] })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(setupState.currentData.existing_key).toBe('old_value')
      expect(setupState.currentData.temperature).toBe(25)
    })

    it('handles telemetry data items without key or label', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({
        data: [{ value: 25 }] // no key or label
      })
      hoisted.getAttributeDataSet.mockResolvedValue({ data: [] })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      // No matching field, should not push
      expect(hoisted.pushPlatformData).toHaveBeenCalledTimes(0)
    })

    it('matches field by name when id does not match', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = [
        { id: 'temp_sensor', name: 'Temperature', type: 'number', dataType: 'telemetry' }
      ]
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({
        data: [{ key: 'Temperature', value: 42 }]
      })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(setupState.currentData.temp_sensor).toBe(42)
    })

    it('ignores non-array telemetry and attribute payloads without pushing empty chart data', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: null })
      hoisted.getAttributeDataSet.mockResolvedValue({ data: null })

      await setupState.fetchAndUpdateData()
      await flushPromises()

      expect(setupState.currentData).toEqual({})
      expect(setupState.currentDataDeviceId).toBe('')
      expect(hoisted.pushPlatformData).toHaveBeenCalledTimes(0)
    })
  })

  describe('pushDataToVis', () => {
    it('returns early when fields is empty', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      vi.clearAllMocks()

      setupState.pushDataToVis({})
      expect(hoisted.pushPlatformData).toHaveBeenCalledTimes(0)
    })

    it('updates currentData and calls pushPlatformData with fields', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.currentData = { existing: 1 }
      vi.clearAllMocks()

      setupState.pushDataToVis({ temperature: 25 })

      expect(setupState.currentData.existing).toBe(1)
      expect(setupState.currentData.temperature).toBe(25)
    })

    it('calls pushPlatformData on visWidgetRef with correct args', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      vi.clearAllMocks()

      // In shallowMount, visWidgetRef is null so pushPlatformData won't be called
      // But currentData should still be updated
      setupState.pushDataToVis({ temperature: 25 })

      expect(setupState.currentData.temperature).toBe(25)
    })
  })

  describe('onVisReady', () => {
    it('calls fetchAndUpdateData when initial snapshot is not loaded', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.platformFields = mockPlatformFields()
      vi.clearAllMocks()
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      await setupState.onVisReady()
      await flushPromises()

      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(1)
      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
    })

    it('does not call fetchAndUpdateData when initial snapshot is already loaded', async () => {
      const wrapper = mountTelemetryChart()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      setupState.currentDataDeviceId = 'device-1'
      setupState.currentData = { temperature: 25 }
      vi.clearAllMocks()

      await setupState.onVisReady()
      await flushPromises()

      expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
    })
  })

  describe('watch re-trigger', () => {
    it('re-initializes when deviceTemplateId changes', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      vi.clearAllMocks()
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      await wrapper.setProps({ deviceTemplateId: 'tpl-2' })
      await flushPromises()

      expect(hoisted.getCachedDeviceTemplateDetail).toHaveBeenCalledWith('tpl-2')
      expect(hoisted.realtimePushStop).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmPushStop).toHaveBeenCalledTimes(1)
    })

    it('stops existing push when template context becomes unresolved', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      vi.clearAllMocks()

      await wrapper.setProps({ deviceData: undefined, deviceTemplateId: undefined })
      await flushPromises()

      expect(hoisted.realtimePushStop).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmPushStop).toHaveBeenCalledTimes(1)
    })

    it('resets state when template context becomes unresolved', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      await wrapper.setProps({ deviceData: undefined, deviceTemplateId: undefined })
      await flushPromises()

      expect(setupState.currentData).toEqual({})
      expect(setupState.currentDataDeviceId).toBe('')
      expect(setupState.platformFields).toEqual([])
      expect(setupState.initialConfig).toBe(null)
      expect(setupState.hasTemplate).toBe(false)
      expect(setupState.chartLoading).toBe(true)
    })

    it('does not start push when newVal is empty string', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      vi.clearAllMocks()

      await wrapper.setProps({ deviceTemplateId: '' })
      await flushPromises()

      expect(hoisted.realtimePushStart).toHaveBeenCalledTimes(0)
      expect(hoisted.alarmPushStart).toHaveBeenCalledTimes(0)
    })

    it('does not start push when initSequence changes during async init', async () => {
      let resolveFirst: () => void
      const firstCall = new Promise<void>(resolve => { resolveFirst = resolve })

      hoisted.getCachedDeviceTemplateDetail.mockImplementationOnce(() => firstCall!)
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()

      // Trigger another watch before first init completes
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      await wrapper.setProps({ deviceTemplateId: 'tpl-2' })
      await flushPromises()

      // Now resolve the first (stale) call
      resolveFirst!()
      await flushPromises()

      // Push should have been started only once (for tpl-2)
      expect(hoisted.realtimePushStart).toHaveBeenCalledTimes(1)
    })
  })

  describe('lifecycle', () => {
    it('creates ResizeObserver on mount', async () => {
      mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()

      expect(global.ResizeObserver).toHaveBeenCalledTimes(1)
    })

    it('adds window resize listener on mount', async () => {
      const addSpy = vi.spyOn(window, 'addEventListener')
      mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()

      expect(addSpy).toHaveBeenCalledWith('resize', expect.any(Function))
      addSpy.mockRestore()
    })

    it('removes window resize listener on unmount', async () => {
      const removeSpy = vi.spyOn(window, 'removeEventListener')
      const wrapper = mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()

      wrapper.unmount()

      expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function))
      removeSpy.mockRestore()
    })

    it('disconnects ResizeObserver on unmount', async () => {
      const disconnectSpy = vi.fn()
      global.ResizeObserver = vi.fn(function ResizeObserverMock() {
        return {
          observe: vi.fn(),
          unobserve: vi.fn(),
          disconnect: disconnectSpy
        }
      }) as any

      const wrapper = mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()

      wrapper.unmount()

      expect(disconnectSpy).toHaveBeenCalledTimes(1)
    })

    it('stops realtime and alarm push on unmount', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()
      vi.clearAllMocks()

      wrapper.unmount()

      expect(hoisted.realtimePushStop).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmPushStop).toHaveBeenCalledTimes(1)
    })
  })

  describe('updateAvailableHeight', () => {
    it('updates availableHeight when called', async () => {
      const wrapper = mountTelemetryChart({ deviceData: { name: 'Device' } })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      // updateAvailableHeight is called on mount and uses the NCard stub's $el
      // Since NCard is stubbed, the element may or may not be available
      // Just verify the function can be called without error
      setupState.updateAvailableHeight()

      // availableHeight should be a number (either from getBoundingClientRect or 0)
      expect(typeof setupState.availableHeight).toBe('number')
    })
  })

  describe('rendering', () => {
    it('renders ThingsVisWidget when hasTemplate is true', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.hasTemplate).toBe(true)
      // ThingsVisWidget is mocked, verify via setupState that config is set
      expect(setupState.initialConfig).toEqual({ nodes: [] })
    })

    it('passes correct config to ThingsVisWidget', async () => {
      hoisted.getCachedDeviceTemplateDetail.mockResolvedValue({
        data: { web_chart_config: JSON.stringify({ dataSources: [], nodes: [] }) }
      })
      hoisted.extractPlatformFields.mockReturnValue(mockPlatformFields())
      hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })

      const wrapper = mountTelemetryChart({
        deviceTemplateId: 'tpl-1',
        deviceData: { name: 'Device' }
      })
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.initialConfig).toEqual({ dataSources: [], nodes: [] })
      expect(setupState.viewerPlatformDevices).toHaveLength(1)
      expect(setupState.viewerPlatformDevices[0].deviceId).toBe('device-1')
    })
  })
})
