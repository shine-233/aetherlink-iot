/**
 * 文件用途: 覆盖测试在产品升级场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getOtaPackageList: vi.fn(),
  getOtaTaskList: vi.fn(),
  getOtaTaskDetail: vi.fn(),
  getOtaTaskSupportBundle: vi.fn(),
  addOtaTask: vi.fn(),
  previewOtaTask: vi.fn(),
  editOtaTaskDetail: vi.fn(),
  deviceList: vi.fn(),
  listFleetSavedFilters: vi.fn(),
  routeQuery: {} as Record<string, any>,
  routerPush: vi.fn()
}))

vi.mock('@/service/product/update-package', () => ({
  getOtaPackageList: hoisted.getOtaPackageList
}))

vi.mock('@/service/product/update-ota', () => ({
  getOtaTaskList: hoisted.getOtaTaskList,
  getOtaTaskDetail: hoisted.getOtaTaskDetail,
  getOtaTaskSupportBundle: hoisted.getOtaTaskSupportBundle,
  addOtaTask: hoisted.addOtaTask,
  previewOtaTask: hoisted.previewOtaTask,
  editOtaTaskDetail: hoisted.editOtaTaskDetail
}))

// index.vue 同时导入 deviceList 与 listFleetSavedFilters（传给 useOtaTaskFlow 的 services）。
// mock 工厂必须补齐，否则 vitest 会报 "No listFleetSavedFilters export is defined"。
vi.mock('@/service/api/device', () => ({
  deviceList: hoisted.deviceList,
  listFleetSavedFilters: hoisted.listFleetSavedFilters
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: hoisted.routerPush })
}))

import UpdateOta from '../index.vue'
import { FLEET_FILTER_RESULT_SCOPE } from '../../../device/modules/fleet-rollout-context'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(UpdateOta, {
    props,
    global: {
      stubs: {
        NSpace: defineComponent({
          props: ['vertical', 'align', 'wrap'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NCard: defineComponent({
          props: ['bordered'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          props: ['loading', 'disabled', 'type', 'size'],
          setup(props, { slots, emit }) {
            return () =>
              h(
                'button',
                { disabled: props.disabled, onClick: () => !props.disabled && emit('click') },
                slots.default?.()
              )
          }
        }),
        NAlert: defineComponent({
          props: ['type', 'showIcon'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: null }, options: { default: () => [] }, multiple: Boolean, filterable: Boolean, remote: Boolean },
          emits: ['update:value', 'search'],
          setup() {
            return () => h('div')
          }
        }),
        NDataTable: defineComponent({
          props: ['data', 'loading', 'columns', 'pagination', 'remote', 'scrollX'],
          setup() {
            return () => h('div')
          }
        }),
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NForm: defineComponent({
          props: ['labelPlacement', 'model'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NFormItem: defineComponent({
          props: ['label', 'required', 'path'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NTag: defineComponent({
          setup(_, { slots }) {
            return () => h('span', slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('UpdateOta', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.routeQuery = {}
    hoisted.getOtaPackageList.mockResolvedValue({
      data: { list: [{ id: 'pkg-1', name: 'Pkg1', version: '1.0', device_config_id: 'dc1' }], total: 1 },
      error: null
    })
    hoisted.getOtaTaskList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.getOtaTaskDetail.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.getOtaTaskSupportBundle.mockResolvedValue({ data: { task_id: 'task-1', failed_count: 2 }, error: null })
    hoisted.previewOtaTask.mockResolvedValue({
      data: { selected_count: 1, total_matched: 1, over_limit: false, max_devices: 5000 },
      error: null
    })
    hoisted.deviceList.mockResolvedValue({ data: { list: [] }, error: null })
    Object.defineProperty(window, '$message', {
      configurable: true,
      value: {
        success: vi.fn(),
        warning: vi.fn()
      }
    })
    Object.defineProperty(window, '$dialog', {
      configurable: true,
      value: {
        warning: vi.fn()
      }
    })
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('should mount and fetch packages and tasks', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getOtaPackageList).toHaveBeenCalledTimes(1)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(hoisted.getOtaTaskList.mock.calls).toContainEqual([
      {
        page: 1,
        page_size: 10,
        ota_upgrade_package_id: 'pkg-1'
      }
    ])
  })

  it('should select the first package and load tasks for that package', async () => {
    hoisted.getOtaPackageList.mockResolvedValue({
      data: {
        list: [
          { id: 'pkg-1', name: 'Pkg1', version: '1.0', device_config_id: 'dc1' },
          { id: 'pkg-2', name: 'Pkg2', version: '2.0', device_config_id: 'dc2' }
        ],
        total: 2
      },
      error: null
    })
    hoisted.getOtaTaskList.mockResolvedValue({
      data: { list: [{ id: 'task-1', name: 'Task 1' }], total: 1 },
      error: null
    })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.selectedPackageId).toBe('pkg-1')
    expect(state.taskList).toEqual([{ id: 'task-1', name: 'Task 1' }])
    expect(state.taskPagination.itemCount).toBe(1)
    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      ota_upgrade_package_id: 'pkg-1'
    })
  })

  it('should compute packageOptions', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.packageOptions).toEqual([{ label: 'Pkg1 (1.0)', value: 'pkg-1' }])
  })

  it('should compute selectedPackage', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedPackageId = 'pkg-1'
    expect(state.selectedPackage?.name).toBe('Pkg1')
  })

  it('should format time', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.formatTime(undefined)).toBe('-')
    expect(state.formatTime('2024-01-01')).toBe('2024-01-01 00:00:00')
  })

  it('should not save task with empty required fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.taskForm.name = ''
    state.taskForm.device_id_list = []
    await state.saveTask()
    expect((window as any).$message.warning).toHaveBeenCalledWith('page.product.update-ota.taskNameRequired')
    expect(hoisted.addOtaTask).toHaveBeenCalledTimes(0)
  })

  it('should warn instead of opening task modal when no package is selected', async () => {
    hoisted.getOtaPackageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    await state.openTaskModal()

    expect((window as any).$message.warning).toHaveBeenCalledWith('page.product.update-package.packagePlaceholder')
    expect(hoisted.deviceList).toHaveBeenCalledTimes(0)
    expect(state.taskModalVisible).toBe(false)
  })

  it('should load selectable devices for the selected package before opening task modal', async () => {
    hoisted.deviceList.mockResolvedValue({
      data: {
        list: [
          { id: 'dev-1', name: 'Device A' },
          { device_id: 'dev-2', device_name: 'Device B' },
          { id: '', name: 'Invalid Device' }
        ]
      },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    await state.openTaskModal()

    expect(hoisted.deviceList).toHaveBeenCalledWith({
      page: 1,
      page_size: 50,
      device_config_id: 'dc1'
    })
    expect(state.deviceOptions).toEqual([
      { label: 'Device A', value: 'dev-1' },
      { label: 'Device B', value: 'dev-2' }
    ])
    expect(state.taskModalVisible).toBe(true)
    expect(state.showNoEligibleDeviceAlert).toBe(false)
  })

  it('should explain when an upgrade package has no eligible devices', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [] }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    await state.openTaskModal()

    expect(state.deviceOptions).toEqual([])
    expect(state.taskModalVisible).toBe(true)
    expect(state.showNoEligibleDeviceAlert).toBe(true)
    // 说明：shallowMount 会把 OtaTaskLaunchContext 子组件 stub 成占位符，
    // noEligibleDevice 文案在子组件模板里，不会渲染进父级 wrapper.text()。
    // 该告警的显隐由上一行 showNoEligibleDeviceAlert 状态断言覆盖，文本断言在此层无效。
  })

  it('routes no-package onboarding to package upload and keeps OTA return context', async () => {
    hoisted.getOtaPackageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    state.handleOtaNextStep()

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      name: 'product_update-package',
      query: { return_to: 'ota_task' }
    })
  })

  it('should expose full-filter summary and backend preview sample rows for the modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    state.fleetPreselectionResult = {
      source: 'device_manage',
      scope: FLEET_FILTER_RESULT_SCOPE,
      requestedCount: 2,
      requestedTotal: 42,
      currentPageCount: 2,
      selectedCount: 2,
      excludedCount: 0,
      selectedDeviceIds: ['dev-1', 'dev-2'],
      deviceFilter: { group_id: 'group-1', is_online: 1 }
    }
    state.filterPreviewResult = {
      selected_count: 42,
      total_matched: 42,
      max_devices: 5000,
      preview_devices: [{ id: 'dev-1', name: 'Pump A', device_number: 'SN-1', current_version: '1.0', is_online: 1 }]
    }

    expect(state.fleetFilterSummaryItems).toEqual([
      { key: 'group_id', label: 'custom.deviceFilter.group', value: 'group-1' },
      { key: 'is_online', label: 'custom.deviceFilter.onlineStatus', value: '1' }
    ])
    expect(state.isFleetFilterScope).toBe(true)
    expect(state.filterPreviewSubsetRows).toEqual([
      { id: 'dev-1', label: 'Pump A', deviceNumber: 'SN-1', currentVersion: '1.0', online: '在线' }
    ])
  })

  it('should create task with trimmed name and selected devices', async () => {
    hoisted.addOtaTask.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.taskForm.name = '  Upgrade Batch  '
    state.taskForm.description = '  staged rollout  '
    state.taskForm.device_id_list = ['dev-1', 'dev-2']
    expect(state.canSaveTask).toBe(true)

    await state.saveTask()
    await flushPromises()

    expect(hoisted.addOtaTask).toHaveBeenCalledWith({
      name: 'Upgrade Batch',
      ota_upgrade_package_id: 'pkg-1',
      description: 'staged rollout',
      device_id_list: ['dev-1', 'dev-2']
    })
    expect(state.taskModalVisible).toBe(false)
    expect(hoisted.getOtaTaskList).toHaveBeenCalledTimes(1)
    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      ota_upgrade_package_id: 'pkg-1'
    })
  })

  it('should omit empty task description when creating an OTA task', async () => {
    hoisted.addOtaTask.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.taskForm.name = 'Batch'
    state.taskForm.description = '   '
    state.taskForm.device_id_list = ['dev-1']

    await state.saveTask()

    expect(hoisted.addOtaTask).toHaveBeenCalledWith(
      expect.objectContaining({
        description: undefined
      })
    )
  })

  it('should open task detail', async () => {
    hoisted.getOtaTaskDetail.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const row = { id: 'task-1', name: 'Task1', description: '', ota_upgrade_package_id: 'pkg-1', device_count: 5 }
    await state.openTaskDetail(row)
    expect(state.detailModalVisible).toBe(true)
    expect(state.selectedTask).toEqual(row)
  })

  it('should open the OTA task detail when Ready Check route context matches the loaded task list', async () => {
    hoisted.routeQuery = { source: 'ready-check', ota_task_id: 'task-1', ota_detail_id: 'detail-1' }
    hoisted.getOtaTaskList.mockResolvedValue({
      data: { list: [{ id: 'task-1', name: 'Task 1', ota_upgrade_package_id: 'pkg-1' }], total: 1 },
      error: null
    })
    hoisted.getOtaTaskDetail.mockResolvedValue({
      data: { list: [{ id: 'detail-1', device_id: 'dev-1', ota_upgrade_task_id: 'task-1' }], total: 1 },
      error: null
    })

    const wrapper = mountComponent()
    await flushPromises()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.readyCheckOtaContextStatus).toBe('matched')
    expect(state.detailModalVisible).toBe(true)
    expect(state.selectedTask?.id).toBe('task-1')
    expect(state.readyCheckOtaDetailMatched).toBe(true)
    expect(hoisted.getOtaTaskDetail).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      ota_upgrade_task_id: 'task-1',
      device_name: '',
      task_status: undefined
    })
  })

  it('should preserve Ready Check OTA context without opening a missing task', async () => {
    hoisted.routeQuery = { source: 'ready-check', ota_task_id: 'missing-task', ota_detail_id: 'detail-1' }
    hoisted.getOtaTaskList.mockResolvedValue({
      data: { list: [{ id: 'task-1', name: 'Task 1', ota_upgrade_package_id: 'pkg-1' }], total: 1 },
      error: null
    })

    const wrapper = mountComponent()
    await flushPromises()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.readyCheckOtaContextStatus).toBe('not-found')
    expect(state.readyCheckOtaContextVisible).toBe(true)
    expect(state.detailModalVisible).toBe(false)
    expect(state.selectedTask).toBeNull()
    expect(hoisted.getOtaTaskDetail).not.toHaveBeenCalled()
  })

  it('should download backend task-level support bundle without relying on current page failures', async () => {
    const createObjectURL = vi.fn(() => 'blob:ota-support-bundle')
    const revokeObjectURL = vi.fn()
    const click = vi.fn()
    const originalCreateElement = document.createElement.bind(document)
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
    vi.spyOn(document, 'createElement').mockImplementation((tagName: string) => {
      if (tagName === 'a') {
        return {
          href: '',
          download: '',
          click
        } as unknown as HTMLElement
      }
      return originalCreateElement(tagName)
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedTask = { id: 'task-1', name: 'Task 1' }

    await state.downloadTaskSupportBundle()

    expect(hoisted.getOtaTaskSupportBundle).toHaveBeenCalledWith('task-1')
    expect(createObjectURL).toHaveBeenCalled()
    expect(click).toHaveBeenCalled()
    expect(window.$message.success).toHaveBeenCalledWith('page.product.update-ota.taskSupportBundleDownloaded')
  })

  it('should fetch task details with filters and normalize total', async () => {
    hoisted.getOtaTaskDetail.mockResolvedValue({
      data: {
        data: {
          list: [{ id: 'detail-1', name: 'Device A', status: 3 }],
          total: 1
        }
      },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedTask = { id: 'task-1', name: 'Task 1' }
    state.detailQuery.device_name = 'Device A'
    state.detailQuery.task_status = 3

    await state.fetchTaskDetails()

    expect(hoisted.getOtaTaskDetail).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      ota_upgrade_task_id: 'task-1',
      device_name: 'Device A',
      task_status: 3
    })
    expect(state.detailList).toEqual([{ id: 'detail-1', name: 'Device A', status: 3 }])
    expect(state.detailPagination.itemCount).toBe(1)
  })

  it('should reset detail query', async () => {
    hoisted.getOtaTaskDetail.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.detailQuery.device_name = 'test'
    state.detailQuery.task_status = 1
    state.resetDetailQuery()
    expect(state.detailQuery.device_name).toBe('')
    expect(state.detailQuery.task_status).toBeNull()
  })

  it('should clear task list when selected package is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedPackageId = null

    await state.fetchTasks()

    expect(state.taskList).toEqual([])
    expect(state.taskPagination.itemCount).toBe(0)
  })

  it('should request the new task page and page size from pagination callbacks', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)

    state.taskPagination.onChange(2)
    state.taskPagination.onUpdatePageSize(20)
    await flushPromises()

    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 2,
      page_size: 10,
      ota_upgrade_package_id: 'pkg-1'
    })
    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      ota_upgrade_package_id: 'pkg-1'
    })
  })

  it('should request the new detail page and page size from pagination callbacks', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.selectedTask = { id: 'task-1', name: 'Task 1' }

    state.detailPagination.onChange(3)
    state.detailPagination.onUpdatePageSize(50)
    await flushPromises()

    expect(hoisted.getOtaTaskDetail).toHaveBeenCalledWith(expect.objectContaining({ page: 3, page_size: 10 }))
    expect(hoisted.getOtaTaskDetail).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 50 }))
  })
})
