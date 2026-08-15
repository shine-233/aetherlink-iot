/**
 * 文件用途: 物模型和模型 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证物模型分页、详情、删除、模型增改查和设备选择接口。
 * 关键注意事项: 物模型与模型的真实约束仍需后端测试证明，这里只锁住前端请求形状。
 * 重构建议: 按物模型、模型、设备选择拆分测试，并补充必填字段缺失边界。
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
  deleteDeviceTemplate,
  deviceTemplate,
  getDeviceListForSelect,
  getDeviceModel,
  getDeviceTemplateDetail,
  postDeviceModel,
  putDeviceModel
} from '../device-template-model'

describe('device-template-model API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('covers thing model list, detail, and delete contracts', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const query = { page: 1, page_size: 10, name: 'temperature' }

    await deviceTemplate(query)
    await getDeviceTemplateDetail('template-1')
    await deleteDeviceTemplate('template-1')

    expect(mockGet).toHaveBeenNthCalledWith(1, '/device/template', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device/template/detail/template-1')
    expect(mockDelete).toHaveBeenCalledWith('/device/template/template-1')
  })

  it('covers telemetry model list, create, and update contracts', async () => {
    mockGet.mockResolvedValue({ data: { total: 0, list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'telemetry-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })

    const listQuery = { page: 1, page_size: 20, device_template_id: 'template-1' }
    const payload = {
      device_template_id: 'template-1',
      data_identifier: 'temperature',
      data_name: 'Temperature',
      data_type: 'float',
      unit: 'C',
      description: 'ambient temperature'
    }

    await getDeviceModel(listQuery)
    await postDeviceModel(payload)
    await putDeviceModel(payload)

    expect(mockGet).toHaveBeenCalledWith('/device/model/telemetry', { params: listQuery })
    expect(mockPost).toHaveBeenCalledWith('/device/model/telemetry', { params: payload })
    expect(mockPut).toHaveBeenCalledWith('/device/model/telemetry', { params: payload })
  })

  it('fetches selectable devices with optional tenant, keyword, and pagination params', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    const params = {
      page: 1,
      page_size: 50,
      tenant_id: 'tenant-1',
      keyword: 'pump'
    } as any

    await getDeviceListForSelect(params)

    expect(mockGet).toHaveBeenCalledWith('/device/selector', { params })
  })
})
