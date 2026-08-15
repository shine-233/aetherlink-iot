import { defineComponent, h, nextTick, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  telemetryHistoryData: vi.fn(),
  messageError: vi.fn(),
  startLoading: vi.fn(),
  endLoading: vi.fn(),
  loadingRef: null as any,
  windowOpen: vi.fn()
}))

vi.mock('@/service/api', () => ({
  telemetryHistoryData: hoisted.telemetryHistoryData
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/tool', () => ({
  getBaseServerUrl: () => 'http://localhost:9999/api/v1'
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({
    error: hoisted.messageError
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

import HistoryData from '../history-data.vue'

const ButtonStub = defineComponent({
  name: 'NButton',
  props: {
    disabled: {
      type: Boolean,
      default: false
    }
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          onClick: () => !props.disabled && emit('click')
        },
        slots.default ? slots.default() : []
      )
  }
})

const DatePickerStub = defineComponent({
  name: 'NDatePicker',
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

const PaginationStub = defineComponent({
  name: 'NPagination',
  props: {
    page: { type: Number, default: 1 },
    pageSize: { type: Number, default: 5 },
    itemCount: { type: Number, default: 0 }
  },
  emits: ['update:page', 'update:pageSize'],
  setup(props) {
    return () =>
      h('div', {
        class: 'pagination-stub',
        'data-page': String(props.page),
        'data-page-size': String(props.pageSize),
        'data-item-count': String(props.itemCount)
      })
  }
})

const DataTableStub = defineComponent({
  name: 'NDataTable',
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

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountHistoryData = (props: { deviceId: string; theKey: string }) => {
  const wrapper = shallowMount(HistoryData, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NFlex: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NDatePicker: DatePickerStub,
        NButton: ButtonStub,
        NAlert: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NText: defineComponent({
          setup(_, { slots }) {
            return () => h('span', slots.default ? slots.default() : [])
          }
        }),
        NDataTable: DataTableStub,
        NPagination: PaginationStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function findButton(wrapper: ReturnType<typeof mountHistoryData>, label: string) {
  const match = wrapper.findAll('button').find((button) => button.text() === label)
  if (!match) {
    throw new Error(`button not found: ${label}`)
  }
  return match
}

async function emitPage(wrapper: ReturnType<typeof mountHistoryData>, page: number) {
  wrapper.findComponent(PaginationStub).vm.$emit('update:page', page)
  await flushPromises()
}

async function emitPageSize(wrapper: ReturnType<typeof mountHistoryData>, pageSize: number) {
  wrapper.findComponent(PaginationStub).vm.$emit('update:pageSize', pageSize)
  await flushPromises()
}

async function emitDateRange(wrapper: ReturnType<typeof mountHistoryData>, value: [Date, Date] | [number, number]) {
  wrapper.findComponent(DatePickerStub).vm.$emit('update:value', value)
  await flushPromises()
}

async function clickButton(wrapper: ReturnType<typeof mountHistoryData>, label: string) {
  await findButton(wrapper, label).trigger('click')
  await flushPromises()
}

describe('history-data.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.telemetryHistoryData.mockResolvedValue({
      data: {
        list: [{ key: 'temp', ts: '2026-06-21T00:00:00Z', value: 12.3 }],
        total: 1
      },
      error: null
    })
    vi.stubGlobal('open', hoisted.windowOpen)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads history on mount when device id and telemetry key are present', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledTimes(1)
    expect(hoisted.telemetryHistoryData.mock.calls[0][0]).toMatchObject({
      device_id: 'device-1',
      key: 'temp',
      export_excel: false,
      page: 1,
      page_size: 5
    })
    expect(wrapper.find('.table-stub').attributes('data-row-count')).toBe('1')
  })

  it('passes loading state to the table and export button while history is pending', async () => {
    let resolveHistory!: (value: unknown) => void
    hoisted.telemetryHistoryData.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveHistory = resolve
        })
    )

    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await nextTick()

    expect(hoisted.startLoading).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.table-stub').attributes('data-loading')).toBe('true')
    expect(findButton(wrapper, 'generate.export').attributes('disabled')).toBe('')

    resolveHistory({
      data: { list: [], total: 0 },
      error: null
    })
    await flushPromises()

    expect(hoisted.endLoading).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.table-stub').attributes('data-loading')).toBe('false')
    expect(findButton(wrapper, 'generate.export').attributes('disabled')).toBeUndefined()
  })

  it('does not query history when either device id or telemetry key is missing', async () => {
    const missingDevice = mountHistoryData({ deviceId: '', theKey: 'temp' })
    const missingKey = mountHistoryData({ deviceId: 'device-1', theKey: '' })

    await flushPromises()

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledTimes(0)
    expect(missingDevice.find('.table-stub').attributes('data-row-count')).toBe('0')
    expect(missingKey.find('.table-stub').attributes('data-row-count')).toBe('0')
  })

  it('switches page number through pagination emits', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    await emitPage(wrapper, 2)

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 2,
        page_size: 5
      })
    )
    expect(wrapper.findComponent(PaginationStub).attributes('data-page')).toBe('2')
  })

  it('switches page size and resets pagination through pagination emits', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    await emitPage(wrapper, 3)
    vi.clearAllMocks()

    await emitPageSize(wrapper, 10)

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10
      })
    )
    expect(wrapper.findComponent(PaginationStub).attributes('data-page')).toBe('1')
    expect(wrapper.findComponent(PaginationStub).attributes('data-page-size')).toBe('10')
  })

  it('resets the request page to 1 when refreshing after pagination changed', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    await emitPage(wrapper, 3)
    await clickButton(wrapper, 'generate.refresh')

    expect(hoisted.telemetryHistoryData.mock.calls[0][0]).toMatchObject({ page: 3 })
    expect(hoisted.telemetryHistoryData.mock.calls[1][0]).toMatchObject({ page: 1, export_excel: false })
    expect(wrapper.findComponent(PaginationStub).attributes('data-page')).toBe('1')
  })

  it('opens the export url returned by the API when clicking export', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    hoisted.telemetryHistoryData.mockResolvedValueOnce({
      data: {
        filePath: 'downloads/history.csv'
      },
      error: null
    })

    await clickButton(wrapper, 'generate.export')

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledWith(
      expect.objectContaining({
        export_excel: true
      })
    )
    expect(hoisted.windowOpen).toHaveBeenCalledWith('http://localhost:9999/downloads/history.csv')
  })

  it('shows an export error when the export response has no safe download path', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    hoisted.telemetryHistoryData.mockResolvedValueOnce({
      data: {
        filePath: '../secret.csv'
      },
      error: null
    })

    await clickButton(wrapper, 'generate.export')

    expect(hoisted.windowOpen).not.toHaveBeenCalled()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.device_details.telemetryExportFailed')
  })

  it('shows an export error when the export request fails', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    hoisted.telemetryHistoryData.mockResolvedValueOnce({
      data: null,
      error: { message: 'Export failed' }
    })

    await clickButton(wrapper, 'generate.export')

    expect(hoisted.windowOpen).not.toHaveBeenCalled()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.device_details.telemetryExportFailed')
  })

  it('rejects date ranges longer than one month', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    const tooLongRange: [Date, Date] = [new Date('2026-01-01T00:00:00Z'), new Date('2026-03-15T00:00:00Z')]

    await emitDateRange(wrapper, tooLongRange)

    expect(hoisted.messageError).toHaveBeenCalledWith('common.withinOneMonth')
    expect(hoisted.telemetryHistoryData).toHaveBeenCalledTimes(0)
    expect(wrapper.text()).toContain('generate.hour-24')
  })

  it('accepts date ranges within one month and queries normally', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()
    vi.clearAllMocks()

    const validRange: [number, number] = [
      new Date('2026-01-01T00:00:00Z').valueOf(),
      new Date('2026-01-15T00:00:00Z').valueOf()
    ]

    await emitDateRange(wrapper, validRange)

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledWith(
      expect.objectContaining({
        start_time: validRange[0],
        end_time: validRange[1],
        export_excel: false
      })
    )
    expect(hoisted.messageError).toHaveBeenCalledTimes(0)
  })

  it('refresh returns export-triggered state to normal history loading', async () => {
    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()

    hoisted.telemetryHistoryData.mockResolvedValueOnce({
      data: {
        filePath: 'downloads/history.csv'
      },
      error: null
    })

    await clickButton(wrapper, 'generate.export')
    vi.clearAllMocks()

    await clickButton(wrapper, 'generate.refresh')

    expect(hoisted.telemetryHistoryData).toHaveBeenCalledWith(
      expect.objectContaining({
        export_excel: false,
        page: 1
      })
    )
  })

  it('handles empty data response without rendering rows', async () => {
    hoisted.telemetryHistoryData.mockResolvedValueOnce({
      data: { list: [], total: 0 },
      error: null
    })

    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()

    expect(wrapper.find('.table-stub').attributes('data-row-count')).toBe('0')
    expect(wrapper.findComponent(PaginationStub).attributes('data-item-count')).toBe('0')
    expect(wrapper.text()).toContain('custom.device_details.telemetryNoData')
  })

  it('shows a history load error without updating table rows', async () => {
    hoisted.telemetryHistoryData.mockResolvedValueOnce({
      data: null,
      error: { message: 'Network error' }
    })

    const wrapper = mountHistoryData({ deviceId: 'device-1', theKey: 'temp' })

    await flushPromises()

    expect(wrapper.find('.table-stub').attributes('data-row-count')).toBe('0')
    expect(wrapper.findComponent(PaginationStub).attributes('data-item-count')).toBe('0')
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.device_details.telemetryHistoryLoadFailed')
  })
})
