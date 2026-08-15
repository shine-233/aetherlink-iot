/**
 * 文件用途: 覆盖测试在自动化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sceneAutomationsAdd: vi.fn(),
  sceneAutomationsEdit: vi.fn(),
  sceneAutomationsInfo: vi.fn(),
  sceneAutomationsDryRun: vi.fn(),
  route: {
    query: {} as Record<string, unknown>,
    path: '/automation/linkage-edit'
  }
}))

vi.mock('@/service/api/automation', () => ({
  sceneAutomationsAdd: hoisted.sceneAutomationsAdd,
  sceneAutomationsEdit: hoisted.sceneAutomationsEdit,
  sceneAutomationsInfo: hoisted.sceneAutomationsInfo,
  sceneAutomationsDryRun: hoisted.sceneAutomationsDryRun
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => hoisted.route,
    useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() })
  }
})

vi.mock('@/store/modules/tab', () => ({
  useTabStore: () => ({ removeTab: vi.fn() })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: vi.fn() }),
    useMessage: () => ({ success: vi.fn(), error: vi.fn() })
  }
})

vi.mock('dayjs', () => {
  const fn = (v?: any) => ({
    subtract: () => ({ valueOf: () => 1000 }),
    format: () => '2024-01-01T00:00:00',
    valueOf: () => {
      if (v === undefined || v === null || v === '') return 1000
      const parsed = new Date(v).getTime()
      return Number.isNaN(parsed) ? 1000 : parsed
    }
  })
  return { default: fn }
})

import LinkageEdit from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(LinkageEdit, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDivider: true,
        EditPremise: true,
        EditAction: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('LinkageEdit', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.route.query = {}
    hoisted.route.path = '/automation/linkage-edit'
    hoisted.sceneAutomationsAdd.mockResolvedValue({ error: null })
    hoisted.sceneAutomationsEdit.mockResolvedValue({ error: null })
    hoisted.sceneAutomationsInfo.mockResolvedValue({ data: null })
    // dry-run 走 { data, error } envelope，data 内是 { can_save, blockers, warnings }。
    // 默认放行，具体用例再按需覆盖成阻塞态。
    hoisted.sceneAutomationsDryRun.mockResolvedValue({ data: { can_save: true }, error: null })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount with default configForm', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.configForm.name).toBeNull()
    expect(state.configForm.enabled).toBe('Y')
    expect(state.configForm.trigger_condition_groups).toHaveLength(0)
    expect(state.configForm.actions).toHaveLength(0)
  })

  it('should have correct configFormRules', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.configFormRules.name.required).toBe(true)
    expect(state.configFormRules.trigger_condition_groups.required).toBe(true)
    expect(state.configFormRules.actions.required).toBe(true)
  })

  it('should set configId from route query', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.configId).toBe('')
  })

  it('should call getSceneAutomationsInfo when configId exists', async () => {
    hoisted.route.query = { id: 'test-id' }
    hoisted.sceneAutomationsInfo.mockResolvedValue({
      data: {
        id: 'test-id',
        name: '回显场景',
        enabled: 'Y',
        description: '回显描述',
        trigger_condition_groups: [],
        actions: []
      }
    })
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.sceneAutomationsInfo).toHaveBeenCalledWith('test-id')
    const state = getState(wrapper)
    expect(state.configId).toBe('test-id')
    expect(state.configForm.id).toBe('test-id')
    expect(state.configForm.name).toBe('回显场景')
  })

  it('should set backType from route query', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.backType).toBe('')
  })

  it('should update conditionsType on conditionChose', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.conditionChose('10')
    expect(state.conditionsType).toBe('10')
  })

  it('should not update conditionsType when data is falsy', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.conditionChose(null)
    expect(state.conditionsType).toBeNull()
  })

  it('should have defaultConfigForm returning correct structure', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.configForm).toEqual({
      id: '',
      name: null,
      description: null,
      enabled: 'Y',
      trigger_condition_groups: [],
      actions: []
    })
  })

  it('should have propsData with device_id and device_config_id', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.propsData).toEqual({
      device_id: '',
      device_config_id: ''
    })
  })

  it('prefills first telemetry rule starter guidance from route query', async () => {
    hoisted.route.query = {
      onboarding: 'first-device',
      starter: 'first-telemetry-rule',
      device_id: 'device-1',
      device_config_id: 'config-1',
      first_device_name: 'Pump',
      telemetry_key: 'temperature',
      telemetry_value: '36.5'
    }

    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.isFirstDeviceAutomationStarter).toBe(true)
    expect(state.propsData).toEqual({
      device_id: 'device-1',
      device_config_id: 'config-1'
    })
    expect(state.configForm.name).toBe('custom.automation.firstRuleName')
    expect(state.firstAutomationStarterTelemetryLabel).toBe('temperature = 36.5')
    expect(state.firstAutomationTelemetryRecommendation.hasTelemetryKey).toBe(true)
    expect(state.firstAutomationTelemetryRecommendation.cards[0]).toMatchObject({
      key: 'key',
      value: 'temperature',
      status: 'ready'
    })
    expect(state.firstAutomationTelemetryRecommendation.nextAction).toMatchObject({
      status: 'ready',
      title: 'custom.automation.firstRuleTelemetryNextActionTitle'
    })
    expect(state.firstAutomationRecommendedConditionDraft).toMatchObject({
      available: true,
      title: 'custom.automation.firstRuleRecommendedConditionTitle',
      condition: {
        trigger_conditions_type: '10',
        trigger_source: 'device-1',
        trigger_param_type: 'telemetry',
        trigger_param: 'temperature',
        trigger_param_key: 'telemetry/temperature',
        trigger_operator: '>',
        trigger_value: '36.5'
      }
    })
    expect(state.conditionData).toEqual([])
    state.applyFirstAutomationRecommendedCondition()
    await flushPromises()
    expect(state.conditionData).toEqual([[
      expect.objectContaining({
        trigger_conditions_type: '10',
        trigger_source: 'device-1',
        trigger_param_type: 'telemetry',
        trigger_param: 'temperature',
        trigger_param_key: 'telemetry/temperature',
        trigger_operator: '>',
        trigger_value: '36.5'
      })
    ]])
    expect(state.conditionsType).toBe('10')
    expect(state.firstAutomationRecommendedConditionApplied).toBe(true)
    expect(state.firstAutomationRecommendedActionDraft).toMatchObject({
      title: 'custom.automation.firstRuleRecommendedActionTitle',
      status: 'needs-target',
      action: {
        actionType: '30',
        action_type: '30',
        action_target: null
      }
    })
    expect(state.actionData).toEqual([])
    expect(state.automationDryRunQuickFixActions).toEqual([
      expect.objectContaining({
        key: 'apply-first-alarm-action',
        title: 'custom.automation.firstRuleRecommendedActionTitle',
        buttonLabel: 'custom.automation.firstRuleApplyRecommendedAction'
      })
    ])
    state.applyFirstAutomationRecommendedAction()
    await flushPromises()
    expect(state.actionData).toEqual([
      {
        actionType: '30',
        action_type: '30',
        action_target: null
      }
    ])
    expect(state.firstAutomationRecommendedActionApplied).toBe(true)
    expect(state.automationDryRunQuickFixActions).toEqual([
      expect.objectContaining({
        key: 'create-first-alarm-target',
        title: 'custom.automation.firstRuleCreateAlarmTarget',
        buttonLabel: 'custom.automation.firstRuleCreateAlarmTarget'
      })
    ])
    expect(state.firstAutomationStarterChecklist.map((item: any) => item.key)).toEqual([
      'condition',
      'action',
      'dry-run',
      'save'
    ])
    expect(state.firstAutomationStarterChecklist[0]).toMatchObject({
      key: 'condition',
      status: 'active',
      title: 'custom.automation.firstRuleChecklistConditionTitle'
    })
    expect(wrapper.text()).toContain('custom.automation.firstRuleStarterTitle')
    expect(wrapper.text()).toContain('custom.automation.firstRuleTelemetryGuideTitle')
    expect(wrapper.text()).toContain('custom.automation.firstRuleChecklistDryRunTitle')
  })

  it('does not show first telemetry starter while editing an existing automation', async () => {
    hoisted.route.query = {
      id: 'scene-1',
      onboarding: 'first-device',
      starter: 'first-telemetry-rule',
      first_device_name: 'Pump'
    }

    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.isFirstDeviceAutomationStarter).toBe(false)
    expect(wrapper.text()).not.toContain('custom.automation.firstRuleStarterTitle')
    expect(hoisted.sceneAutomationsInfo).toHaveBeenCalledWith('scene-1')
  })

  it('shows general dry-run quick fixes in the normal automation editor', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.isFirstDeviceAutomationStarter).toBe(false)
    expect(state.automationDryRunQuickFixActions.map((item: any) => item.key)).toEqual([
      'add-condition-group',
      'add-alarm-action-slot'
    ])

    state.handleAutomationDryRunQuickFix('add-condition-group')
    await flushPromises()
    expect(state.conditionData).toEqual([[{ ifType: null }]])

    state.handleAutomationDryRunQuickFix('add-alarm-action-slot')
    await flushPromises()
    expect(state.actionData).toEqual([
      {
        actionType: '30',
        action_type: '30',
        action_target: null
      }
    ])
    expect(state.automationDryRunQuickFixActions).toEqual([
      expect.objectContaining({
        key: 'create-alarm-target',
        title: 'generate.automationDryRunQuickFixCreateAlarmTargetTitle'
      })
    ])
  })

  const mountWithEchoDetail = async (detail: Record<string, unknown>) => {
    hoisted.route.query = { id: 'scene-echo' }
    hoisted.sceneAutomationsInfo.mockResolvedValue({
      data: {
        id: 'scene-echo',
        name: 'echo scene',
        enabled: 'Y',
        description: null,
        trigger_condition_groups: [],
        actions: [],
        ...detail
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    return getState(wrapper)
  }

  it('echoes device telemetry trigger conditions into editable state on detail load', async () => {
    const state = await mountWithEchoDetail({
      trigger_condition_groups: [[{
        trigger_conditions_type: '10',
        trigger_param_type: 'telemetry',
        trigger_operator: '>',
        trigger_value: '50',
        trigger_param: 'temperature',
        trigger_source: 'device1'
      }]]
    })

    expect(hoisted.sceneAutomationsInfo).toHaveBeenCalledWith('scene-echo')
    expect(state.conditionData[0][0].ifType).toBe('1')
    expect(state.conditionData[0][0].trigger_param_key).toBe('telemetry/temperature')
  })

  it('echoes one-shot schedule trigger conditions into editable state on detail load', async () => {
    const state = await mountWithEchoDetail({
      trigger_condition_groups: [[{
        trigger_conditions_type: '20',
        execution_time: '2024-01-01T10:00:00'
      }]]
    })

    expect(state.conditionData[0][0].ifType).toBe('2')
    expect(state.conditionData[0][0].onceTimeValue).toBe(
      new Date('2024-01-01T10:00:00').getTime()
    )
  })

  it('echoes weekly schedule trigger conditions into editable state on detail load', async () => {
    const state = await mountWithEchoDetail({
      trigger_condition_groups: [[{
        trigger_conditions_type: '22',
        trigger_value: '12345|08:00:00+08:00|18:00:00+08:00'
      }]]
    })

    expect(state.conditionData[0][0].ifType).toBe('2')
    expect(state.conditionData[0][0].weekChoseValue).toEqual(['1', '2', '3', '4', '5'])
  })

  it('groups device instruction actions into a single instruction group on detail load', async () => {
    const state = await mountWithEchoDetail({
      actions: [{
        action_type: '10',
        action_param_type: 'telemetry',
        action_param: 'temperature',
        action_value: '{"temperature":50}'
      }]
    })

    expect(state.actionData).toHaveLength(1)
    expect(state.actionData[0].actionType).toBe('1')
    expect(state.actionData[0].actionInstructList[0].actionValue).toBe(50)
  })

  it('keeps non-device actions as their own action rows on detail load', async () => {
    const state = await mountWithEchoDetail({
      actions: [{
        action_type: '20',
        action_target: 'scene1'
      }]
    })

    expect(state.actionData).toHaveLength(1)
    expect(state.actionData[0].actionType).toBe('20')
    expect(state.actionData[0].action_target).toBe('scene1')
  })
})
