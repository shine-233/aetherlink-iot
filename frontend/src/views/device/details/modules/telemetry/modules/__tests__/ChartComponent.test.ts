/**
 * 文件用途: 遥测子模块 ChartComponent 的组件测试。
 * 核心逻辑: 挂载对应遥测组件，验证时间范围、聚合、图表或历史数据交互。
 * 关键注意事项: 接口 mock 的时间戳、聚合枚举和空数据要贴近后端契约。
 * 重构建议: 沉淀共享 telemetry fixture，减少各测试重复构造边界数据。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const updateOptionsMock = vi.fn()

vi.mock('@/hooks/tp-chart/use-tp-echarts', () => ({
  useTpECharts: () => ({
    domRef: { value: null },
    updateOptions: updateOptionsMock
  })
}))

vi.mock('echarts/core', () => ({
  use: vi.fn()
}))

vi.mock('echarts/components', () => ({
  DataZoomComponent: {},
  GridComponent: {},
  LegendComponent: {},
  ToolboxComponent: {},
  TooltipComponent: {}
}))

import ChartComponent from '../ChartComponent.vue'

describe('ChartComponent.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('mounts with initialOptions and calls updateOptions via watcher', () => {
    const options = { title: { text: 'Test Chart' }, series: [{ data: [1, 2, 3] }] }
    shallowMount(ChartComponent, {
      props: { initialOptions: options }
    })
    expect(updateOptionsMock).toHaveBeenCalledTimes(1)
    const callback = updateOptionsMock.mock.calls[0][0]
    expect(callback({ grid: { left: 20 } })).toMatchObject({
      grid: { left: 20 },
      title: { text: 'Test Chart' },
      series: [{ data: [1, 2, 3] }]
    })
  })

  it('renders a chart container div', () => {
    const wrapper = shallowMount(ChartComponent, {
      props: { initialOptions: {} }
    })
    expect(wrapper.get('.chart-container').classes()).toEqual(['chart-container'])
  })

  it('calls updateOptions when initialOptions change', async () => {
    const wrapper = shallowMount(ChartComponent, {
      props: { initialOptions: { series: [{ data: [1] }] } }
    })
    updateOptionsMock.mockClear()
    await wrapper.setProps({ initialOptions: { series: [{ data: [2, 3] }] } })
    expect(updateOptionsMock).toHaveBeenCalledTimes(1)
    const callback = updateOptionsMock.mock.calls[0][0]
    expect(callback({ title: { text: 'Old' } })).toMatchObject({
      title: { text: 'Old' },
      series: [{ data: [2, 3] }]
    })
  })

  it('passes a callback to updateOptions that merges options', async () => {
    const wrapper = shallowMount(ChartComponent, {
      props: { initialOptions: { title: { text: 'A' } } }
    })
    updateOptionsMock.mockClear()
    await wrapper.setProps({ initialOptions: { series: [{ data: [1] }] } })
    expect(updateOptionsMock).toHaveBeenCalledTimes(1)
    const callback = updateOptionsMock.mock.calls[0][0]
    const current = { title: { text: 'A' }, legend: { show: true } }
    const merged = callback(current)
    expect(merged).toMatchObject({
      title: { text: 'A' },
      legend: { show: true },
      series: [{ data: [1] }]
    })
  })

  it('does not call updateOptions when newOptions is falsy', async () => {
    const wrapper = shallowMount(ChartComponent, {
      props: { initialOptions: { series: [{ data: [1] }] } }
    })
    updateOptionsMock.mockClear()
    await wrapper.setProps({ initialOptions: null as any })
    expect(updateOptionsMock).toHaveBeenCalledTimes(0)
  })
})
