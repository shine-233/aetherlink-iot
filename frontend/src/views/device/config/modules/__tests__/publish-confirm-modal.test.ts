/**
 * 文件用途: 覆盖Publish Confirm Modal在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  publishToMarket: vi.fn(),
  deviceConfigInfo: vi.fn()
}))

vi.mock('@/service/api/market', () => ({
  publishToMarket: hoisted.publishToMarket
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigInfo: hoisted.deviceConfigInfo
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import Component from '../publish-confirm-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({ props: ['show'], emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'type', 'placeholder', 'maxlength', 'clearable'], emits: ['update:value'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options', 'clearable', 'placeholder'], emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ props: ['type', 'loading'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NAlert: defineComponent({ props: ['type'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config/modules/publish-confirm-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.publishToMarket.mockResolvedValue({ error: null })
    hoisted.deviceConfigInfo.mockResolvedValue({ error: null, data: { name: 'Config1', brand: 'Brand1' } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    sessionStorage.removeItem('market_token')
  })

  it('initializes publish form with required market metadata rules', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.visible).toBe(false)
    expect(state.loading).toBe(false)
    expect(state.deviceConfigIdValue).toBe('')
    expect(state.formModel).toEqual({
      market_name: '',
      brand: '',
      model: '',
      category: '',
      version: '1.0.0',
      author: '',
      description: ''
    })
    expect(Object.keys(state.rules)).toEqual([
      'market_name',
      'brand',
      'model',
      'category',
      'version',
      'author',
      'description'
    ])
    expect(state.categoryOptions.map((option: any) => option.value)).toEqual(['IoT', '工业', '农业', '智慧城市', '其他'])
  })

  it('initializes with visible false', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.visible).toBe(false)
  })

  it('open fetches device config info and sets visible true', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.open('cfg-1', 'TestConfig')
    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
    expect(state.visible).toBe(true)
  })

  it('open sets deviceConfigIdValue', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.open('cfg-123')
    expect(state.deviceConfigIdValue).toBe('cfg-123')
  })

  it('open uses defaultName when provided', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.open('cfg-1', 'MyConfig')
    expect(state.formModel.market_name).toBe('MyConfig')
  })

  it('open auto-fills from device config when no defaultName', async () => {
    hoisted.deviceConfigInfo.mockResolvedValue({ error: null, data: { name: 'AutoName', brand: 'Brand1' } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.open('cfg-1')
    expect(state.formModel.market_name).toBe('AutoName')
  })

  it('handlePublish shows error when no market_token', async () => {
    sessionStorage.removeItem('market_token')
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    await wrapper.vm.handlePublish()
    expect(window.$message?.error).toHaveBeenCalledTimes(1)
  })

  it('handlePublish calls publishToMarket with token', async () => {
    sessionStorage.setItem('market_token', 'test-token')
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    state.formModel.market_name = 'Test'
    state.formModel.brand = 'Brand'
    state.formModel.model = 'Model'
    state.formModel.category = 'IoT'
    state.formModel.version = '1.0.0'
    state.formModel.author = 'Author'
    state.formModel.description = 'Desc'
    await wrapper.vm.handlePublish()
    expect(hoisted.publishToMarket).toHaveBeenCalledWith({
      device_config_id: '',
      market_token: 'test-token',
      market_name: 'Test',
      brand: 'Brand',
      model: 'Model',
      category: 'IoT',
      version: '1.0.0',
      author: 'Author',
      description: 'Desc'
    })
  })

  it('handlePublish emits publish-success on success', async () => {
    sessionStorage.setItem('market_token', 'test-token')
    hoisted.publishToMarket.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    state.formModel.market_name = 'Test'
    state.formModel.brand = 'Brand'
    state.formModel.model = 'Model'
    state.formModel.category = 'IoT'
    state.formModel.version = '1.0.0'
    state.formModel.author = 'Author'
    state.formModel.description = 'Desc'
    await wrapper.vm.handlePublish()
    expect(wrapper.emitted('publish-success')).toEqual([[]])
  })

  it('handlePublish shows error on 401', async () => {
    sessionStorage.setItem('market_token', 'test-token')
    hoisted.publishToMarket.mockRejectedValue({ response: { status: 401 } })
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    state.formModel.market_name = 'Test'
    state.formModel.brand = 'Brand'
    state.formModel.model = 'Model'
    state.formModel.category = 'IoT'
    state.formModel.version = '1.0.0'
    state.formModel.author = 'Author'
    state.formModel.description = 'Desc'
    await wrapper.vm.handlePublish()
    expect(window.$message?.error).toHaveBeenCalledTimes(1)
  })

  it('handleCancel sets visible to false', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.visible = true
    state.handleCancel()
    expect(state.visible).toBe(false)
  })

  it('exposes open method', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(typeof wrapper.vm.open).toBe('function')
  })
})
