/**
 * 文件用途: 覆盖ServiceConfigModal在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  batchAddServiceMenuList: vi.fn(),
  getSelectServiceMenuList: vi.fn(),
  getServiceListDrop: vi.fn(),
  deviceConfigMenu: vi.fn()
}))

vi.mock('@/service/api/plugin', () => ({
  batchAddServiceMenuList: hoisted.batchAddServiceMenuList,
  getSelectServiceMenuList: hoisted.getSelectServiceMenuList,
  getServiceListDrop: hoisted.getServiceListDrop
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigMenu: hoisted.deviceConfigMenu
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { service_identifier: 'si1', service_type: '2' } })
}))

vi.mock('naive-ui', () => ({
  NAlert: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } })
}))

import Component from '../serviceConfigModal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
        NButton: true,
        NSpace: true,
        NPagination: true,
        NPopconfirm: true,
        FormInput: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/service-details/components/serviceConfigModal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getSelectServiceMenuList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.getServiceListDrop.mockResolvedValue({ data: [] })
    hoisted.deviceConfigMenu.mockResolvedValue({ data: [] })
    hoisted.batchAddServiceMenuList.mockResolvedValue({ error: null })
    Object.defineProperty(window, '$message', {
      configurable: true,
      value: {
        success: vi.fn(),
        error: vi.fn()
      }
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('starts with third-party device pagination and selection safety contracts', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.serviceModal).toBe(false)
    expect(state.queryInfo).toMatchObject({
      voucher: '',
      service_type: '2',
      page: 1,
      pageSize: 10,
      itemCount: 0,
      showSizePicker: true
    })
    expect(state.queryInfo.prefix({ itemCount: 7 })).toBe('common.total: 7')
    expect(state.columns.map((column: { key?: string; type?: string }) => column.key || column.type)).toEqual([
      'selection',
      'device_name',
      'device_number',
      'create_at'
    ])
    expect(state.columns[0].disabled({ is_bind: true })).toBe(true)
    expect(state.columns[0].disabled({ is_bind: false })).toBe(false)
  })

  it('normalizeTemplateOptions filters invalid options', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const result = state.normalizeTemplateOptions([{ id: '1', name: 'T1' }, { id: '2' }, null])
    expect(result).toHaveLength(1)
    expect(result[0].id).toBe('1')
  })

  it('modalTitle returns default when no access point', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.modalTitle).toBe('custom.serviceAccess.configAccessPointDevices')
  })

  it('openModal loads third-party devices with protocol scoped template options', async () => {
    hoisted.getServiceListDrop.mockResolvedValue({
      data: {
        list: [
          {
            device_number: 'dn-1',
            device_name: '',
            protocol_config: '{"a":1}',
            additional_info: '{"b":2}'
          }
        ],
        total: 1
      }
    })
    hoisted.getSelectServiceMenuList.mockResolvedValue({
      data: [{ id: 'tpl-1', name: 'Access Config 1' }]
    })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('voucher-json', { id: 'acc-1', name: 'Access 1' }, false)
    await flushPromises()

    expect(hoisted.getServiceListDrop).toHaveBeenCalledWith({
      voucher: 'voucher-json',
      service_type: '2',
      page: 1,
      page_size: 10
    })
    expect(hoisted.getSelectServiceMenuList).toHaveBeenCalledWith({
      device_type: '',
      device_config_name: '',
      protocol_type: 'si1'
    })
    expect(state.serviceModal).toBe(true)
    expect(state.device_config_id).toBe('acc-1')
    expect(state.pageData.tableData[0]).toMatchObject({
      device_number: 'dn-1',
      device_name: 'dn-1',
      options: [{ id: 'tpl-1', name: 'Access Config 1' }]
    })
    expect(state.queryInfo.itemCount).toBe(1)
  })

  it('falls back to deviceConfigMenu when protocol scoped templates are empty', async () => {
    hoisted.getServiceListDrop.mockResolvedValue({ data: { list: [{ device_number: 'dn-1' }], total: 1 } })
    hoisted.getSelectServiceMenuList.mockResolvedValue({ data: [] })
    hoisted.deviceConfigMenu.mockResolvedValue({ data: [{ id: 'fallback-1', name: 'Fallback' }] })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('voucher-json', { id: 'acc-1', name: 'Access 1' }, false)
    await flushPromises()

    expect(hoisted.deviceConfigMenu).toHaveBeenCalledWith({ name: '' })
    expect(state.pageData.tableData[0].options).toEqual([{ id: 'fallback-1', name: 'Fallback' }])
  })

  it('keeps bound devices checked and cached when list is loaded', async () => {
    hoisted.getServiceListDrop.mockResolvedValue({
      data: {
        list: [
          { device_number: 'bound-1', device_name: 'Bound', is_bind: true, device_config_id: 'tpl-1' },
          { device_number: 'free-1', device_name: 'Free', is_bind: false }
        ],
        total: 2
      }
    })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('voucher-json', { id: 'acc-1', name: 'Access 1' }, false)
    await flushPromises()

    expect(state.checkedRowKeys).toContain('bound-1')
    expect(state.boundDeviceKeys.has('bound-1')).toBe(true)
    expect(state.selectedDeviceDrafts.get('bound-1')).toMatchObject({ device_name: 'Bound' })
  })

  it('handleCheck caches selected rows and removes unselected free rows', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.pageData.tableData = [
      { device_number: 'bound-1', device_name: 'Bound', is_bind: true },
      { device_number: 'free-1', device_name: 'Free', is_bind: false },
      { device_number: 'free-2', device_name: 'Other', is_bind: false }
    ]
    state.selectedDeviceDrafts.set('free-2', { device_number: 'free-2', device_name: 'Other' })

    state.handleCheck(['free-1'])

    expect(state.checkedRowKeys).toEqual(expect.arrayContaining(['bound-1', 'free-1']))
    expect(state.selectedDeviceDrafts.has('free-1')).toBe(true)
    expect(state.selectedDeviceDrafts.has('free-2')).toBe(false)
  })

  it('shows error when saving without service access id', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.device_config_id = ''

    await state.submitSevice()

    expect((window as any).$message.error).toHaveBeenCalledWith('card.serviceAccessIdNotSet')
    expect(hoisted.batchAddServiceMenuList).toHaveBeenCalledTimes(0)
  })

  it('closes and refreshes when no new unbound devices are selected', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.serviceModal = true
    state.device_config_id = 'acc-1'
    state.boundDeviceKeys.add('bound-1')
    state.checkedRowKeys = ['bound-1']

    await state.submitSevice()

    expect((window as any).$message.success).toHaveBeenCalledWith('custom.serviceAccess.configSaved')
    expect(hoisted.batchAddServiceMenuList).toHaveBeenCalledTimes(0)
    expect(state.serviceModal).toBe(false)
    expect(wrapper.emitted('getList')).toHaveLength(1)
  })

  it('submits selected current-page devices with parsed protocol and additional info', async () => {
    hoisted.batchAddServiceMenuList.mockResolvedValue({ data: true })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.serviceModal = true
    state.device_config_id = 'acc-1'
    state.checkedRowKeys = ['dn-1']
    state.pageData.tableData = [
      {
        device_number: 'dn-1',
        device_name: 'Device 1',
        description: 'Desc',
        device_config_id: 'tpl-1',
        protocol_config: '{"host":"127.0.0.1"}',
        additional_info: '{"site":"A"}'
      }
    ]

    await state.submitSevice()

    expect(hoisted.batchAddServiceMenuList).toHaveBeenCalledWith({
      service_access_id: 'acc-1',
      device_list: [
        {
          device_number: 'dn-1',
          device_name: 'Device 1',
          description: 'Desc',
          device_config_id: 'tpl-1',
          protocol_config: { host: '127.0.0.1' },
          additional_info: { site: 'A' }
        }
      ]
    })
    expect((window as any).$message.success).toHaveBeenCalledWith('common.operationSuccess')
    expect(wrapper.emitted('getList')).toHaveLength(1)
  })

  it('submits cross-page selected devices with only device number when no draft exists', async () => {
    hoisted.batchAddServiceMenuList.mockResolvedValue({ data: true })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.device_config_id = 'acc-1'
    state.checkedRowKeys = ['off-page-1']
    state.pageData.tableData = []

    await state.submitSevice()

    expect(hoisted.batchAddServiceMenuList).toHaveBeenCalledWith({
      service_access_id: 'acc-1',
      device_list: [{ device_number: 'off-page-1' }]
    })
  })

  it('shows backend message when batch binding returns a message without data', async () => {
    hoisted.batchAddServiceMenuList.mockResolvedValue({ data: null, message: 'bind failed' })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.device_config_id = 'acc-1'
    state.checkedRowKeys = ['dn-1']

    await state.submitSevice()

    expect((window as any).$message.error).toHaveBeenCalledWith('bind failed')
  })

  it('shows fetch error and clears table when device loading fails', async () => {
    hoisted.getServiceListDrop.mockRejectedValue({ response: { data: { message: 'fetch failed' } } })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('voucher-json', { id: 'acc-1', name: 'Access 1' }, false)
    await flushPromises()

    expect(state.pageData.tableData).toEqual([])
    expect(state.queryInfo.itemCount).toBe(0)
    expect((window as any).$message.error).toHaveBeenCalledWith('fetch failed')
    expect(state.pageData.loading).toBe(false)
  })

  it('goes back to access point config with the original voucher context', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.serviceModal = true
    state.accessPointContext = {
      voucher: 'voucher-json',
      row: { id: 'acc-1', name: 'Access 1' },
      edit: true
    }

    state.backToAccessPointConfig()

    expect(state.serviceModal).toBe(false)
    expect(wrapper.emitted('go-back')?.[0]).toEqual([{ id: 'acc-1', name: 'Access 1', voucher: 'voucher-json' }])
  })

  it('close resets selected device state and loaded rows', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.serviceModal = true
    state.isEdit = true
    state.checkedRowKeys = ['dn-1']
    state.selectedDeviceDrafts.set('dn-1', { device_number: 'dn-1' })
    state.boundDeviceKeys.add('bound-1')
    state.pageData.tableData = [{ device_number: 'dn-1' }]
    state.accessPointContext = { voucher: 'voucher-json', row: { id: 'acc-1' }, edit: true }

    state.close()

    expect(state.serviceModal).toBe(false)
    expect(state.isEdit).toBe(false)
    expect(state.checkedRowKeys).toEqual([])
    expect(state.selectedDeviceDrafts.size).toBe(0)
    expect(state.boundDeviceKeys.size).toBe(0)
    expect(state.pageData.tableData).toEqual([])
    expect(state.accessPointContext).toBeNull()
  })
})
