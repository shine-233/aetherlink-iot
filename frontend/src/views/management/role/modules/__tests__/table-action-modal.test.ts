/**
 * 文件用途：覆盖 table-action-modal 在角色管理场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  createRole: vi.fn(),
  updateRole: vi.fn()
}))

vi.mock('@/service/api', () => ({
  createRole: hoisted.createRole,
  updateRole: hoisted.updateRole
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg, trigger: ['input', 'blur'] })
}))

import TableActionModal from '../table-action-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(TableActionModal, {
    props: {
      visible: false,
      type: 'add',
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({ name: 'NModal', props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ name: 'NForm', props: { model: Object, rules: [Object, Array] }, setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ name: 'NFormItem', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
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

describe('management/role/modules/table-action-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.createRole.mockResolvedValue({ error: null })
    hoisted.updateRole.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds the role modal, form model and validation rules', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.modalVisible).toBe(false)
    expect(state.formModel).toEqual({
      name: '',
      email: '',
      description: ''
    })
    expect(state.rules).toMatchObject({
      name: { required: true, trigger: ['input', 'blur'] },
      description: { required: true, trigger: ['input', 'blur'] },
      email: {}
    })
  })

  it('title returns add translation for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('page.manage.role.title')
  })

  it('title returns edit translation for edit type', () => {
    const wrapper = mountComponent({ type: 'edit' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('page.manage.role.editRole')
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
    expect(model.name).toBe('')
    expect(model.email).toBe('')
    expect(model.description).toBe('')
  })

  it('handleUpdateFormModel merges model into formModel', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleUpdateFormModel({ name: 'Admin', description: 'Admin role' })
    expect(state.formModel.name).toBe('Admin')
    expect(state.formModel.description).toBe('Admin role')
  })

  it('handleUpdateFormModelByModalType resets form for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    state.formModel.name = 'Old Name'
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('')
  })

  it('handleUpdateFormModelByModalType populates form with editData for edit type', () => {
    const editData = { id: 'r-1', name: 'Admin', description: 'Admin role', email: 'admin@test.com' } as any
    const wrapper = mountComponent({ type: 'edit', editData })
    const state = getSetupState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('Admin')
    expect(state.formModel.description).toBe('Admin role')
  })

  it('handleSubmit calls createRole for add type and emits success', async () => {
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.name = 'Operator'
    state.formModel.description = 'Operate devices'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.createRole).toHaveBeenCalledWith({
      name: 'Operator',
      email: '',
      description: 'Operate devices'
    })
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit calls updateRole for edit type and emits success', async () => {
    const editData = { id: 'r-1', name: 'Admin', description: 'Admin role', email: 'admin@test.com' } as any
    const wrapper = mountComponent({ type: 'edit', editData, visible: false })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.description = 'Updated role'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.updateRole).toHaveBeenCalledWith({
      id: 'r-1',
      name: 'Admin',
      description: 'Updated role',
      email: 'admin@test.com'
    })
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit does not emit success when API returns error', async () => {
    hoisted.createRole.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    await state.handleSubmit()
    await flushPromises()
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('watch on visible triggers handleUpdateFormModelByModalType when visible becomes true', async () => {
    const wrapper = mountComponent({ type: 'add', visible: false })
    const state = getSetupState(wrapper)
    state.formModel.name = 'Old'
    await wrapper.setProps({ visible: true })
    await flushPromises()
    expect(state.formModel.name).toBe('')
  })

  it('rules are defined for name, description and email', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.rules.name).toMatchObject({ required: true, message: expect.any(String), trigger: ['input', 'blur'] })
    expect(state.rules.description).toMatchObject({ required: true, message: expect.any(String), trigger: ['input', 'blur'] })
    expect(state.rules.email).toEqual({})
  })
})
