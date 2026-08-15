/**
 * 文件用途: 覆盖Market Template Drawer在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getMarketTemplateDetail: vi.fn()
}))

vi.mock('@/service/api/market', () => ({
  getMarketTemplateDetail: hoisted.getMarketTemplateDetail
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/assets/imgs/default_template_cover.png', () => ({
  default: 'default-cover.png'
}))

import Component from '../market-template-drawer.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      visible: true,
      templateId: 'tpl-1',
      ...props
    },
    global: {
      stubs: {
        NDrawer: defineComponent({ props: ['show', 'width'], emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDrawerContent: defineComponent({ props: ['title', 'closable'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpin: defineComponent({ props: ['show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDescriptions: defineComponent({ props: ['column', 'labelPlacement', 'bordered', 'size'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDescriptionsItem: defineComponent({ props: ['label'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTag: defineComponent({ props: ['size', 'type'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ props: ['type', 'block'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config/modules/market-template-drawer.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getMarketTemplateDetail.mockResolvedValue({ error: null, data: { id: 'tpl-1', name: 'Test Template' } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads selected market template detail into the visible drawer contract', async () => {
    hoisted.getMarketTemplateDetail.mockResolvedValue({
      error: null,
      data: {
        id: 'tpl-1',
        name: 'Test Template',
        brand: 'Brand1',
        model: 'Model1',
        latest_version: '1.0.0',
        install_count: 7,
        tags: ['mqtt']
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.getMarketTemplateDetail).toHaveBeenCalledWith('tpl-1')
    expect(state.loading).toBe(false)
    expect(state.detail).toMatchObject({
      id: 'tpl-1',
      name: 'Test Template',
      brand: 'Brand1',
      model: 'Model1',
      latest_version: '1.0.0',
      install_count: 7,
      tags: ['mqtt']
    })
    expect(wrapper.text()).toContain('Test Template')
  })

  it('fetches detail when templateId and visible are set', async () => {
    mountComponent({ visible: true, templateId: 'tpl-1' })
    await flushPromises()
    expect(hoisted.getMarketTemplateDetail).toHaveBeenCalledWith('tpl-1')
  })

  it('does not fetch detail when templateId is empty', async () => {
    vi.clearAllMocks()
    mountComponent({ visible: true, templateId: '' })
    await flushPromises()
    expect(hoisted.getMarketTemplateDetail).toHaveBeenCalledTimes(0)
  })

  it('sets detail on successful fetch', async () => {
    hoisted.getMarketTemplateDetail.mockResolvedValue({
      error: null,
      data: { id: 'tpl-1', name: 'Test Template', brand: 'Brand1' }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.detail).toEqual({ id: 'tpl-1', name: 'Test Template', brand: 'Brand1' })
  })

  it('sets loading to false after fetch', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(false)
  })

  it('handleClose emits update:visible false', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleClose()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('emits install when install button clicked', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.$emit('install', 'tpl-1')
    expect(wrapper.emitted('install')).toEqual([['tpl-1']])
  })
})
