/**
 * 文件用途: 测试小组件预设配置步骤。
 * 核心逻辑: 挂载组件并模拟模板接口，验证预设读取、编辑和保存路径。
 * 关键注意事项: Mock 数据要覆盖已有配置和空配置两类场景。
 * 重构建议: 与图表配置测试共享模板接口 mock 和弹窗断言工具。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getTemplat: vi.fn(),
  putTemplat: vi.fn(),
  buildPresetEditorConfig: vi.fn(),
  extractFirstNodeFromWidgetConfig: vi.fn(),
  getTemplatePresetEntries: vi.fn(),
  getTemplatePresetKey: vi.fn(),
  parseTemplateChartConfig: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/service/api', () => ({
  getTemplat: hoisted.getTemplat,
  putTemplat: hoisted.putTemplat
}))

vi.mock('@/components/thingsvis/ThingsVisWidget.vue', () => ({
  default: defineComponent({ setup() { return () => h('div') } })
}))

vi.mock('@/utils/thingsvis/template-presets', () => ({
  buildPresetEditorConfig: hoisted.buildPresetEditorConfig,
  extractFirstNodeFromWidgetConfig: hoisted.extractFirstNodeFromWidgetConfig,
  getTemplatePresetEntries: hoisted.getTemplatePresetEntries,
  getTemplatePresetKey: hoisted.getTemplatePresetKey,
  parseTemplateChartConfig: hoisted.parseTemplateChartConfig
}))

vi.mock('naive-ui', () => ({
  NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } })
}))

import Component from '../widget-preset-config.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      presetModalVisible: false,
      deviceTemplateId: 'tpl-1',
      property: { id: 'p1', name: 'Prop 1', identifier: 'prop1', dataType: 'string' },
      propertyType: 'telemetry',
      ...props
    },
    global: {
      stubs: {
        ThingsVisWidget: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/template/components/step/widget-preset-config.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getTemplat.mockResolvedValue({
      data: {
        id: 'tpl-1',
        web_chart_config: JSON.stringify({
          widgets: [{ id: 'dashboard-widget' }],
          device_widget_presets: {
            'telemetry:prop1': [{ id: 'preset_telemetry:prop1', widget: { id: 'old-node' } }]
          }
        })
      },
      error: null
    })
    hoisted.putTemplat.mockResolvedValue({ error: null })
    hoisted.getTemplatePresetEntries.mockReturnValue([{ widget: { id: 'old-node', type: 'line' } }])
    hoisted.buildPresetEditorConfig.mockImplementation((widget: unknown) => ({ nodes: [widget] }))
    hoisted.parseTemplateChartConfig.mockReturnValue({
      widgets: [{ id: 'dashboard-widget' }],
      device_widget_presets: {
        'telemetry:prop1': [{ id: 'preset_telemetry:prop1', widget: { id: 'old-node' } }],
        'attributes:other': [{ id: 'preset_attributes:other', widget: { id: 'other-node' } }]
      }
    })
    hoisted.extractFirstNodeFromWidgetConfig.mockReturnValue({ id: 'new-node', type: 'gauge' })
    hoisted.getTemplatePresetKey.mockReturnValue('telemetry:prop1')
    ;(window as any).$message = { success: vi.fn(), error: vi.fn() }
  })

  afterEach(() => {
    ;(window as any).$message = undefined
    while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() }
  })

  it('opens preset editor with a single platform field for the selected property', async () => {
    const wrapper = mountComponent({
      presetModalVisible: true,
      property: { id: 'p1', name: 'Temperature', identifier: 'temperature', dataType: 'number', unit: 'C' }
    })
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    expect(hoisted.getTemplat).toHaveBeenCalledWith('tpl-1')
    expect(hoisted.getTemplatePresetEntries).toHaveBeenCalledWith(
      JSON.stringify({
        widgets: [{ id: 'dashboard-widget' }],
        device_widget_presets: {
          'telemetry:prop1': [{ id: 'preset_telemetry:prop1', widget: { id: 'old-node' } }]
        }
      }),
      'telemetry',
      'temperature'
    )
    expect(hoisted.buildPresetEditorConfig).toHaveBeenCalledWith({ id: 'old-node', type: 'line' })
    expect(state.loading).toBe(false)
    expect(state.platformFields).toEqual([
      { id: 'temperature', name: 'Temperature', type: 'number', dataType: 'telemetry', unit: 'C' }
    ])
    expect(state.initialConfig).toEqual({ nodes: [{ id: 'old-node', type: 'line' }] })
  })

  it('normalizes attribute presets and unsupported property data types for editor fields', async () => {
    const wrapper = mountComponent({
      presetModalVisible: true,
      propertyType: 'attributes',
      property: { id: 'p2', name: 'Mode', identifier: 'mode', dataType: 'enum' }
    })
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.platformFields).toEqual([{ id: 'mode', name: 'Mode', type: 'number', dataType: 'attribute', unit: undefined }])
  })

  it('saves the first editor widget into web chart device widget presets', async () => {
    const wrapper = mountComponent({ presetModalVisible: true })
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    await state.handleSave({ nodes: [{ id: 'new-node' }] })

    expect(hoisted.getTemplat).toHaveBeenCalledWith('tpl-1')
    expect(hoisted.extractFirstNodeFromWidgetConfig).toHaveBeenCalledWith({ nodes: [{ id: 'new-node' }] })
    expect(hoisted.getTemplatePresetKey).toHaveBeenCalledWith('telemetry', 'prop1')
    expect(hoisted.putTemplat).toHaveBeenCalledTimes(1)
    const saved = hoisted.putTemplat.mock.calls[0][0]
    const webConfig = JSON.parse(saved.web_chart_config)
    expect(saved.id).toBe('tpl-1')
    expect(webConfig.widgets).toEqual([{ id: 'dashboard-widget' }])
    expect(webConfig.device_widget_presets['telemetry:prop1'][0].id).toBe('preset_telemetry:prop1')
    expect(webConfig.device_widget_presets['telemetry:prop1'][0].widget).toEqual({ id: 'new-node', type: 'gauge' })
    expect(webConfig.device_widget_presets['attributes:other'][0].widget).toEqual({ id: 'other-node' })
    expect(wrapper.emitted('update:presetModalVisible')).toEqual([[false]])
  })

  it('clears the selected preset when the editor has no widget node', async () => {
    hoisted.extractFirstNodeFromWidgetConfig.mockReturnValue(null)
    const wrapper = mountComponent({ presetModalVisible: true })
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    await state.handleSave({ nodes: [] })

    const saved = hoisted.putTemplat.mock.calls[0][0]
    const webConfig = JSON.parse(saved.web_chart_config)
    expect(Object.prototype.hasOwnProperty.call(webConfig.device_widget_presets, 'telemetry:prop1')).toBe(false)
    expect(webConfig.device_widget_presets['attributes:other'][0].id).toBe('preset_attributes:other')
  })
})
