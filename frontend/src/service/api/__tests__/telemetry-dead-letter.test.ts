/**
 * 文件用途: telemetry dead-letter API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证 dead-letter list、status 更新和 drain 请求。
 * 关键注意事项: 本测试只覆盖前端请求合同，不覆盖真实后端 replay/drain 执行结果。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockRequest, mockGet, mockPost } = vi.hoisted(() => {
  const request = vi.fn()
  return {
    mockRequest: request,
    mockGet: vi.fn(),
    mockPost: vi.fn()
  }
})

vi.mock('@/service/request', () => ({
  request: Object.assign(mockRequest, {
    get: mockGet,
    post: mockPost
  })
}))

import {
  drainTelemetryDeadLetters,
  getTelemetryDeadLetters,
  updateTelemetryDeadLetterStatus
} from '../telemetry-dead-letter'

describe('telemetry dead-letter API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('keeps list, status update, and drain request contracts aligned with backend routes', async () => {
    mockGet.mockResolvedValue({ data: { total: 0, list: [] }, error: null })
    mockRequest.mockResolvedValue({ data: null, error: null })
    mockPost.mockResolvedValue({
      data: { total_ready: 1, attempted: 1, replayed: 1, failed: 0, items: [] },
      error: null
    })

    const listParams = {
      page: 1,
      page_size: 20,
      device_id: 'device-1',
      key: 'temperature',
      status: 'processing' as const
    }
    const drainParams = {
      tenant_id: 'tenant-1',
      device_id: 'device-1',
      limit: 10
    }

    await getTelemetryDeadLetters(listParams)
    await updateTelemetryDeadLetterStatus('dead-letter-1', 'replay')
    await drainTelemetryDeadLetters(drainParams)

    expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/dead-letters', { params: listParams })
    expect(mockRequest).toHaveBeenCalledWith({
      url: '/telemetry/datas/dead-letters/dead-letter-1/status',
      method: 'patch',
      data: { action: 'replay' }
    })
    expect(mockPost).toHaveBeenCalledWith('/telemetry/datas/dead-letters/drain', drainParams)
  })
})
