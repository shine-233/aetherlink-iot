/**
 * 文件用途: 覆盖AutomaticModeStep在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfig: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfig: hoisted.deviceConfig
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { service_identifier: 'si1' } })
}))

vi.mock('naive-ui', () => ({
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] }, pagination: { type: Object, default: () => ({}) } }, setup() { return () => h('div') } }),
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() })
}))

import Component from '../AutomaticModeStep.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {}
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/service-details/components/AutomaticModeStep.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfig.mockResolvedValue({ data: { list: [], total: 0 } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads service-scoped device configs and masks service secrets', async () => {
    hoisted.deviceConfig.mockResolvedValue({
      data: {
        list: [{ id: 'tpl-1', name: 'Config 1', template_secret: 'secret' }],
        total: 1
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceConfig).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      protocol_type: 'si1'
    })
    expect(state.pageData.loading).toBe(false)
    expect(state.pageData.tableData).toEqual([{ id: 'tpl-1', name: 'Config 1', template_secret: 'secret' }])
    expect(state.pagination.itemCount).toBe(1)
    expect(state.columns[2].render({ template_secret: 'secret' })).toBe('******')
    expect(state.columns[2].render({})).toBe('card.serviceSecretNotConfigured')
  })

  it('has columns defined', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.columns.map((column: any) => column.key)).toEqual(['name', 'id', 'template_secret'])
  })

  it('pagination has correct default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.pagination.page).toBe(1)
    expect(state.pagination.pageSize).toBe(10)
  })
})
