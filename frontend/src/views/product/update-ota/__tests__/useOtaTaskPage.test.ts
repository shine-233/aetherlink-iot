/**
 * 文件用途: 覆盖 useOtaTaskPage 页面编排 composable 的关键契约。
 * 核心逻辑: 通过挂载 harness 组件(内含 vue-router 与接口 mock),验证路由初始化、useListPage 分页、
 *           失败诊断跳转、预览行兜底与远程搜索防抖,全部走真实编排链路而非直接注入状态。
 * 关键注意事项: 断言必须落在请求参数/路由 query 上,避免只证明函数可调用。
 * 重构建议: 若 harness 需求增多,可抽取公共挂载工厂与 fixture。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
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

import { useOtaTaskPage } from '../useOtaTaskPage'
import { OTA_TASK_DETAIL_STATUS } from '../ota-task-actions'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountHarness = () => {
  const wrapper = mount(
    defineComponent({
      name: 'UseOtaTaskPageHarness',
      setup() {
        return { ...useOtaTaskPage() }
      },
      render: () => h('div')
    })
  )
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof mount>) => wrapper.vm.$.setupState as Record<string, any>

describe('useOtaTaskPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.routeQuery = {}
    hoisted.getOtaPackageList.mockResolvedValue({
      data: { list: [{ id: 'pkg-1', name: 'Pkg1', version: '1.0', device_config_id: 'dc1' }], total: 1 },
      error: null
    })
    hoisted.getOtaTaskList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.getOtaTaskDetail.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.getOtaTaskSupportBundle.mockResolvedValue({ data: { task_id: 'task-1' }, error: null })
    hoisted.addOtaTask.mockResolvedValue({ error: null })
    hoisted.previewOtaTask.mockResolvedValue({ data: { selected_count: 1, total_matched: 1 }, error: null })
    hoisted.deviceList.mockResolvedValue({ data: { list: [] }, error: null })
    Object.defineProperty(window, '$message', {
      configurable: true,
      value: { success: vi.fn(), warning: vi.fn() }
    })
    Object.defineProperty(window, '$dialog', {
      configurable: true,
      value: { warning: vi.fn() }
    })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
    vi.useRealTimers()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('mounts, auto-selects the first package and loads its task page through useListPage', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.selectedPackageId).toBe('pkg-1')
    expect(hoisted.getOtaPackageList).toHaveBeenCalledTimes(1)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      ota_upgrade_package_id: 'pkg-1'
    })
  })

  it('seeds the selected package from the ota_package_id route query (trimmed)', async () => {
    hoisted.routeQuery = { ota_package_id: ' pkg-9 ' }
    hoisted.getOtaPackageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })

    const wrapper = mountHarness()
    await flushPromises()

    expect(getState(wrapper).selectedPackageId).toBe('pkg-9')
    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      ota_upgrade_package_id: 'pkg-9'
    })
  })

  it('requests the next task page from the useListPage pagination callback', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.getOtaTaskList.mockResolvedValue({
      data: { list: [{ id: 'task-2' }], total: 11 },
      error: null
    })

    const state = getState(wrapper)
    state.taskPagination.onUpdatePage(2)
    await flushPromises()

    expect(hoisted.getOtaTaskList).toHaveBeenCalledWith({
      page: 2,
      page_size: 10,
      ota_upgrade_package_id: 'pkg-1'
    })
    expect(state.taskPagination.itemCount).toBe(11)
  })

  it('routes failed-device diagnostics to the device ready-check tab with OTA context', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedTask = { id: 'task-1' }

    state.openFailedDeviceDiagnostics({ id: 'detail-7', device_id: 'dev-9' })

    expect(hoisted.routerPush).toHaveBeenCalledTimes(1)
    expect(hoisted.routerPush).toHaveBeenCalledWith({
      name: 'device_details',
      query: {
        d_id: 'dev-9',
        tab: 'ready-check',
        source: 'ota',
        ota_task_id: 'task-1',
        ota_detail_id: 'detail-7'
      }
    })
  })

  it('warns and stays on the page when a failure row has no device id', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedTask = { id: 'task-1' }

    state.openFailedDeviceDiagnostics({ id: 'detail-8' })

    expect(window.$message.warning).toHaveBeenCalledWith('page.product.update-ota.failureDiagnosticsMissingDevice')
    expect(hoisted.routerPush).not.toHaveBeenCalled()
  })

  it('opens diagnostics for the first failed device that carries a device id', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    const state = getState(wrapper)
    state.selectedTask = { id: 'task-1' }
    state.detailList = [
      { id: 'detail-no-device', status: OTA_TASK_DETAIL_STATUS.failed },
      { id: 'detail-1', device_id: 'dev-1', status: OTA_TASK_DETAIL_STATUS.failed },
      { id: 'detail-2', device_id: 'dev-2', status: OTA_TASK_DETAIL_STATUS.failed },
      { id: 'detail-ok', device_id: 'dev-3', status: OTA_TASK_DETAIL_STATUS.success }
    ]

    state.openFirstFailedDeviceDiagnostics()

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      name: 'device_details',
      query: {
        d_id: 'dev-1',
        tab: 'ready-check',
        source: 'ota',
        ota_task_id: 'task-1',
        ota_detail_id: 'detail-1'
      }
    })
    expect(state.firstFailedDiagnosticDevice?.id).toBe('detail-1')
  })

  it('warns when no failed device can be opened for diagnostics', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    const state = getState(wrapper)
    state.detailList = [{ id: 'detail-1', status: OTA_TASK_DETAIL_STATUS.failed }]

    state.openFirstFailedDeviceDiagnostics()

    expect(window.$message.warning).toHaveBeenCalledWith('page.product.update-ota.failureDiagnosticsMissingDevice')
    expect(hoisted.routerPush).not.toHaveBeenCalled()
  })

  it('falls back to selected device candidates when no filter preview rows exist', async () => {
    const wrapper = mountHarness()
    await flushPromises()
    const state = getState(wrapper)
    state.deviceCandidates = [
      { id: 'dev-1', name: 'Pump A', device_number: 'SN-1', current_version: '1.0', is_online: 1 }
    ]

    expect(state.filterPreviewSubsetRows).toEqual([
      { id: 'dev-1', label: 'Pump A', deviceNumber: 'SN-1', currentVersion: '1.0', online: '在线' }
    ])
  })

  it('debounces package remote search into a single getOtaPackageList call', async () => {
    vi.useFakeTimers()
    const wrapper = mountHarness()
    await vi.advanceTimersByTimeAsync(0)
    vi.clearAllMocks()
    const state = getState(wrapper)

    state.handlePackageSearch('ab')
    state.handlePackageSearch('abc')
    await vi.advanceTimersByTimeAsync(299)
    expect(hoisted.getOtaPackageList).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(10)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledTimes(1)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({ page: 1, page_size: 20, name: 'abc' })
  })

  it('gates device remote search behind the open create-task modal', async () => {
    vi.useFakeTimers()
    const wrapper = mountHarness()
    await vi.advanceTimersByTimeAsync(0)
    vi.clearAllMocks()
    const state = getState(wrapper)

    state.handleDeviceSearch('pump')
    await vi.advanceTimersByTimeAsync(300)
    expect(hoisted.deviceList).not.toHaveBeenCalled()

    state.taskModalVisible = true
    state.handleDeviceSearch('pump')
    await vi.advanceTimersByTimeAsync(300)
    expect(hoisted.deviceList).toHaveBeenCalledWith({
      page: 1,
      page_size: 50,
      device_config_id: 'dc1',
      search: 'pump'
    })
  })
})
