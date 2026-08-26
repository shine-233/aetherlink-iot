/**
 * 文件用途：覆盖 member-type-data 在 告警通知组管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getCurrentName: vi.fn(() => 'user1'),
  handleDeleteMember: vi.fn(),
  handleScroll: vi.fn(),
  handleSearch: vi.fn(),
  handleUpdateMember: vi.fn(),
  memberOptionsLoading: { value: false },
  notificationTypeOptions: { value: [] }
}))

vi.mock('../../utils', () => ({
  getCurrentName: hoisted.getCurrentName,
  handleDeleteMember: hoisted.handleDeleteMember,
  handleScroll: hoisted.handleScroll,
  handleSearch: hoisted.handleSearch,
  handleUpdateMember: hoisted.handleUpdateMember,
  memberOptionsLoading: hoisted.memberOptionsLoading,
  notificationTypeOptions: hoisted.notificationTypeOptions
}))

vi.mock('@/constants/business', () => ({
  MemberNotificationOptions: [
    { value: 'EMAIL', label: 'Email' },
    { value: 'SMS', label: 'SMS' },
    { value: 'VOICE', label: 'Voice' }
  ]
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import MemberTypeData from '../member-type-data.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(MemberTypeData, {
    props: {
      index: 0,
      selectedNotificationType: [],
      ...props
    },
    global: {
      stubs: {
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSelect: defineComponent({ name: 'NSelect', props: { value: { default: '' }, options: { default: () => [] } }, emits: ['update:value', 'search', 'scroll'], setup() { return () => h('div') } }),
        NCheckboxGroup: defineComponent({ props: { value: { default: () => [] } }, emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NCheckbox: defineComponent({ name: 'NCheckbox', props: { value: { default: '' }, label: { default: '' } }, emits: ['update:checked'], setup(_, { slots }) { return () => h('label', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        // 删除按钮已包进 Popconfirm 二次确认；桩只渲染 trigger，保持原断言可定位按钮文案。
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('span', slots.trigger?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('notification-group/components/member-type-data.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders the member notification selector contract by default', () => {
    hoisted.notificationTypeOptions.value = [{ label: 'User One', value: 'user1' }]
    const wrapper = mountComponent({ selectedNotificationType: ['EMAIL', 'VOICE'] })
    const state = getState(wrapper)

    expect(hoisted.getCurrentName).toHaveBeenCalledWith(0)
    expect(state.selectedMember).toBe('user1')
    expect(state.selectNotificationType).toEqual(['EMAIL', 'VOICE'])
    expect(state.formModel).toMatchObject({ name: '', signMode: null })
    expect(state.rules).toMatchObject({
      name: { required: true, message: 'generate.ruleName' },
      signMode: { required: true, message: 'generate.signatureMethod' },
      ip: { required: true, message: 'generate.IPwhitelist' }
    })
    expect(wrapper.findAllComponents({ name: 'NSelect' })).toHaveLength(1)
    expect(wrapper.findAllComponents({ name: 'NCheckbox' })).toHaveLength(3)
    expect(wrapper.text()).toContain('Email')
    expect(wrapper.text()).toContain('SMS')
    expect(wrapper.text()).toContain('Voice')
    expect(wrapper.text()).toContain('common.delete')
  })

  it('initializes selectedMember from getCurrentName', () => {
    const wrapper = mountComponent({ index: 2 })
    expect(hoisted.getCurrentName).toHaveBeenCalledWith(2)
  })

  it('initializes selectNotificationType from props', () => {
    const wrapper = mountComponent({ selectedNotificationType: ['EMAIL'] })
    const state = getState(wrapper)
    expect(state.selectNotificationType).toEqual(['EMAIL'])
  })

  it('calls handleDeleteMember when handleDelete is called', () => {
    const wrapper = mountComponent({ index: 3 })
    const state = getState(wrapper)
    state.handleDelete(3)
    expect(hoisted.handleDeleteMember).toHaveBeenCalledWith(3)
  })

  it('calls handleUpdateMember when handleUpdate is called', () => {
    const wrapper = mountComponent({ index: 1, selectedNotificationType: ['SMS'] })
    const state = getState(wrapper)
    state.selectedMember = 'testUser'
    state.handleUpdate()
    expect(hoisted.handleUpdateMember).toHaveBeenCalledWith(1, { name: 'testUser', notificationType: ['SMS'] })
  })

  it('calls handleUpdateMember when handleChange is triggered', () => {
    hoisted.getCurrentName.mockReturnValue('changeUser')
    const wrapper = mountComponent({ index: 0, selectedNotificationType: ['email'] })
    const state = getState(wrapper)
    state.handleChange()
    expect(hoisted.handleUpdateMember).toHaveBeenCalledWith(0, {
      name: 'changeUser',
      notificationType: ['email'],
    })
  })

  it('has formModel with default values', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    expect(state.formModel.name).toBe('')
    expect(state.formModel.signMode).toBeNull()
  })

  it('has form rules defined', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    expect(state.rules.name).toEqual({ required: true, message: 'generate.ruleName' })
    expect(state.rules.signMode).toEqual({ required: true, message: 'generate.signatureMethod' })
    expect(state.rules.ip).toEqual({ required: true, message: 'generate.IPwhitelist' })
  })
})
