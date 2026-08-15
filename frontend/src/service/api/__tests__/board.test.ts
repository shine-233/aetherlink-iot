/**
 * 文件用途: 看板菜单 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证看板菜单配置的获取、保存和删除请求。
 * 关键注意事项: 看板 ID 归属和菜单内容持久化仍需后端或页面集成测试确认。
 * 重构建议: 合并或明确区分 `board` 与 `dashboard-menu` 命名，并补路径参数边界测试。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    put: mockPut,
    delete: mockDelete
  }
}))

import {
  fetchDashboardMenuConfig,
  saveDashboardMenuConfig,
  deleteDashboardMenuConfig
} from '../dashboard-menu'

describe('board (dashboard-menu) API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('fetchDashboardMenuConfig', () => {
    it('calls GET /dashboard-menu/{dashboardId}', async () => {
      mockGet.mockResolvedValue({ data: { dashboard_id: 'dash-1', menu_name: 'Test' }, error: null })
      await fetchDashboardMenuConfig('dash-1')
      expect(mockGet).toHaveBeenCalledTimes(1)
      expect(mockGet).toHaveBeenCalledWith('/dashboard-menu/dash-1')
    })
  })

  describe('saveDashboardMenuConfig', () => {
    it('calls PUT /dashboard-menu/{dashboardId} with payload', async () => {
      mockPut.mockResolvedValue({ data: { dashboard_id: 'dash-1', menu_name: 'Updated' }, error: null })
      const payload = { menu_name: 'Updated Menu', sort: 1, enabled: true }
      await saveDashboardMenuConfig('dash-1', payload)
      expect(mockPut).toHaveBeenCalledTimes(1)
      expect(mockPut).toHaveBeenCalledWith('/dashboard-menu/dash-1', payload)
    })

    it('sends only menu_name when other fields are omitted', async () => {
      mockPut.mockResolvedValue({ data: {}, error: null })
      await saveDashboardMenuConfig('dash-1', { menu_name: 'Test' })
      expect(mockPut).toHaveBeenCalledWith('/dashboard-menu/dash-1', { menu_name: 'Test' })
    })

    it('sends dashboard_name when provided', async () => {
      mockPut.mockResolvedValue({ data: {}, error: null })
      await saveDashboardMenuConfig('dash-1', { menu_name: 'Test', dashboard_name: 'My Dashboard' })
      expect(mockPut).toHaveBeenCalledWith('/dashboard-menu/dash-1', {
        menu_name: 'Test',
        dashboard_name: 'My Dashboard'
      })
    })
  })

  describe('deleteDashboardMenuConfig', () => {
    it('calls DELETE /dashboard-menu/{dashboardId}', async () => {
      mockDelete.mockResolvedValue({ data: null, error: null })
      await deleteDashboardMenuConfig('dash-1')
      expect(mockDelete).toHaveBeenCalledTimes(1)
      expect(mockDelete).toHaveBeenCalledWith('/dashboard-menu/dash-1')
    })
  })
})
