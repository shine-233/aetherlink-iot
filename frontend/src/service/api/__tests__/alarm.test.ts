/**
 * 文件用途: 告警 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后断言告警消息、历史、处理、通知和配置接口的方法、路径与参数。
 * 关键注意事项: 这些测试只证明前端发起的请求形状，不证明后端告警闭环真实生效。
 * 重构建议: 按告警规则、历史、处理和通知分组拆分用例，并增加失败分支断言。
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
  addWarningMessage,
  warningMessageList,
  editInfo,
  editInfoText,
  delInfo,
  infoList,
  alarmHistory,
  alarmHistoryMonthlyTrend,
  processingOperation,
  batchProcessing,
  batchActionAlarmHistory,
  acknowledgeAlarmHistory,
  resetAlarmHistory
} from '../alarm'

describe('Alarm API 层 - alarm.ts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('addWarningMessage', () => {
    it('调用 POST /alarm/config 并发送告警配置数据', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} })
      const params = { name: '温度告警', level: 1 }
      const result = await addWarningMessage(params)
      expect(mockPost).toHaveBeenCalledTimes(1)
      expect(mockPost).toHaveBeenCalledWith('/alarm/config', params)
      expect(result).toEqual({ error: null, data: {} })
    })
  })

  describe('warningMessageList', () => {
    it('调用 GET /alarm/config 并携带查询参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { list: [] } })
      const params = { page: 1, page_size: 10 }
      const result = await warningMessageList(params)
      expect(mockGet).toHaveBeenCalledTimes(1)
      expect(mockGet).toHaveBeenCalledWith('/alarm/config', { params })
      expect(result).toEqual({ error: null, data: { list: [] } })
    })
  })

  describe('editInfo', () => {
    it('调用 PUT /alarm/config 并发送更新数据', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      const params = { id: 'a1', name: '更新告警', enabled: true }
      const result = await editInfo(params)
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/config', params)
      expect(result).toEqual({ error: null, data: {} })
    })
  })

  describe('editInfoText', () => {
    it('调用 PUT /alarm/config 并发送文本更新数据', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      const params = { id: 'a1', description: '修改描述' }
      const result = await editInfoText(params)
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/config', params)
      expect(result).toEqual({ error: null, data: {} })
    })
  })

  describe('delInfo', () => {
    it('调用 DELETE /alarm/config/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} })
      const result = await delInfo('a1')
      expect(mockDelete).toHaveBeenCalledTimes(1)
      expect(mockDelete).toHaveBeenCalledWith('/alarm/config/a1')
      expect(result).toEqual({ error: null, data: {} })
    })
  })

  describe('infoList', () => {
    it('调用 GET /alarm/info 并携带查询参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { list: [] } })
      const params = { page: 1, page_size: 10 }
      const result = await infoList(params)
      expect(mockGet).toHaveBeenCalledTimes(1)
      expect(mockGet).toHaveBeenCalledWith('/alarm/info', { params })
      expect(result).toEqual({ error: null, data: { list: [] } })
    })
  })

  describe('alarmHistory', () => {
    it('调用 GET /alarm/info/history 并携带查询参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: { list: [] } })
      const params = { page: 1, page_size: 10 }
      const result = await alarmHistory(params)
      expect(mockGet).toHaveBeenCalledTimes(1)
      expect(mockGet).toHaveBeenCalledWith('/alarm/info/history', { params })
      expect(result).toEqual({ error: null, data: { list: [] } })
    })
  })

  describe('alarmHistoryMonthlyTrend', () => {
    it('调用 GET /alarm/info/history/monthly 并携带年份', async () => {
      mockGet.mockResolvedValue({ error: null, data: { year: 2026, months: [] } })
      const result = await alarmHistoryMonthlyTrend(2026, 'Asia/Shanghai')
      expect(mockGet).toHaveBeenCalledTimes(1)
      expect(mockGet).toHaveBeenCalledWith('/alarm/info/history/monthly', {
        params: { year: 2026, timezone: 'Asia/Shanghai' }
      })
      expect(result).toEqual({ error: null, data: { year: 2026, months: [] } })
    })

    it('显式传递 SYS_ADMIN 全租户趋势范围', async () => {
      mockGet.mockResolvedValue({ error: null, data: { year: 2026, months: [] } })

      await alarmHistoryMonthlyTrend(2026, 'UTC', { all_tenants: true })

      expect(mockGet).toHaveBeenCalledWith('/alarm/info/history/monthly', {
        params: { year: 2026, timezone: 'UTC', all_tenants: true }
      })
    })
  })

  describe('processingOperation', () => {
    it('调用 PUT /alarm/info 并发送处理操作数据', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      const params = { id: 'a1', status: 'processed' }
      const result = await processingOperation(params)
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/info', params)
      expect(result).toEqual({ error: null, data: {} })
    })
  })

  describe('batchProcessing', () => {
    it('调用 PUT /alarm/info/batch 并发送批量处理数据', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      const params = { ids: ['a1', 'a2'], status: 'processed' }
      const result = await batchProcessing(params)
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/batch', params)
      expect(result).toEqual({ error: null, data: {} })
    })
  })

  describe('batchActionAlarmHistory', () => {
    it('calls PUT /alarm/info/history/batch-action with ids, action, and note', async () => {
      mockPut.mockResolvedValue({ error: null, data: { success_count: 2, failure_count: 0 } })
      const params = { ids: ['a1', 'a2'], action: 'acknowledge', note: 'checked' }
      const result = await batchActionAlarmHistory(params)

      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/history/batch-action', params)
      expect(result).toEqual({ error: null, data: { success_count: 2, failure_count: 0 } })
    })
  })

  describe('acknowledgeAlarmHistory', () => {
    it('调用 PUT /alarm/info/history/{id}/acknowledge', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      const result = await acknowledgeAlarmHistory('a1')
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/history/a1/acknowledge')
      expect(result).toEqual({ error: null, data: {} })
    })

    it('对 id 中的特殊字符进行 URL 编码', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      await acknowledgeAlarmHistory('alarm/id')
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/history/alarm%2Fid/acknowledge')
    })
  })

  describe('resetAlarmHistory', () => {
    it('调用 PUT /alarm/info/history/{id}/reset', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      const result = await resetAlarmHistory('a1')
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/history/a1/reset')
      expect(result).toEqual({ error: null, data: {} })
    })

    it('对 id 中的特殊字符进行 URL 编码', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} })
      await resetAlarmHistory('alarm/id')
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/history/alarm%2Fid/reset')
    })
  })
})
