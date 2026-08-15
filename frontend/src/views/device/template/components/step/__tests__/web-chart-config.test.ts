/**
 * 文件用途: 测试 Web 图表配置步骤。
 * 核心逻辑: 模拟图表编辑器和模板接口，验证打开编辑器、保存和回填行为。
 * 关键注意事项: 断言应关注配置是否写回模板，而不只是弹窗是否出现。
 * 重构建议: 与 App 图表配置测试共用编辑器 mock 和保存断言。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getTemplat: vi.fn(),
  putTemplat: vi.fn(),
  telemetryApi: vi.fn(),
  attributesApi: vi.fn(),
  eventsApi: vi.fn(),
  commandsApi: vi.fn(),
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
  attributesApi: hoisted.attributesApi,
  eventsApi: hoisted.eventsApi,
  commandsApi: hoisted.commandsApi
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

import Component from '../web-chart-config.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []
const platformFields = [
  { id: 'temperature', name: 'Temperature', type: 'number', dataType: 'telemetry', unit: 'C' },
  { id: 'online', name: 'Online', type: 'boolean', dataType: 'attributes' },
  { id: 'overheat', name: 'Overheat', type: 'json', dataType: 'event' },
  { id: 'reboot', name: 'Reboot', type: 'string', dataType: 'command' }
]
const fallbackFields = [
  { id: 'temperature', name: 'Template Temperature', type: 'number', dataType: 'telemetry', unit: 'C' },
  { id: 'voltage', name: 'Voltage', type: 'number', dataType: 'telemetry', unit: 'V' }
]

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { stepCurrent: 3, modalVisible: false, deviceTemplateId: 'tpl-1', ...props },
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

describe('device/template/components/step/web-chart-config.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getTemplat.mockResolvedValue({
      data: {
        id: 'tpl-1',
        name: 'Telemetry Model 1',
        web_chart_config: JSON.stringify({ widgets: [{ id: 'web-widget' }], refreshInterval: 30000 })
      },
      error: null
    })
    hoisted.putTemplat.mockResolvedValue({ error: null })
    hoisted.telemetryApi.mockResolvedValue({ data: { list: [{ identifier: 'temperature' }] } })
    hoisted.attributesApi.mockResolvedValue({ data: { list: [{ identifier: 'online' }] } })
    hoisted.eventsApi.mockResolvedValue({ data: { list: [{ identifier: 'overheat' }] } })
    hoisted.commandsApi.mockResolvedValue({ data: { list: [{ identifier: 'reboot' }] } })
    hoisted.extractPlatformFields.mockImplementation((source: unknown) => {
      if (source && typeof source === 'object' && Object.prototype.hasOwnProperty.call(source, 'web_chart_config')) {
        return fallbackFields
      }
      return platformFields
    })
    ;(window as any).$message = { success: vi.fn(), error: vi.fn() }
  })

  afterEach(() => {
    ;(window as any).$message = undefined
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads persisted web chart config and all thing-model field APIs for the editor', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.getTemplat).toHaveBeenCalledWith('tpl-1')
    expect(hoisted.telemetryApi).toHaveBeenCalledWith(query)
    expect(hoisted.attributesApi).toHaveBeenCalledWith(query)
    expect(hoisted.eventsApi).toHaveBeenCalledWith(query)
    expect(hoisted.commandsApi).toHaveBeenCalledWith(query)
    expect(hoisted.extractPlatformFields).toHaveBeenCalledWith({
      telemetry: [{ identifier: 'temperature' }],
      attributes: [{ identifier: 'online' }],
      events: [{ identifier: 'overheat' }],
      commands: [{ identifier: 'reboot' }]
    })
    expect(state.loading).toBe(false)
    expect(state.hasConfig).toBe(true)
    expect(state.initialConfig).toEqual({ widgets: [{ id: 'web-widget' }], refreshInterval: 30000 })
    expect(state.refreshInterval).toBe(30000)
    const expectedFields = [...platformFields, fallbackFields[1]]
    expect(hoisted.extractPlatformFields).toHaveBeenCalledWith(expect.objectContaining({ id: 'tpl-1' }))
    expect(hoisted.mergePlatformFieldsById).toHaveBeenCalledWith(platformFields, fallbackFields)
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

  it('emits wizard navigation events and opens the web editor modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    state.openEditor()
    state.next()
    state.back()
    state.cancellation()

    expect(state.showEditorModal).toBe(true)
    expect(wrapper.emitted('update:stepCurrent')).toEqual([[4], [2]])
    expect(wrapper.emitted('update:modalVisible')).toEqual([[]])
  })

  it('saves web chart config with virtual device ids stripped from platform fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.refreshInterval = 10000

    await state.handleSave({
      widgets: [{ id: 'chart-1' }],
      dataSources: [
        { type: 'PLATFORM_FIELD', config: { deviceId: 'runtime-device', field: 'temperature' } },
        { type: 'STATIC', config: { deviceId: 'static-device' } }
      ]
    })

    expect(hoisted.putTemplat).toHaveBeenCalledTimes(1)
    const saved = hoisted.putTemplat.mock.calls[0][0]
    const savedConfig = JSON.parse(saved.web_chart_config)
    expect(saved.id).toBe('tpl-1')
    expect(savedConfig.widgets).toEqual([{ id: 'chart-1' }])
    expect(savedConfig.refreshInterval).toBe(10000)
    expect(Object.prototype.hasOwnProperty.call(savedConfig.dataSources[0].config, 'deviceId')).toBe(false)
    expect(savedConfig.dataSources[0].config.field).toBe('temperature')
    expect(savedConfig.dataSources[1].config.deviceId).toBe('static-device')
    expect(state.initialConfig).toEqual(savedConfig)
    expect(state.hasConfig).toBe(true)
    expect(state.showEditorModal).toBe(false)
  })
})
