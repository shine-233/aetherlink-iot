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
  addDevice,
  addProduct,
  delDeviceConfig,
  deleteProduct,
  editProduct,
  exportDevice,
  getDeviceList,
  getDict,
  getPreProductList,
  getProductList
} from '../list'

describe('service/product/list', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGet.mockResolvedValue({ error: null, data: {} })
    mockPost.mockResolvedValue({ error: null, data: {} })
    mockPut.mockResolvedValue({ error: null, data: {} })
    mockDelete.mockResolvedValue({ error: null, data: {} })
  })

  it('requests product list with pagination and filters', async () => {
    const params = { page: 1, page_size: 20, name: 'gateway', device_type: '2' }

    await getProductList(params)

    expect(mockGet).toHaveBeenCalledWith('/product', { params })
  })

  it('creates and updates products without rewriting request bodies', async () => {
    const createBody = { name: 'Gateway product', device_type: '2', protocol_type: 'MQTT' }
    const updateBody = { id: 'product-1', name: 'Gateway product v2' }

    await addProduct(createBody)
    await editProduct(updateBody)

    expect(mockPost).toHaveBeenCalledWith('/product', createBody)
    expect(mockPut).toHaveBeenCalledWith('/product', updateBody)
  })

  it('deletes products by id through the product path', async () => {
    await deleteProduct('product-1')

    expect(mockDelete).toHaveBeenCalledWith('/product/product-1')
  })

  it('requests active and pre-registered device lists with params intact', async () => {
    const deviceParams = { page: 1, page_size: 10, device_config_id: 'config-1' }
    const preRegisterParams = { page: 2, page_size: 10, keyword: 'SN001' }

    await getDeviceList(deviceParams)
    await getPreProductList(preRegisterParams)

    expect(mockGet).toHaveBeenNthCalledWith(1, '/device', { params: deviceParams })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device/preRegister', { params: preRegisterParams })
  })

  it('creates pre-registered devices and exports them through the expected endpoints', async () => {
    const deviceBody = { device_number: 'SN001', device_config_id: 'config-1' }
    const exportParams = { device_config_id: 'config-1', status: 'pending' }

    await addDevice(deviceBody)
    await exportDevice(exportParams)

    expect(mockPost).toHaveBeenCalledWith('/device/preRegister', deviceBody)
    expect(mockGet).toHaveBeenCalledWith('/device/preRegister/export', { params: exportParams })
  })

  it('deletes device configs by id and requests dictionary values', async () => {
    const dictParams = { dict_code: 'protocol_type', lang: 'zh-CN' }

    await delDeviceConfig('config-1')
    await getDict(dictParams)

    expect(mockDelete).toHaveBeenCalledWith('/device_config/config-1')
    expect(mockGet).toHaveBeenCalledWith('/dict', { params: dictParams })
  })
})
