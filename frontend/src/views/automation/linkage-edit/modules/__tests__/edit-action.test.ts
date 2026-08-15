/**
 * 文件用途: 覆盖Edit Action在自动化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceGroupTree: vi.fn(),
  deviceListAll: vi.fn(),
  deviceConfigAll: vi.fn(),
  deviceMetricsMenu: vi.fn(),
  deviceConfigMetricsMenu: vi.fn(),
  sceneGet: vi.fn(),
  warningMessageList: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceGroupTree: hoisted.deviceGroupTree
}))

vi.mock('@/service/api/automation', () => ({
  deviceListAll: hoisted.deviceListAll,
  deviceConfigAll: hoisted.deviceConfigAll,
  deviceMetricsMenu: hoisted.deviceMetricsMenu,
  deviceConfigMetricsMenu: hoisted.deviceConfigMetricsMenu,
  sceneGet: hoisted.sceneGet
}))

vi.mock('@/service/api/alarm', () => ({
  warningMessageList: hoisted.warningMessageList
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: vi.fn() }),
    useMessage: () => ({ success: vi.fn(), error: vi.fn() })
  }
})

import EditAction from '../edit-action.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(EditAction, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ name: 'NCard', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFlex: defineComponent({ name: 'NFlex', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ name: 'NButton', emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        Button: defineComponent({ name: 'NButton', emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NInput: defineComponent({ name: 'NInput', props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ name: 'NSelect', props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NForm: defineComponent({ name: 'NForm', expose: ['validate'], setup(_, { slots }) { const validate = () => Promise.resolve(); return { validate } }, render() { return h('form', this.$slots.default?.()) } }),
        NFormItem: defineComponent({ name: 'NFormItem', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDivider: true,
        NTooltip: true,
        NCheckbox: true,
        NInputNumber: true,
        NRadioGroup: true,
        NRadio: true,
        NIcon: true,
        NTag: true,
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        PopUp: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('EditAction', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceGroupTree.mockResolvedValue({ data: [] })
    hoisted.deviceListAll.mockResolvedValue({ data: [] })
    hoisted.deviceConfigAll.mockResolvedValue({ data: [] })
    hoisted.deviceMetricsMenu.mockResolvedValue({ data: [] })
    hoisted.deviceConfigMetricsMenu.mockResolvedValue({ data: [] })
    hoisted.sceneGet.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.warningMessageList.mockResolvedValue({ data: { list: [], total: 0 } })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and initialize with one action group', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.actionForm.actionGroups).toHaveLength(1)
  })

  it('should not preload target catalogs on add-mode mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.deviceGroupTree).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceListAll).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceConfigAll).toHaveBeenCalledTimes(0)
    expect(hoisted.warningMessageList).toHaveBeenCalledTimes(0)
    expect(hoisted.sceneGet).toHaveBeenCalledTimes(0)
    expect(getState(wrapper).actionForm.actionGroups).toHaveLength(1)
  })

  it('should add action group item', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.findAllComponents({ name: 'NButton' }).at(-1)!.trigger('click')
    const state = getState(wrapper)
    expect(state.actionForm.actionGroups.length).toBe(2)
  })

  it('should delete action group item', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.findAllComponents({ name: 'NButton' }).at(-1)!.trigger('click')
    const deleteButton = wrapper.findAllComponents({ name: 'NButton' }).at(-2)
    await deleteButton!.trigger('click')
    const state = getState(wrapper)
    expect(state.actionForm.actionGroups.length).toBe(1)
  })

  it('should add instruct item to action group', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    await state.addIfGroupsSubItem(0)
    expect(state.actionForm.actionGroups[0].actionInstructList.length).toBe(1)
  })

  it('should delete instruct item from action group', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    await state.addIfGroupsSubItem(0)
    await state.addIfGroupsSubItem(0)
    state.deleteIfGroupsSubItem(0, 0)
    expect(state.actionForm.actionGroups[0].actionInstructList.length).toBe(1)
  })

  it('should expose actionGroupsReturn and actionFormRefReturn', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(typeof wrapper.vm.actionGroupsReturn).toBe('function')
    expect(typeof wrapper.vm.actionFormRefReturn).toBe('function')
    expect(typeof wrapper.vm.openCreateAlarm).toBe('function')
  })

  it('should expose an alarm creation opener for the first-rule starter', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.popUpVisible).toBe(false)
    wrapper.vm.openCreateAlarm()
    await flushPromises()
    expect(state.popUpVisible).toBe(true)
  })

  it('selects the newly created alarm in the first empty alarm action target', async () => {
    const wrapper = mountComponent({
      actionData: [
        { actionType: '30', action_type: '30', action_target: null },
        { actionType: '30', action_type: '30', action_target: 'alarm-existing' }
      ]
    })
    await flushPromises()
    const state = getState(wrapper)

    await state.handleAlarmSaved({ id: 'alarm-new', name: 'Created alarm' })
    await flushPromises()

    expect(hoisted.warningMessageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: ''
    })
    expect(state.actionForm.actionGroups[0].action_target).toBe('alarm-new')
    expect(state.actionForm.actionGroups[1].action_target).toBe('alarm-existing')
    expect(state.alarmList[0]).toMatchObject({
      id: 'alarm-new',
      name: 'Created alarm'
    })
  })

  it('does not overwrite an existing alarm action target after alarm creation', async () => {
    const wrapper = mountComponent({
      actionData: [
        { actionType: '30', action_type: '30', action_target: 'alarm-existing' }
      ]
    })
    await flushPromises()
    const state = getState(wrapper)

    await state.handleAlarmSaved({ id: 'alarm-new', name: 'Created alarm' })
    await flushPromises()

    expect(state.actionForm.actionGroups[0].action_target).toBe('alarm-existing')
  })

  it('does not fill a non-alarm action target after alarm creation', async () => {
    const wrapper = mountComponent({
      actionData: [
        { actionType: '20', action_target: null },
        { actionType: '30', action_type: '30', action_target: null }
      ]
    })
    await flushPromises()
    const state = getState(wrapper)

    await state.handleAlarmSaved({ id: 'alarm-new', name: 'Created alarm' })
    await flushPromises()

    expect(state.actionForm.actionGroups[0].action_target).toBeNull()
    expect(state.actionForm.actionGroups[1].action_target).toBe('alarm-new')
  })

  it('should return actionGroups from actionGroupsReturn', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const data = wrapper.vm.actionGroupsReturn()
    expect(Array.isArray(data)).toBe(true)
  })

  it('should have correct actionOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.actionOptions).toHaveLength(3)
    expect(state.actionOptions[0].value).toBe('1')
    expect(state.actionOptions[1].value).toBe('20')
    expect(state.actionOptions[2].value).toBe('30')
  })

  it('should have correct actionTypeOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.actionTypeOptions).toHaveLength(2)
    expect(state.actionTypeOptions[0].value).toBe('10')
    expect(state.actionTypeOptions[1].value).toBe('11')
  })

  it('should reset fields on actionTypeChange with data=10', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const instruct = { action_target: 'x', action_param_type: 'y', action_param: 'z', actionValue: 'v' }
    state.actionTypeChange(instruct, '10')
    expect(instruct.action_target).toBeNull()
    await flushPromises()
    expect(hoisted.deviceGroupTree).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceListAll).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceListAll).toHaveBeenLastCalledWith({
      group_id: null,
      device_name: null,
      bind_config: 0
    })
  })

  it('should reset fields on actionTypeChange with data=11', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const instruct = { action_target: 'x', action_param_type: 'y', action_param: 'z', actionValue: 'v' }
    state.actionTypeChange(instruct, '11')
    expect(instruct.action_target).toBeNull()
    await flushPromises()
    expect(hoisted.deviceConfigAll).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigAll).toHaveBeenLastCalledWith({
      device_config_name: ''
    })
  })

  it('should reset fields on actionTargetChange', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const instruct = { action_param_type: 'y', action_param: 'z', actionValue: 'v', actionParamOptionsData: [1], actionParamTypeOptions: [2], actionParamOptions: [3] }
    state.actionTargetChange(instruct)
    expect(instruct.action_param_type).toBeNull()
    expect(instruct.actionParamOptionsData).toHaveLength(0)
  })

  it('should handle actionParamTypeChange', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.setProps({
      actionData: [{
        actionType: '1',
        actionInstructList: [{
          action_type: '10',
          action_target: 'dev1',
          action_param_type: 'legacy',
          action_param: 'old',
          actionValue: 'val',
          actionParamData: 'data',
          actionParamOptions: [],
          actionParamOptionsData: [{ data_source_type: 'telemetry', options: [] }],
          showSubSelect: false,
          placeholder: ''
        }]
      }]
    })
    await flushPromises()
    await wrapper.findAllComponents({ name: 'NSelect' })[3].vm.$emit('update:value', 'telemetry')
    const state = getState(wrapper)
    const instruct = state.actionForm.actionGroups[0].actionInstructList[0]
    expect(instruct.action_param).toBeNull()
    expect(instruct.actionParamData).toBeNull()
    expect(instruct.actionValue).toBeNull()
  })

  it('should handle actionParamChange', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.setProps({
      actionData: [{
        actionType: '1',
        actionInstructList: [{
          action_type: '10',
          action_target: 'dev1',
          action_param_type: 'telemetry',
          action_param: 'old',
          actionValue: 'old',
          actionParamData: { key: 'old', data_type: 'string' },
          actionParamOptions: [{ key: 'temp', data_type: 'String' }],
          showSubSelect: true
        }]
      }]
    })
    await flushPromises()
    const state = getState(wrapper)
    const instruct = state.actionForm.actionGroups[0].actionInstructList[0]
    const parameterSelect = wrapper
      .findAllComponents({ name: 'NSelect' })
      .find(select => select.props('value') === 'old')
    await parameterSelect!.vm.$emit('update:value', 'temp')
    await flushPromises()
    expect(instruct.actionValue).toBeNull()
    expect(instruct.actionParamData).toEqual({ key: 'temp', data_type: 'string' })
  })

  it('should validate JSON on actionValueChange for command type with valid JSON', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const instruct = { action_param_type: 'command', actionValue: '{"key":"val"}', inputFeedback: '', inputValidationStatus: undefined }
    state.actionValueChange(instruct)
    expect(instruct.inputValidationStatus).toBeUndefined()
  })

  it('should set error on actionValueChange for invalid JSON', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const instruct = { action_param_type: 'command', actionValue: 'not-json', inputFeedback: '', inputValidationStatus: undefined }
    state.actionValueChange(instruct)
    expect(instruct.inputValidationStatus).toBe('error')
  })

  it('should have correct configFormRules', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.configFormRules.actionType.required).toBe(true)
    expect(state.configFormRules.action_type.required).toBe(true)
    expect(state.configFormRules.action_target.required).toBe(true)
  })

  it('should set action_type to 11 when conditionsType is 11 on addIfGroupsSubItem', async () => {
    const wrapper = mountComponent({ conditionsType: '11' })
    await flushPromises()
    const state = getState(wrapper)
    await state.addIfGroupsSubItem(0)
    expect(state.actionForm.actionGroups[0].actionInstructList[0].action_type).toBe('11')
  })

  it('should handle resetActionData when conditionsType changes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.setProps({
      actionData: [{
        actionType: '1',
        actionInstructList: [{ action_type: '10', action_target: 'dev1' }]
      }]
    })
    await flushPromises()
    await wrapper.setProps({ conditionsType: '11' })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.actionForm.actionGroups[0].actionInstructList[0].action_type).toBe('11')
  })

  it('should handle applyActionData with non-array', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.setProps({ actionData: null })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.actionForm.actionGroups).toHaveLength(1)
  })

  it('should handle applyActionData with valid array', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const actionData = [{ actionType: '1', actionInstructList: [{ action_type: '10', action_target: 'dev1', actionParamOptions: [] }] }]
    await wrapper.setProps({ actionData })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.actionForm.actionGroups).toHaveLength(1)
    expect(state.actionForm.actionGroups).toMatchObject(actionData)
  })

  it('hydrates each target catalog once for edit echo data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await wrapper.setProps({
      actionData: [
        {
          actionType: '1',
          actionInstructList: [
            { action_type: '10', action_target: 'dev1', actionParamOptions: [] },
            { action_type: '10', action_target: 'dev2', actionParamOptions: [] },
            { action_type: '11', action_target: 'profile1', actionParamOptions: [] }
          ]
        },
        { actionType: '20', action_target: 'scene1' },
        { actionType: '30', action_target: 'alarm1' }
      ]
    })
    await flushPromises()

    expect(hoisted.deviceGroupTree).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceListAll).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigAll).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneGet).toHaveBeenCalledTimes(1)
    expect(hoisted.warningMessageList).toHaveBeenCalledTimes(1)
  })
})
