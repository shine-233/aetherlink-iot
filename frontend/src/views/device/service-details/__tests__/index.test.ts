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
  delServiceAccess: vi.fn(),
  getServiceAccess: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/service/api/plugin', () => ({
  delServiceAccess: hoisted.delServiceAccess,
  getServiceAccess: hoisted.getServiceAccess
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { id: 'svc-1', service_identifier: 'si1' } }),
  useRouter: () => ({ push: hoisted.routerPush })
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 00:00:00') }))
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('../components/serviceModal.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('../components/serviceConfigModal.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
        NPagination: true,
        serviceModal: true,
        serviceConfigModal: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/service-details/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getServiceAccess.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.delServiceAccess.mockResolvedValue({})
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('starts with service access query and table action column contracts', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.queryInfo).toMatchObject({
      service_plugin_id: 'svc-1',
      page: 1,
      page_size: 10,
      pageSizes: [10, 15, 20, 25, 30]
    })
    expect(state.columns.map((column: { key: string }) => column.key)).toEqual(['name', 'create_at', 'actions'])
    expect(state.columns[0].title).toBe('card.accessPointName')
    expect(state.columns[1].title).toBe('common.creationTime')
    expect(state.columns[2].width).toBe('420px')
  })

  it('loads list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getServiceAccess).toHaveBeenCalledTimes(1)
  })

  it('stores fetched access points and total count', async () => {
    hoisted.getServiceAccess.mockResolvedValue({
      data: {
        list: [{ id: 'acc-1', name: 'Access 1' }],
        total: 1
      }
    })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.pageData.tableData).toEqual([{ id: 'acc-1', name: 'Access 1' }])
    expect(state.queryInfo.itemCount).toBe(1)
  })

  it('see navigates to device manage', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.see({ id: 'acc-1', name: 'Access 1' })
    expect(hoisted.routerPush).toHaveBeenCalledWith(
      '/device/manage?service_identifier=si1&device_name=Access 1&service_access_id=acc-1'
    )
  })

  it('del calls delServiceAccess and refreshes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.del({ id: 'acc-1' })
    expect(hoisted.delServiceAccess).toHaveBeenCalledWith({ id: 'acc-1' })
  })

  it('page changes and page-size changes reload the access list', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getSetupState(wrapper)
    const seenQueries: any[] = []
    hoisted.getServiceAccess.mockImplementation(async query => {
      seenQueries.push({ ...query })
      return { data: { list: [], total: 0 } }
    })

    state.queryInfo.onChange(3)
    state.queryInfo.onUpdatePageSize(20)
    await flushPromises()

    expect(hoisted.getServiceAccess).toHaveBeenCalledTimes(2)
    expect(seenQueries).toEqual([
      expect.objectContaining({ page: 3, page_size: 10 }),
      expect.objectContaining({ page: 1, page_size: 20 })
    ])
  })

  it('opens the access point modal for creating and editing config', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const openModal = vi.fn()
    state.serviceModalRef = { openModal }

    state.addData()
    state.config({ id: 'acc-1', name: 'Access 1' })

    expect(openModal).toHaveBeenNthCalledWith(1, 'svc-1')
    expect(openModal).toHaveBeenNthCalledWith(2, 'svc-1', { id: 'acc-1', name: 'Access 1' })
  })

  it('opens device config directly after manual access point save', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getSetupState(wrapper)
    const openModal = vi.fn()
    state.serviceConfigModalRef = { openModal }

    state.isEdit('voucher-json', 'acc-1', false)
    await flushPromises()

    expect(openModal).toHaveBeenCalledWith('voucher-json', 'acc-1')
    expect(hoisted.getServiceAccess).toHaveBeenCalledTimes(1)
  })

  it('adapts automatic auth rows before opening device config in edit flow', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getSetupState(wrapper)
    const openModal = vi.fn()
    state.serviceConfigModalRef = { openModal }

    state.isEdit('voucher-json', { id: 'acc-1', auth_type: 'auto', name: 'Access 1' }, true)
    await flushPromises()

    expect(openModal).toHaveBeenCalledWith(
      'voucher-json',
      { id: 'acc-1', auth_type: 'auto', name: 'Access 1', mode: 'automatic' },
      true
    )
    expect(hoisted.getServiceAccess).toHaveBeenCalledTimes(1)
  })
})
