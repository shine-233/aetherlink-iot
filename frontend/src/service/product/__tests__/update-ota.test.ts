/**
 * 文件用途：验证 产品服务 API 单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete
  }
}))

import {
  addOtaPackage,
  addOtaTask,
  deleteOtaPackage,
  editOtaPackage,
  editOtaTaskDetail,
  getDeviceList,
  getOtaTaskDetail,
  getOtaTaskList,
  getOtaTaskSupportBundle,
  previewOtaTask
} from '../update-ota'

describe('service/product/update-ota', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGet.mockResolvedValue({ error: null, data: {} })
    mockPost.mockResolvedValue({ error: null, data: {} })
    mockPut.mockResolvedValue({ error: null, data: {} })
    mockDelete.mockResolvedValue({ error: null, data: {} })
  })

  it('requests OTA task list with the selected package id', async () => {
    const params = { page: 1, page_size: 10, ota_upgrade_package_id: 'pkg-1' }

    await getOtaTaskList(params)

    expect(mockGet).toHaveBeenCalledWith('/ota/task', { params })
  })

  it('requests device list for OTA task device selection', async () => {
    const params = { page: 1, page_size: 1000, device_config_id: 'cfg-1' }

    await getDeviceList(params)

    expect(mockGet).toHaveBeenCalledWith('/device', { params })
  })

  it('keeps the current package create endpoint available from this module', async () => {
    const data = { name: 'Pkg 1', version: '1.0.0', package_url: '/files/pkg.bin' }

    await addOtaPackage(data)

    expect(mockPost).toHaveBeenCalledWith('/ota/package', data)
  })

  it('keeps the current package edit endpoint available from this module', async () => {
    const data = { id: 'pkg-1', name: 'Pkg 1', version: '1.0.1' }

    await editOtaPackage(data)

    expect(mockPut).toHaveBeenCalledWith('/ota/package', data)
  })

  it('keeps the current package delete endpoint available from this module', async () => {
    await deleteOtaPackage(12)

    expect(mockDelete).toHaveBeenCalledWith('/ota/package/12')
  })

  it('creates OTA task with selected package and devices in the body', async () => {
    const data = {
      name: 'Upgrade batch',
      ota_upgrade_package_id: 'pkg-1',
      device_id_list: ['dev-1', 'dev-2']
    }

    await addOtaTask(data)

    expect(mockPost).toHaveBeenCalledWith('/ota/task', data)
  })

  it('previews a backend filter-based OTA task before creation', async () => {
    const data = {
      ota_upgrade_package_id: 'pkg-1',
      device_filter: { group_id: 'group-1' },
      max_devices: 5000
    }

    await previewOtaTask(data)

    expect(mockPost).toHaveBeenCalledWith('/ota/task/preview', data)
  })

  it('downloads a backend task-level OTA support bundle by task id', async () => {
    await getOtaTaskSupportBundle('task 1')

    expect(mockGet).toHaveBeenCalledWith('/ota/task/task%201/support-bundle', { silentError: true })
  })

  it('requests OTA task details with list filters as params', async () => {
    const params = {
      page: 2,
      page_size: 20,
      ota_upgrade_task_id: 'task-1',
      device_name: 'Device A',
      task_status: 3
    }

    await getOtaTaskDetail(params)

    expect(mockGet).toHaveBeenCalledWith('/ota/task/detail', { params })
  })

  it('updates OTA task detail action with body params', async () => {
    const params = { id: 'detail-1', action: 6 }

    await editOtaTaskDetail(params)

    expect(mockPut).toHaveBeenCalledWith('/ota/task/detail', params)
  })
})
