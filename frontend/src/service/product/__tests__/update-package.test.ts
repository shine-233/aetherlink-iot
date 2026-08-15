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

import { addOtaPackage, deleteOtaPackage, editOtaPackage, getDeviceList, getOtaPackageList } from '../update-package'

describe('service/product/update-package', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGet.mockResolvedValue({ error: null, data: {} })
    mockPost.mockResolvedValue({ error: null, data: {} })
    mockPut.mockResolvedValue({ error: null, data: {} })
    mockDelete.mockResolvedValue({ error: null, data: {} })
  })

  it('requests OTA package list with query params', async () => {
    const params = { page: 2, page_size: 20, name: 'firmware', device_config_id: 'cfg-1' }

    await getOtaPackageList(params)

    expect(mockGet).toHaveBeenCalledWith('/ota/package', { params })
  })

  it('requests device list with query params for package binding', async () => {
    const params = { page: 1, page_size: 1000, device_config_id: 'cfg-1' }

    await getDeviceList(params)

    expect(mockGet).toHaveBeenCalledWith('/device', { params })
  })

  it('creates an OTA package with the request body unchanged', async () => {
    const data = {
      name: 'Pkg 1',
      version: '1.0.0',
      device_config_id: 'cfg-1',
      package_url: '/files/pkg.bin',
      additional_info: '{}'
    }

    await addOtaPackage(data)

    expect(mockPost).toHaveBeenCalledWith('/ota/package', data)
  })

  it('updates an OTA package with the request body unchanged', async () => {
    const data = {
      id: 'pkg-1',
      name: 'Pkg 1',
      version: '1.0.1',
      package_url: '/files/pkg-101.bin'
    }

    await editOtaPackage(data)

    expect(mockPut).toHaveBeenCalledWith('/ota/package', data)
  })

  it('deletes an OTA package by id in the path', async () => {
    await deleteOtaPackage('pkg-1')

    expect(mockDelete).toHaveBeenCalledWith('/ota/package/pkg-1')
  })
})
