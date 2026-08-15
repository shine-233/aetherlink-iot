/**
 * 文件用途: 通知组和通知历史 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证通知组增删改查、用户列表和通知历史查询请求。
 * 关键注意事项: 通知组绑定告警后的真实触达效果不由本测试覆盖。
 * 重构建议: 按通知组、历史记录和用户选择拆分用例，并补空成员边界测试。
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
  deleteNotificationGroup,
  getNotificationGroupDetail,
  getNotificationGroupList,
  getNotificationHistoryList,
  getUserList,
  postNotificationGroup,
  putNotificationGroup
} from '../notification'

describe('notification API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('covers notification group list, detail, create, update, and delete contracts', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'group-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const query = { page: 1, page_size: 10, name: 'ops' } as any
    const createPayload = {
      name: 'ops group',
      notification_type: 'EMAIL',
      notification_config: '{"emails":["ops@example.com"]}',
      description: 'ops alarms',
      status: 'OPEN',
      tenant_id: 'tenant-1'
    } as any
    const updatePayload = {
      description: 'ops alarms updated',
      name: 'ops group',
      notification_config: '{"emails":["ops@example.com","audit@example.com"]}',
      notification_type: 'EMAIL',
      remark: 'keep alerting',
      status: 'OPEN',
      tenant_id: 'tenant-1'
    }

    await getNotificationGroupList(query)
    await getNotificationGroupDetail({ id: 'group-1' })
    await postNotificationGroup(createPayload)
    await putNotificationGroup(updatePayload, 'group-1')
    await deleteNotificationGroup({ id: 'group-1' })

    expect(mockGet).toHaveBeenNthCalledWith(1, '/notification_group/list', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/notification_group/group-1')
    expect(mockPost).toHaveBeenCalledWith('/notification_group', createPayload)
    expect(mockPut).toHaveBeenCalledWith('/notification_group/group-1', updatePayload)
    expect(mockDelete).toHaveBeenCalledWith('/notification_group/group-1')
  })

  it('fetches selectable users and notification history with filters', async () => {
    mockGet.mockResolvedValue({ data: { total: 0, list: [] }, error: null })

    const userQuery = { page: 1, page_size: 20, name: 'operator' }
    const historyQuery = {
      page: 2,
      page_size: 10,
      notification_type: 'EMAIL',
      status: 'FAILURE'
    } as any

    await getUserList(userQuery)
    await getNotificationHistoryList(historyQuery)

    expect(mockGet).toHaveBeenNthCalledWith(1, '/user/selector', { params: userQuery })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/notification_history/list', { params: historyQuery })
  })
})
