/**
 * 文件用途: 覆盖DataList在自动化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sceneAutomationsGet: vi.fn(),
  sceneAutomationsSwitch: vi.fn(),
  sceneAutomationsDel: vi.fn(),
  sceneAutomationsLog: vi.fn(),
  deviceAlarmList: vi.fn(),
  routerPushByKey: vi.fn()
}))

vi.mock('@/service/api/automation', () => ({
  sceneAutomationsGet: hoisted.sceneAutomationsGet,
  sceneAutomationsSwitch: hoisted.sceneAutomationsSwitch,
  sceneAutomationsDel: hoisted.sceneAutomationsDel,
  sceneAutomationsLog: hoisted.sceneAutomationsLog
}))

vi.mock('@/service/api', () => ({
  deviceAlarmList: hoisted.deviceAlarmList
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
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
    valueOf: () => v || 1000
  })
  return { default: fn }
})

vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    getCurrentInstance: () => ({ proxy: { getPlatform: () => false } })
  }
})

import DataList from '../dataList.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(DataList, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NPagination: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGridItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSwitch: defineComponent({ props: { value: Boolean }, emits: ['update:value'], setup() { return () => h('div') } }),
        NTooltip: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NIcon: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        ItemCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NEmpty: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTable: defineComponent({ setup(_, { slots }) { return () => h('table', slots.default?.()) } }),
        NDatePicker: defineComponent({ setup() { return () => h('div') } }),
        NEllipsis: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('DataList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.sceneAutomationsGet.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.sceneAutomationsSwitch.mockResolvedValue({ error: null })
    hoisted.sceneAutomationsDel.mockResolvedValue({ error: null })
    hoisted.sceneAutomationsLog.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceAlarmList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and call getData on init', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.sceneAutomationsGet).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneAutomationsGet).toHaveBeenCalledWith({
      name: '',
      page: 1,
      page_size: 12,
      device_id: '',
      device_config_id: ''
    })
    expect(getState(wrapper).sceneLinkageList).toEqual([])
  })

  it('should call deviceAlarmList when isAlarm is true', async () => {
    const wrapper = mountComponent({ isAlarm: true })
    await flushPromises()
    expect(hoisted.sceneAutomationsGet).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceAlarmList).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceAlarmList).toHaveBeenCalledWith({
      name: '',
      page: 1,
      page_size: 12,
      device_id: '',
      device_config_id: ''
    })
  })

  it('should load scene linkage list data', async () => {
    hoisted.sceneAutomationsGet.mockResolvedValue({
      data: { list: [{ id: '1', name: 'Linkage1', description: 'desc', enabled: 'Y' }], total: 1 },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.sceneLinkageList).toHaveLength(1)
    expect(state.dataTotal).toBe(1)
  })

  it('should reset page and call getData on handleQuery', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.queryData.page = 3
    state.handleQuery()
    await flushPromises()
    expect(state.queryData.page).toBe(1)
    expect(hoisted.sceneAutomationsGet).toHaveBeenCalledTimes(2)
    expect(hoisted.sceneAutomationsGet).toHaveBeenLastCalledWith({
      name: '',
      page: 1,
      page_size: 12,
      device_id: '',
      device_config_id: ''
    })
  })

  it('should call sceneAutomationsSwitch and refresh on linkActivation', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    await state.linkActivation({ id: '123' })
    await flushPromises()
    expect(hoisted.sceneAutomationsSwitch).toHaveBeenCalledWith('123')
    expect(hoisted.sceneAutomationsGet).toHaveBeenCalledTimes(2)
    expect(hoisted.sceneAutomationsGet).toHaveBeenLastCalledWith({
      name: '',
      page: 1,
      page_size: 12,
      device_id: '',
      device_config_id: ''
    })
  })

  it('should open log modal on openLog', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.openLog({ id: 'log-id' })
    await flushPromises()
    expect(state.showLog).toBe(true)
    expect(state.logQuery.scene_automation_id).toBe('log-id')
  })

  it('should reset logQuery and close modal on closeLog', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.showLog = true
    state.logQuery.scene_automation_id = 'some-id'
    state.closeLog()
    expect(state.showLog).toBe(false)
    expect(state.logQuery.scene_automation_id).toBe('')
  })

  it('should reset page and call getLogList on queryLog', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.logQuery.page = 5
    state.queryLog()
    await flushPromises()
    expect(state.logQuery.page).toBe(1)
    expect(hoisted.sceneAutomationsLog).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneAutomationsLog).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        execution_start_time: '2024-01-01T00:00:00',
        execution_end_time: '2024-01-01T00:00:00'
      })
    )
  })

  it('should pass device_id and device_config_id to queryData', async () => {
    const wrapper = mountComponent({ deviceId: 'dev1', deviceConfigId: 'cfg1' })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.queryData.device_id).toBe('dev1')
    expect(state.queryData.device_config_id).toBe('cfg1')
  })

  it('should route device-scoped add flow into the first telemetry rule starter', async () => {
    const wrapper = mountComponent({ deviceId: 'dev1', deviceConfigId: 'cfg1', backType: 'device' })
    await flushPromises()
    const state = getState(wrapper)

    expect(wrapper.text()).toContain('custom.automation.firstTelemetryRuleEmptyTitle')
    expect(wrapper.text()).toContain('custom.automation.firstTelemetryRuleEmptyDesc')
    expect(wrapper.text()).toContain('custom.automation.createFirstTelemetryRule')

    state.linkAdd()

    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_linkage-edit', {
      query: {
        device_id: 'dev1',
        device_config_id: 'cfg1',
        backType: 'device',
        onboarding: 'first-device',
        starter: 'first-telemetry-rule'
      }
    })
  })

  it('should preserve homepage onboarding context when creating the first automation from an empty list', async () => {
    const wrapper = mountComponent({
      onboarding: 'first-device',
      starter: 'first-telemetry-rule',
      deviceId: 'dev1',
      deviceConfigId: 'cfg1',
      firstDeviceName: 'Pump Controller',
      firstDeviceNumber: 'D-001',
      telemetryKey: 'temperature',
      telemetryValue: '23',
      telemetryAt: '2026-07-06T12:00:00Z'
    })
    await flushPromises()
    const state = getState(wrapper)

    expect(wrapper.text()).toContain('custom.automation.firstTelemetryRuleEmptyTitle')
    state.linkAdd()

    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_linkage-edit', {
      query: {
        device_id: 'dev1',
        device_config_id: 'cfg1',
        backType: 'automation',
        onboarding: 'first-device',
        starter: 'first-telemetry-rule',
        first_device_name: 'Pump Controller',
        first_device_number: 'D-001',
        telemetry_key: 'temperature',
        telemetry_value: '23',
        telemetry_at: '2026-07-06T12:00:00Z'
      }
    })
  })

  it('should keep alarm empty state out of the first telemetry rule starter', async () => {
    const wrapper = mountComponent({ isAlarm: true, deviceId: 'dev1', backType: 'device' })
    await flushPromises()

    expect(wrapper.text()).not.toContain('custom.automation.firstTelemetryRuleEmptyTitle')
    expect(wrapper.text()).not.toContain('custom.automation.createFirstTelemetryRule')
    expect(getState(wrapper).isDeviceAutomationStarter).toBe(false)
  })

  it('should have correct execution_result_options', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.execution_result_options).toHaveLength(3)
  })
})
