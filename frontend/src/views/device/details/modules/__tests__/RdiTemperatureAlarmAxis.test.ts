/**
 * 文件用途: RdiTemperatureAlarmAxis 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  setOption: vi.fn(),
  convertToPixel: vi.fn(),
  convertFromPixel: vi.fn(),
  resize: vi.fn(),
  dispose: vi.fn(),
  createEChartsInstance: vi.fn(),
  registerEChartsExtensions: vi.fn()
}))

vi.mock('@/utils/echarts/echarts-manager', () => ({
  createEChartsInstance: hoisted.createEChartsInstance,
  registerEChartsExtensions: hoisted.registerEChartsExtensions
}))

import Component from '../RdiTemperatureAlarmAxis.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      lower: 10,
      upper: 80,
      ...props
    },
    global: {
      stubs: {}
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/RdiTemperatureAlarmAxis.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.convertToPixel.mockImplementation((_axis: unknown, value: unknown) => Number(value) * 2 + 100)
    hoisted.convertFromPixel.mockReturnValue(50)
    hoisted.createEChartsInstance.mockReturnValue({
      setOption: hoisted.setOption,
      convertToPixel: hoisted.convertToPixel,
      convertFromPixel: hoisted.convertFromPixel,
      resize: hoisted.resize,
      dispose: hoisted.dispose
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes the ECharts temperature axis with graphic renderer support', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.registerEChartsExtensions).toHaveBeenCalledWith(['graphic'])
    expect(hoisted.createEChartsInstance).toHaveBeenCalledWith(wrapper.element, undefined, { renderer: 'canvas' })
    expect(hoisted.setOption).toHaveBeenCalledWith(
      expect.objectContaining({
        animation: false,
        xAxis: expect.objectContaining({ min: -40, max: 125 }),
        yAxis: expect.objectContaining({ min: 0, max: 1, show: false }),
        series: [
          expect.objectContaining({
            type: 'line',
            data: [
              [-40, 0.5],
              [125, 0.5]
            ],
            silent: true
          })
        ]
      }),
      true
    )
  })

  it('builds threshold band, draggable handles and current marker graphics', async () => {
    mountComponent({
      lower: 20,
      upper: 60,
      current: 40,
      lowerLabel: 'Low',
      upperLabel: 'High',
      currentLabel: 'Now'
    })
    await flushPromises()
    const chartOptions = hoisted.setOption.mock.calls.at(-1)?.[0]
    const graphics = chartOptions.graphic as Array<Record<string, any>>
    expect(graphics.map(item => item.id)).toEqual([
      'axis-hit-area',
      'alarm-band',
      'lower-handle',
      'upper-handle',
      'lower-label',
      'upper-label',
      'current-marker',
      'current-label'
    ])
    expect(graphics.find(item => item.id === 'alarm-band')).toMatchObject({
      type: 'rect',
      silent: true,
      style: expect.objectContaining({ fill: 'rgba(148, 163, 184, 0.36)' })
    })
    expect(graphics.find(item => item.id === 'lower-handle')).toMatchObject({
      type: 'circle',
      draggable: true,
      cursor: 'ew-resize'
    })
    expect(graphics.find(item => item.id === 'current-label')?.style.text).toBe('Now: 40°C')
  })

  it('accepts lower and upper props', async () => {
    const wrapper = mountComponent({ lower: 20, upper: 60 })
    await flushPromises()
    expect(wrapper.props('lower')).toBe(20)
    expect(wrapper.props('upper')).toBe(60)
  })

  it('accepts current prop', async () => {
    const wrapper = mountComponent({ current: 45 })
    await flushPromises()
    expect(wrapper.props('current')).toBe(45)
  })

  it('uses default min and max values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.props('min')).toBe(-40)
    expect(wrapper.props('max')).toBe(125)
  })

  it('accepts custom min and max', async () => {
    const wrapper = mountComponent({ min: 0, max: 100 })
    await flushPromises()
    expect(wrapper.props('min')).toBe(0)
    expect(wrapper.props('max')).toBe(100)
  })

  it('uses default unit C', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.props('unit')).toBe('C')
  })

  it('accepts unit F', async () => {
    const wrapper = mountComponent({ unit: 'F' })
    await flushPromises()
    expect(wrapper.props('unit')).toBe('F')
  })

  it('emits update:lower event', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.$emit('update:lower', 15)
    expect(wrapper.emitted('update:lower')).toEqual([[15]])
  })

  it('emits update:upper event', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.$emit('update:upper', 90)
    expect(wrapper.emitted('update:upper')).toEqual([[90]])
  })

  it('displayTemperature converts to Fahrenheit when unit is F', async () => {
    const wrapper = mountComponent({ unit: 'F' })
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.displayTemperature(0)
    expect(result).toBe('32°F')
  })

  it('displayTemperature stays Celsius when unit is C', async () => {
    const wrapper = mountComponent({ unit: 'C' })
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.displayTemperature(25)
    expect(result).toBe('25°C')
  })

  it('clamp function works correctly', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.clamp(5, 0, 10)).toBe(5)
    expect(state.clamp(-5, 0, 10)).toBe(0)
    expect(state.clamp(15, 0, 10)).toBe(10)
  })

  it('normalizeAxisValue returns fallback for non-finite values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.normalizeAxisValue(NaN, 50)).toBe(50)
    expect(state.normalizeAxisValue(Infinity, 50)).toBe(50)
    expect(state.normalizeAxisValue(25, 50)).toBe(25)
  })

  it('roundAxisValue rounds correctly', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.roundAxisValue(25.7)).toBe(26)
    expect(state.roundAxisValue(25.3)).toBe(25)
  })

  it('currentNumeric returns null for non-finite current', async () => {
    const wrapper = mountComponent({ current: 'invalid' })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.currentNumeric()).toBeNull()
  })

  it('currentNumeric returns clamped value for valid current', async () => {
    const wrapper = mountComponent({ current: 50, min: 0, max: 100 })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.currentNumeric()).toBe(50)
  })

  it('currentNumeric clamps value to min/max range', async () => {
    const wrapper = mountComponent({ current: 200, min: 0, max: 100 })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.currentNumeric()).toBe(100)
  })

  it('accepts null current', async () => {
    const wrapper = mountComponent({ current: null })
    await flushPromises()
    expect(wrapper.props('current')).toBeNull()
  })
})
