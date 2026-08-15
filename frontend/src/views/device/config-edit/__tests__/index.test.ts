/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigAdd: vi.fn(),
  deviceConfigEdit: vi.fn(),
  deviceConfigInfo: vi.fn(),
  deviceConfigVoucherType: vi.fn(),
  deviceProtocolServiceList: vi.fn(),
  deviceTemplate: vi.fn(),
  protocolPluginConfigForm: vi.fn(),
  routerGo: vi.fn(),
  formValidate: vi.fn(),
  formRestoreValidation: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigAdd: hoisted.deviceConfigAdd,
  deviceConfigEdit: hoisted.deviceConfigEdit,
  deviceConfigInfo: hoisted.deviceConfigInfo,
  deviceConfigVoucherType: hoisted.deviceConfigVoucherType,
  deviceProtocolServiceList: hoisted.deviceProtocolServiceList,
  deviceTemplate: hoisted.deviceTemplate,
  protocolPluginConfigForm: hoisted.protocolPluginConfigForm
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/router', () => ({
  router: { go: hoisted.routerGo }
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { id: 'cfg-1' } })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NTooltip: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NIcon: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('@vicons/ionicons5', () => ({ HelpCircle: defineComponent({ setup: () => () => h('div') }) }))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({
          setup(_, { slots, expose }) {
            expose({
              validate: hoisted.formValidate,
              restoreValidation: hoisted.formRestoreValidation
            })
            return () => h('form', slots.default ? slots.default() : [])
          }
        }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NRadioGroup: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NRadio: defineComponent({ props: { value: { default: null } }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        FormInput: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-edit/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfigInfo.mockResolvedValue({ data: { id: 'cfg-1', name: 'cfg', device_type: '1', protocol_config: '{}' } })
    hoisted.deviceProtocolServiceList.mockResolvedValue({ data: { protocol: [], service: [] } })
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceConfigVoucherType.mockResolvedValue({ data: {} })
    hoisted.protocolPluginConfigForm.mockResolvedValue({ data: [] })
    hoisted.formValidate.mockResolvedValue(undefined)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes edit mode with config, templates, protocols, voucher options and form schema', async () => {
    hoisted.deviceConfigInfo.mockResolvedValue({
      data: {
        id: 'cfg-1',
        name: 'MQTT Template',
        device_type: '1',
        protocol_type: 'mqtt',
        voucher_type: 'token',
        device_template_id: 'tpl-1',
        protocol_config: '{"host":"127.0.0.1"}'
      }
    })
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [{ id: 'tpl-1', name: 'Telemetry Model 1' }], total: 1 } })
    hoisted.deviceProtocolServiceList.mockResolvedValue({
      data: {
        protocol: [{ name: 'MQTT', service_identifier: 'mqtt' }],
        service: [{ name: 'HTTP Service', service_identifier: 'http-service' }]
      }
    })
    hoisted.deviceConfigVoucherType.mockResolvedValue({ data: { token: 'token', basic: 'basic' } })
    hoisted.protocolPluginConfigForm.mockResolvedValue({ data: [{ type: 'input', dataKey: 'host', label: 'Host' }] })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
    expect(hoisted.deviceTemplate).toHaveBeenCalledWith({ page: 1, page_size: 20, total: 0 })
    expect(hoisted.deviceProtocolServiceList).toHaveBeenCalledWith({ device_type: '1' })
    expect(hoisted.deviceConfigVoucherType).toHaveBeenCalledWith({ device_type: '1', protocol_type: 'mqtt' })
    expect(hoisted.protocolPluginConfigForm).toHaveBeenCalledWith({ device_type: '1', protocol_type: 'mqtt' })
    expect(state.isEdit).toBe(true)
    expect(state.modalTitle).toBe('common.edit')
    expect(state.configForm).toMatchObject({
      id: 'cfg-1',
      name: 'MQTT Template',
      device_type: '1',
      protocol_type: 'mqtt',
      voucher_type: 'token',
      device_template_id: 'tpl-1'
    })
    expect(state.protocol_config).toEqual({ host: '127.0.0.1' })
    expect(state.deviceTemplateOptions[1]).toEqual({ id: 'tpl-1', name: 'Telemetry Model 1' })
    expect(state.typeOptions).toEqual([
      {
        type: 'group',
        label: 'common.protocol',
        key: 'protocol',
        children: [{ label: 'MQTT', value: 'mqtt' }]
      },
      {
        type: 'group',
        label: 'common.service',
        key: 'service',
        children: [{ label: 'HTTP Service', value: 'http-service' }]
      }
    ])
    expect(state.connectOptions).toEqual([
      { label: 'token', value: 'token' },
      { label: 'basic', value: 'basic' }
    ])
    expect(state.formElements).toEqual([{ type: 'input', dataKey: 'host', label: 'Host' }])
  })

  it('loads config on mount when id exists', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
  })

  it('handleClose resets form and navigates back', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleClose()
    expect(hoisted.formRestoreValidation).toHaveBeenCalledTimes(1)
    expect(hoisted.routerGo).toHaveBeenCalledWith(-1)
  })

  it('handleSubmit calls deviceConfigEdit in edit mode', async () => {
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleSubmit()
    expect(hoisted.formValidate).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'cfg-1',
        name: 'cfg',
        device_type: '1',
        protocol_config: '{}'
      })
    )
    expect(hoisted.routerGo).toHaveBeenCalledWith(-1)
  })

  it('handleSubmit preserves dynamic protocol_config values', async () => {
    hoisted.deviceConfigInfo.mockResolvedValue({
      data: {
        id: 'cfg-1',
        name: 'cfg',
        device_type: '1',
        protocol_type: 'http-service',
        protocol_config: '{"host":"127.0.0.1"}'
      }
    })
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.protocol_config = { host: '10.0.0.5', routes: [{ path: '/telemetry', qos: 1 }] }

    await state.handleSubmit()

    expect(JSON.parse(hoisted.deviceConfigEdit.mock.calls[0][0].protocol_config)).toEqual({
      host: '10.0.0.5',
      routes: [{ path: '/telemetry', qos: 1 }]
    })
  })

  it('choseProtocolType clears stale protocol_config before loading the new schema', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.protocol_config = { host: 'old-plugin-host' }
    state.formElements = [{ type: 'input', dataKey: 'host', label: 'Host' }]

    await state.choseProtocolType('http-service')

    expect(state.protocol_config).toEqual({})
    expect(hoisted.deviceConfigVoucherType).toHaveBeenLastCalledWith({ device_type: '1', protocol_type: 'http-service' })
    expect(hoisted.protocolPluginConfigForm).toHaveBeenLastCalledWith({ device_type: '1', protocol_type: 'http-service' })
  })
})
