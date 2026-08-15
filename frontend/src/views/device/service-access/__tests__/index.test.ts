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
  getServiceList: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  getServiceList: hoisted.getServiceList
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: hoisted.routerPush })
}))

vi.mock('@vicons/ionicons5', () => ({
  GridOutline: defineComponent({ setup: () => () => h('div') })
}))

vi.mock('naive-ui', () => ({
  NSpin: defineComponent({ props: { show: Boolean }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NGi: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('@/components/dev-card-item/index.vue', () => ({
  default: defineComponent({ emits: ['click-card'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('@/components/list-page/index.vue', () => ({
  default: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        AdvancedListLayout: true,
        DevCardItem: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/service-access/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getServiceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads service access cards with default service type and pagination contract', async () => {
    hoisted.getServiceList.mockResolvedValue({
      data: {
        list: [{ id: 'svc-1', name: 'MQTT Service', service_type: 2, service_identifier: 'mqtt' }],
        total: 24
      },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.getServiceList).toHaveBeenCalledWith({
      page: 1,
      page_size: 15,
      service_type: 2
    })
    expect(state.loading).toBe(false)
    expect(state.deviceTemplateList).toEqual([
      { id: 'svc-1', name: 'MQTT Service', service_type: 2, service_identifier: 'mqtt' }
    ])
    expect(state.pagination).toMatchObject({ page: 1, pageSize: 15, pageCount: 2 })
  })

  it('loads data on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getServiceList).toHaveBeenCalledTimes(1)
  })

  it('clickDevice navigates to service-details', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.clickDevice({ id: 'svc-1', service_type: 2, name: 'Service 1', service_identifier: 'si1' })
    expect(hoisted.routerPush).toHaveBeenCalledWith('/device/service-details?id=svc-1&service_type=2&service_name=Service 1&service_identifier=si1')
  })

  it('handleRefresh reloads data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleRefresh()
    await flushPromises()
    expect(hoisted.getServiceList).toHaveBeenCalledTimes(2)
  })
})
