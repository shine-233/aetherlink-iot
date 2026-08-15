/**
 * 文件用途: 覆盖测试在移动端设备详情场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceDetail: vi.fn(),
  deviceTemplateDetail: vi.fn(),
  telemetryDataCurrent: vi.fn(),
  getAttributeDataSet: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceDetail: hoisted.deviceDetail,
  deviceTemplateDetail: hoisted.deviceTemplateDetail,
  telemetryDataCurrent: hoisted.telemetryDataCurrent,
  getAttributeDataSet: hoisted.getAttributeDataSet
}))

vi.mock('@/service/api', () => ({
  telemetryApi: vi.fn().mockResolvedValue({ data: [] }),
  attributesApi: vi.fn().mockResolvedValue({ data: [] }),
  eventsApi: vi.fn().mockResolvedValue({ data: [] }),
  commandsApi: vi.fn().mockResolvedValue({ data: [] })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key,
  setLocale: vi.fn()
}))

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: vi.fn(() => '2024-01-01 12:00:00')
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: vi.fn(() => 'test-token'),
    set: vi.fn()
  }
}))

vi.mock('@/utils/thingsvis/platform-fields', () => ({
  extractPlatformFields: vi.fn(() => [])
}))

vi.mock('@/components/thingsvis/ThingsVisWidget.vue', () => ({
  default: defineComponent({ props: ['mode', 'config', 'data', 'platformFields', 'platformDevices', 'height', 'bufferSize', 'deviceId'], emits: ['ready'], setup() { return () => h('div') } })
}))

vi.mock('@/hooks/thingsvis/useRealtimePush', () => ({
  useRealtimePush: vi.fn(() => ({ start: vi.fn(), stop: vi.fn() }))
}))

vi.mock('@/hooks/thingsvis/useAlarmPush', () => ({
  useAlarmPush: vi.fn(() => ({ start: vi.fn(), stop: vi.fn() }))
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { d_id: 'device-1', token: 'test-token', lang: 'zh-CN' } }),
  useRouter: () => ({ push: hoisted.routerPush })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        SvgIcon: defineComponent({
          props: ['localIcon', 'stroke'],
          setup(stubProps) {
            return () => h('span', { 'data-test': 'svg-icon', 'data-local-icon': stubProps.localIcon as string, 'data-stroke': stubProps.stroke as string })
          }
        }),
        NDivider: defineComponent({ setup() { return () => h('hr') } }),
        TelemetryDataCards: defineComponent({
          props: ['id', 'cardHeight', 'cardMargin'],
          setup(stubProps) {
            return () =>
              h('div', {
                'data-test': 'telemetry-data-cards',
                'data-id': stubProps.id as string,
                'data-card-height': String(stubProps.cardHeight),
                'data-card-margin': String(stubProps.cardMargin)
              })
          }
        }),
        ThingsVisWidget: defineComponent({
          props: ['mode', 'config', 'data', 'platformFields', 'platformDevices', 'height', 'bufferSize', 'deviceId'],
          emits: ['ready'],
          setup(stubProps) {
            return () =>
              h('div', {
                'data-test': 'thingsvis-widget',
                'data-mode': stubProps.mode as string,
                'data-height': stubProps.height as string,
                'data-device-id': stubProps.deviceId as string
              })
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device-details-app/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceDetail.mockResolvedValue({
      data: {
        name: 'Test Device',
        device_number: 'DN001',
        is_online: 1,
        ts: Date.now(),
        device_config: { device_template_id: 'tpl-1' }
      },
      error: null
    })
    hoisted.deviceTemplateDetail.mockResolvedValue({
      data: { app_chart_config: null }
    })
    hoisted.telemetryDataCurrent.mockResolvedValue({ data: [] })
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [] })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders the mobile device detail fallback contract after loading device data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(wrapper.classes()).toEqual(expect.arrayContaining(['device-details-app', 'bg-gray-50']))
    expect(wrapper.text()).toContain('Test Device')
    expect(wrapper.text()).toContain('custom.device_details.online')
    expect(wrapper.text()).toContain('custom.device_details.lastUpdate')
    expect(wrapper.get('[data-test="svg-icon"]').attributes()).toMatchObject({
      'data-local-icon': 'CellTowerRound',
      'data-stroke': 'rgb(2,153,52)'
    })
    expect(wrapper.get('[data-test="telemetry-data-cards"]').attributes()).toMatchObject({
      'data-id': 'device-1',
      'data-card-height': '160',
      'data-card-margin': '15'
    })
    expect(state.showDefaultCards).toBe(true)
    expect(state.showAppChart).toBe(false)
  })

  it('calls deviceDetail on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceDetail).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceDetail).toHaveBeenCalledWith('device-1')
  })

  it('sets deviceData on successful fetch', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.deviceData.name).toBe('Test Device')
  })

  it('sets showDefaultCards when no app_chart_config', async () => {
    hoisted.deviceTemplateDetail.mockResolvedValue({
      data: { app_chart_config: null }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.showDefaultCards).toBe(true)
  })

  it('sets icon_type based on online status', async () => {
    hoisted.deviceDetail.mockResolvedValue({
      data: { name: 'Test', device_number: 'DN001', is_online: 1, ts: Date.now(), device_config: {} },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.icon_type).toBe('rgb(2,153,52)')
  })

  it('sets icon_type for offline device', async () => {
    hoisted.deviceDetail.mockResolvedValue({
      data: { name: 'Test', device_number: 'DN001', is_online: 0, ts: Date.now(), device_config: {} },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.icon_type).toBe('#ccc')
  })

  it('extractResponseList handles array data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.extractResponseList({ data: [{ id: 1 }] })
    expect(result).toEqual([{ id: 1 }])
  })

  it('extractResponseList handles list data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.extractResponseList({ data: { list: [{ id: 1 }] } })
    expect(result).toEqual([{ id: 1 }])
  })

  it('extractResponseList handles empty data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.extractResponseList(null)
    expect(result).toEqual([])
  })

  it('viewerHeight returns default when no config', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.viewerHeight).toBe('400px')
  })

  it('calls setLocale when lang is provided', async () => {
    mountComponent()
    await flushPromises()
    const { setLocale } = await import('@/locales')
    expect(setLocale).toHaveBeenCalledTimes(1)
    expect(setLocale).toHaveBeenCalledWith('zh-CN')
  })
})
