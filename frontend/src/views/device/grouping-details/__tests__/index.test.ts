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
  deleteDeviceGroup: vi.fn(),
  deleteDeviceGroupRelation: vi.fn(),
  deviceGroupDetail: vi.fn(),
  deviceListByGroup: vi.fn(),
  getDeviceGroup: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deleteDeviceGroup: hoisted.deleteDeviceGroup,
  deleteDeviceGroupRelation: hoisted.deleteDeviceGroupRelation,
  deviceGroupDetail: hoisted.deviceGroupDetail,
  deviceListByGroup: hoisted.deviceListByGroup,
  getDeviceGroup: hoisted.getDeviceGroup
}))

vi.mock('@/views/device/modules/all-columns', () => ({
  createNoSelectDeviceColumns: vi.fn(() => []),
  group_columns: vi.fn(() => [])
}))

vi.mock('@/hooks/common/use-loading-empty', () => {
  // Vitest hoists this mock before ESM imports, so use a local require here.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { ref } = require('vue')
  return {
  default: (init = false) => {
    const loading = ref(init)
    return { loading, startLoading: vi.fn(() => { loading.value = true }), endLoading: vi.fn(() => { loading.value = false }) }
  }
  }
})

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPush: hoisted.routerPush })
}))

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: vi.fn(() => '2024-01-01 00:00:00')
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { id: 'grp-1' } }),
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } })
}))

vi.mock('@/views/device/grouping/components', () => ({
  AddOrEditDevices: defineComponent({ setup() { return () => h('div') } })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const SlotStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default ? slots.default() : [])
  }
})

const CardStub = defineComponent({
  setup(_, { slots }) {
    return () =>
      h('section', [
        slots['header-extra'] ? h('div', slots['header-extra']()) : null,
        slots.default ? h('div', slots.default()) : null
      ])
  }
})

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        AddOrEditDevices: true,
        DeviceSelectList: true,
        NPagination: true,
        NCard: CardStub,
        NSpace: SlotStub,
        NTabs: SlotStub,
        NTabPane: SlotStub,
        NFlex: SlotStub,
        NDescriptions: SlotStub,
        NDescriptionsItem: SlotStub,
        NModal: SlotStub,
        'svg-icon': true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/grouping-details/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceGroupDetail.mockResolvedValue({
      data: {
        detail: {
          id: 'grp-1',
          name: 'Group 1',
          description: '',
          parent_id: '',
          created_at: '',
          updated_at: '',
          remark: '',
          tenant_id: '',
          tier: 0
        },
        tier: { group_path: '' },
        statistics: {
          device_total: 0,
          online_total: 0,
          offline_total: 0,
          alarm_total: 0
        }
      },
      error: null
    })
    hoisted.getDeviceGroup.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceListByGroup.mockResolvedValue({ data: { list: [], total: 0 } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads group detail and child groups on mount without loading bound devices', async () => {
    hoisted.deviceGroupDetail.mockResolvedValue({
      data: {
        detail: {
          id: 'grp-1',
          name: 'Group 1',
          description: 'Main group',
          parent_id: '0',
          created_at: '2024-01-01',
          updated_at: '',
          remark: '',
          tenant_id: '',
          tier: 1
        },
        tier: { group_path: 'Root / Group 1' },
        statistics: {
          device_total: 8,
          online_total: 5,
          offline_total: 3,
          alarm_total: 2
        }
      },
      error: null
    })
    hoisted.getDeviceGroup.mockResolvedValue({
      data: { list: [{ id: 'child-1', name: 'Child 1' }], total: 1 }
    })
    hoisted.deviceListByGroup.mockResolvedValue({
      data: { list: [{ id: 'dev-1', name: 'Device 1' }], total: 6 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceGroupDetail).toHaveBeenCalledWith({ id: 'grp-1' })
    expect(hoisted.getDeviceGroup).toHaveBeenCalledWith({ parent_id: 'grp-1', page: 1, page_size: 10 })
    expect(hoisted.deviceListByGroup).not.toHaveBeenCalled()
    expect(state.details_data.detail.name).toBe('Group 1')
    expect(state.details_data.tier.group_path).toBe('Root / Group 1')
    expect(state.details_data.statistics).toEqual({
      device_total: 8,
      online_total: 5,
      offline_total: 3,
      alarm_total: 2
    })
    expect(wrapper.text()).toContain('custom.grouping_details.totalDevices')
    expect(wrapper.text()).toContain('custom.grouping_details.onlineDevices')
    expect(wrapper.text()).toContain('custom.grouping_details.offlineDevices')
    expect(wrapper.text()).toContain('custom.grouping_details.alarmDevices')
    expect(wrapper.text()).toContain('8')
    expect(wrapper.text()).toContain('5')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('2')
    expect(state.editData).toEqual({ id: 'grp-1', parent_id: '0', name: 'Group 1', description: 'Main group' })
    expect(state.group_data).toEqual([{ id: 'child-1', name: 'Child 1' }])
    expect(state.group_pagination.itemCount).toBe(1)
    expect(state.device_data).toEqual([])
  })

  it('loads bound devices only after the device tab is opened', async () => {
    hoisted.deviceListByGroup.mockResolvedValue({
      data: { list: [{ id: 'dev-1', name: 'Device 1' }], total: 6 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceListByGroup).not.toHaveBeenCalled()
    await state.handleGroupDetailTabUpdate('device')

    expect(hoisted.deviceListByGroup).toHaveBeenCalledWith({ group_id: 'grp-1', page: 1, page_size: 5 })
    expect(state.device_data).toEqual([{ id: 'dev-1', name: 'Device 1' }])
    expect(state.devicePagination.pageCount).toBe(2)
  })

  it('loads details on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceGroupDetail).toHaveBeenCalledTimes(1)
  })

  it('getDetails fetches group detail', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.getDetails('grp-1')
    expect(hoisted.deviceGroupDetail).toHaveBeenCalledWith({ id: 'grp-1' })
  })
})
