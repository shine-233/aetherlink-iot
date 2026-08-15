/**
 * 文件用途: 系统日志 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证系统日志列表查询的路径和筛选参数。
 * 关键注意事项: 日志完整性和审计可靠性不由前端 mock 测试证明。
 * 重构建议: 增加时间范围、用户筛选、操作类型和分页边界断言。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet } = vi.hoisted(() => ({
  mockGet: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet
  }
}))

import { getSystemLogList } from '../system-management-user'

describe('system-management-user API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches operation logs with pagination and filters', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    const params = {
      page: 1,
      page_size: 20,
      user_name: 'operator',
      operation_type: 'update'
    } as any

    const result = await getSystemLogList(params)

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/operation_logs', { params })
    expect(result).toEqual({ data: { total: 1, list: [] }, error: null })
  })
})
