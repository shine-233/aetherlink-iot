/**
 * 文件用途：验证 frontend/src/views/personal-center/components/__tests__/change-information 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  changeInformation: vi.fn(),
  passwordModification: vi.fn()
}))

vi.mock('@/service/api/personal-center', () => ({
  changeInformation: hoisted.changeInformation,
  passwordModification: hoisted.passwordModification
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/form', () => ({
  useNaiveForm: () => ({
    formRef: { value: null },
    validate: vi.fn(() => Promise.resolve())
  })
}))

vi.mock('@/utils/form/rule', () => ({
  getConfirmPwdRule: () => []
}))

vi.mock('@/utils/common/tool', () => ({
  generateRandomHexString: vi.fn(() => 'randomhex'),
  validName: vi.fn(() => true),
  validPasswordByExp: vi.fn(() => true)
}))

vi.mock('@/utils/security/rsa-encrypt', () => ({
  encryptDataByRsa: vi.fn((v: string) => `encrypted-${v}`)
}))

import ChangeInformation from '../change-information.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (propsOverrides = {}) => {
  const wrapper = shallowMount(ChangeInformation, {
    props: {
      visible: false,
      type: 'amend',
      ...propsOverrides
    },
    global: {
      stubs: {
        NModal: defineComponent({
          props: ['show', 'preset', 'title', 'class'],
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NForm: defineComponent({
          expose: ['validate'],
          setup(_, { slots }) {
            const validate = () => Promise.resolve()
            return { validate }
          },
          render() {
            return h('form', this.$slots.default?.())
          }
        }),
        NGrid: defineComponent({
          props: ['cols', 'xGap'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NFormItemGridItem: defineComponent({
          props: ['span', 'label', 'path', 'labelWidth', 'type', 'showPasswordOn'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: ['value', 'type', 'showPasswordOn'],
          emits: ['update:value'],
          setup(_, { slots }) {
            return () => h('input', slots.default?.())
          }
        }),
        NSpace: defineComponent({
          props: ['size', 'justify', 'class'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NButton: defineComponent({
          props: ['type', 'class'],
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

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('ChangeInformation', () => {
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
    expect(vm.formData.name).toBe('')
    expect(vm.formData.old_password).toBe('')
    expect(vm.formData.password).toBe('')
    expect(vm.formData.passwords).toBe('')
  })

  it('should compute title for amend type', () => {
    const wrapper = mountComponent({ type: 'amend' })
    const vm = wrapper.vm as any
    expect(vm.title).toBe('custom.personalCenter.modifyBasicInfo')
  })

  it('should compute title for changePassword type', () => {
    const wrapper = mountComponent({ type: 'changePassword' })
    const vm = wrapper.vm as any
    expect(vm.title).toBe('custom.personalCenter.changePassword')
  })

  it('should compute estimate for amend type', () => {
    const wrapper = mountComponent({ type: 'amend' })
    const vm = wrapper.vm as any
    expect(vm.estimate).toBe('amend')
  })

  it('should compute estimate for changePassword type', () => {
    const wrapper = mountComponent({ type: 'changePassword' })
    const vm = wrapper.vm as any
    expect(vm.estimate).toBe('changePassword')
  })

  it('should close modal and reset name', () => {
    const wrapper = mountComponent({ visible: true })
    const vm = wrapper.vm as any
    vm.formData.name = 'Test'
    vm.closeModal()
    expect(vm.formData.name).toBe('')
  })

  it('should emit update:visible when closing modal', () => {
    const wrapper = mountComponent({ visible: true })
    const vm = wrapper.vm as any
    vm.closeModal()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('should submit name change for amend type', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ type: 'amend', visible: true })
    const vm = wrapper.vm as any
    vm.formData.name = 'NewName'
    await vm.editName()
    await flushPromises()
    expect(hoisted.changeInformation).toHaveBeenCalledWith({ name: 'NewName' })
  })

  it('should emit modification with name after successful name change', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ type: 'amend', visible: true })
    const vm = wrapper.vm as any
    vm.formData.name = 'NewName'
    await vm.editName()
    await flushPromises()
    expect(wrapper.emitted('modification')).toEqual([['NewName']])
  })

  it('should submit password change for changePassword type', async () => {
    hoisted.passwordModification.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ type: 'changePassword', visible: true })
    const vm = wrapper.vm as any
    vm.formData.old_password = 'old'
    vm.formData.password = 'newpassword1'
    await vm.password()
    await flushPromises()
    expect(hoisted.passwordModification).toHaveBeenCalledTimes(1)
    expect(hoisted.passwordModification).toHaveBeenCalledWith({
      old_password: 'old',
      password: 'newpassword1',
      salt: null
    })
  })

  it('should emit modification after successful password change', async () => {
    hoisted.passwordModification.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ type: 'changePassword', visible: true })
    const vm = wrapper.vm as any
    vm.formData.old_password = 'old'
    vm.formData.password = 'newpassword1'
    await vm.password()
    await flushPromises()
    expect(wrapper.emitted('modification')).toEqual([[]])
  })

  it('should call editName when submit with amend estimate', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ type: 'amend', visible: true })
    const vm = wrapper.vm as any
    vm.formData.name = 'NewName'
    await vm.submit()
    await flushPromises()
    expect(hoisted.changeInformation).toHaveBeenCalledTimes(1)
    expect(hoisted.changeInformation).toHaveBeenCalledWith({ name: 'NewName' })
  })

  it('should call password when submit with changePassword estimate', async () => {
    hoisted.passwordModification.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ type: 'changePassword', visible: true })
    const vm = wrapper.vm as any
    vm.formData.old_password = 'old'
    vm.formData.password = 'newpassword1'
    await vm.submit()
    await flushPromises()
    expect(hoisted.passwordModification).toHaveBeenCalledTimes(1)
    expect(hoisted.passwordModification).toHaveBeenCalledWith({
      old_password: 'old',
      password: 'newpassword1',
      salt: null
    })
  })

  it('should encrypt password when frontend_res enable_flag is enable', async () => {
    hoisted.passwordModification.mockResolvedValue({ error: null })
    // Set the localStorage value that the component reads
    localStorage.setItem('enableZcAndYzm', JSON.stringify([{ name: 'frontend_res', enable_flag: 'enable' }]))
    const wrapper = mountComponent({ type: 'changePassword', visible: true })
    const vm = wrapper.vm as any
    vm.formData.old_password = 'old'
    vm.formData.password = 'newpassword1'
    await vm.password()
    await flushPromises()
    const callArgs = hoisted.passwordModification.mock.calls[0][0]
    expect(callArgs.salt).toBe('randomhex')
    localStorage.removeItem('enableZcAndYzm')
  })
})
