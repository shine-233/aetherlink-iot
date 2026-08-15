/**
 * 文件用途: `dashboard-menu.ts` 的请求合同测试。
 * 核心逻辑: mock request 层后断言看板菜单配置读取、保存和删除的 HTTP 调用。
 * 关键注意事项: 测试关注 wrapper 合同，不验证菜单在真实看板中的渲染效果。
 * 重构建议: 与 `board.test.ts` 去重并保留更清晰的领域命名。
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
  deleteDashboardMenuConfig,
  fetchDashboardMenuConfig,
  fetchDashboardMenuConfigs,
  saveDashboardMenuConfig
} from '../dashboard-menu'

describe('dashboard-menu API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches a tenant dashboard menu config by dashboard id', async () => {
    mockGet.mockResolvedValue({ data: { dashboard_id: 'board-1', menu_name: 'Energy' }, error: null })

    await fetchDashboardMenuConfig('board-1')

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/dashboard-menu/board-1')
  })

  it('fetches tenant dashboard menu configs in a batch', async () => {
    mockPost.mockResolvedValue({ data: { 'board-1': { dashboard_id: 'board-1', menu_name: 'Energy' } }, error: null })

    await fetchDashboardMenuConfigs(['board-1'])

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/dashboard-menu/batch', { dashboard_ids: ['board-1'] })
  })

  it('saves dashboard menu display name, dashboard name, sort, and enabled state', async () => {
    mockPut.mockResolvedValue({ data: { dashboard_id: 'board-1' }, error: null })
    const payload = {
      menu_name: 'Energy dashboard',
      dashboard_name: 'Energy',
      sort: 20,
      enabled: true
    }

    await saveDashboardMenuConfig('board-1', payload)

    expect(mockPut).toHaveBeenCalledTimes(1)
    expect(mockPut).toHaveBeenCalledWith('/dashboard-menu/board-1', payload)
  })

  it('deletes a tenant dashboard menu config by dashboard id', async () => {
    mockDelete.mockResolvedValue({ data: null, error: null })

    await deleteDashboardMenuConfig('board-1')

    expect(mockDelete).toHaveBeenCalledTimes(1)
    expect(mockDelete).toHaveBeenCalledWith('/dashboard-menu/board-1')
  })
})
