/**
 * 文件用途：覆盖 serviceModal 在 接入插件管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  putRegisterService: vi.fn(),
  registerService: vi.fn(),
}))

vi.mock('@/service/api/plugin', () => ({
  putRegisterService: hoisted.putRegisterService,
  registerService: hoisted.registerService,
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import ServiceModal from '../serviceModal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(ServiceModal, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({ props: ['show', 'preset', 'title'], emits: ['update:show', 'after-leave'], setup(_, { slots, emit }) { return () => h('div', { onAfterleave: () => emit('after-leave') }, slots.default?.()) } }),
        NSpace: defineComponent({ props: ['vertical'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpin: defineComponent({ props: ['show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['model', 'rules', 'labelPlacement', 'labelWidth', 'requireMarkPlacement', 'disabled'], setup(_, { slots }) { return () => h('form', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'placeholder', 'type'], emits: ['update:value'], setup(_, { slots }) { return () => h('input', slots.default?.()) } }),
        NSelect: defineComponent({ props: ['value', 'options', 'disabled', 'placeholder'], emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ props: ['type'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('ServiceModal', () => {
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
    expect(vm.isEdit).toBe(false)
    expect(vm.serviceModal).toBe(false)
    expect(vm.loading).toBe(false)
  })

  it('should open modal for add (no row)', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal(null)
    expect(vm.isEdit).toBe(false)
    expect(vm.serviceModal).toBe(true)
  })

  it('should open modal for edit (with row)', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ name: 'TestService', service_identifier: 'test-1', service_type: 1 })
    expect(vm.isEdit).toBe(true)
    expect(vm.serviceModal).toBe(true)
    expect(vm.form.name).toBe('TestService')
  })

  it('should close modal and reset form', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ name: 'TestService', service_identifier: 'test-1', service_type: 1 })
    vm.close()
    expect(vm.serviceModal).toBe(false)
    expect(vm.form.name).toBe('')
  })

  it('should submit create service on valid form', async () => {
    hoisted.registerService.mockResolvedValue({ data: true })
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal(null)
    vm.form = { name: 'New', service_identifier: 'new-1', service_type: 1, version: '', description: '', service_config: '', remark: '' }
    // Mock formRef.validate to call callback with no errors
    vm.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    await vm.submitSevice()
    await flushPromises()
    expect(hoisted.registerService).toHaveBeenCalledTimes(1)
    expect(hoisted.registerService).toHaveBeenCalledWith({
      name: 'New',
      service_identifier: 'new-1',
      service_type: 1,
      version: '',
      description: '',
      service_config: '',
      remark: ''
    })
    expect(wrapper.emitted('getList')).toEqual([[]])
  })

  it('should submit update service on valid form when editing', async () => {
    hoisted.putRegisterService.mockResolvedValue({ data: true })
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.openModal({ name: 'Existing', service_identifier: 'ex-1', service_type: 2 })
    vm.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    await vm.submitSevice()
    await flushPromises()
    expect(hoisted.putRegisterService).toHaveBeenCalledTimes(1)
    expect(hoisted.putRegisterService).toHaveBeenCalledWith({
      name: 'Existing',
      service_identifier: 'ex-1',
      service_type: 2,
      version: '',
      description: '',
      service_config: '',
      remark: ''
    })
    expect(wrapper.emitted('getList')).toEqual([[]])
  })

  it('should not submit when validation fails', async () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    vm.formRef = { validate: (cb: (errors?: unknown) => void) => cb(true) }
    await vm.submitSevice()
    expect(hoisted.registerService).toHaveBeenCalledTimes(0)
    expect(hoisted.putRegisterService).toHaveBeenCalledTimes(0)
  })

  it('should expose openModal method', () => {
    const wrapper = mountComponent()
    expect(typeof wrapper.vm.$.exposed?.openModal).toBe('function')
  })

  it('should have correct options', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    expect(vm.options).toHaveLength(2)
    expect(vm.options[0].value).toBe(1)
    expect(vm.options[1].value).toBe(2)
  })
})
