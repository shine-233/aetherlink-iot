import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  send: vi.fn(),
  close: vi.fn(),
  getTelemetryLogList: vi.fn(),
  telemetryDataCurrent: vi.fn(),
  telemetryDataPub: vi.fn(),
  telemetryDataDel: vi.fn(),
  getSimulationInit: vi.fn(),
  sendSimulationData: vi.fn(),
  expectMessageAdd: vi.fn(),
  deviceCustomControlList: vi.fn(),
  useWebSocketOptions: null as any
}))

vi.mock('@vueuse/core', () => ({
  useWebSocket: (_url: string, options: unknown) => {
    hoisted.useWebSocketOptions = options
    return {
      status: ref('OPEN'),
      send: hoisted.send,
      close: hoisted.close
    }
  }
}))

vi.mock('@/service/api', () => ({
  expectMessageAdd: hoisted.expectMessageAdd,
  getSimulationInit: hoisted.getSimulationInit,
  getTelemetryLogList: hoisted.getTelemetryLogList,
  sendSimulationData: hoisted.sendSimulationData,
  telemetryDataCurrent: hoisted.telemetryDataCurrent,
  telemetryDataDel: hoisted.telemetryDataDel,
  telemetryDataPub: hoisted.telemetryDataPub
}))

vi.mock('@/service/api/system-data', () => ({
  deviceCustomControlList: hoisted.deviceCustomControlList
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: vi.fn(() => 'mock-token')
  }
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/tool', () => ({
  getWebsocketServerUrl: () => 'ws://socket.test',
  getBaseServerUrl: () => 'http://localhost:9999/api/v1',
  isJSON: (value: string) => {
    try {
      JSON.parse(value)
      return true
    } catch {
      return false
    }
  }
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: () => ({
    loading: ref(false),
    startLoading: vi.fn(() => {}),
    endLoading: vi.fn(() => {})
  })
}))

vi.mock('@/components/common/AnimatedNumber.vue', () => ({
  default: defineComponent({
    name: 'AnimatedNumberStub',
    setup() {
      return () => h('span', { class: 'animated-number-stub' })
    }
  })
}))

import TelemetryPage from '../telemetry.vue'
import { TELEMETRY_CARD_FRESHNESS_FILTER, TELEMETRY_CARD_FRESHNESS_STATUS } from '../telemetryCardViewState'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const SlotStub = defineComponent({
  name: 'SlotStub',
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const CardStub = defineComponent({
  name: 'NCard',
  setup(_, { slots }) {
    return () => h('div', [slots.header?.(), slots.default?.()])
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  props: ['disabled', 'loading'],
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          'data-loading': props.loading,
          onClick: () => !props.disabled && emit('click')
        },
        slots.default?.()
      )
  }
})

const SelectStub = defineComponent({
  name: 'NSelect',
  props: ['value', 'options'],
  emits: ['update:value'],
  setup() {
    return () => h('div', { class: 'select-stub' })
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: ['value'],
  emits: ['update:value'],
  setup() {
    return () => h('input')
  }
})

const NumberInputStub = defineComponent({
  name: 'NInputNumber',
  props: ['value'],
  emits: ['update:value'],
  setup() {
    return () => h('input', { type: 'number' })
  }
})

const SwitchStub = defineComponent({
  name: 'NSwitch',
  props: ['value'],
  emits: ['update:value'],
  setup() {
    return () => h('button', { class: 'switch-stub' })
  }
})

const ModalStub = defineComponent({
  name: 'NModal',
  props: ['show'],
  emits: ['update:show'],
  setup(props, { slots }) {
    return () => h('div', { class: 'modal-stub', 'data-show': String(Boolean(props.show)) }, props.show ? slots.default?.() : [])
  }
})

const PaginationStub = defineComponent({
  name: 'NPagination',
  props: ['pageCount', 'pageSize'],
  emits: ['update:page'],
  setup() {
    return () => h('div', { class: 'pagination-stub' })
  }
})

const PopconfirmStub = defineComponent({
  name: 'NPopconfirm',
  emits: ['positive-click'],
  setup(_, { emit, slots }) {
    return () =>
      h('div', [slots.trigger?.(), h('button', { class: 'positive', onClick: () => emit('positive-click') }, 'ok')])
  }
})

const EmptyStub = defineComponent({
  name: 'NEmpty',
  props: ['description'],
  setup(props, { slots }) {
    return () => h('div', { class: 'empty-stub' }, [h('span', props.description || ''), slots.extra?.()])
  }
})

const TelemetryOperationsHeaderStub = defineComponent({
  name: 'TelemetryOperationsHeader',
  props: ['controlList', 'controlListLoaded', 'controlListLoading', 'showLog'],
  emits: ['publish', 'simulate', 'load-controls', 'control-change'],
  setup() {
    return () => h('div', { class: 'telemetry-operations-header-stub' })
  }
})

const TelemetryRealtimeViewStub = defineComponent({
  name: 'TelemetryRealtimeView',
  props: [
    'attentionTelemetryCount',
    'cardHeight',
    'cardMargin',
    'deleteOptions',
    'displayTelemetryCount',
    'displayTelemetryData',
    'getTelemetryFreshnessBadge',
    'hasTelemetryCardFilters',
    'isTelemetryHardRenderCapped',
    'nowTime',
    'showAllTelemetryCards',
    'telemetryAccentColor',
    'telemetryDataCount',
    'telemetryFreshnessOptions',
    'telemetryLoadError',
    'telemetryLoadStatus',
    'telemetrySortOptions',
    'visibleTelemetryCount',
    'visibleTelemetryData',
    'searchQuery',
    'sortMode',
    'freshnessFilter'
  ],
  emits: [
    'clear-filters',
    'delete-select',
    'export-csv',
    'history',
    'sequence',
    'toggle-display-limit',
    'update:searchQuery',
    'update:sortMode',
    'update:freshnessFilter'
  ],
  setup() {
    return () => h('div', { class: 'telemetry-realtime-view-stub' })
  }
})

const HistoryDataStub = defineComponent({
  name: 'HistoryData',
  props: ['deviceId', 'theKey', 'theName', 'theUnit'],
  setup() {
    return () => h('div', { class: 'history-data-stub' })
  }
})

const TimeSeriesDataStub = defineComponent({
  name: 'TimeSeriesData',
  props: ['deviceId', 'theKey', 'theName', 'theUnit'],
  setup() {
    return () => h('div', { class: 'time-series-data-stub' })
  }
})

const createGlobal = () => ({
  config: {
    globalProperties: {
      getPlatform: () => false
    }
  },
  stubs: {
    NFlex: SlotStub,
    NButton: ButtonStub,
    NGrid: SlotStub,
    NGridItem: SlotStub,
    NCard: CardStub,
    NGi: SlotStub,
    NTooltip: SlotStub,
    NIcon: SlotStub,
    NDivider: true,
    NDropdown: defineComponent({ name: 'NDropdown', emits: ['select'], setup: () => () => h('div') }),
    NSpace: SlotStub,
    NSelect: SelectStub,
    NDataTable: true,
    NPagination: PaginationStub,
    NModal: ModalStub,
    NForm: SlotStub,
    NAlert: SlotStub,
    NInput: InputStub,
    NInputNumber: NumberInputStub,
    NCollapseTransition: SlotStub,
    NFormItem: SlotStub,
    NPopover: SlotStub,
    NSwitch: SwitchStub,
    NPopconfirm: PopconfirmStub,
    NEmpty: EmptyStub,
    SvgIcon: true,
    HistoryData: HistoryDataStub,
    TimeSeriesData: TimeSeriesDataStub,
    TelemetryOperationsHeader: TelemetryOperationsHeaderStub,
    TelemetryRealtimeView: TelemetryRealtimeViewStub,
    DocumentTextOutline: true,
    TrendingUpOutline: true
  }
})

const mountTelemetryPage = (
  props: Partial<{ id: string; deviceTemplateId: string; deviceData: Record<string, any> }> = {}
) => {
  const wrapper = shallowMount(TelemetryPage, {
    props: {
      id: 'device-1',
      deviceTemplateId: 'tpl-1',
      ...props
    },
    global: createGlobal()
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof mountTelemetryPage>) => wrapper.vm.$.setupState as Record<string, any>

const getOperationsHeader = (wrapper: ReturnType<typeof mountTelemetryPage>) => wrapper.getComponent(TelemetryOperationsHeaderStub)

const getRealtimeView = (wrapper: ReturnType<typeof mountTelemetryPage>) => wrapper.getComponent(TelemetryRealtimeViewStub)

const findButtonByText = (wrapper: ReturnType<typeof mountTelemetryPage>, text: string) =>
  wrapper.findAllComponents(ButtonStub).find((button) => button.text() === text)

const openOperationLogs = async (wrapper: ReturnType<typeof mountTelemetryPage>) => {
  const button = findButtonByText(wrapper, 'generate.log')
  await button!.trigger('click')
  await flushPromises()
}

describe('telemetry.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.useWebSocketOptions = null
    hoisted.getTelemetryLogList.mockResolvedValue({
      data: {
        list: [{ data: 'payload', status: '1', created_at: '2026-06-21T00:00:00Z', operation_type: '1' }],
        count: 5
      },
      error: null
    })
    hoisted.telemetryDataCurrent.mockResolvedValue({
      data: [
        {
          key: 'temperature_1',
          label: 'Temperature 1',
          value: 25,
          ts: '2026-06-21T00:00:00Z',
          device_id: 'device-1',
          unit: 'C'
        }
      ],
      error: null
    })
    hoisted.deviceCustomControlList.mockResolvedValue({
      data: {
        list: [{ id: 'control-1', name: 'Start', content: '{"switch":true}' }]
      }
    })
    hoisted.telemetryDataPub.mockResolvedValue({ error: null })
    hoisted.getSimulationInit.mockResolvedValue({
      data: null,
      error: new Error('network')
    })
    hoisted.sendSimulationData.mockResolvedValue({ error: null })
    hoisted.expectMessageAdd.mockResolvedValue({ error: null })
    ;(window as any).$message = {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    }
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads the realtime snapshot on startup and defers logs plus controls until requested', async () => {
    const wrapper = mountTelemetryPage()

    await flushPromises()

    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
    expect(hoisted.send).toHaveBeenCalledWith(JSON.stringify({ device_id: 'device-1', token: 'mock-token' }))
    expect(hoisted.getTelemetryLogList).not.toHaveBeenCalled()
    expect(hoisted.deviceCustomControlList).not.toHaveBeenCalled()
    expect(getOperationsHeader(wrapper).props('controlListLoaded')).toBe(false)
    expect(getOperationsHeader(wrapper).props('controlList')).toEqual([])
    expect(getRealtimeView(wrapper).props('telemetryLoadStatus')).toBe('ready')
    expect(getRealtimeView(wrapper).props('telemetryDataCount')).toBe(1)
  })

  it('batches websocket updates and preserves zero values', async () => {
    vi.useFakeTimers()
    const wrapper = mountTelemetryPage()

    await flushPromises()

    const setupState = getSetupState(wrapper)
    await hoisted.useWebSocketOptions.onMessage?.(
      {} as WebSocket,
      {
        data: JSON.stringify({
          temperature_1: 0,
          systime: '2026-06-21T01:02:03Z'
        })
      } as MessageEvent
    )

    expect(setupState.telemetryData[0].value).toBe(25)
    await vi.advanceTimersByTimeAsync(121)
    await flushPromises()

    expect(setupState.telemetryData[0].value).toBe(0)
    expect(setupState.telemetryData[0].ts).toBe('2026-06-21T01:02:03Z')
  })

  it('filters telemetry cards through TelemetryRealtimeView model events', async () => {
    hoisted.telemetryDataCurrent.mockResolvedValue({
      data: [
        {
          key: 'temperature',
          label: 'Temperature',
          value: 25,
          ts: '2099-01-01T00:00:00Z',
          device_id: 'device-1',
          unit: 'C'
        },
        {
          key: 'pressure',
          label: 'Pressure',
          value: 99,
          ts: null,
          device_id: 'device-1',
          unit: 'kPa'
        }
      ],
      error: null
    })

    const wrapper = mountTelemetryPage()
    await flushPromises()

    expect(getRealtimeView(wrapper).props('attentionTelemetryCount')).toBe(1)
    expect(getRealtimeView(wrapper).props('visibleTelemetryCount')).toBe(2)

    await getRealtimeView(wrapper).vm.$emit('update:freshnessFilter', TELEMETRY_CARD_FRESHNESS_FILTER.attention)
    await flushPromises()

    const filteredItems = getRealtimeView(wrapper).props('visibleTelemetryData') as Array<Record<string, any>>
    const freshnessBadge = getRealtimeView(wrapper).props('getTelemetryFreshnessBadge') as (telemetry: Record<string, any>) => Record<string, any>

    expect(getRealtimeView(wrapper).props('visibleTelemetryCount')).toBe(1)
    expect(filteredItems[0]).toMatchObject({ key: 'pressure' })
    expect(freshnessBadge(filteredItems[0])).toMatchObject({
      status: TELEMETRY_CARD_FRESHNESS_STATUS.missingTimestamp,
      i18nKey: 'custom.device_details.telemetryFreshnessNoTimestamp'
    })

    expect(getRealtimeView(wrapper).props('hasTelemetryCardFilters')).toBe(true)

    const setupState = getSetupState(wrapper)
    setupState.clearTelemetryCardFilters()
    await flushPromises()

    expect(getRealtimeView(wrapper).props('visibleTelemetryCount')).toBe(2)
    expect(getRealtimeView(wrapper).props('hasTelemetryCardFilters')).toBe(false)
  })

  it('applies realtime view model updates, toggles the display limit, and exports visible cards', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const realtimeView = getRealtimeView(wrapper)
    await realtimeView.vm.$emit('update:searchQuery', 'temperature')
    await realtimeView.vm.$emit('update:sortMode', 'name')
    await realtimeView.vm.$emit('toggle-display-limit')
    await flushPromises()

    expect(realtimeView.props('searchQuery')).toBe('temperature')
    expect(realtimeView.props('sortMode')).toBe('name')
    expect(getSetupState(wrapper).showAllTelemetryCards).toBe(true)

    const createObjectURL = vi.fn(() => 'blob:telemetry-export')
    const revokeObjectURL = vi.fn()
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
    expect(window.location.origin).not.toBe('null')
    await realtimeView.vm.$emit('export-csv')
    expect(createObjectURL).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:telemetry-export')
    expect(clickSpy).toHaveBeenCalledTimes(1)

    await wrapper.findAllComponents(ModalStub)[0].vm.$emit('update:show', false)
    expect(getSetupState(wrapper).showLogDialog).toBe(false)
    await wrapper.findAllComponents(ModalStub)[2].vm.$emit('update:show', false)
    expect(getSetupState(wrapper).showHistory).toBe(false)
    clickSpy.mockRestore()
    vi.unstubAllGlobals()
    expect(window.location.origin).not.toBe('null')
  })

  it('passes empty snapshot state to TelemetryRealtimeView when current telemetry returns no rows', async () => {
    hoisted.telemetryDataCurrent.mockResolvedValue({
      data: [],
      error: null
    })

    const wrapper = mountTelemetryPage()
    await flushPromises()

    expect(getRealtimeView(wrapper).props('telemetryLoadStatus')).toBe('empty')
    expect(getRealtimeView(wrapper).props('telemetryDataCount')).toBe(0)
    expect(getRealtimeView(wrapper).props('visibleTelemetryCount')).toBe(0)
    expect(hoisted.send).toHaveBeenCalledWith(JSON.stringify({ device_id: 'device-1', token: 'mock-token' }))
  })

  it('passes error snapshot state and backend message to TelemetryRealtimeView', async () => {
    hoisted.telemetryDataCurrent.mockResolvedValue({
      data: null,
      error: new Error('snapshot down')
    })

    const wrapper = mountTelemetryPage()
    await flushPromises()

    expect(getRealtimeView(wrapper).props('telemetryLoadStatus')).toBe('error')
    expect(getRealtimeView(wrapper).props('telemetryLoadError')).toBe('snapshot down')
    expect(getRealtimeView(wrapper).props('telemetryDataCount')).toBe(0)
    expect(hoisted.send).not.toHaveBeenCalled()
  })

  it('ignores non-JSON websocket messages', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const before = JSON.stringify(setupState.telemetryData)

    await hoisted.useWebSocketOptions.onMessage?.(
      {} as WebSocket,
      {
        data: 'not-a-json'
      } as MessageEvent
    )

    expect(JSON.stringify(setupState.telemetryData)).toBe(before)
  })

  it('preserves the old telemetry value when incoming value is null', async () => {
    vi.useFakeTimers()
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    await hoisted.useWebSocketOptions.onMessage?.(
      {} as WebSocket,
      {
        data: JSON.stringify({ temperature_1: null })
      } as MessageEvent
    )

    await vi.advanceTimersByTimeAsync(121)
    await flushPromises()

    expect(setupState.telemetryData[0].value).toBe(25)
  })

  it('appends unknown keys as new telemetry cards after the realtime flush', async () => {
    vi.useFakeTimers()
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const before = setupState.telemetryData.length

    await hoisted.useWebSocketOptions.onMessage?.(
      {} as WebSocket,
      {
        data: JSON.stringify({ new_sensor: 99, systime: '2026-06-21T02:00:00Z' })
      } as MessageEvent
    )

    await vi.advanceTimersByTimeAsync(121)
    await flushPromises()

    expect(setupState.telemetryData.length).toBe(before + 1)
    const added = setupState.telemetryData.find((telemetry: any) => telemetry.key === 'new_sensor')
    expect(added).toMatchObject({
      key: 'new_sensor',
      value: 99,
      ts: '2026-06-21T02:00:00Z'
    })
  })

  it('resets form state when opening the publish dialog', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formValue = 'old'
    setupState.form.expected = true
    setupState.form.time = 5

    setupState.openDialog()

    expect(setupState.showDialog).toBe(true)
    expect(setupState.formValue).toBe('')
    expect(setupState.form.expected).toBe(false)
    expect(setupState.form.time).toBeNull()
  })

  it('validates JSON and exposes error state for invalid payloads', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formValue = 'invalid-json'

    expect(setupState.validationJson).toBe('error')
    expect(setupState.inputFeedback).toBe('generate.inputRightJson')
  })

  it('clears validation errors for valid JSON payloads', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formValue = '{"valid":true}'

    expect(setupState.validationJson).toBeUndefined()
    expect(setupState.inputFeedback).toBe('')
  })

  it('calls expectMessageAdd when expected delivery is enabled', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.openDialog()
    setupState.formValue = '{"cmd":"on"}'
    setupState.form.expected = true
    setupState.form.time = 2

    await setupState.handlePositiveClick()
    await flushPromises()

    expect(hoisted.expectMessageAdd).toHaveBeenCalledWith(
      expect.objectContaining({
        device_id: 'device-1',
        payload: '{"cmd":"on"}',
        send_type: 'telemetry'
      })
    )
    expect(setupState.showDialog).toBe(false)
  })

  it('calls telemetryDataPub when expected delivery is disabled', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.openDialog()
    setupState.formValue = '{"cmd":"off"}'
    setupState.form.expected = false

    await setupState.handlePositiveClick()
    await flushPromises()

    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({
      device_id: 'device-1',
      value: '{"cmd":"off"}'
    })
    expect(setupState.showDialog).toBe(false)
  })

  it('keeps the publish dialog open when submit fails', async () => {
    hoisted.telemetryDataPub.mockResolvedValue({ error: { message: 'fail' } })

    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.openDialog()
    setupState.formValue = '{"cmd":"off"}'

    await setupState.handlePositiveClick()
    await flushPromises()

    expect(setupState.showDialog).toBe(true)
  })

  it('does not submit publish actions when the JSON payload is invalid', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.openDialog()
    setupState.formValue = 'not-json'

    await setupState.handlePositiveClick()
    await flushPromises()

    expect(hoisted.telemetryDataPub).not.toHaveBeenCalled()
    expect(hoisted.expectMessageAdd).not.toHaveBeenCalled()
  })

  it('publishes custom controls from the header and refreshes only realtime data while logs stay collapsed', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    vi.clearAllMocks()

    await getOperationsHeader(wrapper).vm.$emit('control-change', { id: 'c1', name: 'Start', content: '{"switch":true}' })
    await flushPromises()

    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({
      device_id: 'device-1',
      value: '{"switch":true}'
    })
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
    expect(hoisted.getTelemetryLogList).not.toHaveBeenCalled()
  })

  it('passes the current telemetry accent-color helper to the realtime view', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const accentColor = getRealtimeView(wrapper).props('telemetryAccentColor') as (telemetry: Record<string, any>) => string

    expect(accentColor({ value: 'text' })).toBe('#cccccc')
    expect(accentColor({ value: 123 })).toBe('')
  })

  it('deletes telemetry attributes through the realtime view event and refreshes realtime data', async () => {
    hoisted.telemetryDataDel.mockResolvedValue({ error: null })

    const wrapper = mountTelemetryPage()
    await flushPromises()
    vi.clearAllMocks()

    await getRealtimeView(wrapper).vm.$emit('delete-select', '1', { key: 'temp', device_id: 'device-1' })
    await flushPromises()

    expect(hoisted.telemetryDataDel).toHaveBeenCalledWith({
      key: 'temp',
      device_id: 'device-1'
    })
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
  })

  it('does not refresh logs or realtime data when delete fails', async () => {
    hoisted.telemetryDataDel.mockResolvedValue({ error: { message: 'delete failed' } })

    const wrapper = mountTelemetryPage()
    await flushPromises()
    await openOperationLogs(wrapper)
    vi.clearAllMocks()

    await getRealtimeView(wrapper).vm.$emit('delete-select', '1', { key: 'temp', device_id: 'device-1' })
    await flushPromises()

    expect(hoisted.telemetryDataCurrent).not.toHaveBeenCalled()
    expect(hoisted.getTelemetryLogList).not.toHaveBeenCalled()
  })

  it('hides the simulate entry when deviceData indicates a non-MQTT protocol', async () => {
    const wrapper = mountTelemetryPage({
      deviceData: {
        device_config: {
          protocol_type: 'HTTP'
        }
      }
    })

    await flushPromises()

    expect(getOperationsHeader(wrapper).props('showLog')).toBe(false)
  })

  it('shows the simulate entry when deviceData is missing', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    expect(getOperationsHeader(wrapper).props('showLog')).toBe(true)
  })

  it('closes the websocket when the page unmounts with an open socket', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    wrapper.unmount()

    expect(hoisted.close).toHaveBeenCalledTimes(1)
  })

  it('loads controls only when the header asks for them', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    vi.clearAllMocks()

    await getOperationsHeader(wrapper).vm.$emit('load-controls')
    await flushPromises()

    expect(hoisted.deviceCustomControlList).toHaveBeenCalledWith({
      device_template_id: 'tpl-1',
      page: 1,
      page_size: 100,
      enable_status: 'enable'
    })
    expect(getOperationsHeader(wrapper).props('controlListLoaded')).toBe(true)
    expect((getOperationsHeader(wrapper).props('controlList') as Array<unknown>).length).toBe(1)
  })

  it('does not load controls when deviceTemplateId is empty even if the header requests it', async () => {
    const wrapper = mountTelemetryPage({ deviceTemplateId: '' })
    await flushPromises()
    vi.clearAllMocks()

    await getOperationsHeader(wrapper).vm.$emit('load-controls')
    await flushPromises()

    expect(hoisted.deviceCustomControlList).not.toHaveBeenCalled()
  })

  it('resets the loaded control list on template changes and reloads only after another explicit request', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    await getOperationsHeader(wrapper).vm.$emit('load-controls')
    await flushPromises()

    expect(getOperationsHeader(wrapper).props('controlListLoaded')).toBe(true)

    vi.clearAllMocks()
    await wrapper.setProps({ deviceTemplateId: 'tpl-2' })
    await flushPromises()

    expect(getOperationsHeader(wrapper).props('controlListLoaded')).toBe(false)
    expect(getOperationsHeader(wrapper).props('controlList')).toEqual([])
    expect(hoisted.deviceCustomControlList).not.toHaveBeenCalled()

    await getOperationsHeader(wrapper).vm.$emit('load-controls')
    await flushPromises()

    expect(hoisted.deviceCustomControlList).toHaveBeenCalledWith({
      device_template_id: 'tpl-2',
      page: 1,
      page_size: 100,
      enable_status: 'enable'
    })
  })

  it('opens operation logs lazily when the user clicks the log button', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    vi.clearAllMocks()

    await openOperationLogs(wrapper)

    expect(hoisted.getTelemetryLogList).toHaveBeenCalledWith({
      page: 1,
      page_size: 5,
      device_id: 'device-1',
      operation_type: '',
      status: ''
    })
  })

  it('re-queries logs when operationType changes after the log section is opened', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    await openOperationLogs(wrapper)
    vi.clearAllMocks()

    const selects = wrapper.findAllComponents(SelectStub)
    expect(selects).toHaveLength(2)

    await selects[0].vm.$emit('update:value', '1')
    await flushPromises()

    expect(hoisted.getTelemetryLogList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        operation_type: '1'
      })
    )
  })

  it('re-queries logs when sendResult changes after the log section is opened', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    await openOperationLogs(wrapper)
    vi.clearAllMocks()

    const selects = wrapper.findAllComponents(SelectStub)
    await selects[1].vm.$emit('update:value', '2')
    await flushPromises()

    expect(hoisted.getTelemetryLogList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        status: '2'
      })
    )
  })

  it('maps log status to success, fail, and pending labels', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const statusColumn = setupState.columns.find((column: any) => column.key === 'status')

    expect(statusColumn.render({ status: '1' })).toBe('custom.devicePage.success')
    expect(statusColumn.render({ status: '2' })).toBe('custom.devicePage.fail')
    expect(statusColumn.render({ status: '' })).toBe('page.expect.pending')
    expect(statusColumn.render({ status: null })).toBe('page.expect.pending')
    expect(statusColumn.render({})).toBe('page.expect.pending')
  })

  it('updates pagination and re-queries logs through the rendered pager', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    await openOperationLogs(wrapper)
    vi.clearAllMocks()

    await wrapper.getComponent(PaginationStub).vm.$emit('update:page', 4)
    await flushPromises()

    expect(hoisted.getTelemetryLogList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 4
      })
    )
  })

  it('opens the publish and simulation dialogs from TelemetryOperationsHeader events', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    await getOperationsHeader(wrapper).vm.$emit('publish')
    await flushPromises()

    expect(wrapper.findAllComponents(ModalStub)[1].props('show')).toBe(true)

    await getOperationsHeader(wrapper).vm.$emit('simulate')
    await flushPromises()

    expect(wrapper.findAllComponents(ModalStub)[0].props('show')).toBe(true)
    expect(hoisted.getSimulationInit).toHaveBeenCalledWith({ device_id: 'device-1' })
  })

  it('opens the time-series dialog when TelemetryRealtimeView emits a numeric sequence event', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    await getRealtimeView(wrapper).vm.$emit('sequence', {
      key: 'temperature_1',
      label: 'Temperature',
      value: 23,
      device_id: 'device-1',
      unit: 'C'
    })
    await flushPromises()

    expect(wrapper.findAllComponents(ModalStub)[2].props('show')).toBe(true)
    expect(wrapper.getComponent(TimeSeriesDataStub).props()).toMatchObject({
      deviceId: 'device-1',
      theKey: 'temperature_1',
      theName: 'Temperature',
      theUnit: 'C'
    })
  })

  it('does not open the time-series dialog when TelemetryRealtimeView emits a non-numeric sequence event', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    await getRealtimeView(wrapper).vm.$emit('sequence', {
      key: 'status',
      label: 'Status',
      value: 'online',
      device_id: 'device-1',
      unit: ''
    })
    await flushPromises()

    expect(wrapper.findAllComponents(ModalStub)[2].props('show')).toBe(false)
    expect(wrapper.findComponent(TimeSeriesDataStub).exists()).toBe(false)
  })

  it('opens the history dialog when TelemetryRealtimeView emits a history event', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()

    await getRealtimeView(wrapper).vm.$emit('history', {
      key: 'status',
      label: 'Status',
      value: 'online',
      device_id: 'device-1',
      unit: ''
    })
    await flushPromises()

    expect(wrapper.findAllComponents(ModalStub)[2].props('show')).toBe(true)
    expect(wrapper.getComponent(HistoryDataStub).props()).toMatchObject({
      deviceId: 'device-1',
      theKey: 'status',
      theName: 'Status',
      theUnit: ''
    })
  })

  it('refreshes both logs and realtime data when a custom control is sent while logs are visible', async () => {
    const wrapper = mountTelemetryPage()
    await flushPromises()
    await openOperationLogs(wrapper)
    vi.clearAllMocks()

    await getOperationsHeader(wrapper).vm.$emit('control-change', { id: 'c1', content: '{"switch":true}' })
    await flushPromises()

    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({
      device_id: 'device-1',
      value: '{"switch":true}'
    })
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('device-1')
    expect(hoisted.getTelemetryLogList).toHaveBeenCalledTimes(1)
  })
})
