/**
 * 文件用途: 接入服务和插件 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证服务注册、访问配置、协议插件和调试接口请求。
 * 关键注意事项: 设备接入链路的真实行为仍需 broker、后端和集成测试证明。
 * 重构建议: 按服务注册、访问配置、协议插件和调试拆分测试，减少单测文件体量。
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
  batchAddServiceMenuList,
  createServiceDrop,
  delRegisterService,
  delServiceAccess,
  getSelectServiceMenuList,
  getServiceAccess,
  getServiceAccessForm,
  getServiceListDrop,
  getServices,
  putRegisterService,
  putServiceDrop,
  registerService
} from '../plugin'

describe('plugin API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches, registers, updates, and deletes service plugins', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'svc-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const query = { page: 1, page_size: 10, name: 'mqtt' }
    const createPayload = {
      name: 'MQTT',
      service_identifier: 'mqtt',
      service_config: { host: '127.0.0.1' }
    }
    const updatePayload = { id: 'svc-1', name: 'MQTT updated', status: 1 }

    await getServices(query)
    await registerService(createPayload)
    await putRegisterService(updatePayload)
    await delRegisterService('svc-1')

    expect(mockGet).toHaveBeenCalledWith('/service/list', { params: query })
    expect(mockPost).toHaveBeenCalledWith('/service', createPayload)
    expect(mockPut).toHaveBeenCalledWith('/service', updatePayload)
    expect(mockDelete).toHaveBeenCalledWith('/service/svc-1')
  })

  it('covers third-party access point list, form, create, update, and delete contracts', async () => {
    mockGet.mockResolvedValue({ data: { total: 0, list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'access-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const listQuery = { page: 1, page_size: 10, service_id: 'svc-1' }
    const formQuery = { service_id: 'svc-1', protocol: 'mqtt' }
    const payload = {
      name: 'factory gateway',
      service_id: 'svc-1',
      voucher: { username: 'device', password: 'secret' }
    }

    await getServiceAccess(listQuery)
    await getServiceAccessForm(formQuery)
    await createServiceDrop(payload)
    await putServiceDrop({ id: 'access-1', ...payload })
    await delServiceAccess('access-1')

    expect(mockGet).toHaveBeenNthCalledWith(1, '/service/access/list', { params: listQuery })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/service/access/voucher/form', { params: formQuery })
    expect(mockPost).toHaveBeenCalledWith('/service/access', payload)
    expect(mockPut).toHaveBeenCalledWith('/service/access', { id: 'access-1', ...payload })
    expect(mockDelete).toHaveBeenCalledWith('/service/access/access-1')
  })

  it('covers service device list and device-config menu binding contracts', async () => {
    mockGet.mockResolvedValue({ data: { list: [] }, error: null })
    mockPost.mockResolvedValue({ data: null, error: null })

    const deviceQuery = { access_id: 'access-1', page: 2, page_size: 5 }
    const menuQuery = { device_config_id: 'cfg-1' }
    const batchPayload = {
      device_config_id: 'cfg-1',
      service_access_ids: ['access-1', 'access-2']
    }

    await getServiceListDrop(deviceQuery)
    await getSelectServiceMenuList(menuQuery)
    await batchAddServiceMenuList(batchPayload)

    expect(mockGet).toHaveBeenNthCalledWith(1, '/service/access/device/list', { params: deviceQuery })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device_config/menu', { params: menuQuery })
    expect(mockPost).toHaveBeenCalledWith('/device/service/access/batch', batchPayload)
  })
})
