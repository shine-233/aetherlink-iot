/**
 * 文件用途: 角色和角色权限管理 API wrapper。
 * 核心逻辑: 封装角色列表、创建、更新、删除，以及权限读取、绑定、修改和清理接口。
 * 关键注意事项: 角色 ID、权限 ID 和租户边界是授权核心合同，前端不能绕过后端权限校验。
 * 重构建议: 明确角色与权限 ID 类型，并补充跨租户、空权限和删除路径测试。
 */
import { request } from '../request'

export const listRoles = async (params: object) => {
  const data = await request.get<Api.UserManagement.Data | null>('/role', {
    params
  })
  return data
}

export const createRole = async (params: Record<string, unknown>) => {
  const data = await request.post<Api.BaseApi.Data>('/role', params)
  return data
}

export const updateRole = async (params: Record<string, unknown>) => {
  const data = await request.put<Api.BaseApi.Data>('/role', params)
  return data
}

export const deleteRole = async (id: string) => {
  const data = await request.delete<Api.BaseApi.Data>(`/role/${id}`)
  return data
}

export const getRolePermissions = async (id: string): Promise<string[]> => {
  const response = await request.get<string[]>(`/casbin/function?role_id=${id}`)
  return response?.data || []
}

export const addRolePermissions = async (id: string, functions_ids: Array<string>) => {
  const data = await request.post<Api.BaseApi.Data>(`/casbin/function`, {
    role_id: id,
    functions_ids
  })
  return data
}

export const modifyRolePermissions = async (id: string, functions_ids: Array<string>) => {
  const data = await request.put<Api.BaseApi.Data>(`/casbin/function`, {
    role_id: id,
    functions_ids
  })
  return data
}

export const deleteRolePermissions = async (id: string) => {
  const data = await request.delete<Api.BaseApi.Data>(`/casbin/function/${id}`)
  return data
}
