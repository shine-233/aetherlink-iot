/**
 * 文件用途：用户组与组权限（TB-46 GPE v1）前端 API 客户端。
 * 核心逻辑：封装 user_groups 的增删改查、组成员全量替换（GET/POST :id/users）、
 *
 *	组权限元素绑定全量替换（GET/POST :id/permissions）九个端点（后端登记见 131.sql）。
 *
 * 关键注意事项：成员与权限均为"全量替换"语义——传空数组即清空；element_code 命名空间
 *
 *	v1 仅支持 board:<id>/asset:<id>，后端对资源存在性与同租户归属 fail-closed 校验。
 *
 * 重构建议：若后续组权限扩展功能权限点（sys_permissions 命名空间），为本模块拆分
 *
 *	资源元素与功能元素两组类型化接口，不要复用同一入参形状。
 */
import { request } from '../request'
import { createResource } from './resource'

/** 用户组主表行（user_groups）。 */
export interface UserGroupItem {
  id: string
  name: string
  tenant_id: string
  description?: string | null
  created_at?: string
  updated_at?: string
}

export interface UserGroupListParams {
  page?: number
  page_size?: number
  name?: string
  /** 仅 SYS_ADMIN 指定租户视图生效 */
  tenant_id?: string
}

export interface UserGroupListResponse {
  list: UserGroupItem[]
  total: number
}

/** 组成员用户摘要（users 表内账号；customer 客户不会出现）。 */
export interface UserGroupMemberSummary {
  id: string
  email: string
  name?: string | null
  authority?: string | null
}

export interface UserGroupMembersResponse {
  group_id: string
  group_name: string
  users: UserGroupMemberSummary[]
}

/** 组权限元素码解析结果（kind + 资源 ID + 可读名称快照）。 */
export interface UserGroupElementInfo {
  code: string
  kind: 'board' | 'asset'
  id: string
  name: string
}

export interface UserGroupPermissionsResponse {
  group_id: string
  group_name: string
  elements: UserGroupElementInfo[]
}

export interface CreateUserGroupParams {
  name: string
  description?: string
  /** 仅 SYS_ADMIN 创建跨租户组时生效 */
  tenant_id?: string
}

export interface UpdateUserGroupParams {
  id: string
  name?: string
  description?: string
}

const userGroups = createResource<
  UserGroupListParams,
  UserGroupListResponse,
  UserGroupItem,
  CreateUserGroupParams,
  UpdateUserGroupParams,
  null
>({
  // 列表端点是复数 /user_groups，单体端点是单数 /user_group —— 后端路由如此，不能统一。
  collection: '/user_group',
  listPath: '/user_groups'
})

/** 分页与条件查询用户组列表（tenant_id 仅 SYS_ADMIN 生效）。 */
export const getUserGroupList = userGroups.list

/** 获取用户组详情 */
export const getUserGroupDetail = userGroups.detail

/** 创建用户组 */
export const createUserGroup = userGroups.create

/** 更新用户组（名称/描述至少一项） */
export const updateUserGroup = userGroups.update

/** 删除用户组（级联清理成员与权限绑定） */
export const deleteUserGroup = userGroups.remove

/** 获取组成员列表 */
export const getUserGroupMembers = async (id: string) => {
  return await request.get<UserGroupMembersResponse>(
    `/user_group/${encodeURIComponent(id)}/users`
  )
}

/** 全量替换组成员（userIds 传空数组=清空成员） */
export const assignUserGroupMembers = async (id: string, userIds: string[]) => {
  return await request.post<{ status: string }>(`/user_group/${encodeURIComponent(id)}/users`, {
    user_ids: userIds
  })
}

/** 获取组权限元素列表 */
export const getUserGroupPermissions = async (id: string) => {
  return await request.get<UserGroupPermissionsResponse>(
    `/user_group/${encodeURIComponent(id)}/permissions`
  )
}

/** 全量替换组权限元素绑定（elementCodes 传空数组=清空绑定） */
export const assignUserGroupPermissions = async (id: string, elementCodes: string[]) => {
  return await request.post<{ status: string }>(
    `/user_group/${encodeURIComponent(id)}/permissions`,
    { element_codes: elementCodes }
  )
}
