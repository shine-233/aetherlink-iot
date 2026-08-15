import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  loadHistory: vi.fn(),
  exportHistory: vi.fn(),
  state: {
    failedHistorySeriesLabels: [] as string[],
    partialHistorySeriesLabels: [] as string[],
    gappedHistorySeriesLabels: [] as string[],
    hasSuccessfulHistoryData: true,
    hasHistoryFailures: false,
    hasHistoryChartData: true,
    energyStatisticsAvailable: true
  }
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({ locale: 'en-US' })
}))

vi.mock('../rdi/composables/useRdiHistory', () => ({
  useRdiHistory: () => ({
    energyLoading: ref(false),
    historyExportLoading: ref(false),
    energyRange: ref('last_1h'),
    energyCustomRange: ref(null),
    historyChartSeriesKeys: ref(['temperature_1', 'electricity_consumption']),
    historyExportKey: ref('electricity_consumption'),
    historyExportFormat: ref('csv'),
    energyStats: {
      latest: 12.5,
      delta: 2.5,
      min: 10,
      max: 12.5,
      sample_count: 8
    },
    historyChartOptions: ref({ series: [] }),
    energyRangeOptions: ref([{ label: 'Last 1 hour', value: 'last_1h' }]),
    historyChartSeriesOptions: ref([]),
    historyExportKeyOptions: ref([]),
    historyExportFormatOptions: ref([]),
    failedHistorySeriesLabels: ref([...mocks.state.failedHistorySeriesLabels]),
    partialHistorySeriesLabels: ref([...mocks.state.partialHistorySeriesLabels]),
    gappedHistorySeriesLabels: ref([...mocks.state.gappedHistorySeriesLabels]),
    hasSuccessfulHistoryData: ref(mocks.state.hasSuccessfulHistoryData),
    hasHistoryFailures: ref(mocks.state.hasHistoryFailures),
    hasHistoryChartData: ref(mocks.state.hasHistoryChartData),
    energyStatisticsAvailable: ref(mocks.state.energyStatisticsAvailable),
    formatEnergyValue: (value: number | null) => (value === null ? '--' : `${value.toFixed(2)} kWh`),
    loadEnergyStatistics: mocks.loadHistory,
    exportHistoryData: mocks.exportHistory
  })
}))

vi.mock('../telemetry/modules/ChartComponent.vue', () => ({
  default: defineComponent({
    name: 'ChartComponent',
    setup() {
      return () => h('div', { 'data-chart': 'rdi-history' })
    }
  })
}))

import RdiDeviceHistoryView from '../RdiDeviceHistoryView.vue'
import { labels } from '../rdi/constants/rdi-labels'

const ButtonStub = defineComponent({
  name: 'NButton',
  emits: ['click'],
  setup(_, { emit, slots }) {
    return () => h('button', { onClick: () => emit('click') }, slots.default?.())
  }
})

const SlotStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const AlertStub = defineComponent({
  name: 'NAlert',
  props: {
    title: String
  },
  setup(props, { slots }) {
    return () => h('div', [h('strong', props.title), slots.default?.()])
  }
})

function mountHistoryView(id = 'device-1') {
  return shallowMount(RdiDeviceHistoryView, {
    props: { id },
    global: {
      stubs: {
        NButton: ButtonStub,
        NSelect: true,
        NDatePicker: true,
        NSpin: SlotStub,
        NAlert: AlertStub,
        NEmpty: true,
        ChartComponent: true
      }
    }
  })
}

describe('RdiDeviceHistoryView', () => {
  beforeEach(() => {
    mocks.loadHistory.mockReset()
    mocks.loadHistory.mockResolvedValue(undefined)
    mocks.exportHistory.mockReset()
    mocks.exportHistory.mockResolvedValue(undefined)
    mocks.state.failedHistorySeriesLabels = []
    mocks.state.partialHistorySeriesLabels = []
    mocks.state.gappedHistorySeriesLabels = []
    mocks.state.hasSuccessfulHistoryData = true
    mocks.state.hasHistoryFailures = false
    mocks.state.hasHistoryChartData = true
    mocks.state.energyStatisticsAvailable = true
  })

  it('loads the default RDI history and exposes load/export actions in the top-level history view', async () => {
    const wrapper = mountHistoryView()
    await flushPromises()

    expect(mocks.loadHistory).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Latest')
    expect(wrapper.text()).toContain('12.50 kWh')
    expect(wrapper.text()).toContain('Data points')
    expect(wrapper.text()).toContain('8')

    const buttons = wrapper.findAll('button')
    expect(buttons.map((button) => button.text())).toEqual(['Load', 'Export data'])

    await buttons[0].trigger('click')
    await buttons[1].trigger('click')
    await flushPromises()

    expect(mocks.loadHistory).toHaveBeenCalledTimes(2)
    expect(mocks.exportHistory).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ id: 'device-2' })
    await flushPromises()
    expect(mocks.loadHistory).toHaveBeenCalledTimes(3)
  })

  it('renders a failed-series error and hides zero-like energy statistics when history loading fails', async () => {
    mocks.state.failedHistorySeriesLabels = ['T1']
    mocks.state.hasSuccessfulHistoryData = false
    mocks.state.hasHistoryFailures = true
    mocks.state.hasHistoryChartData = false
    mocks.state.energyStatisticsAvailable = false

    const wrapper = mountHistoryView()
    await flushPromises()

    expect(wrapper.text()).toContain(labels['en-US'].historyLoadFailed)
    expect(wrapper.text()).toContain('T1')
    expect(wrapper.findAll('.rdi-history-stat strong').map((item) => item.text())).toEqual([
      '--',
      '--',
      '-- / --',
      '--'
    ])
  })

  it('keeps partial data visible while warning which history series could not be fully loaded', async () => {
    mocks.state.partialHistorySeriesLabels = ['T1']
    mocks.state.hasSuccessfulHistoryData = true
    mocks.state.hasHistoryFailures = true
    mocks.state.hasHistoryChartData = true

    const wrapper = mountHistoryView()
    await flushPromises()

    expect(wrapper.text()).toContain(labels['en-US'].historyPartialData)
    expect(wrapper.text()).toContain('T1')
  })

  it('warns about detected sampling gaps and states that missing intervals are not connected', async () => {
    mocks.state.gappedHistorySeriesLabels = ['SW1']

    const wrapper = mountHistoryView()
    await flushPromises()

    expect(wrapper.text()).toContain(labels['en-US'].historyGapDetected)
    expect(wrapper.text()).toContain('SW1')
    expect(wrapper.text()).toContain(labels['en-US'].historyGapNotConnected)
  })
})
