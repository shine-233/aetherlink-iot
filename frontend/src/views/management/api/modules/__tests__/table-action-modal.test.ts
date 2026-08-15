/**
 * 文件用途：覆盖 table-action-modal 在 API 密钥管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addKey: vi.fn(),
  updateKey: vi.fn()
}))

vi.mock('@/service/api', () => ({
  addKey: hoisted.addKey,
  updateKey: hoisted.updateKey
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg, trigger: ['input', 'blur'] })
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: { id: '1', tenant_id: 'tenant-1', email: 'admin@test.com' },
    isLogin: true
  })
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

describe('management/api/modules/table-action-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addKey.mockResolvedValue({ error: null })
    hoisted.updateKey.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds API-key modal, form model and tenant validation rules', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.modalVisible).toBe(false)
    expect(state.formModel).toEqual({
      name: '',
      tenant_id: 'tenant-1'
    })
    expect(state.rules).toMatchObject({
      name: { required: true, trigger: ['input', 'blur'] },
      tenant_id: { required: true, message: '', trigger: ['input', 'blur'] }
    })
  })

  it('title returns add translation for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('page.manage.api.addApiKey')
  })

  it('title returns edit translation for edit type', () => {
    const wrapper = mountComponent({ type: 'edit' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('page.manage.api.editAPi')
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

  it('createDefaultFormModel returns default values with tenant_id from authStore', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.name).toBe('')
    expect(model.tenant_id).toBe('tenant-1')
  })

  it('handleUpdateFormModel merges model into formModel', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleUpdateFormModel({ name: 'New Key', tenant_id: 'new-tenant' })
    expect(state.formModel.name).toBe('New Key')
    expect(state.formModel.tenant_id).toBe('new-tenant')
  })

  it('handleUpdateFormModelByModalType resets form for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    state.formModel.name = 'Old Name'
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('')
    expect(state.formModel.tenant_id).toBe('tenant-1')
  })

  it('handleUpdateFormModelByModalType populates form with editData for edit type', () => {
    const editData = { id: 'k-1', name: 'Edit Key', tenant_id: 'tenant-2' } as any
    const wrapper = mountComponent({ type: 'edit', editData })
    const state = getSetupState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('Edit Key')
    expect(state.formModel.tenant_id).toBe('tenant-2')
  })

  it('handleSubmit calls addKey for add type and emits success', async () => {
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.name = 'Edge collector key'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.addKey).toHaveBeenCalledWith({
      name: 'Edge collector key',
      tenant_id: 'tenant-1'
    })
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit calls updateKey for edit type and emits success', async () => {
    const editData = { id: 'k-1', name: 'Edit Key', tenant_id: 'tenant-2' } as any
    const wrapper = mountComponent({ type: 'edit', editData, visible: false })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.name = 'Renamed key'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.updateKey).toHaveBeenCalledWith({
      id: 'k-1',
      name: 'Renamed key',
      tenant_id: 'tenant-2'
    })
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit does not emit success when API returns error', async () => {
    hoisted.addKey.mockResolvedValue({ error: 'fail' })
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

  it('rules are defined for name and tenant_id', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.rules.name).toMatchObject({ required: true, message: expect.any(String), trigger: ['input', 'blur'] })
    expect(state.rules.tenant_id).toMatchObject({ required: true, message: '', trigger: ['input', 'blur'] })
  })
})
