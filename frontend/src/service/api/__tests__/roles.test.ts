/**
 * 文件用途: 角色与权限 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证角色 CRUD 和权限绑定、修改、删除请求。
 * 关键注意事项: 授权安全必须以后端权限测试为准，本测试只锁住前端调用形状。
 * 重构建议: 补充空权限、跨租户权限边界和删除路径的精确断言。
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
  addRolePermissions,
  createRole,
  deleteRole,
  deleteRolePermissions,
  getRolePermissions,
  listRoles,
  modifyRolePermissions,
  updateRole
} from '../roles'

describe('roles API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('covers role list, create, update, and delete contracts', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'role-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const query = { page: 1, page_size: 10, name: 'operator' }
    const createPayload = { name: 'operator', description: 'tenant operator' }
    const updatePayload = { id: 'role-1', name: 'operator-updated' }

    await listRoles(query)
    await createRole(createPayload)
    await updateRole(updatePayload)
    await deleteRole('role-1')

    expect(mockGet).toHaveBeenCalledWith('/role', { params: query })
    expect(mockPost).toHaveBeenCalledWith('/role', createPayload)
    expect(mockPut).toHaveBeenCalledWith('/role', updatePayload)
    expect(mockDelete).toHaveBeenCalledWith('/role/role-1')
  })

  it('reads role permissions and falls back to an empty array when response data is missing', async () => {
    mockGet.mockResolvedValueOnce({ data: ['fn-dashboard', 'fn-device'] })
    expect(await getRolePermissions('role-1')).toEqual(['fn-dashboard', 'fn-device'])
    expect(mockGet).toHaveBeenCalledWith('/casbin/function?role_id=role-1')

    mockGet.mockResolvedValueOnce(null)
    expect(await getRolePermissions('role-2')).toEqual([])
  })

  it('covers role permission create, update, and delete contracts', async () => {
    mockPost.mockResolvedValue({ data: null, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    await addRolePermissions('role-1', ['fn-dashboard'])
    await modifyRolePermissions('role-1', ['fn-dashboard', 'fn-device'])
    await deleteRolePermissions('role-1')

    expect(mockPost).toHaveBeenCalledWith('/casbin/function', {
      role_id: 'role-1',
      functions_ids: ['fn-dashboard']
    })
    expect(mockPut).toHaveBeenCalledWith('/casbin/function', {
      role_id: 'role-1',
      functions_ids: ['fn-dashboard', 'fn-device']
    })
    expect(mockDelete).toHaveBeenCalledWith('/casbin/function/role-1')
  })
})
