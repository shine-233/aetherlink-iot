/**
 * 文件用途: Customer（客户管理）API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证客户 CRUD 与设备分配各端点的 URL、方法与参数传递。
 * 关键注意事项: 分配即移动语义由后端保证，这里只锁定前端调用契约。
 * 重构建议: 若后续增加客户详情扩展字段，可补 additional_info 的序列化断言。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: Object.assign(vi.fn(), {
    get: hoisted.mockGet,
    post: hoisted.mockPost,
    put: hoisted.mockPut,
    delete: hoisted.mockDelete
  })
}))

import {
  assignCustomerDevices,
  deleteCustomer,
  getCustomerDevices,
  getCustomerDetail,
  getCustomersList,
  saveCustomer,
  unassignCustomerDevice
} from '../customer'

describe('customer API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists customers with pagination and search params', async () => {
    hoisted.mockGet.mockResolvedValue({ list: [], total: 0 })
    await getCustomersList({ page: 2, page_size: 10, search: '客户A' })
    expect(hoisted.mockGet).toHaveBeenCalledWith('/customers', {
      params: { page: 2, page_size: 10, search: '客户A' }
    })
  })

  it('fetches customer detail with encoded id', async () => {
    hoisted.mockGet.mockResolvedValue({ id: 'c1' })
    await getCustomerDetail('c1')
    // createResource 的读路径经 dedupe 包装，总是透传 config（未提供时为 {}）
    expect(hoisted.mockGet).toHaveBeenCalledWith('/customer/c1', {})
  })

  it('creates customer without id and updates with id in the same payload', async () => {
    hoisted.mockPost.mockResolvedValue({ id: 'c1' })
    await saveCustomer({ name: '客户A', phone: '138' })
    expect(hoisted.mockPost).toHaveBeenCalledWith('/customer', { name: '客户A', phone: '138' }, {})

    await saveCustomer({ id: 'c1', name: '客户A改' })
    expect(hoisted.mockPost).toHaveBeenCalledWith('/customer', { id: 'c1', name: '客户A改' }, {})
  })

  it('deletes customer by encoded id', async () => {
    hoisted.mockDelete.mockResolvedValue(true)
    await deleteCustomer('c1')
    expect(hoisted.mockDelete).toHaveBeenCalledWith('/customer/c1', {})
  })

  it('assigns devices with device_ids payload', async () => {
    hoisted.mockPost.mockResolvedValue(true)
    await assignCustomerDevices('c1', ['d1', 'd2'])
    expect(hoisted.mockPost).toHaveBeenCalledWith('/customer/c1/devices', { device_ids: ['d1', 'd2'] })
  })

  it('unassigns one device via nested device path', async () => {
    hoisted.mockDelete.mockResolvedValue(true)
    await unassignCustomerDevice('c1', 'd1')
    expect(hoisted.mockDelete).toHaveBeenCalledWith('/customer/c1/device/d1')
  })

  it('lists assigned device ids for a customer', async () => {
    hoisted.mockGet.mockResolvedValue({ device_ids: ['d1'] })
    await getCustomerDevices('c1')
    expect(hoisted.mockGet).toHaveBeenCalledWith('/customer/c1/devices')
  })
})
