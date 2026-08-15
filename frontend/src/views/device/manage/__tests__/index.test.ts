/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  route: {
    path: '/device/manage',
    query: {
      service_identifier: '',
      service_access_id: '',
      page: 1,
      page_size: 10
    }
  },
  routerPush: vi.fn(),
  routerPushByKey: vi.fn(),
  deviceDictProtocolServiceFirstLevel: vi.fn(),
  deviceDictProtocolServiceSecondLevel: vi.fn(),
  deviceGroupRelation: vi.fn(),
  deviceGroupTree: vi.fn(),
  getDeviceConfigList: vi.fn(),
  deviceList: vi.fn(),
  listFleetSavedFilters: vi.fn(),
  createFleetSavedFilter: vi.fn(),
  deleteFleetSavedFilter: vi.fn(),
  updateFleetSavedFilter: vi.fn(),
  deviceConnectForm: vi.fn(),
  checkDevice: vi.fn(),
  activateRdiDevice: vi.fn(),
  wsConnect: vi.fn(),
  wsDisconnect: vi.fn(),
  loggerError: vi.fn(),
  createModuleStub: (name: string) => ({
    name,
    render() {
      return null
    }
  })
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => hoisted.route,
    useRouter: () => ({
      push: hoisted.routerPush
    })
  }
})

vi.mock('lodash-es', () => ({
  debounce: (fn: (...args: any[]) => any) => {
    const wrapped = (...args: any[]) => fn(...args)
    ;(wrapped as any).cancel = vi.fn()
    return wrapped
  }
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: vi.fn((key: string) => {
      if (key === 'lang') return 'en-US'
      if (key === 'token') return 'mock-token'
      return null
    })
  }
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    error: hoisted.loggerError
  })
}))

vi.mock('@/utils/deviceStatusWebSocket', () => ({
  useDeviceStatusWebSocket: () => ({
    connect: hoisted.wsConnect,
    disconnect: hoisted.wsDisconnect,
    updateSubscription: vi.fn(),
    getStatus: vi.fn()
  })
}))

vi.mock('@/service/api/device', () => ({
  checkDevice: hoisted.checkDevice,
  deviceConnectForm: hoisted.deviceConnectForm,
  deviceDictProtocolServiceFirstLevel: hoisted.deviceDictProtocolServiceFirstLevel,
  deviceDictProtocolServiceSecondLevel: hoisted.deviceDictProtocolServiceSecondLevel,
  deviceGroupRelation: hoisted.deviceGroupRelation,
  deviceGroupTree: hoisted.deviceGroupTree,
  deviceList: hoisted.deviceList,
  getDeviceConfigList: hoisted.getDeviceConfigList,
  listFleetSavedFilters: hoisted.listFleetSavedFilters,
  createFleetSavedFilter: hoisted.createFleetSavedFilter,
  deleteFleetSavedFilter: hoisted.deleteFleetSavedFilter,
  updateFleetSavedFilter: hoisted.updateFleetSavedFilter
}))

vi.mock('@/service/api/rdi', () => ({
  activateRdiDevice: hoisted.activateRdiDevice
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    routerPushByKey: hoisted.routerPushByKey
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/views/device/manage/modules/add-devices-step1.vue', () => ({
  default: hoisted.createModuleStub('AddDevicesStep1Stub')
}))
vi.mock('@/views/device/manage/modules/add-devices-step2.vue', () => ({
  default: hoisted.createModuleStub('AddDevicesStep2Stub')
}))
vi.mock('@/views/device/manage/modules/add-devices-step3.vue', () => ({
  default: hoisted.createModuleStub('AddDevicesStep3Stub')
}))

import DeviceManage from '../index.vue'
import deviceManageSource from '../index.vue?raw'

const ButtonStub = defineComponent({
  name: 'ButtonStub',
  props: {
    disabled: Boolean,
    type: {
      type: String,
      default: ''
    }
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          'data-type': props.type,
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
        onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
      })
  }
})

const DataTablePageStub = defineComponent({
  name: 'DataTablePageStub',
  props: {
    fetchData: {
      type: Function,
      required: true
    },
    topActions: {
      type: Array,
      default: () => []
    }
  },
  emits: ['params-update'],
  setup(props, { expose }) {
    const dataList = ref([{ id: 'device-1', is_online: 0 }])
    const handleSearch = vi.fn()
    const forceChangeParamsByKey = vi.fn()

    expose({
      dataList,
      handleSearch,
      forceChangeParamsByKey
    })

    void (props.fetchData as any)({ page: 1, page_size: 10 })

    return () =>
      h('div', { class: 'data-table-page-stub' }, [
        h(
          'div',
          { class: 'data-table-page-top-actions' },
          (props.topActions as Array<{ element: () => any }>).map((action, index) =>
            h('div', { class: 'top-action-item', key: index }, [h(action.element)])
          )
        )
      ])
  }
})

const DropdownStub = defineComponent({
  name: 'DropdownStub',
  props: {
    options: {
      type: Array,
      default: () => []
    },
    trigger: {
      type: String,
      default: 'hover'
    }
  },
  emits: ['select'],
  setup(props, { emit, slots }) {
    return () =>
      h('div', { class: 'dropdown-stub', 'data-trigger': props.trigger }, [
        slots.default ? slots.default() : null,
        h(
          'div',
          { class: 'dropdown-options' },
          props.options.map((option: any) =>
            h(
              'button',
              {
                class: 'dropdown-option',
                disabled: option.disabled,
                onClick: () => emit('select', option.key)
              },
              typeof option.label === 'function' ? option.label() : option.label
            )
          )
        )
      ])
  }
})

const baseStubs = {
  'data-table-page': DataTablePageStub,
  'n-button': ButtonStub,
  NButton: ButtonStub,
  NButtonGroup: defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'button-group-stub' }, slots.default ? slots.default() : [])
    }
  }),
  'n-input': InputStub,
  NInput: InputStub,
  'n-dropdown': DropdownStub,
  NDropdown: DropdownStub,
  'n-alert': defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'alert-stub' }, slots.default ? slots.default() : [])
    }
  }),
  NAlert: defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'alert-stub' }, slots.default ? slots.default() : [])
    }
  }),
  'n-drawer': defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  'n-drawer-content': defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  'n-steps': defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  'n-step': defineComponent({
    setup() {
      return () => h('div')
    }
  }),
  'n-card': defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  'n-h4': defineComponent({
    setup(_, { slots }) {
      return () => h('h4', slots.default ? slots.default() : [])
    }
  }),
  'n-li': defineComponent({
    setup(_, { slots }) {
      return () => h('li', slots.default ? slots.default() : [])
    }
  }),
  NText: defineComponent({
    setup(_, { slots }) {
      return () => h('span', slots.default ? slots.default() : [])
    }
  }),
  NSpace: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  'n-flex': defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'flex-stub' }, slots.default ? slots.default() : [])
    }
  }),
  NFlex: defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'flex-stub' }, slots.default ? slots.default() : [])
    }
  }),
  'n-modal': defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'modal-stub' }, slots.default ? slots.default() : [])
    }
  }),
  NModal: defineComponent({
    setup(_, { slots }) {
      return () => h('div', { class: 'modal-stub' }, slots.default ? slots.default() : [])
    }
  }),
  NTag: defineComponent({
    setup(_, { slots }) {
      return () => h('span', slots.default ? slots.default() : [])
    }
  }),
  'n-tree-select': defineComponent({
    setup() {
      return () => h('div', { class: 'tree-select-stub' })
    }
  }),
  NTreeSelect: defineComponent({
    setup() {
      return () => h('div', { class: 'tree-select-stub' })
    }
  })
}

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountDeviceManage = () => {
  const wrapper = shallowMount(DeviceManage, {
    global: {
      stubs: baseStubs
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>
const getTableRef = (wrapper: ReturnType<typeof shallowMount>) => getSetupState(wrapper).tablePageRef

afterEach(() => {
  while (mountedWrappers.length > 0) {
    mountedWrappers.pop()?.unmount()
  }
})

describe('device/manage/index.vue', () => {
  beforeEach(() => {
    window.localStorage.clear()
    hoisted.route.query.service_identifier = ''
    hoisted.route.query.service_access_id = ''
    hoisted.route.query.page = 1
    hoisted.route.query.page_size = 10

    hoisted.routerPush.mockReset()
    hoisted.routerPushByKey.mockReset()
    hoisted.deviceDictProtocolServiceFirstLevel.mockReset()
    hoisted.deviceDictProtocolServiceSecondLevel.mockReset()
    hoisted.deviceGroupRelation.mockReset()
    hoisted.deviceGroupTree.mockReset()
    hoisted.getDeviceConfigList.mockReset()
    hoisted.deviceList.mockReset()
    hoisted.listFleetSavedFilters.mockReset()
    hoisted.createFleetSavedFilter.mockReset()
    hoisted.deleteFleetSavedFilter.mockReset()
    hoisted.updateFleetSavedFilter.mockReset()
    hoisted.deviceConnectForm.mockReset()
    hoisted.checkDevice.mockReset()
    hoisted.activateRdiDevice.mockReset()
    hoisted.wsConnect.mockReset()
    hoisted.wsDisconnect.mockReset()
    hoisted.loggerError.mockReset()

    hoisted.deviceDictProtocolServiceFirstLevel.mockResolvedValue({
      data: {
        protocol: [],
        service: []
      }
    })
    hoisted.deviceDictProtocolServiceSecondLevel.mockResolvedValue({
      data: {
        list: [],
        total: 0
      }
    })
    hoisted.deviceGroupRelation.mockResolvedValue({ data: [] })
    hoisted.deviceGroupTree.mockResolvedValue({ data: [] })
    hoisted.getDeviceConfigList.mockResolvedValue({
      data: {
        list: []
      }
    })
    hoisted.deviceList.mockResolvedValue({
      data: {
        list: [],
        total: 0
      }
    })
    hoisted.listFleetSavedFilters.mockResolvedValue({
      data: {
        list: []
      }
    })
    hoisted.createFleetSavedFilter.mockRejectedValue(new Error('saved filter API unavailable in this test'))
    hoisted.deleteFleetSavedFilter.mockResolvedValue({})
    hoisted.updateFleetSavedFilter.mockResolvedValue({})
    hoisted.deviceConnectForm.mockResolvedValue({ data: {} })
    hoisted.checkDevice.mockResolvedValue({
      error: null,
      data: {
        is_available: true
      }
    })
    hoisted.activateRdiDevice.mockResolvedValue({
      error: null
    })
  })

  it('normalizes uppercase PID and marks valid numbers as available', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.deviceNumber = 'abc123def456'
    await flushPromises()
    await flushPromises()

    expect(setupState.deviceNumber).toBe('ABC123DEF456')

    await flushPromises()

    expect(hoisted.checkDevice).toHaveBeenCalledWith('ABC123DEF456')
    expect(setupState.buttonDisabled).toBe(false)
    expect(setupState.showMessage).toBe(true)
    expect(setupState.messageStyle.color).toBe('rgb(2,153,52)')
  })

  it('rejects malformed PID locally without calling availability API', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.deviceNumber = 'bad-pid'
    await flushPromises()
    await flushPromises()

    expect(hoisted.checkDevice).toHaveBeenCalledTimes(0)
    expect(setupState.buttonDisabled).toBe(true)
    expect(setupState.showMessage).toBe(true)
    expect(setupState.messageStyle.color).toBe('rgb(255, 26, 26)')
  })

  it('activates a valid PID and refreshes the table after success', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const tableRef = getTableRef(wrapper)

    setupState.active = true
    setupState.deviceNumber = 'abc123def456'
    await setupState.completeAdd()
    await flushPromises()

    expect(hoisted.activateRdiDevice).toHaveBeenCalledWith({
      pid_number: 'ABC123DEF456'
    })
    expect(setupState.active).toBe(false)
    expect(setupState.deviceNumber).toBe('')
    expect(setupState.showMessage).toBe(false)
    expect(setupState.buttonDisabled).toBe(true)
    expect(tableRef.handleSearch).toHaveBeenCalledTimes(1)
  })

  it('uses click trigger for add-device dropdown and opens manual add drawer on manual selection', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(deviceManageSource).toContain('trigger="click"')

    setupState.handleSelect('hands')
    await flushPromises()

    expect(setupState.addKey).toBe('hands')
    expect(setupState.active).toBe(true)
    expect(setupState.placement).toBe('bottom')
  })

  it('routes to service access when selecting add by server', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)

    setupState.handleSelect('server')
    await flushPromises()

    expect(hoisted.routerPush).toHaveBeenCalledWith('/device/service-access')
  })

  it('applies fleet target presets through the table query bridge', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.applyFleetTargetPreset('offline')
    await flushPromises()

    expect(getTableRef(wrapper).forceChangeParamsByKey).toHaveBeenCalledWith({
      is_online: 0,
      warn_status: null,
      shared_status: null,
      device_type: null,
      last_reported_after: null,
      last_reported_before: null,
      never_reported: null
    })
  })

  it('applies the never-reported fleet preset without retaining other target filters', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.applyFleetTargetPreset('never_reported')
    await flushPromises()

    expect(getTableRef(wrapper).forceChangeParamsByKey).toHaveBeenCalledWith({
      is_online: null,
      warn_status: null,
      shared_status: null,
      device_type: null,
      last_reported_after: null,
      last_reported_before: null,
      never_reported: true
    })
  })

  it('updates the fleet target preview count from the device list total', async () => {
    hoisted.deviceList.mockResolvedValueOnce({
      data: {
        list: [],
        total: 42
      }
    })

    const wrapper = mountDeviceManage()
    await flushPromises()

    expect(getSetupState(wrapper).targetPreviewTotal).toBe(42)
  })

  it('saves and reapplies the current fleet filter locally', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.syncFleetQueryResult({
      is_online: 1,
      warn_status: 'Y',
      page: 1
    }, 5, [])

    setupState.saveCurrentFleetFilter()
    await flushPromises()

    expect(setupState.savedFleetFilters).toHaveLength(1)
    expect(setupState.savedFleetFilters[0].previewTotal).toBe(5)
    expect(setupState.savedFleetFilterOptions[0]).toMatchObject({
      rawName: setupState.savedFleetFilters[0].name
    })

    setupState.applySavedFleetFilter(setupState.savedFleetFilters[0].id)
    await flushPromises()

    expect(getTableRef(wrapper).forceChangeParamsByKey).toHaveBeenCalledWith(
      expect.objectContaining({
        is_online: 1,
        warn_status: 'Y'
      })
    )
  })

  it('deletes local-only saved fleet filters without calling the backend', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.syncFleetQueryResult({
      is_online: 1,
      warn_status: 'Y'
    }, 5, [])

    setupState.saveCurrentFleetFilter()
    await flushPromises()

    const filterID = setupState.savedFleetFilters[0].id
    await setupState.deleteSavedFleetFilter(filterID)

    expect(hoisted.deleteFleetSavedFilter).not.toHaveBeenCalled()
    expect(setupState.savedFleetFilters).toHaveLength(0)
  })

  it('deletes backend saved fleet filters through the saved-filter API', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.savedFleetFilters = [
      {
        id: 'server-filter-1',
        name: 'Online devices',
        params: { is_online: 1 },
        previewTotal: 5,
        createdAt: '2026-07-05T12:00:00Z'
      }
    ]

    await setupState.deleteSavedFleetFilter('server-filter-1')

    expect(hoisted.deleteFleetSavedFilter).toHaveBeenCalledWith('server-filter-1')
    expect(setupState.savedFleetFilters).toHaveLength(0)
  })

  it('renames local-only saved fleet filters without calling the backend', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.syncFleetQueryResult({
      is_online: 1
    }, 5, [])

    setupState.saveCurrentFleetFilter()
    await flushPromises()

    const filterID = setupState.savedFleetFilters[0].id
    const renamed = await setupState.renameSavedFleetFilter(filterID, 'Online devices')

    expect(renamed).toBe(true)
    expect(hoisted.updateFleetSavedFilter).not.toHaveBeenCalled()
    expect(setupState.savedFleetFilters[0].name).toBe('Online devices')
  })

  it('renames backend saved fleet filters through the saved-filter API', async () => {
    hoisted.updateFleetSavedFilter.mockResolvedValueOnce({
      data: {
        id: 'server-filter-1',
        name: 'Online pumps',
        device_filter: { is_online: 1 },
        preview_total: 5,
        updated_at: '2026-07-06T00:00:00Z'
      }
    })

    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.savedFleetFilters = [
      {
        id: 'server-filter-1',
        name: 'Online devices',
        params: { is_online: 1 },
        previewTotal: 5,
        createdAt: '2026-07-05T12:00:00Z'
      }
    ]

    const renamed = await setupState.renameSavedFleetFilter('server-filter-1', 'Online pumps')

    expect(renamed).toBe(true)
    expect(hoisted.updateFleetSavedFilter).toHaveBeenCalledWith(
      'server-filter-1',
      expect.objectContaining({
        name: 'Online pumps',
        device_filter: {
          is_online: 1
        },
        preview_total: 5
      })
    )
    expect(setupState.savedFleetFilters[0].name).toBe('Online pumps')
  })

  it('opens selected-device command center with fleet context', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.handleFleetSelectionUpdate([{ id: 'device-command-1' }, { id: 'device-command-2' }])
    setupState.openSelectedDeviceCommandContext()
    await flushPromises()

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      path: '/device/command-center',
      query: {
        device_ids: 'device-command-1,device-command-2',
        fleet_source: 'device_manage',
        fleet_scope: 'selected_devices',
        fleet_selected_count: 2,
        first_device_id: 'device-command-1'
      }
    })
  })

  it('requires scope confirmation before opening filter-result fleet OTA', async () => {
    const wrapper = mountDeviceManage()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.syncFleetQueryResult({
      page: 1,
      page_size: 10,
      group_id: 'group-1',
      is_online: 1
    }, 42, [])
    setupState.openFleetOtaContext()
    await flushPromises()

    expect(setupState.fleetScopeConfirmVisible).toBe(true)
    expect(setupState.pendingFleetScopeAction?.path).toBe('/product/update-ota')
    expect(hoisted.routerPush).not.toHaveBeenCalled()

    setupState.confirmFleetCurrentPageAction()
    await flushPromises()

    expect(hoisted.routerPush).toHaveBeenCalledWith(
      expect.objectContaining({
        path: '/product/update-ota',
        query: expect.objectContaining({
          fleet_source: 'device_manage',
          fleet_scope: 'filter_result',
          fleet_requested_total: 42,
          group_id: 'group-1',
          is_online: 1
        })
      })
    )
  })

  it('cancels pending device-status subscription when unmounted', async () => {
    vi.useFakeTimers()

    try {
      const wrapper = mountDeviceManage()
      await flushPromises()

      wrapper.unmount()
      vi.advanceTimersByTime(100)

      expect(hoisted.wsConnect).not.toHaveBeenCalled()
      expect(hoisted.wsDisconnect).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })
})
