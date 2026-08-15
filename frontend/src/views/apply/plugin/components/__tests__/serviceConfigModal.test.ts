/**
 * 文件用途：覆盖 serviceConfigModal 在 接入插件管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  putRegisterService: vi.fn(),
}))

vi.mock('@/service/api/plugin', () => ({
  putRegisterService: hoisted.putRegisterService,
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import ServiceConfigModal from '../serviceConfigModal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(ServiceConfigModal, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({ props: ['show', 'preset', 'title'], emits: ['update:show', 'after-leave'], setup(_, { slots, emit }) { return () => h('div', { onAfterleave: () => emit('after-leave') }, slots.default?.()) } }),
        NSpace: defineComponent({ props: ['vertical'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpin: defineComponent({ props: ['show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['model', 'rules', 'labelPlacement', 'labelWidth', 'requireMarkPlacement', 'disabled'], setup(_, { slots }) { return () => h('form', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'placeholder', 'type'], emits: ['update:value'], setup(_, { slots }) { return () => h('input', slots.default?.()) } }),
        NSelect: defineComponent({ props: ['value', 'options', 'placeholder'], emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ props: ['type'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('ServiceConfigModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount with default state', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    expect(vm.serviceModal).toBe(false)
    expect(vm.loading).toBe(false)
    expect(vm.details).toEqual({})
  })

  it('should open modal with row having service_type 1 (access protocol)', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ service_type: 1, service_config: '{"http_address":"http://localhost"}' })
    expect(vm.serviceType).toBe('card.accessProtocol')
    expect(vm.serviceModal).toBe(true)
    expect(vm.form.http_address).toBe('http://localhost')
  })

  it('should open modal with row having service_type 2 (access service)', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ service_type: 2, service_config: '{"http_address":"http://example.com"}' })
    expect(vm.serviceType).toBe('card.accessService')
    expect(vm.serviceModal).toBe(true)
  })

  it('should open modal without parsing when service_config is empty', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ service_type: 1, service_config: '' })
    expect(vm.serviceModal).toBe(false)
    expect(vm.details).toEqual({ service_type: 1, service_config: '' })
    expect(vm.form.http_address).toBe('')
  })

  it('should close modal and reset state', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ service_type: 1, service_config: '{"http_address":"http://test"}' })
    vm.close()
    expect(vm.serviceModal).toBe(false)
    expect(vm.details).toEqual({})
  })

  it('should submit config on valid form', async () => {
    hoisted.putRegisterService.mockResolvedValue({ data: true })
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({
      id: 'svc-1',
      name: 'Plugin Service',
      service_type: 1,
      service_config: '{"http_address":"http://test","device_type":2,"sub_topic_prefix":"plugin/a/","access_address":"mqtt://a"}'
    })
    vm.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    await vm.submitSevice()
    await flushPromises()
    expect(hoisted.putRegisterService).toHaveBeenCalledTimes(1)
    expect(hoisted.putRegisterService).toHaveBeenCalledWith({
      id: 'svc-1',
      name: 'Plugin Service',
      service_type: 1,
      service_config: JSON.stringify({
        http_address: 'http://test',
        device_type: 2,
        sub_topic_prefix: 'plugin/a/',
        access_address: 'mqtt://a'
      })
    })
    expect(wrapper.emitted('getList')).toEqual([[]])
  })

  it('should not submit when validation fails', async () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.formRef = { validate: (cb: (errors?: unknown) => void) => cb(true) }
    await vm.submitSevice()
    expect(hoisted.putRegisterService).toHaveBeenCalledTimes(0)
  })

  it('should have correct device type options', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    expect(vm.options).toHaveLength(3)
    expect(vm.options[0].value).toBe(1)
    expect(vm.options[1].value).toBe(2)
    expect(vm.options[2].value).toBe(3)
  })

  it('should expose openModal method', () => {
    const wrapper = mountComponent()
    expect(typeof wrapper.vm.$.exposed!.openModal).toBe('function')
  })
})
