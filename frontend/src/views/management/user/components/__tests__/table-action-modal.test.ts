/**
 * 文件用途：覆盖 table-action-modal 在 后台用户管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addUser: vi.fn(),
  editUser: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/auth', () => ({
  addUser: hoisted.addUser,
  editUser: hoisted.editUser
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg, trigger: ['input', 'blur'] }),
  formRules: {
    email: [{ required: true, message: 'email required', trigger: ['input', 'blur'] }],
    pwd: [{ required: true, message: 'pwd required', trigger: ['input', 'blur'] }]
  },
  getConfirmPwdRule: () => ({ required: true, message: 'confirm pwd', trigger: ['input', 'blur'] })
}))

vi.mock('@/constants/business', () => ({
  userStatusOptions: [
    { label: 'Normal', value: 'N' },
    { label: 'Freeze', value: 'F' }
  ]
}))

vi.mock('@/components/common/ProvinceCityDistrictSelector.vue', () => ({
  default: defineComponent({
    name: 'ProvinceCityDistrictSelectorStub',
    setup() {
      return () => h('div', { class: 'province-city-district-selector-stub' })
    }
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
        NFormItemGridItem: defineComponent({ name: 'NFormItemGridItem', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NGrid: defineComponent({ name: 'NGrid', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ name: 'NInput', props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ name: 'NSelect', props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NRadioGroup: defineComponent({ name: 'NRadioGroup', props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NRadio: defineComponent({ name: 'NRadio', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ name: 'NSpace', setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ name: 'NButton', emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/user/components/table-action-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addUser.mockResolvedValue({ error: null })
    hoisted.editUser.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds the user modal, form model, rules and default add fields', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const modal = wrapper.getComponent({ name: 'Modal' })
    const form = wrapper.getComponent({ name: 'Form' })

    expect(modal.props('show')).toBe(false)
    expect(form.props('model')).toBe(state.formModel)
    expect(form.props('rules')).toBe(state.rules)
    expect(state.formModel).toMatchObject({
      name: '',
      email: '',
      phone_number: '',
      status: 'N',
      timezone: 'Asia/Shanghai',
      default_language: 'en-US',
      country_code: '+86',
      phone_only: '',
      address: {
        province: '',
        city: '',
        district: '',
        detailed_address: ''
      }
    })
    expect(state.rules).toMatchObject({
      name: { required: true, message: 'common.pleaseCheckValue', trigger: ['input', 'blur'] },
      email: [{ required: true, message: 'email required', trigger: ['input', 'blur'] }],
      status: { required: true, message: 'common.pleaseCheckValue', trigger: ['input', 'blur'] }
    })
  })

  it('title returns add translation for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('common.add')
  })

  it('title returns edit translation for edit type', () => {
    const wrapper = mountComponent({ type: 'edit' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('common.edit')
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
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('customUserStatusOptions maps status options', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const options = state.customUserStatusOptions
    expect(options).toHaveLength(2)
    expect(options[0]).toEqual({ label: 'page.manage.user.status.normal', value: 'N' })
    expect(options[1]).toEqual({ label: 'page.manage.user.status.freeze', value: 'F' })
  })

  it('fullPhoneNumber combines country code and phone only', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formModel.country_code = '+86'
    state.formModel.phone_only = '13800000000'
    expect(state.fullPhoneNumber).toBe('+8613800000000')
  })

  it('createDefaultFormModel returns default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.name).toBe('')
    expect(model.status).toBe('N')
    expect(model.timezone).toBe('Asia/Shanghai')
    expect(model.default_language).toBe('en-US')
    expect(model.country_code).toBe('+86')
    expect(model.address).toEqual({ province: '', city: '', district: '', detailed_address: '' })
  })

  it('handleAddressChange updates form model address', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleAddressChange({ province: '北京市', city: '北京市', district: '东城区' })
    expect(state.formModel.address.province).toBe('北京市')
    expect(state.formModel.address.city).toBe('北京市')
    expect(state.formModel.address.district).toBe('东城区')
  })

  it('handleUpdateFormModel merges model into formModel', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleUpdateFormModel({ name: 'Updated', email: 'updated@test.com' })
    expect(state.formModel.name).toBe('Updated')
    expect(state.formModel.email).toBe('updated@test.com')
  })

  it('handleUpdateFormModelByModalType resets form for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    state.formModel.name = 'Old Name'
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('')
  })

  it('handleUpdateFormModelByModalType populates form with editData for edit type', () => {
    const editData = {
      id: 'u-1',
      name: 'Edit User',
      email: 'edit@test.com',
      phone_number: '+8613800000000',
      address: { province: '北京市', city: '北京市', district: '东城区', detailed_address: 'addr' }
    } as any
    const wrapper = mountComponent({ type: 'edit', editData })
    const state = getSetupState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('Edit User')
    expect(state.formModel.email).toBe('edit@test.com')
    expect(state.formModel.country_code).toBe('+86')
    expect(state.formModel.phone_only).toBe('13800000000')
  })

  it('handleSubmit calls addUser for add type and emits success', async () => {
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.name = 'New User'
    state.formModel.email = 'new@test.com'
    state.formModel.phone_number = '+8613800000000'
    state.formModel.country_code = '+86'
    state.formModel.phone_only = '13800000000'
    state.formModel.password = 'Password1'
    state.formModel.confirmPwd = 'Password1'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.addUser).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'New User',
        email: 'new@test.com',
        phone_number: '+8613800000000',
        password: 'Password1',
        country_code: '+86',
        phone_only: '13800000000'
      })
    )
    expect(hoisted.addUser).toHaveBeenCalledWith(expect.not.objectContaining({ confirmPwd: expect.anything() }))
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit calls editUser for edit type and emits success', async () => {
    const editData = { id: 'u-1', name: 'User', email: 'user@test.com' } as any
    const wrapper = mountComponent({ type: 'edit', editData, visible: false })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.name = 'Edited User'
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.editUser).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'u-1',
        name: 'Edited User',
        email: 'user@test.com'
      })
    )
    expect(hoisted.editUser).toHaveBeenCalledWith(expect.not.objectContaining({ confirmPwd: expect.anything() }))
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit does not emit success when API returns error', async () => {
    hoisted.addUser.mockResolvedValue({ error: 'fail' })
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

  it('parsePhoneNumber extracts country code and phone number', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const result = state.parsePhoneNumber('+8613800000000')
    expect(result.country_code).toBe('+86')
    expect(result.phone_only).toBe('13800000000')
  })

  it('parsePhoneNumber returns default when empty', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const result = state.parsePhoneNumber('')
    expect(result.country_code).toBe('+86')
    expect(result.phone_only).toBe('')
  })

  it('parsePhoneNumber defaults to +86 when no code matches', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const result = state.parsePhoneNumber('9999999')
    expect(result.country_code).toBe('+86')
    expect(result.phone_only).toBe('9999999')
  })
})
