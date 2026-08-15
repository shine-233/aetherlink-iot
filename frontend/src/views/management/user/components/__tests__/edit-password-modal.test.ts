/**
 * 文件用途：覆盖 edit-password-modal 在 后台用户管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  editUser: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/auth', () => ({
  editUser: hoisted.editUser
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/form/rule', () => ({
  formRules: {
    email: [{ required: true, message: 'email required', trigger: ['input', 'blur'] }],
    pwd: [{ required: true, message: 'pwd required', trigger: ['input', 'blur'] }]
  },
  getConfirmPwdRule: () => ({ required: true, message: 'confirm pwd', trigger: ['input', 'blur'] })
}))

import EditPasswordModal from '../edit-password-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(EditPasswordModal, {
    props: {
      visible: false,
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({ name: 'NModal', props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ name: 'NForm', props: { model: Object, rules: [Object, Array] }, setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ name: 'NFormItem', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NFormItemGridItem: defineComponent({ name: 'NFormItemGridItem', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NGrid: defineComponent({ name: 'NGrid', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ name: 'NInput', props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ name: 'NSpace', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ name: 'NButton', emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/user/components/edit-password-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.editUser.mockResolvedValue({ error: null, msg: 'success' })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds password modal, readonly email form model and password rules', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.modalVisible).toBe(false)
    expect(state.formModel).toEqual({
      email: '',
      password: '',
      confirmPwd: ''
    })
    expect(state.rules).toMatchObject({
      email: [{ required: true, message: 'email required', trigger: ['input', 'blur'] }],
      password: [{ required: true, message: 'pwd required', trigger: ['input', 'blur'] }],
      confirmPwd: { required: true, message: 'confirm pwd', trigger: ['input', 'blur'] }
    })
  })

  it('modalVisible get returns props.visible', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getSetupState(wrapper)
    expect(state.modalVisible).toBe(true)
  })

  it('modalVisible set emits update:visible', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.modalVisible = false
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('closeModal emits update:visible false', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.closeModal()
    expect(wrapper.emitted('update:visible')![0]).toEqual([false])
  })

  it('createDefaultFormModel returns default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.email).toBe('')
    expect(model.password).toBe('')
    expect(model.confirmPwd).toBe('')
  })

  it('handleUpdateFormModel merges model into formModel', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleUpdateFormModel({ email: 'test@test.com', password: '123456' })
    expect(state.formModel.email).toBe('test@test.com')
    expect(state.formModel.password).toBe('123456')
  })

  it('handleUpdateFormModelByModalType populates form with editData', () => {
    const editData = { id: 'u-1', email: 'edit@test.com' } as any
    const wrapper = mountComponent({ editData })
    const state = getSetupState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.email).toBe('edit@test.com')
  })

  it('handleUpdateFormModelByModalType does nothing when editData is null', () => {
    const wrapper = mountComponent({ editData: null })
    const state = getSetupState(wrapper)
    state.formModel.email = 'existing@test.com'
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.email).toBe('existing@test.com')
  })

  it('handleSubmit validates, calls editUser and emits success', async () => {
    const wrapper = mountComponent({ visible: false, editData: { id: 'u-1', email: 'user@test.com' } as any })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.password = 'NewPass123'
    state.formModel.confirmPwd = 'NewPass123'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.editUser).toHaveBeenCalledWith({
      id: 'u-1',
      email: 'user@test.com',
      password: 'NewPass123'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit does not emit success when API returns error', async () => {
    hoisted.editUser.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent({ visible: false, editData: { id: 'u-1', email: 'user@test.com' } as any })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    await state.handleSubmit()
    await flushPromises()
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('update:visible')).toBeUndefined()
  })

  it('handleSubmit resets form model after success', async () => {
    const wrapper = mountComponent({ visible: false, editData: { id: 'u-1', email: 'user@test.com' } as any })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.password = 'oldpass'
    await state.handleSubmit()
    await flushPromises()
    expect(state.formModel.password).toBe('')
    expect(state.formModel.confirmPwd).toBe('')
  })

  it('watch on visible triggers handleUpdateFormModelByModalType when visible becomes true', async () => {
    const editData = { id: 'u-1', email: 'watch@test.com' } as any
    const wrapper = mountComponent({ visible: false, editData })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    expect(state.formModel.email).toBe('watch@test.com')
  })

  it('rules are defined for email, password and confirmPwd', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.rules.email).toEqual([{ required: true, message: 'email required', trigger: ['input', 'blur'] }])
    expect(state.rules.password).toEqual([{ required: true, message: 'pwd required', trigger: ['input', 'blur'] }])
    expect(state.rules.confirmPwd).toEqual({ required: true, message: 'confirm pwd', trigger: ['input', 'blur'] })
  })
})
