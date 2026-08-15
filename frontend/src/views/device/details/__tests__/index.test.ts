/**
 * 文件用途: index 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h, nextTick, reactive, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => {
  const createViewStub = (name: string) => ({
    name,
    render() {
      return null
    }
  })

  return {
    mockSend: vi.fn(),
    mockDeviceDetail: vi.fn(),
    mockDeviceUpdate: vi.fn(),
    mockRouterBack: vi.fn(),
    mockRouterReplace: vi.fn(),
    mockRouterPushByKey: vi.fn(),
    mockMessageError: vi.fn(),
    mockGetCachedDeviceTemplateDetail: vi.fn(),
    mockHasThingsVisChartContent: vi.fn(),
    routeRaw: {
      query: {
        d_id: 'device-1'
      }
    },
    route: null as any,
    appStore: {
      locale: 'zh-CN'
    },
    createViewStub
  }
})

const mockSend = hoisted.mockSend
const mockDeviceDetail = hoisted.mockDeviceDetail
const mockDeviceUpdate = hoisted.mockDeviceUpdate
const mockRouterBack = hoisted.mockRouterBack
const mockRouterReplace = hoisted.mockRouterReplace
const mockRouterPushByKey = hoisted.mockRouterPushByKey
const mockMessageError = hoisted.mockMessageError
const mockGetCachedDeviceTemplateDetail = hoisted.mockGetCachedDeviceTemplateDetail
const mockHasThingsVisChartContent = hoisted.mockHasThingsVisChartContent
const getRoute = () =>
  hoisted.route as { query: { d_id: string; tab?: string; shared?: string; access?: string } }
const appStore = hoisted.appStore

vi.mock('vue-router', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-router')>()
  const { reactive } = await import('vue')
  if (!hoisted.route) {
    hoisted.route = reactive(hoisted.routeRaw)
  }
  return {
    ...actual,
    useRoute: () => hoisted.route,
    useRouter: () => ({
      back: hoisted.mockRouterBack,
      replace: hoisted.mockRouterReplace
    })
  }
})

vi.mock('@aetherlink/hooks', () => {
  const loading = ref(false)
  return {
    useLoading: () => ({
      loading,
      startLoading: vi.fn(() => {
        loading.value = true
      }),
      endLoading: vi.fn(() => {
        loading.value = false
      })
    })
  }
})

vi.mock('@vueuse/core', () => ({
  useWebSocket: () => ({
    send: hoisted.mockSend
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => hoisted.appStore
}))

vi.mock('@/service/api/device', () => ({
  deviceDetail: hoisted.mockDeviceDetail,
  deviceUpdate: hoisted.mockDeviceUpdate
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: vi.fn((key: string) => {
      if (key === 'token') return 'mock-token'
      return null
    })
  }
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    routerPushByKey: hoisted.mockRouterPushByKey
  })
}))

vi.mock('@/utils/common/tool', () => ({
  getWebsocketServerUrl: () => 'ws://socket.test'
}))

vi.mock('@/utils/thingsvis/template-presets', () => ({
  hasThingsVisChartContent: hoisted.mockHasThingsVisChartContent
}))

vi.mock('@/utils/thingsvis/template-detail-cache', () => ({
  getCachedDeviceTemplateDetail: hoisted.mockGetCachedDeviceTemplateDetail
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn()
  })
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    error: hoisted.mockMessageError
  }
}))

vi.mock('@/views/device/details/modules/telemetry/telemetry.vue', () => ({
  default: hoisted.createViewStub('TelemetryView')
}))
vi.mock('@/views/device/details/modules/telemetry-chart.vue', () => ({
  default: hoisted.createViewStub('TelemetryChartView')
}))
vi.mock('@/views/device/details/modules/join.vue', () => ({
  default: hoisted.createViewStub('JoinView')
}))
vi.mock('@/views/device/details/modules/device-analysis.vue', () => ({
  default: hoisted.createViewStub('DeviceAnalysisView')
}))
vi.mock('@/views/device/details/modules/message.vue', () => ({
  default: hoisted.createViewStub('MessageView')
}))
vi.mock('@/views/device/details/modules/stats.vue', () => ({
  default: hoisted.createViewStub('StatsView')
}))
vi.mock('@/views/device/details/modules/event-report.vue', () => ({
  default: hoisted.createViewStub('EventReportView')
}))
vi.mock('@/views/device/details/modules/command-delivery.vue', () => ({
  default: hoisted.createViewStub('CommandDeliveryView')
}))
vi.mock('@/views/device/details/modules/expect-message.vue', () => ({
  default: hoisted.createViewStub('ExpectMessageView')
}))
vi.mock('@/views/device/details/modules/automate.vue', () => ({
  default: hoisted.createViewStub('AutomateView')
}))
vi.mock('@/views/device/details/modules/give-an-alarm.vue', () => ({
  default: hoisted.createViewStub('GiveAlarmView')
}))
vi.mock('@/views/device/details/modules/settings.vue', () => ({
  default: hoisted.createViewStub('SettingsView')
}))
vi.mock('@/views/device/details/modules/device-status.vue', () => ({
  default: hoisted.createViewStub('DeviceStatusHistoryView')
}))
vi.mock('@/views/device/details/modules/device-diagnosis.vue', () => ({
  default: hoisted.createViewStub('DeviceDiagnosisView')
}))
vi.mock('@/views/device/details/modules/RdiDeviceOperationsView.vue', () => ({
  default: hoisted.createViewStub('RdiDeviceOperationsView')
}))
vi.mock('@/views/device/details/modules/RdiDeviceHistoryView.vue', () => ({
  default: hoisted.createViewStub('RdiDeviceHistoryView')
}))
vi.mock('@/views/device/details/modules/RdiDeviceDetailsView.vue', () => ({
  default: hoisted.createViewStub('RdiDeviceDetailsView')
}))

import DeviceDetails from '../index.vue'

const ButtonStub = defineComponent({
  name: 'ButtonStub',
  props: {
    loading: Boolean,
    type: String
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.loading,
          'data-type': props.type || '',
          onClick: () => emit('click')
        },
        slots.default ? slots.default() : []
      )
  }
})

const InputStub = defineComponent({
  name: 'InputStub',
  props: {
    value: {
      type: String,
      default: ''
    }
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.value,
        onInput: (event: Event) => {
          emit('update:value', (event.target as HTMLInputElement).value)
        }
      })
  }
})

const ModalStub = defineComponent({
  name: 'ModalStub',
  props: {
    show: Boolean,
    title: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () => (props.show ? h('div', { 'data-modal-title': props.title }, slots.default ? slots.default() : []) : null)
  }
})

const TabsStub = defineComponent({
  name: 'TabsStub',
  props: {
    value: {
      type: String,
      default: ''
    }
  },
  emits: ['update:value'],
  setup(_props, { slots }) {
    return () => h('div', { class: 'tabs-stub' }, slots.default ? slots.default() : [])
  }
})

const TabPaneStub = defineComponent({
  name: 'TabPaneStub',
  props: {
    tab: {
      type: String,
      default: ''
    },
    name: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () =>
      h('section', { 'data-tab-name': props.name }, [
        h('header', { class: 'tab-label' }, props.tab),
        ...(slots.default ? slots.default() : [])
      ])
  }
})

const DynamicTagsStub = defineComponent({
  name: 'DynamicTagsStub',
  props: {
    value: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:value'],
  setup(props) {
    return () => h('div', { class: 'dynamic-tags-stub' }, JSON.stringify(props.value))
  }
})

const baseStubs = {
  NButton: ButtonStub,
  'n-button': ButtonStub,
  NModal: ModalStub,
  'n-modal': ModalStub,
  NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  'n-card': defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  'n-form': defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  'n-form-item': defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInput: InputStub,
  'n-input': InputStub,
  NDynamicTags: DynamicTagsStub,
  'n-dynamic-tags': DynamicTagsStub,
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSpin: defineComponent({
    props: { show: Boolean },
    setup(_props, { slots }) {
      return () => h('div', { class: 'spin-stub' }, slots.default ? slots.default() : [])
    }
  }),
  'n-spin': defineComponent({
    props: { show: Boolean },
    setup(_props, { slots }) {
      return () => h('div', { class: 'spin-stub' }, slots.default ? slots.default() : [])
    }
  }),
  NTabs: TabsStub,
  'n-tabs': TabsStub,
  NTabPane: TabPaneStub,
  'n-tab-pane': TabPaneStub,
  SvgIcon: defineComponent({
    props: {
      localIcon: {
        type: String,
        default: ''
      }
    },
    setup(props) {
      return () => h('i', { 'data-icon': props.localIcon })
    }
  }),
  NH3: defineComponent({ setup(_, { slots }) { return () => h('h3', slots.default ? slots.default() : []) } }),
  DeviceStatusHistory: defineComponent({
    props: {
      visible: Boolean,
      deviceId: {
        type: String,
        default: ''
      }
    },
    emits: ['update:visible'],
    setup(props) {
      return () => h('div', { 'data-status-history': props.deviceId })
    }
  })
}

const createDeviceDetailPayload = (overrides: Record<string, any> = {}) => ({
  id: 'device-1',
  name: 'Cold Room',
  device_number: 'ABCDEFGHIJKL',
  is_online: 1,
  label: 'north,freezer',
  description: 'primary freezer',
  warn_status: 'N',
  device_config_id: 'cfg-1',
  device_config_name: 'Config A',
  device_config: {
    device_type: '2',
    device_template_id: 'tpl-1'
  },
  additional_info: '{}',
  ...overrides
})

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountDeviceDetails = () => {
  const wrapper = shallowMount(DeviceDetails, {
    global: {
      stubs: baseStubs,
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

afterEach(() => {
  while (mountedWrappers.length > 0) {
    mountedWrappers.pop()?.unmount()
  }
})

describe('device/details/index.vue', () => {
  beforeEach(() => {
    mockSend.mockReset()
    mockDeviceDetail.mockReset()
    mockDeviceUpdate.mockReset()
    mockRouterBack.mockReset()
    mockRouterReplace.mockReset()
    mockRouterPushByKey.mockReset()
    mockMessageError.mockReset()
    mockGetCachedDeviceTemplateDetail.mockReset()
    mockHasThingsVisChartContent.mockReset()
    getRoute().query.d_id = 'device-1'
    delete getRoute().query.tab
    delete getRoute().query.shared
    delete getRoute().query.access
    appStore.locale = 'zh-CN'
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload()
    })
    mockDeviceUpdate.mockResolvedValue({ error: null })
    mockGetCachedDeviceTemplateDetail.mockResolvedValue({
      data: {
        web_chart_config: { blocks: [1] }
      }
    })
    mockHasThingsVisChartContent.mockReturnValue(true)
  })

  it('activates a chart deep link after deferred chart capability resolution restores the tab', async () => {
    getRoute().query.tab = 'chart'

    let resolveTemplateDetail!: (value: unknown) => void
    mockGetCachedDeviceTemplateDetail.mockReturnValue(new Promise(resolve => {
      resolveTemplateDetail = resolve
    }))

    const wrapper = mountDeviceDetails()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.tabValue).toBe('chart')
    expect(wrapper.text()).not.toContain('custom.device_details.chart')

    resolveTemplateDetail({
      data: {
        web_chart_config: { blocks: [1] }
      }
    })
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('custom.device_details.history')
    expect(setupState.tabValue).toBe('chart')
  })

  it('shows only supported tabs for a non-RDI device and subscribes websocket with the current device id', async () => {
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload({
        device_number: 'short-id',
        device_config_name: '',
        device_config: null,
        additional_info: '{}'
      })
    })

    const wrapper = mountDeviceDetails()

    await flushPromises()

    const renderedText = wrapper.text()
    expect(renderedText).toContain('custom.device_details.telemetry')
    expect(renderedText).not.toContain('RDI')
    expect(renderedText).not.toContain('custom.device_details.chart')
    expect(renderedText).not.toContain('custom.device_details.subdevice')
    expect(mockSend).toHaveBeenCalledWith(JSON.stringify({ device_id: 'device-1', token: 'mock-token' }))
  })

  it('keeps only the four customer RDI tabs and refreshes when route id changes', async () => {
    mockDeviceDetail
      .mockResolvedValueOnce({
        error: null,
        data: createDeviceDetailPayload({
          id: 'device-1',
          name: 'Gateway A'
        })
      })
      .mockResolvedValueOnce({
        error: null,
        data: createDeviceDetailPayload({
          id: 'device-2',
          name: 'Gateway B'
        })
      })
      .mockResolvedValue({
        error: null,
        data: createDeviceDetailPayload({
          id: 'device-2',
          name: 'Gateway B'
        })
      })

    const wrapper = mountDeviceDetails()

    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.components.map((item: { key: string }) => item.key)).toEqual([
      'message',
      'chart',
      'give-an-alarm',
      'rdi'
    ])
    expect(wrapper.text()).toContain('custom.device_details.history')
    expect(wrapper.text()).not.toContain('custom.device_details.subdevice')
    expect(wrapper.text()).toContain('custom.device_details.rdiCurrentParameterSettings')

    getRoute().query.d_id = 'device-2'
    await flushPromises()
    await flushPromises()

    expect(mockDeviceDetail).toHaveBeenNthCalledWith(1, 'device-1')
    expect(mockDeviceDetail).toHaveBeenNthCalledWith(2, 'device-2')
    expect(mockSend).toHaveBeenLastCalledWith(JSON.stringify({ device_id: 'device-2', token: 'mock-token' }))
    expect(wrapper.text()).toContain('Gateway B')
  })

  it('defaults to the RDI message tab for an RDI-capable device when no route tab is requested', async () => {
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload()
    })

    const wrapper = mountDeviceDetails()

    await flushPromises()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.tabValue).toBe('message')
    expect(setupState.components.map((item: { key: string }) => item.key)).toEqual([
      'message',
      'chart',
      'give-an-alarm',
      'rdi'
    ])
    expect(wrapper.text()).toContain('custom.device_details.rdiDetailedInfo')
    expect(wrapper.text()).toContain('custom.device_details.history')
    expect(wrapper.text()).toContain('custom.device_details.rdiAlarmInfo')
    expect(wrapper.text()).toContain('custom.device_details.rdiCurrentParameterSettings')
  })

  it('keeps the RDI history tab visible even when template chart capability is unavailable', async () => {
    mockGetCachedDeviceTemplateDetail.mockResolvedValue({
      data: {
        web_chart_config: null
      }
    })
    mockHasThingsVisChartContent.mockReturnValue(false)
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload({
        has_chart_config: false,
        device_config: {
          device_type: '2',
          device_template_id: ''
        }
      })
    })

    const wrapper = mountDeviceDetails()

    await flushPromises()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.tabValue).toBe('message')
    expect(wrapper.text()).toContain('custom.device_details.rdiDetailedInfo')
    expect(wrapper.text()).toContain('custom.device_details.history')
    expect(wrapper.text()).toContain('custom.device_details.rdiAlarmInfo')
    expect(wrapper.text()).toContain('custom.device_details.rdiCurrentParameterSettings')
    const chartTab = setupState.components.find((item: { key: string }) => item.key === 'chart')
    const chartView = await chartTab.component.__asyncLoader()
    expect(chartView.default.name).toBe('RdiDeviceHistoryView')
    const messageTab = setupState.components.find((item: { key: string }) => item.key === 'message')
    const detailsView = await messageTab.component.__asyncLoader()
    expect(detailsView.default.name).toBe('RdiDeviceDetailsView')
  })

  it('shows validation errors before saving invalid device edits', async () => {
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload({
        name: '',
        device_number: ''
      })
    })

    const wrapper = mountDeviceDetails()

    await flushPromises()

    const editButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'common.edit')

    expect(wrapper.text()).not.toContain('common.save')
    expect(editButton?.text()).toBe('common.edit')

    await editButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('common.save')
    expect(wrapper.text()).toContain('generate.modify-device-info')

    const saveButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'common.save')

    expect(saveButton?.text()).toBe('common.save')

    await saveButton!.trigger('click')

    expect(mockMessageError).toHaveBeenCalledWith('custom.devicePage.enterDeviceName')
    expect(mockDeviceUpdate).toHaveBeenCalledTimes(0)
  })

  it('renders an accepted share as read-only and exposes only detail and history tabs', async () => {
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload({
        shared_read_only: true,
        device_config: {
          device_type: '1',
          device_template_id: 'tpl-rdi'
        }
      })
    })

    const wrapper = mountDeviceDetails()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.isSharedReadOnly).toBe(true)
    expect(setupState.visibleDetailComponents.map((item: { key: string }) => item.key)).toEqual([
      'message',
      'chart'
    ])
    expect(wrapper.text()).toContain('script.readonly')
    expect(wrapper.text()).not.toContain('common.edit')
    expect(wrapper.text()).not.toContain('custom.device_details.rdiAlarmInfo')
    expect(wrapper.text()).not.toContain('custom.device_details.rdiCurrentParameterSettings')

    setupState.editConfig()
    setupState.clickConfig()
    await setupState.save()
    expect(setupState.showDialog).toBe(false)
    expect(mockRouterPushByKey).not.toHaveBeenCalled()
    expect(mockDeviceUpdate).not.toHaveBeenCalled()
  })

  it('does not mount the writable generic message tab for a non-RDI shared device', async () => {
    mockDeviceDetail.mockResolvedValue({
      error: null,
      data: createDeviceDetailPayload({
        shared_read_only: true,
        device_number: 'generic-device',
        additional_info: '{}',
        has_chart_config: true,
        device_config: {
          device_type: '1',
          device_template_id: 'tpl-generic'
        }
      })
    })

    const wrapper = mountDeviceDetails()
    await flushPromises()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.visibleDetailComponents.map((item: { key: string }) => item.key)).toEqual(['chart'])
    expect(setupState.components.some((item: { key: string }) => item.key === 'message')).toBe(true)
    expect(wrapper.text()).not.toContain('custom.device_details.AdditionalDetails')
  })

  it('discards an older device response after the route switches to a shared device', async () => {
    const firstRequest = createDeferred<{ error: null; data: Record<string, any> }>()
    const secondRequest = createDeferred<{ error: null; data: Record<string, any> }>()
    mockDeviceDetail
      .mockImplementationOnce(() => firstRequest.promise)
      .mockImplementationOnce(() => secondRequest.promise)

    const wrapper = mountDeviceDetails()
    expect(mockDeviceDetail).toHaveBeenCalledWith('device-1')

    getRoute().query.d_id = 'device-2'
    await nextTick()
    await nextTick()
    expect(mockDeviceDetail).toHaveBeenCalledWith('device-2')

    secondRequest.resolve({
      error: null,
      data: createDeviceDetailPayload({
        id: 'device-2',
        name: 'Accepted share',
        shared_read_only: true
      })
    })
    await flushPromises()

    let setupState = getSetupState(wrapper)
    expect(setupState.loadedDeviceId).toBe('device-2')
    expect(setupState.deviceData.id).toBe('device-2')
    expect(setupState.isSharedReadOnly).toBe(true)

    firstRequest.resolve({
      error: null,
      data: createDeviceDetailPayload({ id: 'device-1', name: 'Owned device' })
    })
    await flushPromises()

    setupState = getSetupState(wrapper)
    expect(setupState.loadedDeviceId).toBe('device-2')
    expect(setupState.deviceData.id).toBe('device-2')
    expect(wrapper.text()).toContain('Accepted share')
    expect(wrapper.text()).not.toContain('Owned device')
    expect(wrapper.text()).not.toContain('common.edit')

    await setupState.save()
    expect(mockDeviceUpdate).not.toHaveBeenCalled()
    expect(mockSend).toHaveBeenCalledTimes(1)
    expect(mockSend).toHaveBeenCalledWith(JSON.stringify({ device_id: 'device-2', token: 'mock-token' }))
  })

  it('keeps the last successful detail state when refresh fails', async () => {
    const wrapper = mountDeviceDetails()
    await flushPromises()

    expect(wrapper.text()).toContain('Cold Room')
    expect(wrapper.text()).toContain('custom.device_details.history')

    mockSend.mockClear()
    mockDeviceDetail.mockResolvedValueOnce({
      error: new Error('network'),
      data: null
    })

    const setupState = getSetupState(wrapper)
    await setupState.refreshCurrentTab()
    await flushPromises()

    expect(wrapper.text()).toContain('Cold Room')
    expect(wrapper.text()).toContain('custom.device_details.history')
    expect(mockSend).toHaveBeenCalledTimes(0)
  })

  it('uses the real header back action to leave the device detail page', async () => {
    const wrapper = mountDeviceDetails()
    await flushPromises()

    const backButton = wrapper.findAll('button').find(button => button.text() === 'common.back')
    expect(backButton).toBeDefined()
    await backButton!.trigger('click')

    expect(mockRouterBack).toHaveBeenCalledTimes(1)
  })
})
