/**
 * 文件用途: 覆盖Detail Modal在系统管理用户侧场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('dayjs', () => ({
  default: () => ({ format: () => '2024-01-01 00:00:00' })
}))

import DetailModal from '../detail-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(DetailModal, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['model', 'labelPlacement', 'labelAlign', 'labelWidth'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('DetailModal', () => {
  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount with default state', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    expect(state.modalVisible).toBe(false)
    expect(state.detailInfo).toEqual({
      id: '',
      email: '',
      username: '',
      ip: '',
      request_message: '',
      response_message: '',
      latency: '',
      name: '',
      path: '',
      created_at: ''
    })
  })

  it('should show modal with info', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    const info = { id: '1', email: 'test@test.com', username: 'admin', ip: '127.0.0.1', request_message: 'req', response_message: 'res', latency: '100', name: 'POST', path: '/api/test', created_at: '2024-01-01' }
    state.show(info)
    expect(state.modalVisible).toBe(true)
    expect(state.detailInfo).toEqual(info)
  })

  it('should close modal', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    state.modalVisible = true
    state.closeModal()
    expect(state.modalVisible).toBe(false)
  })

  it('should expose show method', () => {
    const wrapper = mountComponent()
    expect(typeof wrapper.vm.show).toBe('function')
  })
})
