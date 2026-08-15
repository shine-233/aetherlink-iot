/**
 * 文件用途: connection-info 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigEdit: vi.fn(),
  deviceConfigVoucherType: vi.fn(),
  deviceProtocolServiceList: vi.fn(),
  protocolPluginConfigForm: vi.fn(),
  getTopicMappingList: vi.fn(),
  createTopicMapping: vi.fn(),
  updateTopicMapping: vi.fn(),
  deleteTopicMapping: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigEdit: hoisted.deviceConfigEdit,
  deviceConfigVoucherType: hoisted.deviceConfigVoucherType,
  deviceProtocolServiceList: hoisted.deviceProtocolServiceList,
  protocolPluginConfigForm: hoisted.protocolPluginConfigForm,
  getTopicMappingList: hoisted.getTopicMappingList,
  createTopicMapping: hoisted.createTopicMapping,
  updateTopicMapping: hoisted.updateTopicMapping,
  deleteTopicMapping: hoisted.deleteTopicMapping
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'en-US' } } })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NButton: defineComponent({
    emits: ['click'],
    setup(_, { slots, emit }) {
      return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
    }
  }),
  NPopconfirm: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NSpace: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NDataTable: defineComponent({
    props: { data: { type: Array, default: () => [] }, loading: Boolean },
    setup() {
      return () => h('div')
    }
  })
}))

import Component from '../connection-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      configInfo: { id: 'cfg-1', device_type: '1', protocol_config: '{}', protocol_type: 'p1', voucher_type: 'v1' },
      ...props
    },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NForm: defineComponent({
          setup(_, { slots }) {
            return () => h('form', slots.default ? slots.default() : [])
          }
        }),
        NFormItem: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: null } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NButton: true,
        NFlex: true,
        NDrawer: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NDrawerContent: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        FormInput: true,
        TopicMappingModal: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/connection-info.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceProtocolServiceList.mockResolvedValue({
      data: {
        protocol: [{ name: 'MQTT', service_identifier: 'MQTT' }],
        service: [{ name: 'HTTP Push', service_identifier: 'HTTP' }]
      }
    })
    hoisted.deviceConfigVoucherType.mockResolvedValue({ data: { AccessToken: 'token', Basic: 'basic' } })
    hoisted.protocolPluginConfigForm.mockResolvedValue({
      data: [
        { type: 'input', dataKey: 'host', label: 'Host' },
        { dataKey: '__topic_mapping__', default: 'false' }
      ]
    })
    hoisted.getTopicMappingList.mockResolvedValue({
      data: {
        list: [
          {
            id: 'tm-1',
            name: 'uplink-map',
            direction: 'up',
            source_topic: 'devices/+/up',
            target_topic: 'internal/up',
            data_identifier: 'temp',
            priority: 2,
            enabled: true
          }
        ]
      }
    })
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes protocol, voucher and topic mapping state from config info', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.protocolPluginConfigForm).toHaveBeenCalledWith({
      device_type: '1',
      protocol_type: 'p1'
    })
    expect(state.extendForm).toMatchObject({
      id: 'cfg-1',
      protocol_type: 'p1',
      voucher_type: 'v1'
    })
    expect(state.formElements).toEqual([{ type: 'input', dataKey: 'host', label: 'Host' }])
    expect(state.showTopicMapping).toBe(false)
    expect(state.topicMappingList).toEqual([
      expect.objectContaining({
        id: 'tm-1',
        mapping_name: 'uplink-map',
        direction: 'up',
        original_topic: 'devices/+/up',
        target_topic: 'internal/up',
        data_identifier: 'temp',
        priority: 2,
        enabled: true
      })
    ])
    expect(state.topicMappingColumns.map((column: any) => column.key)).toEqual([
      'mapping_name',
      'description',
      'original_topic',
      'target_topic',
      'data_identifier',
      'actions'
    ])
  })

  it('falls back to an empty protocol config when stored JSON is malformed', async () => {
    const wrapper = mountComponent({
      configInfo: {
        id: 'cfg-1',
        device_type: '1',
        protocol_config: '{bad-json',
        protocol_type: 'p1',
        voucher_type: 'v1'
      }
    })
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.protocol_config).toEqual({})
    await state.handleSubmit()
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith(
      expect.objectContaining({
        protocol_config: '{}'
      })
    )
  })

  it('loads topic mappings on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getTopicMappingList).toHaveBeenCalledWith({ device_config_id: 'cfg-1' })
  })

  it('handleAddTopicMapping opens modal with null edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddTopicMapping()
    expect(state.topicMappingModalVisible).toBe(true)
    expect(state.currentEditTopicMapping).toBeNull()
  })

  it('handleEditTopicMapping opens modal with row data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const row = { id: '1', mapping_name: 'test' }
    state.handleEditTopicMapping(row)
    expect(state.topicMappingModalVisible).toBe(true)
    expect(state.currentEditTopicMapping).toMatchObject({ id: '1', mapping_name: 'test' })
  })

  it('handleDeleteTopicMapping calls deleteTopicMapping when id exists', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleDeleteTopicMapping({ id: '1', mapping_name: 'test' })
    expect(hoisted.deleteTopicMapping).toHaveBeenCalledWith('1')
  })

  it('handleSubmit calls deviceConfigEdit and emits upDateConfig', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleSubmit()
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'cfg-1',
        protocol_type: 'p1',
        voucher_type: 'v1',
        protocol_config: '{}'
      })
    )
    expect(wrapper.emitted('upDateConfig')).toEqual([[]])
  })

  it('normalizeTopicMapping maps fields correctly', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.normalizeTopicMapping({
      id: '1',
      name: 'test',
      direction: 'up',
      description: 'desc',
      source_topic: 'src',
      target_topic: 'tgt',
      data_identifier: 'id1',
      priority: 5,
      enabled: false
    })
    expect(result).toMatchObject({
      id: '1',
      mapping_name: 'test',
      direction: 'up',
      description: 'desc',
      original_topic: 'src',
      target_topic: 'tgt',
      data_identifier: 'id1',
      priority: 5,
      enabled: false
    })
  })
})
