/**
 * 文件用途：覆盖 table-action-modal 在 告警通知组管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  postNotificationGroup: vi.fn(() => Promise.resolve()),
  putNotificationGroup: vi.fn(() => Promise.resolve()),
  handleSearch: vi.fn(),
  initMemberData: { name: '', notificationType: [] },
  memberTypeData: { value: [{ name: '', notificationType: [] }] },
  notificationTypeOptions: { value: [] }
}))

vi.mock('@/service/api/notification', () => ({
  postNotificationGroup: hoisted.postNotificationGroup,
  putNotificationGroup: hoisted.putNotificationGroup
}))

vi.mock('../../utils', () => ({
  handleSearch: hoisted.handleSearch,
  initMemberData: hoisted.initMemberData,
  memberTypeData: hoisted.memberTypeData,
  notificationTypeOptions: hoisted.notificationTypeOptions
}))

vi.mock('@/constants/business', () => ({
  notificationOptions: [
    { value: 'MEMBER', label: 'Member' },
    { value: 'EMAIL', label: 'Email' },
    { value: 'SME', label: 'SMS' },
    { value: 'VOICE', label: 'Voice' },
    { value: 'WEBHOOK', label: 'Webhook' }
  ]
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import TableActionModal from '../table-action-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(TableActionModal, {
    props: {
      visible: false,
      type: 'add',
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NCheckboxGroup: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        MemberTypeData: true,
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('notification-group/components/table-action-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(globalThis as any).$message = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders the add-notification-group form contract by default', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)

    expect(state.title).toBe('common.createNotificationGroup')
    expect(state.modalVisible).toBe(false)
    expect(state.formModel).toMatchObject({
      name: '',
      description: '',
      notification_type: '',
      status: 'CLOSE'
    })
    expect(state.notificationConfig).toMatchObject({
      MEMBER: '',
      EMAIL: '',
      SME: '',
      VOICE: '',
      WEBHOOK: '',
      PayloadURL: '',
      Secret: ''
    })
    expect(state.rules).toMatchObject({
      name: { required: true, message: 'generate.ruleName' },
      description: { required: true, message: 'common.notificationGroupDesc' },
      notification_type: { required: true, message: 'common.chooseNotificationMethod' }
    })
    expect(wrapper.html()).toContain('generate.notification-group-name')
    expect(wrapper.html()).toContain('generate.notification-method')
    expect(wrapper.html()).toContain('common.save')
  })

  it('computes title based on type - add', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getState(wrapper)
    expect(state.title).toBe('common.createNotificationGroup')
  })

  it('computes title based on type - edit', () => {
    const wrapper = mountComponent({ type: 'edit' })
    const state = getState(wrapper)
    expect(state.title).toBe('common.editNotificationGroup')
  })

  it('initializes formModel with default values', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    expect(state.formModel.name).toBe('')
    expect(state.formModel.description).toBe('')
    expect(state.formModel.notification_type).toBe('')
    expect(state.formModel.status).toBe('CLOSE')
  })

  it('computes modalVisible correctly', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getState(wrapper)
    expect(state.modalVisible).toBe(true)
  })

  it('emits update:visible when closeModal is called', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getState(wrapper)
    state.closeModal()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('resets formModel when closeModal is called', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getState(wrapper)
    state.formModel.name = 'test'
    state.closeModal()
    expect(state.formModel.name).toBe('')
  })

  it('calls handleUpdateFormModelByModalType on visible change', async () => {
    const wrapper = mountComponent({ visible: false })
    const state = getState(wrapper)
    hoisted.notificationTypeOptions.value = [{ value: 'stale', label: 'Stale Option' }]
    state.formModel.name = 'dirty value'

    await wrapper.setProps({ visible: true })
    await flushPromises()

    expect(hoisted.handleSearch).toHaveBeenCalledTimes(1)
    expect(hoisted.notificationTypeOptions.value).toEqual([])
    expect(state.formModel).toMatchObject({
      name: '',
      description: '',
      notification_type: '',
      status: 'CLOSE'
    })
  })

  it('handleAddMember pushes to memberTypeData', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getState(wrapper)
    const initialLength = hoisted.memberTypeData.value.length
    state.handleAddMember()
    expect(hoisted.memberTypeData.value.length).toBe(initialLength + 1)
  })

  it('has form rules defined', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    expect(state.rules.name).toEqual({ required: true, message: 'generate.ruleName' })
    expect(state.rules.description).toEqual({ required: true, message: 'common.notificationGroupDesc' })
    expect(state.rules.notification_type).toEqual({ required: true, message: 'common.chooseNotificationMethod' })
  })

  it('handleUpdateFormModelByModalType for add type resets form', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getState(wrapper)
    state.formModel.name = 'test'
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('')
  })

  it('handleUpdateFormModelByModalType for edit type updates form with editData', () => {
    const editData = {
      id: '1',
      name: 'Test Group',
      description: 'Test Desc',
      notification_type: 'EMAIL',
      notification_config: '{"EMAIL":"test@test.com"}',
      status: 'OPEN',
      tenant_id: 't1'
    } as any
    const wrapper = mountComponent({ type: 'edit', editData })
    const state = getState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('Test Group')
  })
})
