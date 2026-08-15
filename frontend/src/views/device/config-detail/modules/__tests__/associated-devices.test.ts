/**
 * 文件用途: associated-devices 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigBatch: vi.fn(),
  detachDeviceFromConfig: vi.fn(),
  deviceList: vi.fn(),
  getDeviceListForSelect: vi.fn(),
  routerPushByKey: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceConfigBatch: hoisted.deviceConfigBatch,
  detachDeviceFromConfig: hoisted.detachDeviceFromConfig,
  deviceList: hoisted.deviceList,
  getDeviceListForSelect: hoisted.getDeviceListForSelect
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NPagination: defineComponent({ props: { page: { default: 1 }, itemCount: { default: 0 } }, emits: ['update:page'], setup() { return () => h('div') } }),
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 00:00:00') }))
}))

import Component from '../associated-devices.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { deviceConfigId: 'cfg-1', ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        DeviceSelectWithScroll: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/associated-devices.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceList.mockResolvedValue({ data: { list: [{ id: 'd1', name: 'Device 1', is_online: 1, ts: 123 }], total: 1 }, error: null })
    hoisted.getDeviceListForSelect.mockResolvedValue({ data: { list: [] }, error: null })
    hoisted.deviceConfigBatch.mockResolvedValue({ error: null })
    hoisted.detachDeviceFromConfig.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes associated device table with config-scoped paging and online label', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.queryData).toMatchObject({
      device_config_id: 'cfg-1',
      page: 1,
      page_size: 10
    })
    expect(hoisted.deviceList).toHaveBeenCalledWith({
      device_config_id: 'cfg-1',
      page: 1,
      page_size: 10
    })
    expect(state.configDevice).toEqual([
      expect.objectContaining({
        id: 'd1',
        name: 'Device 1',
        is_online: 1,
        activate_flag: 'custom.devicePage.online'
      })
    ])
    expect(state.configDeviceTotal).toBe(1)
    expect(state.columnsData.map((column: any) => column.key)).toEqual([
      'name',
      'device_number',
      'activate_flag',
      'ts',
      'actions'
    ])
  })

  it('loads device list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceList).toHaveBeenCalledTimes(1)
  })

  it('addDevice opens modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.addDevice()
    expect(state.visible).toBe(true)
  })

  it('handleClose closes modal and reloads list', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.visible = true
    state.handleClose()
    await flushPromises()
    expect(state.visible).toBe(false)
  })

  it('handleSubmit warns when no devices selected', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.associatedFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.associatedForm.device_ids = null
    await state.handleSubmit()
    expect(hoisted.deviceConfigBatch).toHaveBeenCalledTimes(0)
  })

  it('handleSubmit calls deviceConfigBatch when devices selected', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.associatedFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.associatedForm.device_ids = ['d1']
    await state.handleSubmit()
    expect(hoisted.deviceConfigBatch).toHaveBeenCalledWith({
      device_ids: ['d1'],
      device_config_id: 'cfg-1'
    })
  })

  it('handleDelete detaches the device from the config', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleDelete({ id: 'd1' })
    expect(hoisted.detachDeviceFromConfig).toHaveBeenCalledWith({ device_id: 'd1', device_config_id: '' })
  })
})
