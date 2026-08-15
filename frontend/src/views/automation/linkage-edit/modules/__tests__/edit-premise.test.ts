/**
 * 文件用途: 覆盖Edit Premise在自动化场景下的前端行为与契约。
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
  deviceMetricsConditionMenu: vi.fn(),
  configMetricsConditionMenu: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceGroupTree: hoisted.deviceGroupTree
}))

vi.mock('@/service/api/automation', () => ({
  deviceListAll: hoisted.deviceListAll,
  deviceConfigAll: hoisted.deviceConfigAll,
  deviceMetricsConditionMenu: hoisted.deviceMetricsConditionMenu,
  configMetricsConditionMenu: hoisted.configMetricsConditionMenu
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

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { __v_isRef: true, value: 'zh-CN' } }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh-CN' } } })
}))

vi.mock('seemly', () => ({
  repeat: (n: number, _: any) => Array.from({ length: n }, (_, i) => i + 1)
}))

import EditPremise from '../edit-premise.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(EditPremise, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ name: 'NCard', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFlex: defineComponent({ name: 'NFlex', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ name: 'NButton', emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        Button: defineComponent({ name: 'NButton', emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NInput: defineComponent({ name: 'NInput', props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({
          name: 'NSelect',
          props: { value: { default: null }, options: { default: () => [] } },
          emits: ['update:value', 'update:show', 'search'],
          setup(props, { emit }) {
            return () => h('div', {
              onClick: () => {
                const nextValue = Array.isArray(props.options) && props.options.length > 0
                  ? (props.options[0] as any).value
                  : props.value
                emit('update:value', nextValue)
              }
            })
          }
        }),
        NForm: defineComponent({ name: 'NForm', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ name: 'NFormItem', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTag: defineComponent({ name: 'NTag', setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NTooltip: defineComponent({ name: 'NTooltip', setup(_, { slots }) { return () => h('div', slots.default?.() || slots.trigger?.()) } }),
        'n-tooltip': defineComponent({ name: 'n-tooltip', setup(_, { slots }) { return () => h('div', slots.default?.() || slots.trigger?.()) } }),
        NCascader: defineComponent({ name: 'NCascader', setup() { return () => h('div') } }),
        NCheckbox: defineComponent({ name: 'NCheckbox', props: { value: { default: null }, label: { default: '' } }, setup(props) { return () => h('label', String(props.label)) } }),
        'n-checkbox': defineComponent({ name: 'n-checkbox', props: { value: { default: null }, label: { default: '' } }, setup(props) { return () => h('label', String(props.label)) } }),
        NCheckboxGroup: defineComponent({ name: 'NCheckboxGroup', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpace: defineComponent({ name: 'NSpace', setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTimePicker: defineComponent({ name: 'NTimePicker', setup() { return () => h('div') } }),
        NDatePicker: defineComponent({ name: 'NDatePicker', setup() { return () => h('div') } }),
        NIcon: defineComponent({ name: 'NIcon', setup(_, { slots }) { return () => h('span', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const findButtonByText = (wrapper: ReturnType<typeof shallowMount>, text: string) => {
  const button = wrapper.findAll('button').find(item => item.text() === text)
  if (!button) throw new Error(`button ${text} should exist`)
  return button!
}

describe('EditPremise', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceGroupTree.mockResolvedValue({ data: [] })
    hoisted.deviceListAll.mockResolvedValue({ data: [] })
    hoisted.deviceConfigAll.mockResolvedValue({ data: [] })
    hoisted.deviceMetricsConditionMenu.mockResolvedValue({ data: [] })
    hoisted.configMetricsConditionMenu.mockResolvedValue({ data: [] })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and initialize with one ifGroup', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups).toHaveLength(1)
  })

  it('does not load all device catalogs on a plain create mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceGroupTree).not.toHaveBeenCalled()
    expect(hoisted.deviceListAll).not.toHaveBeenCalled()
    expect(hoisted.deviceConfigAll).not.toHaveBeenCalled()
  })

  it('loads device catalog when create flow is opened for a known device', async () => {
    mountComponent({ device_id: 'device-1' })
    await flushPromises()
    expect(hoisted.deviceGroupTree).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceGroupTree).toHaveBeenCalledWith({})
    expect(hoisted.deviceListAll).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceListAll).toHaveBeenCalledWith({
      group_id: null,
      device_name: null,
      bind_config: 0
    })
    expect(hoisted.deviceConfigAll).not.toHaveBeenCalled()
  })

  it('loads device-config catalog when create flow is opened for a known device config', async () => {
    mountComponent({ device_config_id: 'config-1' })
    await flushPromises()
    expect(hoisted.deviceGroupTree).not.toHaveBeenCalled()
    expect(hoisted.deviceListAll).not.toHaveBeenCalled()
    expect(hoisted.deviceConfigAll).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigAll).toHaveBeenCalledWith({
      device_config_name: ''
    })
  })

  it('should add if group item', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await findButtonByText(wrapper, 'generate.add-group').trigger('click')
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups.length).toBe(2)
  })

  it('should delete if group item', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await findButtonByText(wrapper, 'generate.add-group').trigger('click')
    await findButtonByText(wrapper, 'generate.delete-group').trigger('click')
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups.length).toBe(1)
  })

  it('should add sub item to if group', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await findButtonByText(wrapper, 'generate.add-condition').trigger('click')
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups[0].length).toBe(2)
  })

  it('should delete sub item from if group', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    await findButtonByText(wrapper, 'generate.add-condition').trigger('click')
    await findButtonByText(wrapper, 'generate.delete-condition').trigger('click')
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups[0].length).toBe(1)
  })

  it('should expose ifGroupsData and premiseFormRefReturn', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(typeof wrapper.vm.ifGroupsData).toBe('function')
    expect(typeof wrapper.vm.premiseFormRefReturn).toBe('function')
  })

  it('should return ifGroups data from ifGroupsData', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const data = wrapper.vm.ifGroupsData()
    expect(Array.isArray(data)).toBe(true)
  })

  it('should reset fields on triggerConditionsTypeChange', async () => {
    const wrapper = mountComponent({
      conditionData: [[{
        ifType: '1',
        trigger_conditions_type: '10',
        trigger_source: 'x',
        trigger_param_type: 'y',
        trigger_param: 'z',
        trigger_param_key: 'k',
        trigger_operator: '=',
        trigger_value: '10',
        minValue: '1',
        maxValue: '2',
        eventParamsRaw: null,
        eventParamOptions: [],
        eventParamConditions: []
      }]]
    })
    await flushPromises()
    await wrapper.findAllComponents({ name: 'NSelect' })[1].vm.$emit('update:value', '10')
    const state = getState(wrapper)
    const ifItem = state.premiseForm.ifGroups[0][0]
    expect(ifItem.trigger_source).toBeNull()
    expect(ifItem.trigger_param_type).toBeNull()
    expect(state.deviceConfigDisabled).toBe(false)
    expect(wrapper.emitted('conditionChose')?.at(-1)).toEqual(['10'])
  })

  it('should set deviceConfigDisabled on triggerConditionsTypeChange with 11', async () => {
    const wrapper = mountComponent({
      conditionData: [[{
        ifType: '1',
        trigger_conditions_type: '10',
        trigger_source: 'x',
        trigger_param_type: 'y',
        trigger_param: 'z',
        trigger_param_key: 'k',
        trigger_operator: '=',
        trigger_value: '10',
        minValue: '1',
        maxValue: '2',
        eventParamsRaw: null,
        eventParamOptions: [],
        eventParamConditions: []
      }]]
    })
    await flushPromises()
    await wrapper.findAllComponents({ name: 'NSelect' })[1].vm.$emit('update:value', '11')
    const state = getState(wrapper)
    expect(state.deviceConfigDisabled).toBe(true)
    expect(wrapper.emitted('conditionChose')?.at(-1)).toEqual(['11'])
  })

  it('should reset fields on triggerSourceChange', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const ifItem = { trigger_param_type: 'y', trigger_param: 'z', trigger_param_key: 'k', trigger_operator: '=', trigger_value: '10', minValue: '1', maxValue: '2', eventParamsRaw: null, eventParamOptions: [], eventParamConditions: [] }
    state.triggerSourceChange(ifItem, 0)
    expect(ifItem.trigger_param_type).toBeNull()
    expect(ifItem.trigger_param).toBeNull()
    expect(ifItem.trigger_operator).toBeNull()
    expect(ifItem.eventParamConditions).toEqual([])
  })

  it('should have correct deviceConditionOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.deviceConditionOptions).toHaveLength(2)
    expect(state.deviceConditionOptions[0].value).toBe('10')
    expect(state.deviceConditionOptions[1].value).toBe('11')
  })

  it('should have correct cycleOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.cycleOptions).toHaveLength(4)
  })

  it('should have correct weekOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.weekOptions).toHaveLength(7)
  })

  it('should have correct determineOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.determineOptions).toHaveLength(8)
  })

  it('should have correct expirationTimeOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.expirationTimeOptions).toHaveLength(5)
  })

  it('should add event param condition through the event-param button', async () => {
    const wrapper = mountComponent({
      conditionData: [[{
        ifType: '1',
        trigger_conditions_type: '10',
        trigger_source: 'event-source',
        trigger_param_type: 'event',
        trigger_param: 'alarm',
        trigger_param_key: 'event/alarm',
        trigger_operator: '=',
        trigger_value: null,
        minValue: null,
        maxValue: null,
        eventParamsRaw: [{ data_identifier: 'level', data_name: 'Level', data_type: 'Number' }],
        eventParamOptions: [{ label: 'Level', value: 'level', dataType: 'Number' }],
        eventParamConditions: []
      }]]
    })
    await flushPromises()
    const eventEditor = wrapper.findComponent({ name: 'PremiseEventParamConditionEditor' })
    await eventEditor.vm.$emit('addCondition')
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups[0][0].eventParamConditions).toHaveLength(1)
  })

  it('should delete event param condition through the event-param button', async () => {
    const wrapper = mountComponent({
      conditionData: [[{
        ifType: '1',
        trigger_conditions_type: '10',
        trigger_source: 'event-source',
        trigger_param_type: 'event',
        trigger_param: 'alarm',
        trigger_param_key: 'event/alarm',
        trigger_operator: '=',
        trigger_value: null,
        minValue: null,
        maxValue: null,
        eventParamsRaw: [{ data_identifier: 'level', data_name: 'Level', data_type: 'Number' }],
        eventParamOptions: [{ label: 'Level', value: 'level', dataType: 'Number' }],
        eventParamConditions: [{ field: 'level', operator: '=', value: 1, minValue: null, maxValue: null }]
      }]]
    })
    await flushPromises()
    const eventEditor = wrapper.findComponent({ name: 'PremiseEventParamConditionEditor' })
    await eventEditor.vm.$emit('deleteCondition', 0)
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups[0][0].eventParamConditions).toHaveLength(0)
  })

  it('should handle triggerParamChange with valid data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const ifItem = { trigger_param_type: null, trigger_param: null, trigger_operator: null, trigger_value: null, eventParamsRaw: null, eventParamOptions: [], eventParamConditions: [] }
    state.triggerParamChange(ifItem, [{ value: 'telemetry' }, { key: 'temp', params: null }])
    expect(ifItem.trigger_param_type).toBe('telemetry')
    expect(ifItem.trigger_param).toBe('temp')
  })

  it('should handle triggerParamChange with invalid data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const ifItem = { trigger_param_type: 'old', trigger_param: 'old', trigger_operator: '=', trigger_value: '10', eventParamsRaw: null, eventParamOptions: [], eventParamConditions: [] }
    state.triggerParamChange(ifItem, [])
    expect(ifItem.trigger_param_type).toBeNull()
  })

  it('should reset event condition value fields through the operator select', async () => {
    const wrapper = mountComponent({
      conditionData: [[{
        ifType: '1',
        trigger_conditions_type: '10',
        trigger_source: 'event-source',
        trigger_param_type: 'event',
        trigger_param: 'alarm',
        trigger_param_key: 'event/alarm',
        trigger_operator: '=',
        trigger_value: null,
        minValue: null,
        maxValue: null,
        eventParamsRaw: [{ data_identifier: 'level', data_name: 'Level', data_type: 'Number' }],
        eventParamOptions: [{ label: 'Level', value: 'level', dataType: 'Number' }],
        eventParamConditions: [{ field: 'level', operator: 'exists', value: 'old', minValue: '5', maxValue: '10' }]
      }]]
    })
    await flushPromises()
    const state = getState(wrapper)
    const condition = state.premiseForm.ifGroups[0][0].eventParamConditions[0]
    const eventEditor = wrapper.findComponent({ name: 'PremiseEventParamConditionEditor' })
    condition.operator = 'exists'
    await eventEditor.vm.$emit('operatorChange', condition)
    await flushPromises()
    expect(condition.value).toBe(true)
    expect(condition.minValue).toBeNull()
    expect(condition.maxValue).toBeNull()
  })

  it('should initialize with device_id prop', async () => {
    const wrapper = mountComponent({ device_id: 'dev1' })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups[0][0].ifType).toBe('1')
    expect(state.premiseForm.ifGroups[0][0].trigger_conditions_type).toBe('10')
    expect(state.premiseForm.ifGroups[0][0].trigger_source).toBe('dev1')
    expect(wrapper.emitted('conditionChose')?.[0]).toEqual(['10'])
  })

  it('should initialize with device_config_id prop', async () => {
    const wrapper = mountComponent({ device_config_id: 'cfg1' })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups[0][0].ifType).toBe('1')
    expect(state.premiseForm.ifGroups[0][0].trigger_conditions_type).toBe('11')
    expect(state.premiseForm.ifGroups[0][0].trigger_source).toBe('cfg1')
    expect(wrapper.emitted('conditionChose')?.[0]).toEqual(['11'])
  })

  it('should apply conditionData updates through props watch', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const conditionData = [[{
      ifType: '1',
      trigger_conditions_type: '11',
      trigger_source: 'cfg1',
      trigger_param_type: 'status',
      trigger_param: 'online',
      trigger_param_key: 'status:online',
      trigger_operator: '=',
      trigger_value: '1',
      minValue: null,
      maxValue: null,
      eventParamsRaw: null,
      eventParamOptions: [],
      eventParamConditions: []
    }]]
    await wrapper.setProps({ conditionData })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseForm.ifGroups).toMatchObject([[{
      ...conditionData[0][0],
      trigger_param_key: 'status/online',
      eventParamConditions: [],
      eventParamOptions: []
    }]])
  })

  it('should have correct premiseFormRules', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.premiseFormRules.ifType.required).toBe(true)
    expect(state.premiseFormRules.trigger_conditions_type.required).toBe(true)
    expect(state.premiseFormRules.trigger_source.required).toBe(true)
  })
})
