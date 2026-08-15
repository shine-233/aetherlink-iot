/**
 * 文件用途: 测试 App 图表配置步骤。
 * 核心逻辑: 模拟移动端图表编辑器和模板保存接口，验证配置回写。
 * 关键注意事项: 断言要关注模板配置内容，避免只验证按钮点击。
 * 重构建议: 与 Web 图表配置测试共用编辑器和保存断言工具。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getTemplat: vi.fn(),
  putTemplat: vi.fn(),
  telemetryApi: vi.fn(),
  attributesApi: vi.fn(),
  extractPlatformFields: vi.fn(),
  mergePlatformFieldsById: vi.fn((primary, fallback) => {
    const seen = new Set<string>()
    return [...primary, ...fallback].filter((field) => {
      if (!field?.id || seen.has(field.id)) return false
      seen.add(field.id)
      return true
    })
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/service/api', () => ({
  getTemplat: hoisted.getTemplat,
  putTemplat: hoisted.putTemplat,
  telemetryApi: hoisted.telemetryApi,
  attributesApi: hoisted.attributesApi
}))

vi.mock('@/utils/thingsvis/platform-fields', () => ({
  extractPlatformFields: hoisted.extractPlatformFields,
  mergePlatformFieldsById: hoisted.mergePlatformFieldsById
}))

vi.mock('@/components/thingsvis/ThingsVisWidget.vue', () => ({
  default: defineComponent({
    setup() {
      return () => h('div')
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  ExpandOutline: defineComponent({ setup: () => () => h('div') }),
  ContractOutline: defineComponent({ setup: () => () => h('div') }),
  CloseOutline: defineComponent({ setup: () => () => h('div') })
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({
    emits: ['click'],
    setup(_, { slots, emit }) {
      return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
    }
  }),
  NModal: defineComponent({
    props: { show: Boolean },
    emits: ['update:show'],
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NCard: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NEmpty: defineComponent({
    setup() {
      return () => h('div')
    }
  }),
  NSelect: defineComponent({
    props: { value: { default: null } },
    emits: ['update:value'],
    setup() {
      return () => h('div')
    }
  }),
  NSpace: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NSpin: defineComponent({
    setup() {
      return () => h('div')
    }
  }),
  NIcon: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  })
}))

import Component from '../app-chart-config.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []
const extractedFields = [
  { id: 'temperature', name: 'Temperature', type: 'number', dataType: 'telemetry', unit: 'C' },
  { id: 'firmware', name: 'Firmware', type: 'string', dataType: 'attributes' },
  { id: 'reboot', name: 'Reboot', type: 'string', dataType: 'command' }
]
const fallbackFields = [
  { id: 'temperature', name: 'Template Temperature', type: 'number', dataType: 'telemetry', unit: 'C' },
  { id: 'humidity', name: 'Humidity', type: 'number', dataType: 'telemetry', unit: '%' }
]

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { stepCurrent: 4, modalVisible: false, deviceTemplateId: 'tpl-1', ...props },
    global: {
      stubs: {
        ThingsVisWidget: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>
const unwrap = <T>(value: any): T => (value && typeof value === 'object' && 'value' in value ? value.value : value)
const query = { page: 1, page_size: 1000, device_template_id: 'tpl-1' }

describe('device/template/components/step/app-chart-config.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getTemplat.mockResolvedValue({
      data: {
        id: 'tpl-1',
        name: 'Telemetry Model 1',
        app_chart_config: JSON.stringify({ widgets: [{ id: 'app-widget' }], refreshInterval: 60000 })
      },
      error: null
    })
    hoisted.putTemplat.mockResolvedValue({ error: null })
    hoisted.telemetryApi.mockResolvedValue({ data: { list: [{ identifier: 'temperature' }] } })
    hoisted.attributesApi.mockResolvedValue({ data: { list: [{ identifier: 'firmware' }] } })
    hoisted.extractPlatformFields.mockImplementation((source: unknown) => {
      if (source && typeof source === 'object' && Object.prototype.hasOwnProperty.call(source, 'app_chart_config')) {
        return fallbackFields
      }
      return extractedFields
    })
    ;(window as any).$message = { success: vi.fn(), error: vi.fn() }
  })

  afterEach(() => {
    ;(window as any).$message = undefined
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads app chart config from telemetry and attribute APIs and filters command fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const expectedFields = [extractedFields[0], extractedFields[1], fallbackFields[1]]

    expect(hoisted.getTemplat).toHaveBeenCalledWith('tpl-1')
    expect(hoisted.telemetryApi).toHaveBeenCalledWith(query)
    expect(hoisted.attributesApi).toHaveBeenCalledWith(query)
    expect(hoisted.extractPlatformFields).toHaveBeenCalledWith({
      telemetry: [{ identifier: 'temperature' }],
      attributes: [{ identifier: 'firmware' }]
    })
    expect(hoisted.extractPlatformFields).toHaveBeenCalledWith(expect.objectContaining({ id: 'tpl-1' }))
    expect(hoisted.mergePlatformFieldsById).toHaveBeenCalledWith(extractedFields, fallbackFields)
    expect(state.loading).toBe(false)
    expect(state.initialConfig).toEqual({ widgets: [{ id: 'app-widget' }], refreshInterval: 60000 })
    expect(state.hasConfig).toBe(true)
    expect(state.refreshInterval).toBe(60000)
    expect(state.refreshOptions.map((item: { value: number }) => item.value)).toEqual([0, 5000, 10000, 30000, 60000])
    expect(state.platformFields).toEqual(expectedFields)
    expect(unwrap<any[]>(state.platformDevices)).toEqual([
      {
        deviceId: '__template__',
        deviceName: 'device_template.currentThingModel',
        groupId: '__template__',
        groupName: 'device_template.thingModelFields',
        fields: expectedFields,
        presets: []
      }
    ])
  })

  it('emits app step navigation events and opens the editor modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    state.openEditor()
    state.next()
    state.back()
    state.cancellation()

    expect(state.showEditorModal).toBe(true)
    expect(wrapper.emitted('update:stepCurrent')).toEqual([[5], [3]])
    expect(wrapper.emitted('update:modalVisible')).toEqual([[]])
  })

  it('saves app chart config with refresh interval and runtime device ids removed', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.refreshInterval = 0

    await state.handleSave({
      widgets: [{ id: 'app-chart' }],
      dataSources: [
        { type: 'PLATFORM_FIELD', config: { deviceId: 'runtime-device', field: 'firmware' } },
        { type: 'STATIC', config: { deviceId: 'static-device' } }
      ]
    })

    expect(hoisted.putTemplat).toHaveBeenCalledTimes(1)
    const saved = hoisted.putTemplat.mock.calls[0][0]
    const savedConfig = JSON.parse(saved.app_chart_config)
    expect(saved.id).toBe('tpl-1')
    expect(savedConfig.widgets).toEqual([{ id: 'app-chart' }])
    expect(savedConfig.refreshInterval).toBe(0)
    expect(Object.prototype.hasOwnProperty.call(savedConfig.dataSources[0].config, 'deviceId')).toBe(false)
    expect(savedConfig.dataSources[0].config.field).toBe('firmware')
    expect(savedConfig.dataSources[1].config.deviceId).toBe('static-device')
    expect(state.initialConfig).toEqual(savedConfig)
    expect(state.hasConfig).toBe(true)
    expect(state.showEditorModal).toBe(false)
  })
})
