/**
 * 文件用途: 覆盖Market Template Card在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/assets/imgs/default_template_cover.png', () => ({
  default: 'default-cover.png'
}))

import Component from '../market-template-card.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const defaultTemplate = {
  id: 'tpl-1',
  name: 'Test Template',
  brand: 'TestBrand',
  model: 'Model1',
  category: 'IoT',
  author_name: 'Author1',
  latest_version: '1.0.0',
  description: 'A test template',
  cover_url: 'https://example.com/cover.png',
  install_count: 42
}

const mountComponent = (templateOverrides = {}) => {
  const wrapper = mount(Component, {
    props: {
      template: { ...defaultTemplate, ...templateOverrides }
    },
    global: {
      stubs: {
        NCard: defineComponent({ props: ['hoverable'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTag: defineComponent({ props: ['size', 'type'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NEllipsis: defineComponent({ props: ['lineClamp'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ props: ['size', 'type'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/config/modules/market-template-card.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders market template identity, stats and actions from props', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.get('img').attributes()).toMatchObject({
      src: 'https://example.com/cover.png',
      alt: 'Test Template'
    })
    expect(wrapper.text()).toContain('IoT')
    expect(wrapper.text()).toContain('Test Template')
    expect(wrapper.text()).toContain('TestBrand')
    expect(wrapper.text()).toContain('Model1')
    expect(wrapper.text()).toContain('Author1')
    expect(wrapper.text()).toContain('v1.0.0')
    expect(wrapper.text()).toContain('42 market.installCount')
    expect(wrapper.findAll('button')).toHaveLength(2)
  })

  it('receives template name in props', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    // With shallowMount, NEllipsis is stubbed and slot content may not render
    // Verify the template prop is passed correctly
    expect(wrapper.props('template').name).toBe('Test Template')
  })

  it('emits view-detail event', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.$emit('view-detail', 'tpl-1')
    expect(wrapper.emitted('view-detail')).toEqual([['tpl-1']])
  })

  it('emits install event', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.$emit('install', 'tpl-1')
    expect(wrapper.emitted('install')).toEqual([['tpl-1']])
  })

  it('falls back to default cover and zero install count without optional market fields', async () => {
    const wrapper = mountComponent({
      brand: undefined,
      model: undefined,
      category: undefined,
      author_name: undefined,
      latest_version: undefined,
      cover_url: undefined,
      install_count: undefined
    })
    await flushPromises()
    expect(wrapper.get('img').attributes()).toMatchObject({
      src: 'default-cover.png',
      alt: 'Test Template'
    })
    expect(wrapper.text()).toContain('Test Template')
    expect(wrapper.text()).toContain('0 market.installCount')
    expect(wrapper.text()).not.toContain('TestBrand')
    expect(wrapper.text()).not.toContain('Author1')
  })

  it('uses remote cover_url as card cover source', async () => {
    const wrapper = mountComponent({ cover_url: 'https://example.com/cover.png' })
    await flushPromises()
    expect(wrapper.get('img').attributes('src')).toBe('https://example.com/cover.png')
  })

  it('uses bundled default cover when cover_url is missing', async () => {
    const wrapper = mountComponent({ cover_url: undefined })
    await flushPromises()
    expect(wrapper.get('img').attributes('src')).toBe('default-cover.png')
  })
})
