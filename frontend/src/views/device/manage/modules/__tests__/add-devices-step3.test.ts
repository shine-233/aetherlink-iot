/**
 * 文件用途: 覆盖Add Devices Step3在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    routerPushByKey: vi.fn().mockResolvedValue(undefined)
  })
}))

import Component from '../add-devices-step3.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      isSuccess: true,
      closeCallback: vi.fn(),
      backCallback: vi.fn(),
      ...props
    },
    global: {
      stubs: {
        NResult: defineComponent({
          name: 'NResult',
          props: ['status', 'title', 'description'],
          setup(props, { slots }) {
            return () => h('div', { 'data-status': props.status }, [slots.default?.(), slots.footer?.()])
          }
        }),
        'n-button': defineComponent({
          name: 'NButton',
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/manage/modules/add-devices-step3.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders success result contract and close action when device creation succeeds', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const result = wrapper.getComponent({ name: 'NResult' })
    expect(result.props()).toMatchObject({
      status: 'success',
      title: 'custom.devicePage.success',
      description: 'custom.devicePage.deviceConfigSuccess'
    })
    await wrapper.get('button').trigger('click')
    expect(wrapper.props('closeCallback')).toHaveBeenCalledTimes(1)
  })

  it('accepts isSuccess prop', async () => {
    const wrapper = mountComponent({ isSuccess: true })
    await flushPromises()
    expect(wrapper.props('isSuccess')).toBe(true)
  })

  it('accepts closeCallback prop', async () => {
    const closeCallback = vi.fn()
    const wrapper = mountComponent({ closeCallback })
    await flushPromises()
    expect(wrapper.props('closeCallback')).toBe(closeCallback)
  })

  it('accepts backCallback prop', async () => {
    const backCallback = vi.fn()
    const wrapper = mountComponent({ backCallback })
    await flushPromises()
    expect(wrapper.props('backCallback')).toBe(backCallback)
  })

  it('renders success result when isSuccess is true', async () => {
    const wrapper = mountComponent({ isSuccess: true })
    await flushPromises()
    const result = wrapper.getComponent({ name: 'NResult' })
    expect(result.props('status')).toBe('success')
  })

  it('renders error result when isSuccess is false', async () => {
    const wrapper = mountComponent({ isSuccess: false })
    await flushPromises()
    const result = wrapper.getComponent({ name: 'NResult' })
    expect(result.props()).toMatchObject({
      status: 'error',
      title: 'custom.devicePage.fail',
      description: 'custom.devicePage.deviceConfigFail'
    })
  })
})
