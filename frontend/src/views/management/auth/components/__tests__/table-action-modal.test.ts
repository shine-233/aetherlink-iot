/**
 * 文件用途：覆盖 table-action-modal 在 权限元素管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchElementList: vi.fn(),
  addElement: vi.fn(),
  editElement: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/route', () => ({
  fetchElementList: hoisted.fetchElementList,
  addElement: hoisted.addElement,
  editElement: hoisted.editElement
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/tool', () => ({
  deepClone: (obj: any) => JSON.parse(JSON.stringify(obj))
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg, trigger: ['input', 'blur'] })
}))

vi.mock('@/constants/business', () => ({
  routeSysFlagOptions: [{ label: 'Admin', value: '1' }, { label: 'User', value: '2' }],
  routeTypeOptions: [{ label: 'Menu', value: '1' }, { label: 'Button', value: '3' }]
}))

vi.mock('@/plugins/icon/icons', () => ({
  icons: {}
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
        NModal: defineComponent({ name: 'NModal', props: { show: Boolean, title: String }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ name: 'NForm', props: { model: Object, rules: Object }, setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NFormItemGridItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NInputNumber: defineComponent({ props: { value: { default: 0 } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NTreeSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NRadioGroup: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NRadio: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NCheckboxGroup: defineComponent({ props: { value: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NCheckbox: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NSwitch: defineComponent({ props: { value: { default: '0' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        IconSelect: defineComponent({ setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/auth/components/table-action-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchElementList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.addElement.mockResolvedValue({ error: null, msg: 'success' })
    hoisted.editElement.mockResolvedValue({ error: null, msg: 'success' })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds the menu modal, form model and validation rules on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const modal = wrapper.getComponent({ name: 'NModal' })
    const form = wrapper.getComponent({ name: 'NForm' })

    expect(modal.props('show')).toBe(false)
    expect(modal.props('title')).toBe('common.add')
    expect(form.props('model')).toBe(state.formModel)
    expect(form.props('rules')).toBe(state.rules)
    expect(state.formModel).toMatchObject({
      parent_id: '0',
      element_code: '',
      multilingual: 'default',
      param3: '0',
      orders: 1,
      element_type: 1,
      authority: []
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
    expect(wrapper.emitted('update:visible')![0]).toEqual([false])
  })

  it('createDefaultFormModel returns default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.parent_id).toBe('0')
    expect(model.element_code).toBe('')
    expect(model.multilingual).toBe('default')
    expect(model.param3).toBe('0')
    expect(model.orders).toBe(1)
    expect(model.element_type).toBe(1)
    expect(model.authority).toEqual([])
  })

  it('handleUpdateFormModel merges model into formModel', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleUpdateFormModel({ description: 'New Menu', element_code: 'new-menu' })
    expect(state.formModel.description).toBe('New Menu')
    expect(state.formModel.element_code).toBe('new-menu')
  })

  it('handleUpdateFormModelByModalType resets form for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    state.formModel.description = 'Old Description'
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.description).toBe('')
  })

  it('handleUpdateFormModelByModalType populates form with editData for edit type', () => {
    const editData = {
      id: 'm-1',
      description: 'Edit Menu',
      element_code: 'edit-menu',
      parent_id: '0',
      param1: 'edit',
      param2: 'mdi-edit',
      element_type: 1,
      authority: ['1'],
      remark: 'remark',
      multilingual: 'default',
      param3: '0',
      orders: 2,
      route_path: ''
    } as any
    const wrapper = mountComponent({ type: 'edit', editData })
    const state = getSetupState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.description).toBe('Edit Menu')
    expect(state.formModel.element_code).toBe('edit-menu')
  })

  it('getTableData fetches element list and populates parentOptions', async () => {
    hoisted.fetchElementList.mockResolvedValue({ data: { list: [{ id: 'm-1', description: 'Menu 1' }], total: 1 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.getTableData()
    await flushPromises()
    expect(hoisted.fetchElementList).toHaveBeenCalledWith({ page: 1, page_size: 99 })
    expect(state.parentOptions).toHaveLength(1)
  })

  it('handleSubmit calls addElement for add type and emits success', async () => {
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.addElement).toHaveBeenCalledWith(expect.objectContaining({
      parent_id: '0',
      authority: JSON.stringify([])
    }))
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit calls editElement for edit type and emits success', async () => {
    const editData = { id: 'm-1', description: 'Edit', element_code: 'edit', parent_id: '0', authority: [] } as any
    const wrapper = mountComponent({ type: 'edit', editData, visible: false })
    const state = getSetupState(wrapper)
    await wrapper.setProps({ visible: true })
    await flushPromises()
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.editElement).toHaveBeenCalledWith(expect.objectContaining({
      id: 'm-1',
      authority: JSON.stringify([])
    }))
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit does not emit success when API returns error', async () => {
    hoisted.addElement.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    await state.handleSubmit()
    await flushPromises()
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit serializes authority to JSON string', async () => {
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.authority = ['1', '2'] as any
    await state.handleSubmit()
    await flushPromises()
    const callArgs = hoisted.addElement.mock.calls[0][0]
    expect(callArgs.authority).toBe(JSON.stringify(['1', '2']))
  })

  it('handleSubmit defaults parent_id to 0 when empty', async () => {
    const wrapper = mountComponent({ type: 'add', visible: true })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.formModel.parent_id = '' as any
    await state.handleSubmit()
    await flushPromises()
    const callArgs = hoisted.addElement.mock.calls[0][0]
    expect(callArgs.parent_id).toBe('0')
  })

  it('watch on visible triggers handleUpdateFormModelByModalType when visible becomes true', async () => {
    const wrapper = mountComponent({ type: 'add', visible: false })
    const state = getSetupState(wrapper)
    state.formModel.description = 'Old'
    await wrapper.setProps({ visible: true })
    await flushPromises()
    expect(state.formModel.description).toBe('')
  })

  it('rules are defined for description, element_code and authority', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.rules.description).toEqual({ required: true, message: 'common.pleaseCheckValue', trigger: ['input', 'blur'] })
    expect(state.rules.element_code).toEqual({ required: true, message: 'common.pleaseCheckValue', trigger: ['input', 'blur'] })
    expect(state.rules.authority).toEqual({ required: true, message: 'common.pleaseCheckValue', trigger: ['input', 'blur'] })
  })
})
