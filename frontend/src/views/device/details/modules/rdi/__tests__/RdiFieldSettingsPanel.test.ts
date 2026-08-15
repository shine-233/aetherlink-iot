/**
 * 文件用途：验证 RDI Field Setting 只读面板（REQ-53）的字段行、原始值/解析值列与只读契约。
 * 关键注意事项：该面板不得出现输入框或下发按钮；真实设备配置下发不由本测试证明。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  rdiDeviceConfig: vi.fn(),
  updateRdiDeviceConfig: vi.fn(),
  appStore: {
    locale: 'en-US' as App.I18n.LangType
  }
}))

vi.mock('@/service/api', () => ({
  rdiDeviceConfig: hoisted.rdiDeviceConfig,
  updateRdiDeviceConfig: hoisted.updateRdiDeviceConfig
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn()
  }
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => hoisted.appStore
}))

import RdiFieldSettingsPanel from '../RdiFieldSettingsPanel.vue'
import { labels } from '../constants/rdi-labels'

const SlotStub = defineComponent({
  name: 'NSpin',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const EmptyStub = defineComponent({
  name: 'NEmpty',
  props: {
    description: {
      type: String,
      default: ''
    }
  },
  setup(props) {
    return () => h('div', { class: 'rdi-empty-stub' }, props.description)
  }
})

function mountPanel(id = 'device-1') {
  return shallowMount(RdiFieldSettingsPanel, {
    props: { id },
    global: {
      stubs: {
        NSpin: SlotStub,
        NEmpty: EmptyStub
      }
    }
  })
}

function rowCells(wrapper: ReturnType<typeof mountPanel>, key: string) {
  const row = wrapper.find(`[data-field-row="${key}"]`)
  return {
    field: row.find('th').text(),
    raw: row.findAll('td')[0].text(),
    interpreted: row.findAll('td')[1].text()
  }
}

describe('RdiFieldSettingsPanel', () => {
  beforeEach(() => {
    hoisted.rdiDeviceConfig.mockReset()
    hoisted.updateRdiDeviceConfig.mockReset()
    hoisted.appStore.locale = 'en-US'
    hoisted.rdiDeviceConfig.mockResolvedValue({
      error: null,
      data: {
        config: {
          field_setting: {
            n00: ['1', '2'],
            n01: 'raw-n01',
            n02: 42,
            n03: true,
            sw1: { label: 'Enabled', value: 1 },
            sw2: { value: 7 }
          }
        },
        system_info: {}
      }
    })
  })

  it('renders all twelve n00-n07/sw1-sw4 rows with the three read-only columns', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(hoisted.rdiDeviceConfig).toHaveBeenCalledWith('device-1')

    const headerTexts = wrapper.findAll('thead th').map((cell) => cell.text())
    expect(headerTexts).toEqual([
      labels['en-US'].fieldSettingsFieldColumn,
      labels['en-US'].fieldSettingsRawValueColumn,
      labels['en-US'].fieldSettingsInterpretedValueColumn
    ])
    expect(headerTexts).toEqual(['Field', 'Raw value', 'Interpreted value'])
    expect(wrapper.find('h3').text()).toBe('Field Settings (read-only)')

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(12)
    expect(rows.map((row) => row.find('th').text())).toEqual([
      'n00',
      'n01',
      'n02',
      'n03',
      'n04',
      'n05',
      'n06',
      'n07',
      'sw1',
      'sw2',
      'sw3',
      'sw4'
    ])
    expect(wrapper.findAll('thead th[scope="col"]')).toHaveLength(3)
    expect(wrapper.findAll('tbody th[scope="row"]')).toHaveLength(12)
  })

  it('shows raw JSON alongside the interpreted value for array, scalar and object field types', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(rowCells(wrapper, 'n00')).toEqual({ field: 'n00', raw: '["1","2"]', interpreted: '1,2' })
    expect(rowCells(wrapper, 'n01')).toEqual({ field: 'n01', raw: 'raw-n01', interpreted: 'raw-n01' })
    expect(rowCells(wrapper, 'n02')).toEqual({ field: 'n02', raw: '42', interpreted: '42' })
    expect(rowCells(wrapper, 'n03')).toEqual({ field: 'n03', raw: 'true', interpreted: 'true' })
    expect(rowCells(wrapper, 'sw1')).toEqual({
      field: 'sw1',
      raw: '{"label":"Enabled","value":1}',
      interpreted: 'Enabled'
    })
    expect(rowCells(wrapper, 'sw2')).toEqual({ field: 'sw2', raw: '{"value":7}', interpreted: '{"value":7}' })
    expect(rowCells(wrapper, 'n04')).toEqual({ field: 'n04', raw: '--', interpreted: '--' })
    expect(rowCells(wrapper, 'sw4')).toEqual({ field: 'sw4', raw: '--', interpreted: '--' })
  })

  it('stays strictly read-only with no inputs, buttons or command triggers', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.findAll('button')).toHaveLength(0)
    expect(wrapper.findAll('textarea')).toHaveLength(0)
    expect(wrapper.text()).not.toContain(labels['en-US'].save)
    expect(wrapper.text()).not.toContain(labels['en-US'].sendField)
    expect(hoisted.updateRdiDeviceConfig).toHaveBeenCalledTimes(0)
  })

  it('renders the localized empty state when no n/sw field is configured', async () => {
    hoisted.appStore.locale = 'zh-CN'
    hoisted.rdiDeviceConfig.mockResolvedValue({
      error: null,
      data: { config: { field_setting: {} }, system_info: {} }
    })

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.findAll('table')).toHaveLength(0)
    expect(wrapper.find('.rdi-empty-stub').text()).toBe(labels['zh-CN'].empty)
    expect(wrapper.find('h3').text()).toBe(labels['zh-CN'].fieldSettingsReadonlyTitle)
  })

  it('reloads the saved field settings when the device id changes', async () => {
    hoisted.rdiDeviceConfig.mockReset()
    hoisted.rdiDeviceConfig
      .mockResolvedValueOnce({
        error: null,
        data: { config: { field_setting: { n00: ['1'] } }, system_info: {} }
      })
      .mockResolvedValueOnce({
        error: null,
        data: { config: { field_setting: { sw3: { label: 'Second device' } } }, system_info: {} }
      })

    const wrapper = mountPanel()
    await flushPromises()
    expect(rowCells(wrapper, 'n00')).toEqual({ field: 'n00', raw: '["1"]', interpreted: '1' })

    await wrapper.setProps({ id: 'device-2' })
    await flushPromises()

    expect(hoisted.rdiDeviceConfig).toHaveBeenCalledTimes(2)
    expect(hoisted.rdiDeviceConfig).toHaveBeenNthCalledWith(2, 'device-2')
    expect(rowCells(wrapper, 'sw3')).toEqual({
      field: 'sw3',
      raw: '{"label":"Second device"}',
      interpreted: 'Second device'
    })
    expect(rowCells(wrapper, 'n00')).toEqual({ field: 'n00', raw: '--', interpreted: '--' })
  })
})
