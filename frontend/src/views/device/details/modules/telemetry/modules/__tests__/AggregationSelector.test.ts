/**
 * 文件用途: 遥测子模块 AggregationSelector 的组件测试。
 * 核心逻辑: 通过子组件事件驱动时间范围、聚合窗口、统计函数变更，并验证对外回传的查询条件。
 * 关键注意事项: 自定义时间范围会联动聚合窗口可选项，超出一年范围时应阻止提交并提示错误。
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { NDatePicker, NFlex, NIcon, NPopselect } from 'naive-ui'

const hoisted = vi.hoisted(() => ({
  messageError: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    error: hoisted.messageError
  }
}))

import AggregationSelector from '../AggregationSelector.vue'

const PopselectStub = defineComponent({
  name: 'NPopselect',
  props: {
    value: { type: [String, Number], default: '' },
    modelValue: { type: [String, Number], default: '' },
    options: { type: Array, default: () => [] }
  },
  emits: ['update:modelValue', 'update:value'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'div',
        {
          class: 'n-popselect-stub',
          'data-value': String(props.value ?? props.modelValue ?? '')
        },
        [
          h(
            'select',
            {
              class: 'n-popselect-options',
              value: props.value ?? props.modelValue,
              onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
            },
            (props.options || []).map((option: any) => h('option', { value: option.value }, option.label))
          ),
          slots.default ? slots.default() : []
        ]
      )
  }
})

const DatePickerStub = defineComponent({
  name: 'NDatePicker',
  props: {
    value: { type: [Array, Number], default: null },
    type: { type: String, default: '' }
  },
  emits: ['update:value'],
  setup() {
    return () => h('div', { class: 'n-date-picker-stub' })
  }
})

const IconStub = defineComponent({
  name: 'NIcon',
  setup(_, { slots }) {
    return () => h('span', { class: 'n-icon-stub' }, slots.default ? slots.default() : [])
  }
})

function createWrapper(props: { device_id?: string; thekey?: string } = {}) {
  return shallowMount(AggregationSelector, {
    props: {
      device_id: props.device_id ?? 'device-1',
      thekey: props.thekey ?? 'temperature_1'
    },
    global: {
      stubs: {
        NFlex: { template: '<div class="n-flex"><slot /></div>' },
        NPopselect: PopselectStub,
        NDatePicker: DatePickerStub,
        NIcon: IconStub,
        TimeOutline: true,
        DiscOutline: true,
        StatsChartOutline: true
      }
    }
  })
}

function createRealNaiveWrapper() {
  return mount(AggregationSelector, {
    props: {
      device_id: 'device-1',
      thekey: 'temperature_1'
    },
    attachTo: document.body,
    global: {
      components: {
        NDatePicker,
        NFlex,
        NIcon,
        NPopselect
      }
    }
  })
}

function getPopselects(wrapper: ReturnType<typeof createWrapper>) {
  return wrapper.findAllComponents(PopselectStub)
}

function getAggregationOptions(wrapper: ReturnType<typeof createWrapper>) {
  return getPopselects(wrapper)[1].props('options') as Array<{ value: string; disabled: boolean }>
}

function getUpdateEvents(wrapper: ReturnType<typeof createWrapper>) {
  return wrapper.emitted('update:value') ?? []
}

function getLastUpdate(wrapper: ReturnType<typeof createWrapper>) {
  const events = getUpdateEvents(wrapper)
  if (events.length === 0) throw new Error('expected AggregationSelector to emit an updated query payload')
  return events[events.length - 1][0] as Record<string, any>
}

async function emitTimeRange(wrapper: ReturnType<typeof createWrapper>, value: string) {
  getPopselects(wrapper)[0].vm.$emit('update:value', value)
  await nextTick()
}

async function emitAggregationWindow(wrapper: ReturnType<typeof createWrapper>, value: string) {
  getPopselects(wrapper)[1].vm.$emit('update:value', value)
  await nextTick()
}

async function emitStatisticsFunction(wrapper: ReturnType<typeof createWrapper>, value: string) {
  getPopselects(wrapper)[2].vm.$emit('update:value', value)
  await nextTick()
}

async function emitDateRange(wrapper: ReturnType<typeof createWrapper>, value: [number, number]) {
  wrapper.getComponent(DatePickerStub).vm.$emit('update:value', value)
  await nextTick()
}

function ts(value: string) {
  return new Date(value).getTime()
}

describe('AggregationSelector.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the time controls and reveals the date picker for custom ranges', async () => {
    const wrapper = createWrapper()

    expect(getPopselects(wrapper)).toHaveLength(2)
    expect(wrapper.findAllComponents(DatePickerStub)).toHaveLength(0)
    expect(getUpdateEvents(wrapper)).toHaveLength(1)

    const timeSelector = getPopselects(wrapper)[0]
    await timeSelector.vm.$emit('update:value', 'custom')
    await nextTick()

    const datePickers = wrapper.findAllComponents(DatePickerStub)
    expect(datePickers).toHaveLength(1)
    expect(datePickers[0].props('type')).toBe('datetimerange')
  })

  it('mounts the real Naive UI controls and updates the query through a rendered popselect option', async () => {
    const wrapper = createRealNaiveWrapper()
    try {
      expect(wrapper.findAllComponents(NPopselect)).toHaveLength(2)
      const timeControl = wrapper.findAllComponents(NPopselect)[0]
      await timeControl.find('i').trigger('click')
      await nextTick()

      const customOption = document.body.querySelector('.n-base-select-menu .n-base-select-option')
      expect(customOption).not.toBeNull()
      await (customOption as HTMLElement).click()
      await nextTick()

      const datePickers = wrapper.findAllComponents(NDatePicker)
      expect(datePickers).toHaveLength(1)
      expect(datePickers[0].props()).toMatchObject({ type: 'datetimerange' })
      expect(getLastUpdate(wrapper)).toMatchObject({
        device_id: 'device-1',
        key: 'temperature_1',
        time_range: 'custom'
      })
    } finally {
      wrapper.unmount()
    }
  })

  it('emits the initial query state seeded from props on mount', () => {
    const wrapper = createWrapper({ device_id: 'dev-1', thekey: 'temp' })
    expect(getUpdateEvents(wrapper)).toEqual([
      [
        {
          device_id: 'dev-1',
          key: 'temp',
          aggregate_window: 'no_aggregate',
          time_range: 'last_1h'
        }
      ]
    ])
  })

  describe('time range interactions', () => {
    it('updates the full query through rendered time, aggregation, and statistics controls', async () => {
      const wrapper = createWrapper()

      await getPopselects(wrapper)[0].get('select').setValue('last_3h')
      expect(getLastUpdate(wrapper)).toMatchObject({
        time_range: 'last_3h',
        aggregate_window: '30s',
        aggregate_function: 'avg'
      })

      await getPopselects(wrapper)[1].get('select').setValue('1m')
      await getPopselects(wrapper)[2].get('select').setValue('max')

      expect(getLastUpdate(wrapper)).toMatchObject({
        device_id: 'device-1',
        key: 'temperature_1',
        time_range: 'last_3h',
        aggregate_window: '1m',
        aggregate_function: 'max'
      })
    })

    it('reacts to time selector updates through child events', async () => {
      const wrapper = createWrapper()

      await emitTimeRange(wrapper, 'last_3h')

      const update = getLastUpdate(wrapper)
      expect(update.time_range).toBe('last_3h')
      expect(update.aggregate_window).toBe('30s')
      expect(update.aggregate_function).toBe('avg')

      const options = getAggregationOptions(wrapper)
      expect(options[0].disabled).toBe(true)
      expect(options[1].disabled).toBe(false)
    })

    it('shows the custom date picker and clears custom timestamps when returning to a preset range', async () => {
      const wrapper = createWrapper()
      const start = ts('2024-01-01T00:00:00Z')
      const end = ts('2024-01-02T00:00:00Z')

      await emitTimeRange(wrapper, 'custom')
      expect(wrapper.findAllComponents(DatePickerStub)).toHaveLength(1)
      expect(wrapper.getComponent(DatePickerStub).props()).toMatchObject({
        value: null,
        type: 'datetimerange'
      })

      await emitDateRange(wrapper, [start, end])
      let update = getLastUpdate(wrapper)
      expect(update.start_time).toBe(start)
      expect(update.end_time).toBe(end)
      expect(wrapper.getComponent(DatePickerStub).props('value')).toEqual([start, end])

      await emitTimeRange(wrapper, 'last_1h')
      expect(wrapper.findAllComponents(DatePickerStub)).toHaveLength(0)

      update = getLastUpdate(wrapper)
      expect(update.time_range).toBe('last_1h')
      expect(update.start_time).toBeUndefined()
      expect(update.end_time).toBeUndefined()
      expect(update.aggregate_window).toBe('no_aggregate')
      expect(update.aggregate_function).toBeUndefined()
    })
  })

  describe('aggregation interactions', () => {
    it('defaults the statistics function when an aggregate window is selected', async () => {
      const wrapper = createWrapper()

      await emitAggregationWindow(wrapper, '1m')

      const update = getLastUpdate(wrapper)
      expect(update.aggregate_window).toBe('1m')
      expect(update.aggregate_function).toBe('avg')
      expect(getPopselects(wrapper)).toHaveLength(3)
    })

    it('keeps the selected statistics function when the aggregate window changes', async () => {
      const wrapper = createWrapper()

      await emitAggregationWindow(wrapper, '1m')
      await emitStatisticsFunction(wrapper, 'max')
      await emitAggregationWindow(wrapper, '5m')

      const update = getLastUpdate(wrapper)
      expect(update.aggregate_window).toBe('5m')
      expect(update.aggregate_function).toBe('max')
    })

    it('clears the statistics function when switching back to raw values', async () => {
      const wrapper = createWrapper()

      await emitAggregationWindow(wrapper, '1m')
      await emitStatisticsFunction(wrapper, 'sum')
      await emitAggregationWindow(wrapper, 'no_aggregate')

      const update = getLastUpdate(wrapper)
      expect(update.aggregate_window).toBe('no_aggregate')
      expect(update.aggregate_function).toBeUndefined()
      expect(getPopselects(wrapper)).toHaveLength(2)
    })
  })

  describe('custom date range behavior', () => {
    it('rejects ranges longer than one year without emitting a new query payload', async () => {
      const wrapper = createWrapper()
      const beforeCount = getUpdateEvents(wrapper).length

      await emitTimeRange(wrapper, 'custom')
      await emitDateRange(wrapper, [ts('2024-01-01T00:00:00Z'), ts('2025-06-01T00:00:00Z')])

      expect(hoisted.messageError).toHaveBeenCalledWith('common.withinOneYear')
      expect(wrapper.getComponent(DatePickerStub).props('value')).toBeNull()
      expect(getUpdateEvents(wrapper)).toHaveLength(beforeCount + 1)
      expect(getLastUpdate(wrapper)).toEqual({
        device_id: 'device-1',
        key: 'temperature_1',
        aggregate_window: 'no_aggregate',
        time_range: 'custom'
      })
    })

    it.each([
      ['1 hour', '2024-01-01T00:00:00Z', '2024-01-01T01:00:00Z', 'no_aggregate', undefined, 0],
      ['1 day', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z', '5m', 'avg', 4],
      ['90 days', '2024-01-01T00:00:00Z', '2024-03-31T00:00:00Z', '1d', 'avg', 10],
      ['1 year', '2024-01-01T00:00:00Z', '2025-01-01T00:00:00Z', '1mo', 'avg', 12]
    ])(
      'maps a %s custom range to the expected aggregate window',
      async (_label, startIso, endIso, expectedWindow, expectedFunction, firstEnabledIndex) => {
        const wrapper = createWrapper()
        const start = ts(startIso)
        const end = ts(endIso)

        await emitTimeRange(wrapper, 'custom')
        await emitDateRange(wrapper, [start, end])

        const update = getLastUpdate(wrapper)
        expect(update.start_time).toBe(start)
        expect(update.end_time).toBe(end)
        expect(update.aggregate_window).toBe(expectedWindow)
        expect(update.aggregate_function).toBe(expectedFunction)

        const options = getAggregationOptions(wrapper)
        expect(options[firstEnabledIndex].disabled).toBe(false)
        if (firstEnabledIndex > 0) {
          expect(options[firstEnabledIndex - 1].disabled).toBe(true)
        }
      }
    )
  })
})
