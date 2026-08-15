import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({ updateOptions: vi.fn(), factory: null as null | (() => unknown) }))
vi.mock('@/hooks/chart/use-echarts', () => ({
  createEChartsHook: vi.fn((factory: () => unknown) => {
    hoisted.factory = factory
    return { domRef: ref<HTMLElement | null>(null), updateOptions: hoisted.updateOptions }
  })
}))

import { createEChartsHook } from '@/hooks/chart/use-echarts'
import LocalEChartsWidget from './LocalEChartsWidget.vue'

describe('LocalEChartsWidget', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the shared ECharts hook and pushes safe option updates', async () => {
    const wrapper = mount(LocalEChartsWidget, {
      props: { type: 'line-chart', config: { categories: ['A'], values: [1] }, fields: {} }
    })
    expect(createEChartsHook).toHaveBeenCalledWith(
      expect.any(Function),
      {},
      expect.objectContaining({ hideLoadingAfterDefaultRender: true })
    )
    expect(hoisted.factory?.()).toMatchObject({ series: [{ type: 'line', data: [1] }] })

    await wrapper.setProps({ config: { categories: ['A'], values: [2] } })
    expect(hoisted.updateOptions).toHaveBeenCalled()
    const updater = hoisted.updateOptions.mock.calls.at(-1)?.[0]
    expect(updater()).toMatchObject({ series: [{ type: 'line', data: [2] }] })
  })

  it('does not expose a chart mount point when field data is unavailable', () => {
    const wrapper = mount(LocalEChartsWidget, {
      props: { type: 'bar-chart', config: { categoryField: 'labels', valueField: 'values' }, fields: {} }
    })
    expect(wrapper.text()).toContain('Unavailable')
    expect(wrapper.find('.local-chart-canvas').exists()).toBe(false)
  })
})
