/**
 * 文件用途: API Key wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后检查密钥列表、新增、更新和删除请求。
 * 关键注意事项: 密钥权限和租户隔离仍需后端测试证明，这里只覆盖前端参数映射。
 * 重构建议: 增加权限字段、删除 ID 和错误返回的精确断言。
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

import { addKey, apiKeyDel, fetchKeyList, updateKey } from '../apikey'

describe('apikey API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches OpenAPI key list with query params', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    const params = {
      page: 1,
      page_size: 10,
      tenant_id: 'tenant-1',
      name: 'integration'
    }

    const result = await fetchKeyList(params)

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/open/keys', { params })
    expect(result).toEqual({ data: { total: 1, list: [] }, error: null })
  })

  it('creates an OpenAPI key with the selected tenant and display name', async () => {
    mockPost.mockResolvedValue({ data: { id: 'key-1' }, error: null })
    const payload = {
      tenant_id: 'tenant-1',
      name: 'third-party integration'
    }

    await addKey(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/open/keys', payload)
  })

  it('updates an OpenAPI key status and name through PUT /open/keys', async () => {
    mockPut.mockResolvedValue({ data: null, error: null })
    const payload = {
      id: 'key-1',
      name: 'disabled integration',
      status: 0
    }

    await updateKey(payload)

    expect(mockPut).toHaveBeenCalledTimes(1)
    expect(mockPut).toHaveBeenCalledWith('/open/keys', payload)
  })

  it('deletes an OpenAPI key by id through DELETE /open/keys/{id}', async () => {
    mockDelete.mockResolvedValue({ data: null, error: null })

    await apiKeyDel('key-1')

    expect(mockDelete).toHaveBeenCalledTimes(1)
    expect(mockDelete).toHaveBeenCalledWith('/open/keys/key-1')
  })
})
