/**
 * 文件用途: 遥测子模块 time-series-data 的组件测试。
 * 核心逻辑: 挂载对应遥测组件，验证时间范围、聚合、图表或历史数据交互。
 * 关键注意事项: 接口 mock 的时间戳、聚合枚举和空数据要贴近后端契约。
 * 重构建议: 沉淀共享 telemetry fixture，减少各测试重复构造边界数据。
 */
import { defineComponent, h, nextTick, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  telemetryDataHistoryList: vi.fn(),
  messageInfo: vi.fn(),
  startLoading: vi.fn(),
  endLoading: vi.fn(),
  loadingRef: null as any,
  toggle: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  telemetryDataHistoryList: hoisted.telemetryDataHistoryList
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    info: hoisted.messageInfo,
    destroyAll: vi.fn()
  }
}))

vi.mock('@vueuse/core', () => ({
  useFullscreen: () => ({
    isFullscreen: ref(false),
    toggle: hoisted.toggle
  })
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: () => {
    const loading = ref(false)
    hoisted.loadingRef = loading
    hoisted.startLoading.mockImplementation(() => {
      loading.value = true
    })
    hoisted.endLoading.mockImplementation(() => {
      loading.value = false
    })

    return {
      loading,
      startLoading: hoisted.startLoading,
      endLoading: hoisted.endLoading
    }
  }
}))

import TimeSeriesData from '../time-series-data.vue'

const ButtonStub = defineComponent({
  name: 'ButtonStub',
  emits: ['click'],
  setup(_props, { emit, slots }) {
    return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
  }
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    value: {
      type: [String, Number, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:value'],
  setup() {
    return () => h('div', { class: 'select-stub' })
  }
})

const DatePickerStub = defineComponent({
  name: 'DatePickerStub',
  props: {
    value: {
      type: Array,
      default: null
    }
  },
  emits: ['update:value'],
  setup() {
    return () => h('div', { class: 'date-picker-stub' })
  }
})

const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    data: {
      type: Array,
      default: () => []
    },
    loading: {
      type: Boolean,
      default: false
    }
  },
  setup(props) {
    return () =>
      h('div', {
        class: 'table-stub',
        'data-loading': String(props.loading),
        'data-row-count': String((props.data as unknown[]).length)
      })
  }
})

/**
 * ChartComponent 在被测组件里是 defineAsyncComponent 动态加载的，
 * vi.mock 拦不到，只能通过 global.stubs 注入，因此 stub 必须在这里定义。
 */
const ChartStub = defineComponent({
  name: 'ChartComponentStub',
  props: {
    initialOptions: {
      type: Object,
      default: () => ({})
    }
  },
  setup() {
    return () => h('div', { class: 'chart-component-stub' })
  }
})

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountTimeSeriesData = () => {
  const wrapper = shallowMount(TimeSeriesData, {
    props: {
      deviceId: 'device-1',
      theKey: 'temp',
      theName: 'Temperature',
      theUnit: 'C'
    },
    global: {
      // stub 的键取决于组件在被测文件里的注册方式，两者不能混：
      //   1. `NSpace` / `NSelect` / `NDatePicker` 是 time-series-data.vue 顶部
      //      `import { ... } from 'naive-ui'` 的局部注册组件，匹配的是组件自身
      //      声明的 name（naive-ui 内部为 `Space` / `Select` / `DatePicker`，不带 N）。
      //   2. `NButton` / `n-data-table` / `FullScreen` 未被 import，靠
      //      unplugin-vue-components 在构建期全局注册；测试环境没有该插件，
      //      只能按模板里书写的名字匹配，写成 `Button` / `DataTable` 会漏掉，
      //      Vue 会报 "Failed to resolve component" 且 findComponent 返回空。
      stubs: {
        Space: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        Select: SelectStub,
        DatePicker: DatePickerStub,
        NButton: ButtonStub,
        'n-data-table': DataTableStub,
        // time-series-data.vue 用 defineAsyncComponent 载入 ChartComponent，
        // vi.mock('../ChartComponent.vue') 拦不到，必须在此显式 stub。
        ChartComponent: ChartStub,
        FullScreen: defineComponent({
          setup() {
            return () => h('div', { class: 'fullscreen-stub' })
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('time-series-data.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.telemetryDataHistoryList.mockResolvedValue({
      data: [
        { x: 1000, y: 20 },
        { x: 3000, y: 40 },
        { x: 2000, y: 'bad' }
      ],
      error: null
    })

    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => 'blob:telemetry-data'),
      revokeObjectURL: vi.fn()
    })
    ;(window as any).NMessage = {
      destroyAll: vi.fn()
    }
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads last-hour history on mount and computes numeric summaries', async () => {
    const wrapper = mountTimeSeriesData()

    await flushPromises()

    expect(hoisted.telemetryDataHistoryList).toHaveBeenCalledTimes(1)
    expect(hoisted.telemetryDataHistoryList.mock.calls[0][0]).toMatchObject({
      device_id: 'device-1',
      key: 'temp',
      time_range: 'last_1h',
      aggregate_window: 'no_aggregate'
    })
    expect(wrapper.findComponent(DataTableStub).attributes('data-row-count')).toBe('3')
    expect(wrapper.text()).toContain('30.00')
    expect(wrapper.text()).toContain('40')
    expect(wrapper.text()).toContain('20')
    // `$t` 被 mock 成恒等函数，模板渲染出的是 i18n key 本身而非英文文案。
    expect(wrapper.text()).toContain('card.latestPoint')
    expect(wrapper.text()).toContain('card.validDataPoints')
    expect(wrapper.text()).toContain('2/3')
    expect(wrapper.text()).toContain('1970-01-01')

    const chartOptions = wrapper.findComponent({ name: 'ChartComponentStub' }).props('initialOptions') as Record<
      string,
      any
    >
    expect(chartOptions.series[0].data).toEqual([
      [1000, 20],
      [3000, 40],
      [2000, null]
    ])
  })

  it('passes loading state to the rendered table while history is pending', async () => {
    let resolveHistory!: (value: unknown) => void
    hoisted.telemetryDataHistoryList.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveHistory = resolve
        })
    )

    const wrapper = mountTimeSeriesData()

    await nextTick()

    expect(hoisted.startLoading).toHaveBeenCalledTimes(1)
    expect(wrapper.findComponent(DataTableStub).attributes('data-loading')).toBe('true')

    resolveHistory({
      data: [],
      error: null
    })
    await flushPromises()

    expect(hoisted.endLoading).toHaveBeenCalledTimes(1)
    expect(wrapper.findComponent(DataTableStub).attributes('data-loading')).toBe('false')
    expect(wrapper.text()).toContain('0/0')
  })

  it('requires a full custom range before querying', async () => {
    const wrapper = mountTimeSeriesData()

    await flushPromises()
    vi.clearAllMocks()

    await wrapper.findAllComponents(SelectStub)[0].vm.$emit('update:value', 'custom')
    await flushPromises()

    expect(hoisted.messageInfo).toHaveBeenCalledWith('common.rangeMustSelected')
    expect(hoisted.telemetryDataHistoryList).toHaveBeenCalledTimes(0)
  })

  it('exports the current data as csv from the rendered button', async () => {
    const wrapper = mountTimeSeriesData()

    await flushPromises()

    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    await wrapper.findAll('button').at(-1)!.trigger('click')

    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)
    expect(clickSpy).toHaveBeenCalledTimes(1)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:telemetry-data')

    clickSpy.mockRestore()
  })

  it('renders an empty telemetry state when the api returns an empty list', async () => {
    hoisted.telemetryDataHistoryList.mockResolvedValueOnce({
      data: [],
      error: null
    })

    const wrapper = mountTimeSeriesData()

    await flushPromises()

    expect(wrapper.findComponent(DataTableStub).attributes('data-row-count')).toBe('0')
    expect(wrapper.text()).toContain('-')
    expect(wrapper.text()).toContain('0/0')

    const chartOptions = wrapper.findComponent({ name: 'ChartComponentStub' }).props('initialOptions') as Record<
      string,
      any
    >
    expect(chartOptions.series[0].data).toEqual([])
  })

  it('keeps the last successful chart and table state when a later history query errors', async () => {
    const wrapper = mountTimeSeriesData()

    await flushPromises()
    vi.clearAllMocks()

    hoisted.telemetryDataHistoryList.mockResolvedValueOnce({
      data: null,
      error: { message: 'history failed' }
    })

    await wrapper.findAllComponents(SelectStub)[1].vm.$emit('update:value', '1m')
    await flushPromises()

    expect(hoisted.telemetryDataHistoryList).toHaveBeenCalledWith(
      expect.objectContaining({
        aggregate_window: '1m',
        aggregate_function: 'avg'
      })
    )
    expect(hoisted.startLoading).toHaveBeenCalledTimes(1)
    expect(hoisted.endLoading).toHaveBeenCalledTimes(1)
    expect(wrapper.findComponent(DataTableStub).attributes('data-row-count')).toBe('3')

    const chartOptions = wrapper.findComponent({ name: 'ChartComponentStub' }).props('initialOptions') as Record<
      string,
      any
    >
    expect(chartOptions.series[0].data).toEqual([
      [1000, 20],
      [3000, 40],
      [2000, null]
    ])
  })

  it('formats chart tooltip rows with telemetry name and unit', async () => {
    const wrapper = mountTimeSeriesData()
    await flushPromises()

    const chartOptions = wrapper.findComponent({ name: 'ChartComponentStub' }).props('initialOptions') as Record<
      string,
      any
    >
    const formatter = chartOptions.tooltip.formatter

    const html = formatter([
      {
        value: [new Date('2026-06-21T00:00:00Z').valueOf(), 25],
        marker: '<span></span>'
      }
    ])

    expect(html).toContain('Temperature: 25C')
    expect(html).toContain('2026-06-21')
  })

  it('drives navigation, custom ranges, aggregation controls, pagination, and chart tools', async () => {
    const wrapper = mountTimeSeriesData()
    await flushPromises()

    const setupState = wrapper.vm.$.setupState as Record<string, any>
    const table = wrapper.findComponent(DataTableStub)
    const pagination = setupState.pagination as { page: number; onChange: (page: number) => void }
    pagination.onChange(3)
    expect(pagination.page).toBe(3)
    expect((setupState.columns as Array<any>)[0].render({ x: 1000 })).toContain('1970-01-01')

    const selects = wrapper.findAllComponents(SelectStub)
    await selects[0].vm.$emit('update:value', 'last_3h')
    await selects[1].vm.$emit('update:value', '1m')
    await flushPromises()

    const statisticsSelect = wrapper.findAllComponents(SelectStub).at(-1)!
    await statisticsSelect.vm.$emit('update:value', 'max')
    expect(setupState.selectedOption.aggregate_function).toBe('max')

    await wrapper.findComponent(DatePickerStub).vm.$emit('update:value', [1000, 2000])
    await flushPromises()
    expect(setupState.selectedOption.time_range).toBe('custom')
    expect(setupState.selectedOption.start_time).toBe(1000)
    expect(setupState.selectedOption.end_time).toBe(2000)

    const navigationButtons = wrapper.findAllComponents(ButtonStub)
    for (const button of navigationButtons.slice(0, 6)) {
      await button.trigger('click')
    }

    const chartOptions = wrapper.findComponent({ name: 'ChartComponentStub' }).props('initialOptions') as Record<string, any>
    chartOptions.toolbox.feature.myTool1.onclick()
    chartOptions.toolbox.feature.myTool2.onclick()
    chartOptions.toolbox.feature.myTool3.onclick()
    expect(chartOptions.series[0].type).toBe('scatter')
  })
})
