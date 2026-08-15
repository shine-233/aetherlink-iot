/**
 * 文件用途: 系统设置和系统管理 wrapper 的组合请求合同测试。
 * 核心逻辑: mock request 层后验证主题、数据清理、字典、功能开关和系统日志请求。
 * 关键注意事项: 组合测试容易掩盖领域边界，失败时要定位到具体 wrapper 文件。
 * 重构建议: 继续按 wrapper 领域拆分组合测试，并补默认值和权限边界。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => {
  const requestFn = vi.fn()
  return {
    requestFn,
    mockGet: vi.fn(),
    mockPut: vi.fn()
  }
})

vi.mock('@/service/request', () => ({
  request: Object.assign(hoisted.requestFn, {
    get: hoisted.mockGet,
    put: hoisted.mockPut
  })
}))

import {
  dictQuery,
  editDataClear,
  editFunction,
  editThemeSetting,
  fetchDataClearList,
  fetchThemeSetting,
  getFunction
} from '../setting'
import { getSystemLogList } from '../system-management-user'

describe('setting and system audit API services', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('covers theme logo and data-retention setting read/write contracts', async () => {
    hoisted.mockGet.mockResolvedValue({ data: null, error: null })
    hoisted.mockPut.mockResolvedValue({ data: null, error: null })

    const logoPayload = { logo: '/logo.png', system_name: 'AetherLink IoT' }
    const policyQuery = { page: 1, page_size: 10 }
    const policyPayload = { id: 'policy-1', retention_days: 30 }

    await fetchThemeSetting()
    await editThemeSetting(logoPayload)
    await fetchDataClearList(policyQuery)
    await editDataClear(policyPayload)

    expect(hoisted.mockGet).toHaveBeenNthCalledWith(1, '/logo')
    expect(hoisted.mockPut).toHaveBeenNthCalledWith(1, '/logo', logoPayload)
    expect(hoisted.mockGet).toHaveBeenNthCalledWith(2, '/datapolicy', { params: policyQuery })
    expect(hoisted.mockPut).toHaveBeenNthCalledWith(2, '/datapolicy', policyPayload)
  })

  it('covers dictionary enum and sys_function feature switch endpoints', async () => {
    hoisted.mockGet.mockResolvedValue({ data: [], error: null })
    hoisted.mockPut.mockResolvedValue({ data: null, error: null })

    await dictQuery({ enum_type: 'device_status' })
    await getFunction()
    await editFunction({ function_id: 'fn-alarm' })

    expect(hoisted.mockGet).toHaveBeenNthCalledWith(1, '/dict/enum', {
      params: { enum_type: 'device_status' }
    })
    expect(hoisted.mockGet).toHaveBeenNthCalledWith(2, '/sys_function')
    expect(hoisted.mockPut).toHaveBeenCalledWith('/sys_function/fn-alarm')
  })

  it('fetches operation logs with audit filters', async () => {
    hoisted.mockGet.mockResolvedValue({ data: { total: 0, list: [] }, error: null })
    const params = {
      page: 1,
      page_size: 20,
      user_name: 'admin',
      start_time: '2026-06-01 00:00:00',
      end_time: '2026-06-27 23:59:59'
    } as any

    await getSystemLogList(params)

    expect(hoisted.mockGet).toHaveBeenCalledWith('/operation_logs', { params })
  })
})
