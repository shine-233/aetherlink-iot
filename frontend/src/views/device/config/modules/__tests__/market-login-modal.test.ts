/**
 * 文件用途: 覆盖Market Login Modal在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  marketLogin: vi.fn(),
  setToken: vi.fn()
}))

vi.mock('@/service/api/market', () => ({
  marketLogin: hoisted.marketLogin
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../../composables/use-market-auth', () => ({
  useMarketAuth: () => ({ setToken: hoisted.setToken })
}))

import Component from '../market-login-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({ props: ['show'], emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'type', 'placeholder'], emits: ['update:value'], setup() { return () => h('input') } }),
        NButton: defineComponent({ props: ['type', 'loading', 'text'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config/modules/market-login-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.marketLogin.mockResolvedValue({ token: 'test-token' })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes hidden login dialog with empty credentials and required rules', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.visible).toBe(false)
    expect(state.loading).toBe(false)
    expect(state.loginForm).toEqual({
      username: '',
      password: ''
    })
    expect(state.loginRules).toMatchObject({
      username: {
        required: true,
        trigger: 'blur'
      },
      password: {
        required: true,
        message: 'market.password',
        trigger: 'blur'
      }
    })
  })

  it('initializes with visible false', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.visible).toBe(false)
  })

  it('open sets visible to true and resets form', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.loginForm.username = 'old'
    state.loginForm.password = 'old'
    state.open()
    expect(state.visible).toBe(true)
    expect(state.loginForm.username).toBe('')
    expect(state.loginForm.password).toBe('')
  })

  it('handleLogin calls marketLogin and sets token on success', async () => {
    hoisted.marketLogin.mockResolvedValue({ token: 'test-token' })
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    state.loginForm.username = 'user'
    state.loginForm.password = 'pass'
    await state.handleLogin()
    expect(hoisted.marketLogin).toHaveBeenCalledWith({ username: 'user', password: 'pass' })
    expect(hoisted.setToken).toHaveBeenCalledWith('test-token')
  })

  it('handleLogin emits login-success on success', async () => {
    hoisted.marketLogin.mockResolvedValue({ token: 'test-token' })
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    await state.handleLogin()
    expect(wrapper.emitted('login-success')).toEqual([['test-token']])
  })

  it('handleLogin sets loading to false in finally block', async () => {
    hoisted.marketLogin.mockRejectedValue(new Error('fail'))
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    await state.handleLogin()
    expect(state.loading).toBe(false)
  })

  it('handleGoToRegister opens new window', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleGoToRegister()
    expect(openSpy).toHaveBeenCalledTimes(1)
    openSpy.mockRestore()
  })

  it('exposes open method', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(typeof wrapper.vm.open).toBe('function')
  })
})
