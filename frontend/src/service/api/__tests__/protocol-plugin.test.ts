/**
 * 文件用途: 协议插件 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证协议插件列表、新增、编辑和删除请求。
 * 关键注意事项: 插件安装、加载和协议解析不由本测试证明。
 * 重构建议: 增加插件 ID、配置字段和删除失败的边界断言。
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

import { addProtocolPlugin, delProtocolPlugin, editProtocolPlugin, fetchProtocolPluginList } from '../protocol-plugin'

describe('protocol-plugin API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches protocol plugins with pagination and keyword filters', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [{ id: 'mqtt' }] }, error: null })
    const params = {
      page: 1,
      page_size: 20,
      name: 'mqtt',
      status: 'enabled'
    }

    const result = await fetchProtocolPluginList(params)

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/service/list', { params })
    expect(result.data.total).toBe(1)
  })

  it('creates protocol plugin records with service identifier and config metadata', async () => {
    mockPost.mockResolvedValue({ data: { id: 'plugin-1' }, error: null })
    const payload = {
      name: 'MQTT TCP',
      service_identifier: 'mqtt_tcp',
      protocol_type: 'MQTT',
      config_schema: [{ key: 'host', type: 'string' }]
    }

    await addProtocolPlugin(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/service', payload)
  })

  it('updates protocol service records through PUT /service', async () => {
    mockPut.mockResolvedValue({ data: null, error: null })
    const payload = {
      id: 'plugin-1',
      name: 'MQTT TCP updated',
      status: 'disabled'
    }

    await editProtocolPlugin(payload)

    expect(mockPut).toHaveBeenCalledTimes(1)
    expect(mockPut).toHaveBeenCalledWith('/service', payload)
  })

  it('deletes protocol plugin records by id', async () => {
    mockDelete.mockResolvedValue({ data: null, error: null })

    await delProtocolPlugin('plugin-1')

    expect(mockDelete).toHaveBeenCalledTimes(1)
    expect(mockDelete).toHaveBeenCalledWith('/service/plugin-1')
  })
})
