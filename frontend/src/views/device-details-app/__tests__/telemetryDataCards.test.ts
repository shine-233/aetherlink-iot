/**
 * 文件用途: 覆盖TelemetryDataCards在移动端设备详情场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  telemetryDataCurrent: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  telemetryDataCurrent: hoisted.telemetryDataCurrent
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({ info: vi.fn(), error: vi.fn(), warn: vi.fn() })
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 12:00:00') }))
}))

vi.mock('@vicons/ionicons5', () => ({
  TrendingUpOutline: defineComponent({ setup: () => () => h('span') }),
  DocumentTextOutline: defineComponent({ setup: () => () => h('span') })
}))

vi.mock('../../device/details/modules/telemetry/modules/history-data.vue', () => ({
  default: defineComponent({ props: ['deviceId', 'theKey'], setup() { return () => h('div') } })
}))

vi.mock('../../device/details/modules/telemetry/modules/time-series-data.vue', () => ({
  default: defineComponent({ props: ['deviceId', 'theKey', 'theName', 'theUnit'], setup() { return () => h('div') } })
}))

import Component from '../telemetryDataCards.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      id: 'device-1',
      cardHeight: 160,
      cardMargin: 15,
      ...props
    },
    global: {
      stubs: {
        NGrid: defineComponent({
          props: ['xGap', 'yGap', 'cols'],
          setup(stubProps, { slots }) {
            return () =>
              h('div', { 'data-test': 'telemetry-grid', 'data-cols': stubProps.cols as string }, slots.default?.())
          }
        }),
        NGi: defineComponent({ setup(_, { slots }) { return () => h('div', { 'data-test': 'telemetry-grid-item' }, slots.default?.()) } }),
        NCard: defineComponent({
          props: ['hoverable', 'headerClass'],
          setup(stubProps, { slots }) {
            return () =>
              h('article', { 'data-test': 'telemetry-card', 'data-hoverable': String(stubProps.hoverable), 'data-header-class': stubProps.headerClass as string }, [
                slots.header?.(),
                slots['header-extra']?.(),
                slots.default?.(),
                slots.footer?.()
              ])
          }
        }),
        NIcon: defineComponent({ props: ['size', 'color'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NDivider: defineComponent({ props: ['vertical'], setup() { return () => h('hr') } }),
        NModal: defineComponent({
          props: ['show', 'title'],
          emits: ['update:show'],
          setup(stubProps, { slots }) {
            return () =>
              h('div', { 'data-test': 'history-modal', 'data-show': String(stubProps.show), 'data-title': stubProps.title as string }, slots.default?.())
          }
        }),
        AnimatedNumber: defineComponent({
          inheritAttrs: false,
          props: ['mNum', 'quantileShow'],
          setup(stubProps, { attrs }) {
            return () =>
              h('span', {
                ...attrs,
                'data-test': 'animated-number',
                'data-value': String(stubProps.mNum),
                'data-quantile': String(stubProps.quantileShow)
              })
          }
        }),
        HistoryData: defineComponent({ props: ['deviceId', 'theKey'], setup() { return () => h('div') } }),
        TimeSeriesData: defineComponent({ props: ['deviceId', 'theKey', 'theName', 'theUnit'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device-details-app/telemetryDataCards.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.telemetryDataCurrent.mockResolvedValue({
      data: [
        { key: 'temperature', value: 25.5, unit: '°C', ts: Date.now(), device_id: 'device-1', label: 'Temperature' },
        { key: 'humidity', value: 60, unit: '%', ts: Date.now(), device_id: 'device-1', label: 'Humidity' }
      ],
      error: null
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders telemetry card data with chart and history affordances', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const cards = wrapper
      .findAll('[data-test="telemetry-card"]')
      .filter(card => card.text().includes('Temperature') || card.text().includes('Humidity'))

    expect(wrapper.get('[data-test="telemetry-grid"]').attributes('data-cols')).toBe('1 600:2 900:3 1200:4')
    expect(cards).toHaveLength(2)
    expect(wrapper.text()).toContain('Temperature')
    expect(wrapper.text()).toContain('(temperature)')
    expect(wrapper.text()).toContain('°C')
    expect(wrapper.text()).toContain('Humidity')
    expect(wrapper.text()).toContain('(humidity)')
    expect(wrapper.text()).toContain('%')
    expect(state.telemetryData.map((item: any) => String(item.value))).toEqual(['25.5', '60'])
    expect(wrapper.findAll('[data-test="animated-number"]').map(number => number.attributes())).toEqual([
      expect.objectContaining({ 'data-index': '0', 'data-value': '25.5', 'data-quantile': 'true' }),
      expect.objectContaining({ 'data-index': '1', 'data-value': '60', 'data-quantile': 'true' })
    ])
    expect(wrapper.get('[data-test="history-modal"]').attributes()).toMatchObject({
      'data-show': 'false',
      'data-title': 'generate.telemetry-history-data'
    })
    expect(state.telemetryData).toHaveLength(2)
    expect(state.telemetryData[0]).toMatchObject({ key: 'temperature', device_id: 'device-1' })
  })

  it('fetches telemetry data on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
  })

  it('populates telemetryData on successful fetch', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.telemetryData).toHaveLength(2)
  })

  it('sets initTelemetryData from first item', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.telemetryData[0].key).toBe('temperature')
    expect(state.telemetryData[0].device_id).toBe('device-1')
  })

  it('isColor returns #cccccc for string values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.isColor({ value: 'online' })).toBe('#cccccc')
  })

  it('isColor returns empty string for numeric values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.isColor({ value: 25.5 })).toBe('')
  })

  it('onTapTableTools sets history state for numeric values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.onTapTableTools({ value: 25.5, key: 'temp', device_id: 'dev-1', label: 'Temperature', unit: '°C' })
    expect(state.showHistory).toBe(true)
    expect(state.telemetryKey).toBe('temp')
    expect(state.telemetryId).toBe('dev-1')
    expect(state.modelType).toBe('custom.device_details.sequential')
  })

  it('onTapTableTools does nothing for string values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.onTapTableTools({ value: 'online', key: 'status' })
    expect(state.showHistory).toBe(false)
  })

  it('accepts cardHeight and cardMargin props', async () => {
    const wrapper = mountComponent({ cardHeight: 200, cardMargin: 20 })
    await flushPromises()
    expect(wrapper.props('cardHeight')).toBe(200)
    expect(wrapper.props('cardMargin')).toBe(20)
  })

  it('handles empty telemetry data', async () => {
    hoisted.telemetryDataCurrent.mockResolvedValue({ data: [], error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.telemetryData).toHaveLength(0)
  })

  it('handles API error gracefully', async () => {
    hoisted.telemetryDataCurrent.mockResolvedValue({ data: null, error: true })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.telemetryData).toHaveLength(0)
  })
})
